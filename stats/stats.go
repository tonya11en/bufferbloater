package stats

import (
	"fmt"
	"math"
	"os"
	"sync"
	"time"

	"go.uber.org/zap"
)

type Sample struct {
	timestamp time.Time
	val       float64
}

// NOTE: This whole thing is a hack and isn't exactly efficient, because the
// whole structure gets locked with every call to a function. This shouldn't
// matter, because performance is not a concern at the moment.
type StatsMgr struct {
	statsVals map[string]float64
	mtx       sync.Mutex
	log       *zap.SugaredLogger

	// Every time Sample() is called, we append the snapshot of the current values
	// to this collection.
	sampleCollection map[string][]Sample
}

func NewStatsMgrImpl(logger *zap.SugaredLogger) *StatsMgr {
	ret := &StatsMgr{
		statsVals:        make(map[string]float64),
		sampleCollection: make(map[string][]Sample),
		log:              logger,
	}
	ret.sampleCollection["client1.rq.success_rate"] = []Sample{}
	ret.sampleCollection["client2.rq.success_rate"] = []Sample{}
	return ret
}

func (s *StatsMgr) Set(k string, val float64) {
	s.mtx.Lock()
	s.statsVals[k] = val
	s.mtx.Unlock()
}

func (s *StatsMgr) Incr(k string) {
	s.mtx.Lock()
	s.statsVals[k] += 1.0
	s.mtx.Unlock()
}

// Direct measurements don't need to be sampled. They just go straight to the
// sample collection.
func (s *StatsMgr) DirectMeasurement(k string, t time.Time, val float64) {
	s.mtx.Lock()
	s.sampleCollection[k] =
		append(s.sampleCollection[k], Sample{timestamp: t, val: val})
	s.mtx.Unlock()
}

func (s *StatsMgr) sample() {
	s.mtx.Lock()
	now := time.Now()

	// Derive success rate.
	// TODO: the stats need to be less hacky. rethink all of this.
	// @tallen extra hacky for queues
	sr := s.statsVals["client1.rq.success.count"] / math.Max(s.statsVals["client1.rq.total.count"], 1.0)
	s.statsVals["client1.rq.success_rate"] = sr
	sr = s.statsVals["client2.rq.success.count"] / math.Max(s.statsVals["client2.rq.total.count"], 1.0)
	s.statsVals["client2.rq.success_rate"] = sr

	for statName, val := range s.statsVals {
		s.sampleCollection[statName] =
			append(s.sampleCollection[statName],
				Sample{timestamp: now, val: val})
	}

	s.statsVals = make(map[string]float64)

	s.mtx.Unlock()
}

func (s *StatsMgr) DumpStatsToFolder(folderName string) error {
	s.mtx.Lock()

	os.RemoveAll(folderName)
	os.MkdirAll(folderName, os.ModePerm)

	for statName, sampleSlice := range s.sampleCollection {
		filename := fmt.Sprintf("%s/%s.csv", folderName, statName)
		s.log.Infow("creating stats file", "filename", filename)
		file, err := os.Create(filename)
		if err != nil {
			fmt.Errorf("unable to create file %s", statName)
			return err
		}

		defer file.Close()

		for _, sample := range sampleSlice {
			_, err := file.Write([]byte(fmt.Sprintf("%d,%f\n", sample.timestamp.UnixNano(), sample.val)))
			if err != nil {
				fmt.Errorf("unable to write to file %+v", file)
				return err
			}
		}
		s.log.Infow("finished writing to stats file", "filename", filename)
	}

	s.mtx.Unlock()

	return nil
}

func (s *StatsMgr) PeriodicStatsCollection(period time.Duration, done chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()

	ticker := time.NewTicker(period)
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			s.sample()
		}
	}
}
