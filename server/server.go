package server

import (
	"context"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
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
	config         Config
	log            *zap.SugaredLogger
	activeRequests int32
	srv            *http.Server
	mux            *http.ServeMux
}

func NewServer(config Config, logger *zap.SugaredLogger) *Server {
	s := Server{
		config: config,
		log:    logger,
		mux:    http.NewServeMux(),
	}

	s.srv = &http.Server{
		Addr:    ":" + strconv.Itoa(int(s.config.ListenPort)),
		Handler: s.mux,
	}

	logger.Infow("done creating server",
		"config", s.config,
		"srv", s.srv)

	return &s
}

func (s *Server) DelayedShutdown() {
	delay := time.Second * 0
	for _, segment := range s.config.Profile {
		delay += segment.SegmentDuration
	}

	time.Sleep(delay)

	s.log.Infow("gracefully shutting down server",
		"delay", delay)
	s.srv.Shutdown(context.Background())
}

func (s *Server) Start(wg *sync.WaitGroup) {
	defer wg.Done()
	startTime := time.Now()

	catchAll := func(w http.ResponseWriter, req *http.Request) {
		atomic.AddInt32(&s.activeRequests, int32(1))
		defer atomic.AddInt32(&s.activeRequests, int32(-1))

		// Deduce how much time we need to sleep.
		// TODO: think of a more efficient way to do this.
		t := startTime
		sleepTime := time.Second * 0
		for _, segment := range s.config.Profile {
			sleepTime = segment.RequestLatency
			t = t.Add(segment.SegmentDuration)
			if t.After(time.Now()) {
				time.Sleep(sleepTime)
				return
			}
		}
	}

	s.mux.HandleFunc("/", catchAll)

	// Make sure the server shuts down after the configured amount of time.
	go s.DelayedShutdown()

	if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		s.log.Fatalw("server error",
			"error", err)
	}
}
