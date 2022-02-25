package server

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	pb "allen.gg/bufferbloater/helloworld"
	"allen.gg/bufferbloater/stats"
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
	Profile    []LatencySegment
	ListenPort uint
	Threads    uint
}

type tokenBucket struct {
	tokens        int64
	maxTokens     int64
	refreshAmount int64
	interval      time.Duration
	mtx           sync.Mutex
}

func NewTokenBucket(interval time.Duration, maxTokens int64, refreshAmount int64) *tokenBucket {
	tb := &tokenBucket{
		tokens:        maxTokens,
		maxTokens:     maxTokens,
		refreshAmount: refreshAmount,
		interval:      interval,
	}

	tb.start()

	return tb
}

func (tb *tokenBucket) start() {
	go func() {
		tick := time.NewTicker(tb.interval)
		for <-tick.C; true; <-tick.C {
			tb.mtx.Lock()
			tb.tokens = tb.tokens + tb.refreshAmount
			if tb.tokens > tb.maxTokens {
				tb.tokens = tb.maxTokens
			}
			tb.mtx.Unlock()
		}
	}()
}

func (tb *tokenBucket) Admit() bool {
	tb.mtx.Lock()
	defer tb.mtx.Unlock()

	if tb.tokens > 0 {
		tb.tokens--
		return true
	}

	return false
}

type Server struct {
	pb.UnimplementedGreeterServer

	config   Config
	log      *zap.SugaredLogger
	srv      *grpc.Server
	mux      *http.ServeMux
	statsMgr *stats.StatsMgr

	// Only allow a certain number of requests to be serviced (sleeped) at a time.
	sem       chan struct{}
	queueSize int32

	// This is relevant for determining where we are time-wise in the simulation.
	startTime time.Time

	// Tenant ID.
	tid uint

	lis     net.Listener
	limiter *tokenBucket
}

func NewServer(tenantId uint, config Config, logger *zap.SugaredLogger, sm *stats.StatsMgr) *Server {
	s := Server{
		tid:      tenantId,
		config:   config,
		log:      logger,
		mux:      http.NewServeMux(),
		sem:      make(chan struct{}, config.Threads),
		statsMgr: sm,
		limiter:  NewTokenBucket(100*time.Millisecond, 1100, 100),
	}

	// Load up the semaphore with tickets.
	for i := 0; i < int(config.Threads); i++ {
		s.sem <- struct{}{}
	}

	var err error
	s.lis, err = net.Listen("tcp", fmt.Sprintf("%s:%d", "localhost", config.ListenPort))
	if err != nil {
		logger.Fatalw("error listening", "err", err)
	}

	s.srv = grpc.NewServer()
	pb.RegisterGreeterServer(s.srv, &s)

	logger.Infow("done creating server", "config", s.config)

	return &s
}

func (s *Server) SayHello(ctx context.Context, in *pb.HelloRequest) (*pb.HelloReply, error) {
	s.log.Debugw("handling request", "name", in.GetName())

	s.statsMgr.Set("server.expected_latency", float64(s.currentRequestLatency().Milliseconds())/1000, s.tid)

	var err error

	// Must increment the queue size before "work" begins.
	sz := atomic.AddInt32(&s.queueSize, 1)
	defer atomic.AddInt32(&s.queueSize, -1)

	admit := s.limiter.Admit()

	if admit {
		s.statsMgr.Incr(fmt.Sprintf("server.%s.processed.success", in.GetName()), s.tid)
	} else {
		s.statsMgr.Incr(fmt.Sprintf("server.%s.processed.throttled", in.GetName()), s.tid)
		err = fmt.Errorf("throttled")
	}

	s.statsMgr.Set("server.queue.size", float64(sz), s.tid)
	s.log.Debugw("server received", "name", in.GetName())
	return &pb.HelloReply{Message: "Hello " + in.GetName()}, err
}

func (s *Server) DelayedShutdown(wg *sync.WaitGroup) {
	defer wg.Done()

	delay := time.Second * 0
	for _, segment := range s.config.Profile {
		delay += segment.SegmentDuration
	}

	time.Sleep(delay)

	s.log.Infow("gracefully shutting down", "service_length", delay)
	s.srv.Stop()
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

func (s *Server) Start(wg *sync.WaitGroup) {
	defer wg.Done()

	s.log.Infow("starting server...")

	// Make sure the server shuts down after the configured amount of time.
	var shutdownWg sync.WaitGroup
	shutdownWg.Add(1)
	go s.DelayedShutdown(&shutdownWg)

	// Set the simulation start time.
	s.startTime = time.Now()

	if err := s.srv.Serve(s.lis); err != nil && err != http.ErrServerClosed {
		s.log.Fatalw("server error", "error", err)
	}

	// Wait for shutdown to occur.
	shutdownWg.Wait()
}
