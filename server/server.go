package server

import (
	"context"
	"fmt"
	"sync"
	"time"

	flatbuffers "github.com/google/flatbuffers/go"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"allen.gg/bufferbloater/bouncer_flatbuf"
	"allen.gg/bufferbloater/stats"
)

type Segment struct {
	PopsPerSec      uint
	SegmentDuration time.Duration
}

type Config struct {
	Profile    []Segment
	ListenPort uint
	Threads    uint
}

type Server struct {
	config     Config
	log        *zap.SugaredLogger
	grpcClient bouncer_flatbuf.MetricsBufferClient
	statsMgr   *stats.StatsMgr

	ctx    context.Context
	cancel context.CancelFunc

	// Only allow a certain number of requests to be serviced (sleeped) at a time.
	queueSize int32

	// This is relevant for determining where we are time-wise in the simulation.
	startTime time.Time

	// Tenant ID.
	tid uint
}

func NewServer(ctx context.Context, tenantId uint, config Config, logger *zap.SugaredLogger, sm *stats.StatsMgr) *Server {
	ctx, cancel := context.WithCancel(ctx)
	s := Server{
		ctx:      ctx,
		cancel:   cancel,
		tid:      tenantId,
		config:   config,
		log:      logger,
		statsMgr: sm,
	}

	var err error
	addr := fmt.Sprintf("127.0.0.1:%d", config.ListenPort)
	logger.Infow("dialing", "addr", addr)
	conn, err := grpc.Dial(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithCodec(flatbuffers.FlatbuffersCodec{}))
	if err != nil {
		logger.Fatal(err.Error())
	}
	logger.Infow("finished dialing", "server_tid", tenantId, "err", err)

	s.grpcClient = bouncer_flatbuf.NewMetricsBufferClient(conn)

	logger.Infow("done creating server", "config", s.config)

	return &s
}

func (s *Server) Start(wg *sync.WaitGroup) {
	defer wg.Done()
	s.log.Infow("starting server...", "pops_per_sec", s.config.Profile[0].PopsPerSec)

	// Set the simulation start time.
	s.startTime = time.Now()

	// @tallen hardcoding
	const numWorkers = 32
	intervalMs := 1000.0 / float64(s.config.Profile[0].PopsPerSec) * numWorkers
	s.log.Infow("spawning workers", "request_spacing_ms", intervalMs, "num_workers", numWorkers)
	for i := 0; i < numWorkers; i++ {
		go s.spawnWorker(time.Duration(intervalMs) * time.Millisecond)
	}
}

func (s *Server) spawnWorker(spacing time.Duration) {
	t := time.NewTicker(spacing)
	defer s.log.Infow("worker exited")

	bpool := sync.Pool{
		New: func() interface{} { return flatbuffers.NewBuilder(1024) },
	}

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-t.C:
		}

		b := bpool.Get().(*flatbuffers.Builder)
		b.Reset()

		bouncer_flatbuf.PopRecordRequestStart(b)
		offset := bouncer_flatbuf.PopRecordRequestEnd(b)
		b.Finish(offset)

		select {
		case <-s.ctx.Done():
			return
		default:
		}

		stream, err := s.grpcClient.PopRecord(s.ctx)
		if err != nil && stream != nil && stream.Context() != nil && err != stream.Context().Err() {
			s.log.Desugar().Error("stream error", zap.Error(err))
			return
		}

		select {
		case <-s.ctx.Done():
			return
		default:
		}

		if err := stream.Send(b); err != nil && stream != nil && stream.Context() != nil && err != stream.Context().Err() {
			s.log.Desugar().Error("stream error", zap.Error(err))
			return
		}

		select {
		case <-s.ctx.Done():
			return
		default:
		}

		reply, err := stream.Recv()
		if err != nil && stream != nil && stream.Context() != nil && err != stream.Context().Err() {
			s.log.Desugar().Error("stream error", zap.Error(err))
			return
		}

		var tbl flatbuffers.Table
		record := reply.Record(nil)
		if !record.Payload(&tbl) {
			s.log.Fatal("@tallen hax - failed record thing")
		}
		var payload bouncer_flatbuf.AnyProto
		payload.Init(tbl.Bytes, tbl.Pos)

		var apb anypb.Any

		err = proto.Unmarshal(payload.BufBytes(), &apb)
		if err != nil {
			s.log.Fatal(err.Error())
		}

		b.Reset()
		strOffset := b.CreateByteString(reply.Uuid())
		bouncer_flatbuf.PopRecordRequestStart(b)
		bouncer_flatbuf.PopRecordRequestAddAckUuid(b, strOffset)
		offset = bouncer_flatbuf.PopRecordRequestEnd(b)
		b.Finish(offset)

		select {
		case <-s.ctx.Done():
			return
		default:
		}

		if err := stream.Send(b); err != nil && stream != nil && stream.Context() != nil && err != stream.Context().Err() {
			s.log.Error(err.Error())
			return
		}

		select {
		case <-s.ctx.Done():
			return
		default:
		}

		if err := stream.CloseSend(); err != nil && stream != nil && stream.Context() != nil && err != stream.Context().Err() {
			s.log.Error(err.Error())
			return
		}

		s.statsMgr.Incr("server."+apb.TypeUrl+".processed.success", s.tid)
	}
}
