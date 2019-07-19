package server

import (
	"context"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/segmentio/stats"
	"github.com/segmentio/stats/datadog"
	"go.uber.org/zap"
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

type Server struct {
	config         Config
	log            *zap.SugaredLogger
	activeRequests int32
	srv            *http.Server
	mux            *http.ServeMux
	statsClient    *datadog.Client

	// The work queue simply stores the time at which the request was received and
	// is protected by a mutex.
	queue []time.Time
	qmtx  sync.Mutex

	// Only allow a certain number of requests to be serviced (sleeped) at a time.
	sem chan struct{}

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

	// For now, we'll ignore what's in the queue and just use it to serialize the
	// requests.
	s.qmtx.Lock()
	l := len(s.queue)
	s.log.Debugw("appending to queue", "queue_length", l)
	s.queue = append(s.queue, time.Now())
	s.qmtx.Unlock()
	stats.Set("server.queue.size", float64(l))

	// This is the "servicing" of a request.
	<-s.sem
	crl := s.currentRequestLatency()
	time.Sleep(crl)
	s.sem <- struct{}{}
	stats.Observe("server.service.time", float64(crl.Nanoseconds()/1000))

	// Pop a request off the queue.
	s.qmtx.Lock()
	s.log.Debugw("popping from queue", "queue_length", len(s.queue))
	s.queue = s.queue[1:]
	stats.Set("server.queue.size", float64(len(s.queue)))
	s.qmtx.Unlock()
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
