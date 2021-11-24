package client

import (
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"

	"allen.gg/bufferbloater/stats"
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

	// Tenant ID.
	tid uint
}

func NewClient(tenantId uint, config Config, logger *zap.SugaredLogger, sm *stats.StatsMgr) *Client {
	c := Client{
		tid:      tenantId,
		config:   config,
		log:      logger,
		statsMgr: sm,
		httpClient: &http.Client{
			Timeout:   config.RequestTimeout,
			Transport: &http.Transport{},
		},
	}

	logger.Infow("done creating client",
		"config", c.config)

	return &c
}

func (c *Client) sendWorkloadRequest(numRetries int) {
	if numRetries < 0 {
		return
	}

	defer c.statsMgr.Incr("client.rq.total.count", c.tid)
	targetString := fmt.Sprintf("http://%s:%d", c.config.TargetServer.Address, c.config.TargetServer.Port)

	rqStart := time.Now()
	defer c.statsMgr.DirectMeasurement("client.rq.total_hist", rqStart, 1.0, c.tid)

	req, err := http.NewRequest("GET", targetString, nil)
	if err != nil {
		c.log.Errorw("error creating request", "error", err, "client", c.tid)
		return
	}

	// Tells the server to close the connection when done.
	req.Close = true

	resp, err := c.httpClient.Do(req)
	latency := time.Since(rqStart)

	// Handle timeouts and report error otherwise.
	if err != nil {
		if err, ok := err.(net.Error); ok && err.Timeout() {
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
	resp.Body.Close()

	switch resp.StatusCode {
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
		c.log.Fatalw("wtf is this", "status", resp.StatusCode, "resp", resp, "client", c.tid)
	}

	if numRetries > 0 {
		c.statsMgr.Incr("client.rq.retry.count", c.tid)
		go c.sendWorkloadRequest(numRetries - 1)
	}
}

func (c *Client) processWorkloadStage(ws WorkloadStage) {

	// Divide the requests/sec evenly into the duration of this stage. We can cast
	// an integral type to a time.Duration since time.Duration is an int64 behind
	// the scenes.
	requestSpacing := time.Second / time.Duration(ws.RPS)
	ticker := time.NewTicker(requestSpacing)

	var wg sync.WaitGroup
	wg.Add(1)
	done := make(chan struct{})
	go func(wg *sync.WaitGroup) {
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
	time.Sleep(ws.Duration)
	done <- struct{}{}
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
