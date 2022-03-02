package client

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	flatbuffers "github.com/google/flatbuffers/go"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/anypb"

	"allen.gg/bufferbloater/bouncer_flatbuf"
	"allen.gg/bufferbloater/stats"
)

var (
	letters = "abcdefghijklmnopqrstuvwxyz"
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
	RetryCount     int
	Priority       string
}

type Client struct {
	ctx        context.Context
	cancel     context.CancelFunc
	config     Config
	log        *zap.SugaredLogger
	statsMgr   *stats.StatsMgr
	grpcClient bouncer_flatbuf.MetricsBufferClient

	// Tenant ID.
	tid uint

	activeRq int32
}

func NewClient(ctx context.Context, tenantId uint, config Config, logger *zap.SugaredLogger, sm *stats.StatsMgr) *Client {
	logger.Infow("creating client", "pri", config.Priority)
	serverAddr := fmt.Sprintf("%s:%d", config.TargetServer.Address, config.TargetServer.Port)
	logger.Infow("dialing", "addr", serverAddr, "tid", tenantId)
	conn, err := grpc.Dial(
		serverAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithCodec(flatbuffers.FlatbuffersCodec{}))
	if err != nil {
		logger.Fatal(err.Error())
	}
	logger.Infow("finished dialing", "client_tid", tenantId)

	grpcClient := bouncer_flatbuf.NewMetricsBufferClient(conn)

	c := Client{
		tid:        tenantId,
		config:     config,
		log:        logger,
		statsMgr:   sm,
		grpcClient: grpcClient,
		activeRq:   0,
	}
	c.ctx, c.cancel = context.WithCancel(ctx)

	logger.Infow("done creating client", "config", c.config)

	return &c
}

func RandStringBytes(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func (c *Client) sendWorkloadRequest(b *flatbuffers.Builder, numRetries int) {
	if numRetries < 0 {
		return
	}

	// just shove the priority in typeurl because why not.
	var any anypb.Any
	any.TypeUrl = c.config.Priority

	buf, err := proto.Marshal(&any)
	if err != nil {
		c.log.Fatal(err.Error())
	}

	c.statsMgr.Incr(fmt.Sprintf("client.rq.%s.count", c.config.Priority), 0)

	offset := b.CreateByteVector(buf)
	bouncer_flatbuf.AnyProtoStart(b)
	bouncer_flatbuf.AnyProtoAddBuf(b, offset)
	anyprotoOffset := bouncer_flatbuf.AnyProtoEnd(b)
	bouncer_flatbuf.PushRecordRequestStart(b)
	bouncer_flatbuf.PushRecordRequestAddPayload(b, anyprotoOffset)
	bouncer_flatbuf.PushRecordRequestAddPayloadType(b, bouncer_flatbuf.PayloadAnyProto)
	endOffset := bouncer_flatbuf.PushRecordRequestEnd(b)
	b.Finish(endOffset)

	_, err = c.grpcClient.PushRecord(context.Background(), b)
	if err != nil {
		c.log.Fatal(err.Error())
	}
}

func (c *Client) processWorkloadStage(ws WorkloadStage) {
	numWorkers := 32

	// Divide the requests/sec evenly into the duration of this stage. We can cast
	// an integral type to a time.Duration since time.Duration is an int64 behind
	// the scenes.
	requestSpacing := time.Second / time.Duration(ws.RPS) * time.Duration(numWorkers)
	c.log.Infow("request spacing set", "spacing", requestSpacing.String(), "tid", c.tid)

	var wg sync.WaitGroup
	done := make(chan struct{})

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(wg *sync.WaitGroup) {
			bpool := sync.Pool{
				New: func() interface{} { return flatbuffers.NewBuilder(1024) },
			}

			// We want a jitter.
			//			jitter := int64(rand.NormFloat64() / math.MaxFloat64 * float64(requestSpacing.Microseconds()) / 3.0)
			//			time.Sleep(requestSpacing + time.Duration(jitter))
			ticker := time.NewTicker(requestSpacing)
			defer wg.Done()
			for {
				select {
				case <-c.ctx.Done():
					c.log.Infow("context expired, client exiting")
					return
				case <-done:
					return
				case <-ticker.C:
					c.statsMgr.Set("client.rps", float64(ws.RPS), c.tid)
					go c.sendWorkloadRequest(bpool.Get().(*flatbuffers.Builder), c.config.RetryCount)
				}
			}
		}(&wg)
	}

	select {
	case <-c.ctx.Done():
	default:
	}

	time.Sleep(ws.Duration)
	close(done)
	wg.Wait()
}

func (c *Client) Start(wg *sync.WaitGroup) {
	defer wg.Done()
	defer c.cancel()

	for _, stage := range c.config.Workload {
		select {
		case <-c.ctx.Done():
			c.log.Infow("context expired, client exiting")
			return
		default:
		}

		c.log.Infow("processing new client workload stage", "stage", stage, "client", c.tid)
		c.processWorkloadStage(stage)
	}

	c.log.Infow("client workload finished", "client", c.tid)
}
