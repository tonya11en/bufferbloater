package server

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/tonya11en/bufferbloater/stats"
)

type WeightedLatency struct {
	Weight  uint
	Latency time.Duration
}

type LatencySegment struct {
	LatencyDistribution []WeightedLatency
	WeightSum           uint
	SegmentDuration     time.Duration
}

type Config struct {
	Profile         []LatencySegment
	ListenPort      uint
	Threads         uint
	MaxQueueSize    uint
	QueueTimeout    time.Duration
	EnableIsolation bool
}

type Rq struct {
	// When the request came in.
	rcvTime time.Time

	// How much "service time" it's received.
	progress time.Duration

	// Whether the request is done and a response can be given.
	done chan struct{}

	w   http.ResponseWriter
	req *http.Request
}

type Server struct {
	config         Config
	log            *zap.SugaredLogger
	activeRequests int32
	srv            *http.Server
	ctx            context.Context
	mux            *http.ServeMux
	statsMgr       *stats.StatsMgr
	tq             *TenantQueue

	// Only allow a certain number of requests to be serviced (sleeped) at a time.
	sem      chan struct{}
	activeRq int32

	// This is relevant for determining where we are time-wise in the simulation.
	startTime time.Time
}

func NewServer(config Config, logger *zap.SugaredLogger, sm *stats.StatsMgr) *Server {
	s := Server{
		config:   config,
		log:      logger,
		mux:      http.NewServeMux(),
		sem:      make(chan struct{}, config.Threads),
		statsMgr: sm,
		tq:       NewTenantQueue(config.MaxQueueSize),
	}

	serverLifetime := 0 * time.Second
	for _, segment := range config.Profile {
		serverLifetime += segment.SegmentDuration
	}
	s.ctx, _ = context.WithTimeout(context.Background(), serverLifetime)

	for i := 0; i < int(config.Threads); i++ {
		logger.Infow("spawning worker thread...")
		go s.worker()
	}

	s.srv = &http.Server{
		Addr:        ":" + strconv.Itoa(int(s.config.ListenPort)),
		Handler:     s.mux,
		ReadTimeout: 10 * time.Second,
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
	s.srv.Shutdown(s.ctx)
	s.log.Infow("graceful shutdown complete")
}

func getLatencyFromDistribution(latencyDistribution []WeightedLatency, rand_num int) (time.Duration, error) {
	for _, wl := range latencyDistribution {
		rand_num -= int(wl.Weight)
		if rand_num < 0 {
			return wl.Latency, nil
		}
	}

	return time.Second, fmt.Errorf("invalid rand num provided for latency distribution")
}

func (s *Server) currentRequestLatency() time.Duration {
	t := s.startTime
	var segment LatencySegment
	for _, segment = range s.config.Profile {
		t = t.Add(segment.SegmentDuration)
		if t.After(time.Now()) {
			break
		}
	}

	rn := rand.Intn(int(segment.WeightSum))
	sleepTime, err := getLatencyFromDistribution(segment.LatencyDistribution, rn)
	if err != nil {
		s.log.Fatalf("error calculating latency", "error", err, "rn", rn, "weight_sum", segment.WeightSum)
	}
	return sleepTime
}

func (s *Server) handleOverload(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusServiceUnavailable)
}

func (s *Server) requestHandler(w http.ResponseWriter, req *http.Request) {
	rq := &Rq{
		rcvTime:  time.Now(),
		progress: 0 * time.Nanosecond,
		done:     make(chan struct{}, 1),
		w:        w,
		req:      req,
	}

	tenantId := req.Header.Get("tenant-id")

	// TODO: make this runtime configurable
	if !s.config.EnableIsolation {
		tenantId = "1"
	}

	successful, qsize := s.tq.Push(tenantId, rq)
	if !successful {
		s.handleOverload(w, req)
	}

	<-rq.done

	s.statsMgr.Set(fmt.Sprintf("server.%s.queued_rq", tenantId), float64(qsize))
}

func (s *Server) worker() {
	for {
		rq := s.tq.Pop(s.ctx)
		select {
		case <-s.ctx.Done():
			return
		default:
			s.doWork(rq)
		}
	}
}

func (s *Server) doWork(rq *Rq) {
	defer func() {
		rq.done <- struct{}{}
	}()

	if (s.config.QueueTimeout > 0) && (time.Now().Sub(rq.rcvTime) > s.config.QueueTimeout) {
		// Timed out. Don't service this.
		s.handleOverload(rq.w, rq.req)
		s.statsMgr.Incr("server.queue_timeout")
		return
	}

	sz := atomic.AddInt32(&s.activeRq, 1)
	s.statsMgr.Set("server.active_rq", float64(sz))

	crl := s.currentRequestLatency()
	workDuration := 500 * time.Microsecond
	for rq.progress < crl {
		// Hard-coding the amount of time a rq is "worked" on.
		// TODO: make this configurable if needed, avoiding now because too many
		// knobs.
		time.Sleep(workDuration)
		rq.progress += workDuration
	}

	sz = atomic.AddInt32(&s.activeRq, -1)
	s.statsMgr.Set("server.active_rq", float64(sz))

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
