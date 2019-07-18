package server

import (
	"time"

	"go.uber.org/zap"
)

type LatencySegment struct {
	RequestLatency  time.Duration
	SegmentDuration time.Duration
}

type Config struct {
	Profile    []LatencySegment
	ListenPort uint
}

type Server struct {
	config Config
	log    *zap.SugaredLogger
}

func NewServer(config Config, logger *zap.SugaredLogger) *Server {
	c := Server{
		config: config,
		log:    logger,
	}

	logger.Infow("done creating server",
		"config", c.config)

	return &c
}
