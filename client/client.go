package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/m3db/prometheus_remote_client_golang/promremote"
	"go.uber.org/zap"

	"allen.gg/bufferbloater/stats"
)

var (
	letters = "abcdefghijklmnopqrstuvwxyz"
)

type WorkloadStage struct {
	RPS      uint
	Duration time.Duration
}

type Target struct {
	Address string
	Port    uint
}

type Config struct {
	Workload       []WorkloadStage
	RequestTimeout time.Duration
	TargetServer   Target
	RetryCount     int
}

type Client struct {
	config     Config
	log        *zap.SugaredLogger
	statsMgr   *stats.StatsMgr
	httpClient *http.Client
	pClient    promremote.Client

	// Tenant ID.
	tid uint

	activeRq int32
}

func NewClient(tenantId uint, config Config, logger *zap.SugaredLogger, sm *stats.StatsMgr) *Client {
	pclientCfg := promremote.NewConfig(
		//promremote.WriteURLOption(targetString),
		promremote.HTTPClientTimeoutOption(60 * time.Second),
	)

	pClient, err := promremote.NewClient(pclientCfg)
	if err != nil {
		logger.Fatalw("failed to create prom client", "err", err)
	}

	c := Client{
		tid:      tenantId,
		config:   config,
		log:      logger,
		statsMgr: sm,
		httpClient: &http.Client{
			Timeout: config.RequestTimeout,
			Transport: &http.Transport{
				MaxIdleConnsPerHost: 4096,
			},
		},
		pClient:  pClient,
		activeRq: 0,
	}

	// TODO: terminate this goroutine gracefully. Ignoring for now, since doing hax.
	go func() {
		t := time.NewTicker(time.Millisecond * 250)
		for {
			select {
			case <-t.C:
				c.statsMgr.Set("client.active_rq", float64(atomic.LoadInt32(&c.activeRq)), c.tid)
			}
		}
	}()

	logger.Infow("done creating client",
		"config", c.config)

	return &c
}

type tagsT struct {
	Name string `json:"__name__"`
	City string `json:"city"`
}

type rqBody struct {
	Tags      tagsT   `json:"tags"`
	Timestamp string  `json:"timestamp"`
	Value     float32 `json:"value"`
}

func RandStringBytes(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func (c *Client) makeHTTPRequest() (*http.Response, error) {
	atomic.AddInt32(&c.activeRq, 1)
	defer atomic.AddInt32(&c.activeRq, -1)

	targetString := fmt.Sprintf("http://%s:%d/api/v1/json/write", c.config.TargetServer.Address, c.config.TargetServer.Port)

	requestBody := rqBody{
		Tags: tagsT{
			Name: RandStringBytes(3),
			City: "caketown",
		},
		Timestamp: strconv.Itoa(int(time.Now().Unix())),
		Value:     rand.Float32(),
	}

	rqJSON, err := json.Marshal(requestBody)
	if err != nil {
		c.log.Fatalw("failed to marshal json", rqJSON)
	}

	resp, err := http.Post(targetString, "application/json",
		bytes.NewBuffer(rqJSON))
	return resp, err
}

func (c *Client) makeRequest() (promremote.WriteResult, error) {

	atomic.AddInt32(&c.activeRq, 1)
	defer atomic.AddInt32(&c.activeRq, -1)

	ts := []promremote.TimeSeries{
		promremote.TimeSeries{
			Labels: []promremote.Label{
				{
					Name:  "__name__",
					Value: "foofoo", //RandStringBytes(3),
				},
				{
					Name:  "random_label",
					Value: RandStringBytes(3),
				},
				{
					Name:  "foo",
					Value: "bar",
				},
			},
			Datapoint: promremote.Datapoint{
				//Timestamp: time.Now(),
				Value: rand.Float64(),
			},
		},
	}

	opts := promremote.WriteOptions{}
	return c.pClient.WriteTimeSeries(context.Background(), ts, opts)
}

func (c *Client) sendWorkloadRequest(numRetries int) {
	if numRetries < 0 {
		return
	}

	defer c.statsMgr.Incr("client.rq.total.count", c.tid)

	rqStart := time.Now()
	defer c.statsMgr.DirectMeasurement("client.rq.total_hist", rqStart, 1.0, c.tid)

	res, err := c.makeHTTPRequest()
	latency := time.Since(rqStart)

	// Handle timeouts and report error otherwise.
	if err != nil {
		if terr, ok := err.(net.Error); ok && terr.Timeout() {
			c.log.Warnw("request timed out", "client", c.tid)

			// Directly measuring timeouts because we only care about the point-in-time
			// the request that timed out was sent.
			c.statsMgr.DirectMeasurement("client.rq.timeout", rqStart, 1.0, c.tid)
		} else {
			c.log.Errorw("request error", "error", err, "client", c.tid)
		}
		c.statsMgr.Incr("client.rq.failure.count", c.tid)
		return
	}

	_, err = io.Copy(io.Discard, res.Body)
	if err != nil {
		c.log.Fatalw("failed noop IO copy", "err", err.Error())
	}
	res.Body.Close()

	switch res.StatusCode {
	case http.StatusOK:
		c.statsMgr.DirectMeasurement("client.rq.latency", rqStart, float64(latency.Seconds()), c.tid)
		c.statsMgr.DirectMeasurement("client.rq.success_hist", rqStart, 1.0, c.tid)
		c.statsMgr.Incr("client.rq.success.count", c.tid)
		return
	case http.StatusServiceUnavailable, http.StatusTooManyRequests:
		c.statsMgr.DirectMeasurement("client.rq.503", rqStart, 1.0, c.tid)
		c.statsMgr.Incr("client.rq.failure.count", c.tid)
	case http.StatusRequestTimeout, http.StatusGatewayTimeout:
		c.statsMgr.DirectMeasurement("client.rq.timeout", rqStart, 1.0, c.tid)
	default:
		c.log.Fatalw("wtf is this", "status", res.StatusCode, "resp", res, "client", c.tid)
	}
}

func (c *Client) processWorkloadStage(ws WorkloadStage) {
	numWorkers := 128

	// Divide the requests/sec evenly into the duration of this stage. We can cast
	// an integral type to a time.Duration since time.Duration is an int64 behind
	// the scenes.
	requestSpacing := time.Second / time.Duration(ws.RPS) * time.Duration(numWorkers)
	c.log.Infow("request spacing set", "spacing", requestSpacing.String())

	var wg sync.WaitGroup
	done := make(chan struct{})
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(wg *sync.WaitGroup) {
			// We want a jitter here.
			jitterUsec := math.Abs(rand.NormFloat64()) / math.MaxFloat64 * float64(requestSpacing.Microseconds())
			time.Sleep(time.Duration(jitterUsec) * time.Microsecond)

			ticker := time.NewTicker(requestSpacing)
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				case <-ticker.C:
					c.statsMgr.Set("client.rps", float64(ws.RPS), c.tid)
					go c.sendWorkloadRequest(c.config.RetryCount)
				}
			}
		}(&wg)
	}
	time.Sleep(ws.Duration)
	close(done)
	wg.Wait()
}

func (c *Client) Start(wg *sync.WaitGroup) {
	defer wg.Done()

	for _, stage := range c.config.Workload {
		c.log.Infow("processing new client workload stage", "stage", stage, "client", c.tid)
		c.processWorkloadStage(stage)
	}

	c.log.Infow("client workload finished", "client", c.tid)
}
