package stats

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type StatsClient interface {
	Set(string, float64)
	Incr(string)
}

type Sample struct {
	timestamp time.Time
	val       float64
}

// NOTE: This whole thing is a hack and isn't exactly efficient, because the
// whole structure gets locked with every call to a function. This shouldn't
// matter, because performance is not a concern at the moment.
type StatsClientImpl struct {
	statsVals map[string]float64
	mtx       sync.Mutex

	// Every time Sample() is called, we append the snapshot of the current values
	// to this collection.
	sampleCollection map[string][]Sample
}

func NewStatsClientImpl() *StatsClientImpl {
	return &StatsClientImpl{
		statsVals:        make(map[string]float64),
		sampleCollection: make(map[string][]Sample),
	}
}

func (s *StatsClientImpl) Set(s string, val float64) {
	s.mtx.Lock()
	statsVals[s] = val
	s.mtx.Unlock()
}

func (s *StatsClientImpl) Incr(s string) {
	s.mtx.Lock()
	statsVals[s] += 1.0
	s.mtx.Unlock()
}

func (s *StatsClientImpl) Sample() {
	s.mtx.Lock()
	for statName, val := range statsVals {
		sampleCollection[statName] = append(sampleCollection[statName], Sample{timestamp: time.Now(), val})
	}
	s.mtx.Unlock()
}

func (s *StatsClientImpl) DumpStatsToFolder(folderName string) error {
	s.mtx.Lock()

	for statName, sampleSlice := range sampleCollection {
		file, err := os.Create(fmt.Sprintf("%s.csv"), statName)
		if err != nil {
			panic(err)
		}

		defer file.Close()

		for _, value := range data {
			err := file.Write(fmt.Sprintf("%d,%f\n"))
			if err != nil {
				panic(err)
			}
		}
	}

	s.mtx.Unlock()
}
