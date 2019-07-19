package server

import (
	"context"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/tonya11en/bufferbloater/stats"
)

type LatencySegment struct {
	RequestLatency  time.Duration
	SegmentDuration time.Duration
}

type Config struct {
	Profile    []LatencySegment
	ListenPort uint
	Threads    uint
}

type request struct {
	// When the request came in.
	rcvTime time.Time

	progress time.Duration
}

type Server struct {
	config         Config
	log            *zap.SugaredLogger
	activeRequests int32
	srv            *http.Server
	mux            *http.ServeMux
	statsClient    *datadog.Client

	// Only allow a certain number of requests to be serviced (sleeped) at a time.
	sem       chan struct{}
	queueSize int32

	// This is relevant for determining where we are time-wise in the simulation.
	startTime time.Time
}

func NewServer(config Config, logger *zap.SugaredLogger) *Server {
	s := Server{
		config: config,
		log:    logger,
		mux:    http.NewServeMux(),
		sem:    make(chan struct{}, config.Threads),
	}

	// Load up the semaphore with tickets.
	for i := 0; i < int(config.Threads); i++ {
		s.sem <- struct{}{}
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

func (s *Server) DelayedShutdown(wg *sync.WaitGroup) {
	defer wg.Done()

	delay := time.Second * 0
	for _, segment := range s.config.Profile {
		delay += segment.SegmentDuration
	}

	time.Sleep(delay)

	s.log.Infow("gracefully shutting down",
		"service_length", delay)
	s.srv.Shutdown(context.Background())
	s.log.Infow("graceful shutdown complete")
}

func (s *Server) currentRequestLatency() time.Duration {
	sleepTime := time.Second * 0

	t := s.startTime
	for _, segment := range s.config.Profile {
		sleepTime = segment.RequestLatency
		t = t.Add(segment.SegmentDuration)
		if t.After(time.Now()) {
			break
		}
	}

	s.log.Debugw("calculated server request sleep time", "t", sleepTime)
	return sleepTime
}

func (s *Server) requestHandler(w http.ResponseWriter, req *http.Request) {
	stats.Incr("server.rq.count")

	rq := request{
		rcvTime:  time.Now(),
		progress: 0 * time.Nanosecond,
	}

	sz := atomic.AddInt32(&s.queueSize, 1)
	s.log.Debugw("increased queue size", "queue_length", sz)

	// This is the "servicing" of a request. The semaphore asserts the
	// concurrency.
	crl := s.currentRequestLatency()
	workDuration := 500 * time.Microsecond
	for rq.progress < crl {
		<-s.sem
		// Hard-coding the amount of time a rq is "worked" on.
		// TODO: make this configurable if needed, avoiding now because too many
		// knobs.
		time.Sleep(workDuration)
		s.sem <- struct{}{}
		rq.progress += workDuration
	}

	sz = atomic.AddInt32(&s.queueSize, -1)
	stats.Set("server.queue.size", float64(sz))
	s.log.Debugw("decreased queue size", "queue_length", sz)

	return
}

func (s *Server) Start(wg *sync.WaitGroup) {
	defer wg.Done()

	s.mux.HandleFunc("/", s.requestHandler)

	// Make sure the server shuts down after the configured amount of time.
	var shutdownWg sync.WaitGroup
	shutdownWg.Add(1)
	go s.DelayedShutdown(&shutdownWg)

	// Set the simulation start time.
	s.startTime = time.Now()

	if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		s.log.Fatalw("server error",
			"error", err)
	}

	// Wait for shutdown to occur.
	shutdownWg.Wait()
}
