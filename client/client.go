package client

import (
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/segmentio/stats"
	"github.com/segmentio/stats/datadog"
	"github.com/segmentio/stats/httpstats"
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
	config      Config
	log         *zap.SugaredLogger
	httpClient  *http.Client
	statsClient *datadog.Client
}

func NewClient(config Config, logger *zap.SugaredLogger) *Client {
	c := Client{
		config: config,
		log:    logger,
	}

	c.httpClient = &http.Client{
		Timeout: c.config.RequestTimeout,
		Transport: httpstats.NewTransport(
			&http.Transport{},
		),
	}

	logger.Infow("done creating client",
		"config", c.config)

	return &c
}

func (c *Client) sendWorkloadRequest() {
	targetString := fmt.Sprintf("http://%s:%d", c.config.TargetServer.Address, c.config.TargetServer.Port)
	c.log.Debugw("sending request")

	_, err := c.httpClient.Get(targetString)

	// Handle timeouts and report error otherwise.
	if err, ok := err.(net.Error); ok && err.Timeout() {
		c.log.Warnw("request timed out")
		stats.Incr("client.rq.timeout")
	} else if err != nil {
		c.log.Errorw("request error", "error", err)
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
	done := make(chan bool)
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
	done <- true
	wg.Wait()
}

func (c *Client) Start(wg *sync.WaitGroup) {
	defer wg.Done()

	for _, stage := range c.config.Workload {
		c.processWorkloadStage(stage)
	}

	c.log.Infow("client workload finished")
}
