package client

import (
	"sync"
	"time"

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
	config Config
	log    *zap.SugaredLogger
}

func NewClient(config Config, logger *zap.SugaredLogger) *Client {
	c := Client{
		config: config,
		log:    logger,
	}

	logger.Infow("done creating client",
		"config", c.config)

	return &c
}

func (c *Client) Start(wg *sync.WaitGroup) {
	defer wg.Done()
	// noop
}
