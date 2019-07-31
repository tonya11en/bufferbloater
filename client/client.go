package client

import (
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/tonya11en/bufferbloater/stats"
	"go.uber.org/zap"
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
}

type Client struct {
	config   Config
	log      *zap.SugaredLogger
	statsMgr *stats.StatsMgr
}

func NewClient(config Config, logger *zap.SugaredLogger, sm *stats.StatsMgr) *Client {
	c := Client{
		config:   config,
		log:      logger,
		statsMgr: sm,
	}

	logger.Infow("done creating client",
		"config", c.config)

	return &c
}

func (c *Client) sendWorkloadRequest() {
	defer c.statsMgr.Incr("client.rq.total.count")
	targetString := fmt.Sprintf("http://%s:%d", c.config.TargetServer.Address, c.config.TargetServer.Port)

	rqStart := time.Now()
	httpClient := &http.Client{
		Timeout: c.config.RequestTimeout,
	}

	req, err := http.NewRequest("GET", targetString, nil)
	if err != nil {
		c.log.Errorw("error creating request", "error", err)
		return
	}

	// Tells the server to close the connection when done.
	req.Close = true

	resp, err := httpClient.Do(req)
	latency := time.Since(rqStart)

	// Handle timeouts and report error otherwise.
	if err != nil {
		if err, ok := err.(net.Error); ok && err.Timeout() {
			c.log.Warnw("request timed out")

			// Directly measuring timeouts because we only care about the point-in-time
			// the request that timed out was sent.
			c.statsMgr.DirectMeasurement("client.rq.timeout", rqStart, 1.0)
		} else {
			c.log.Errorw("request error", "error", err)
		}
		c.statsMgr.Incr("client.rq.failure.count")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		c.statsMgr.DirectMeasurement("client.rq.latency", rqStart, float64(latency.Seconds()))
		c.statsMgr.Incr("client.rq.success.count")
	} else if resp.StatusCode == http.StatusServiceUnavailable {
		c.statsMgr.DirectMeasurement("client.rq.503", rqStart, 1.0)
		c.statsMgr.Incr("client.rq.failure.count")
	}
}

func (c *Client) processWorkloadStage(ws WorkloadStage) {
	c.statsMgr.Set("client.rps", float64(ws.RPS))

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
				go c.sendWorkloadRequest()
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
		c.log.Infow("processing new client workload stage", "stage", stage)
		c.processWorkloadStage(stage)
	}

	c.log.Infow("client workload finished")
}
