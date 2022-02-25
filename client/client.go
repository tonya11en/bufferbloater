package client

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	pb "allen.gg/bufferbloater/helloworld"
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
}

type Client struct {
	config     Config
	log        *zap.SugaredLogger
	statsMgr   *stats.StatsMgr
	grpcClient pb.GreeterClient

	// Tenant ID.
	tid uint

	activeRq int32
}

func NewClient(tenantId uint, config Config, logger *zap.SugaredLogger, sm *stats.StatsMgr) *Client {
	serverAddr := fmt.Sprintf("%s:%d", config.TargetServer.Address, config.TargetServer.Port)
	conn, err := grpc.Dial(serverAddr, grpc.WithInsecure())
	if err != nil {
		panic(err.Error())
	}

	grpcClient := pb.NewGreeterClient(conn)

	c := Client{
		tid:        tenantId,
		config:     config,
		log:        logger,
		statsMgr:   sm,
		grpcClient: grpcClient,
		activeRq:   0,
	}

	// TODO: terminate this goroutine gracefully. Ignoring for now, since doing hax.
	go func() {
		t := time.NewTicker(time.Millisecond * 250)
		for {
			select {
			case <-t.C:
				c.statsMgr.Set("client.active_rq", float64(atomic.LoadInt32(&c.activeRq)), c.tid)
			}
		}
	}()

	logger.Infow("done creating client",
		"config", c.config)

	return &c
}

func RandStringBytes(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func randomPriority() string {
	//weights := []int{1, 1, 1}
	priorities := []string{"high", "default", "low"}

	return priorities[rand.Intn(3)]
}

func (c *Client) sendWorkloadRequest(numRetries int) {
	if numRetries < 0 {
		return
	}

	defer c.statsMgr.Incr("client.rq.total.count", c.tid)

	rqStart := time.Now()
	defer c.statsMgr.DirectMeasurement("client.rq.total_hist", rqStart, 1.0, c.tid)

	pri := randomPriority()
	c.statsMgr.Incr(fmt.Sprintf("client.rq.%s.count", pri), c.tid)
	_, err := c.grpcClient.SayHello(context.Background(), &pb.HelloRequest{Name: pri})
	if err != nil {
		c.log.Errorw("error sending request", "err", err.Error())
		return
	}
	latency := time.Since(rqStart)

	c.statsMgr.DirectMeasurement("client.rq.latency", rqStart, float64(latency.Seconds()), c.tid)
	c.statsMgr.DirectMeasurement("client.rq.success_hist", rqStart, 1.0, c.tid)
	c.statsMgr.Incr("client.rq.success.count", c.tid)

	/*
		case http.StatusServiceUnavailable, http.StatusTooManyRequests:
			c.statsMgr.DirectMeasurement("client.rq.503", rqStart, 1.0, c.tid)
			c.statsMgr.Incr("client.rq.failure.count", c.tid)
		case http.StatusRequestTimeout, http.StatusGatewayTimeout:
			c.statsMgr.DirectMeasurement("client.rq.timeout", rqStart, 1.0, c.tid)
		default:
			c.log.Fatalw("wtf is this", "status", res.StatusCode, "resp", res, "client", c.tid)
		}
	*/
}

func (c *Client) processWorkloadStage(ws WorkloadStage) {
	numWorkers := 128

	// Divide the requests/sec evenly into the duration of this stage. We can cast
	// an integral type to a time.Duration since time.Duration is an int64 behind
	// the scenes.
	requestSpacing := time.Second / time.Duration(ws.RPS) * time.Duration(numWorkers)
	c.log.Infow("request spacing set", "spacing", requestSpacing.String())

	var wg sync.WaitGroup
	done := make(chan struct{})
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(wg *sync.WaitGroup) {
			// We want a jitter here.
			jitterUsec := math.Abs(rand.NormFloat64()) / math.MaxFloat64 * float64(requestSpacing.Microseconds())
			time.Sleep(time.Duration(jitterUsec) * time.Microsecond)

			ticker := time.NewTicker(requestSpacing)
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				case <-ticker.C:
					c.statsMgr.Set("client.rps", float64(ws.RPS), c.tid)
					go c.sendWorkloadRequest(c.config.RetryCount)
				}
			}
		}(&wg)
	}
	time.Sleep(ws.Duration)
	close(done)
	wg.Wait()
}

func (c *Client) Start(wg *sync.WaitGroup) {
	defer wg.Done()

	for _, stage := range c.config.Workload {
		c.log.Infow("processing new client workload stage", "stage", stage, "client", c.tid)
		c.processWorkloadStage(stage)
	}

	c.log.Infow("client workload finished", "client", c.tid)
}
