package stats

import (
	"fmt"
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
	return ret
}

func (s *StatsMgr) Set(k string, val float64, tid uint) {
	s.mtx.Lock()
	s.statsVals[keyWithTid(k, tid)] = val
	s.mtx.Unlock()
}

func (s *StatsMgr) Incr(k string, tid uint) {
	s.mtx.Lock()
	s.statsVals[keyWithTid(k, tid)] += 1.0
	s.mtx.Unlock()
}

// Direct measurements don't need to be sampled. They just go straight to the
// sample collection.
func (s *StatsMgr) DirectMeasurement(k string, t time.Time, val float64, tid uint) {
	s.mtx.Lock()
	s.sampleCollection[keyWithTid(k, tid)] =
		append(s.sampleCollection[keyWithTid(k, tid)], Sample{timestamp: t, val: val})
	s.mtx.Unlock()
}

func (s *StatsMgr) sample() {
	now := time.Now()

	for statName, val := range s.statsVals {
		s.sampleCollection[statName] =
			append(s.sampleCollection[statName],
				Sample{timestamp: now, val: val})
		s.statsVals[statName] = 0.0
	}
}

func (s *StatsMgr) DumpStatsToFolder(folderName string) error {
	s.mtx.Lock()

	s.log.Infow("removing old stats data", "dir", folderName)
	os.RemoveAll(folderName)
	os.MkdirAll(folderName, os.ModePerm)

	for statName, sampleSlice := range s.sampleCollection {
		filename := fmt.Sprintf("%s/%s.csv", folderName, statName)
		s.log.Infow("creating stats file", "filename", filename)
		file, err := os.Create(filename)
		if err != nil {
			return fmt.Errorf("unable to create file %s: %s", statName, err.Error())
		}

		defer file.Close()

		for _, sample := range sampleSlice {
			_, err := file.Write([]byte(fmt.Sprintf("%d,%f\n", sample.timestamp.UnixNano(), sample.val)))
			if err != nil {
				return fmt.Errorf("unable to write to file %+v: %s", file, err.Error())
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
			s.log.Infow("stats collection completed")
			return
		case <-ticker.C:
			// Gross...
			{
				s.mtx.Lock()
				s.sample()
				s.mtx.Unlock()
			}
		}
	}
}

func keyWithTid(k string, tid uint) string {
	return fmt.Sprintf(k+".%d", tid)
}
