package server

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTenantQueuePops(t *testing.T) {
	queue := NewTenantQueue(1)

	rq1 := Rq{progress: time.Second * 1}
	rq2 := Rq{progress: time.Second * 2}
	rq3 := Rq{progress: time.Second * 3}
	rq4 := Rq{progress: time.Second * 4}

	success, size := queue.Push("t1", &rq1)
	assert.True(t, success)
	// Should be empty since the queue was loaded.
	assert.Equal(t, 0, size)

	success, size = queue.Push("t1", &rq2)
	assert.True(t, success)
	assert.Equal(t, 1, size)

	// Should fail.
	success, size = queue.Push("t1", &rq3)
	assert.False(t, success)
	assert.Equal(t, 1, size)

	assert.Equal(t, &rq1, queue.Pop(context.Background()))

	success, size = queue.Push("t1", &rq4)
	assert.True(t, success)
	assert.Equal(t, 1, size)

	assert.Equal(t, &rq2, queue.Pop(context.Background()))
	assert.Equal(t, &rq4, queue.Pop(context.Background()))
}

func TestTenantQueueBasic(t *testing.T) {
	queue := NewTenantQueue(1000)

	rq1 := Rq{progress: time.Second * 1}
	rq2 := Rq{progress: time.Second * 2}

	// A lot of t1 tenants.
	for ii := 0; ii < 1000; ii++ {
		success, _ := queue.Push("t1", &rq1)
		assert.True(t, success)
	}

	// Not as many t2 tenants.
	for ii := 0; ii < 100; ii++ {
		success, _ := queue.Push("t2", &rq2)
		assert.True(t, success)
	}

	// See who is popped off.
	count_rq1 := 0
	count_rq2 := 0
	for ii := 0; ii < 200; ii++ {
		val := queue.Pop(context.Background())
		if val == &rq1 {
			count_rq1++
		} else if val == &rq2 {
			count_rq2++
		} else {
			// This should never happen.
			assert.True(t, false)
		}
	}

	diff := math.Abs(float64(count_rq1) - float64(count_rq2))
	assert.Less(t, diff, 10.0)
}
