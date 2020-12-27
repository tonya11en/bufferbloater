package server

import (
	"context"
	"sync"
)

// The actual queue for each tenant.
type innerQueue struct {
	Fifo chan *Rq

	// Indicates whether we need to push the next request into the main queue.
	Loaded bool
}

func newInnerQueue(maxQueueSize uint) *innerQueue {
	return &innerQueue{
		Fifo:   make(chan *Rq, maxQueueSize),
		Loaded: true,
	}
}

type rqMeta struct {
	Rq       *Rq
	TenantId string
}

type TenantQueue struct {
	tenantQueueMap map[string](*innerQueue)

	// Holds 1 of each tenant's requests.
	mainQueue chan *rqMeta

	// All of this locking is poorly optimized and there's probably a better way. It doesn't really
	// matter for the extremely small RPS that BB is used for.
	mtx sync.Mutex

	// Index for the basic RR scheduling.
	tenantIdx uint

	maxQueueSize uint
}

func NewTenantQueue(maxQueueSize uint) *TenantQueue {
	return &TenantQueue{
		tenantQueueMap: make(map[string](*innerQueue)),
		tenantIdx:      0,
		maxQueueSize:   maxQueueSize,

		mainQueue: make(chan *rqMeta, 1024),
	}
}

func (t *TenantQueue) getInnerQueue(tenant string) *innerQueue {
	t.mtx.Lock()
	defer t.mtx.Unlock()

	// perf: If it ends up mattering, don't look up in the map 3 times.
	if _, ok := t.tenantQueueMap[tenant]; !ok {
		t.tenantQueueMap[tenant] = newInnerQueue(t.maxQueueSize)
	}
	return t.tenantQueueMap[tenant]
}

func (t *TenantQueue) loadMainQueue(iq *innerQueue, tenant string) {
	// Loaded implies there's no active rq in the main queue.
	t.mainQueue <- &rqMeta{
		Rq:       <-iq.Fifo,
		TenantId: tenant,
	}
	iq.Loaded = false
}

// This can return a bogus request value if the server is shutdown.. a bit ugly, but works for now.
func (t *TenantQueue) Pop(ctx context.Context) *Rq {
	for {
		select {
		case meta := <-t.mainQueue:
			iq := t.getInnerQueue(meta.TenantId)

			t.mtx.Lock()
			defer t.mtx.Unlock()

			if len(iq.Fifo) == 0 {
				iq.Loaded = true
			} else {
				t.loadMainQueue(iq, meta.TenantId)
			}
			return meta.Rq
		case <-ctx.Done():
			return nil
		}
	}
}

// Returns true if successful. If not successful, it implies the queue is full/overloaded.
// and the size of the queue.
func (t *TenantQueue) Push(tenant string, rq *Rq) (bool, int) {
	iq := t.getInnerQueue(tenant)

	t.mtx.Lock()
	defer t.mtx.Unlock()

	select {
	case iq.Fifo <- rq:
		if iq.Loaded {
			t.loadMainQueue(iq, tenant)
		}
		return true, int(len(iq.Fifo))

	default:
		return false, int(len(iq.Fifo))
	}
}
