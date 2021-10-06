package main

import (
	"flag"
	"io/ioutil"
	"sync"
	"time"

	"go.uber.org/zap"
	"gopkg.in/yaml.v2"

	"allen.gg/bufferbloater/client"
	"allen.gg/bufferbloater/server"
	"allen.gg/bufferbloater/stats"
)

var statsDataDir = flag.String("data_dir", "bufferbloater_data", "Specifies the directory to drop the CSV data")

type Bufferbloater struct {
	log      *zap.SugaredLogger
	c        []*client.Client
	s        []*server.Server
	statsMgr *stats.StatsMgr
}

type clientConfig struct {
	Workload []struct {
		Rps      uint
		Duration string
	} `yaml:"workload"`
	RqTimeout    string `yaml:"rq_timeout"`
	TargetServer struct {
		Address string
		Port    uint
	} `yaml:"target_server"`
}

type serverConfig struct {
	Profile []struct {
		LatencyDistribution []struct {
			Weight  uint   `yaml:"weight"`
			Latency string `yaml:"latency"`
		} `yaml:"latency_distribution"`
		Duration string
	} `yaml:"profile"`
	ListenPort uint `yaml:"listen_port"`
	Threads    uint `yaml:"threads"`
}

// Basic representation of the parsed yaml file before the durations are parsed.
type parsedYamlConfig struct {
	Clients []clientConfig `yaml:"clients"`
	Servers []serverConfig `yaml:"servers"`
}

// Creates a properly typed client config.
func clientConfigParse(cc clientConfig) (client.Config, error) {
	// TODO: validate config

	conf := client.Config{
		TargetServer: client.Target{
			Address: cc.TargetServer.Address,
			Port:    cc.TargetServer.Port,
		},
	}

	d, err := time.ParseDuration(cc.RqTimeout)
	if err != nil {
		return client.Config{}, err
	}
	conf.RequestTimeout = d

	for _, stage := range cc.Workload {
		d, err := time.ParseDuration(stage.Duration)
		if err != nil {
			return client.Config{}, err
		}

		workloadStage := client.WorkloadStage{
			RPS:      stage.Rps,
			Duration: d,
		}
		conf.Workload = append(conf.Workload, workloadStage)
	}

	return conf, nil
}

func serverConfigParse(sc serverConfig) (server.Config, error) {
	// TODO: validate config

	serverConfig := server.Config{
		ListenPort: sc.ListenPort,
		Threads:    sc.Threads,
	}

	for _, segment := range sc.Profile {
		s := server.LatencySegment{}

		// Calculate the latency distribution.
		s.WeightSum = 0
		s.LatencyDistribution = []server.WeightedLatency{}
		for _, wl := range segment.LatencyDistribution {
			l, err := time.ParseDuration(wl.Latency)
			if err != nil {
				return server.Config{}, err
			}

			weightedLatency := server.WeightedLatency{
				Weight:  wl.Weight,
				Latency: l,
			}
			s.LatencyDistribution = append(s.LatencyDistribution, weightedLatency)
			s.WeightSum += wl.Weight
		}

		d, err := time.ParseDuration(segment.Duration)
		if err != nil {
			return server.Config{}, err
		}
		s.SegmentDuration = d

		serverConfig.Profile = append(serverConfig.Profile, s)
	}

	return serverConfig, nil
}

func parseConfigFromFile(configFilename string) (parsedYamlConfig, error) {
	// Read the config file.
	data, err := ioutil.ReadFile(configFilename)
	if err != nil {
		return parsedYamlConfig{}, err
	}

	// Parse the config file yaml.
	var parsedConfig parsedYamlConfig
	err = yaml.UnmarshalStrict([]byte(data), &parsedConfig)
	if err != nil {
		return parsedYamlConfig{}, err
	}

	return parsedConfig, nil
}

func NewBufferbloater(configFilename string, logger *zap.SugaredLogger) (*Bufferbloater, error) {
	bb := Bufferbloater{
		log:      logger,
		statsMgr: stats.NewStatsMgrImpl(logger),
	}

	parsedConfig, err := parseConfigFromFile(configFilename)
	if err != nil {
		bb.log.Fatalw("failed to parse yaml file",
			"error", err)
	}

	// Create clients.
	for tid, arg := range parsedConfig.Clients {
		cc, err := clientConfigParse(arg)
		if err != nil {
			bb.log.Fatalw("failed to create server config",
				"error", err)
		}
		bb.c = append(bb.c, client.NewClient(uint(tid), cc, logger, bb.statsMgr))
	}

	// Create servers.
	for tid, arg := range parsedConfig.Servers {
		sc, err := serverConfigParse(arg)
		if err != nil {
			bb.log.Fatalw("failed to create server config",
				"error", err)
		}
		bb.s = append(bb.s, server.NewServer(uint(tid), sc, logger, bb.statsMgr))
	}

	return &bb, nil
}

func (bb *Bufferbloater) Run() {
	// TODO: make folder configurable.
	defer bb.statsMgr.DumpStatsToFolder(*statsDataDir)

	stopStats := make(chan struct{}, 1)
	var statsWg sync.WaitGroup
	statsWg.Add(1)

	go bb.statsMgr.PeriodicStatsCollection(500*time.Millisecond, stopStats, &statsWg)

	var wg sync.WaitGroup

	// Start servers.
	for _, s := range bb.s {
		wg.Add(1)
		go s.Start(&wg)
	}

	// Start clients.
	for _, c := range bb.c {
		wg.Add(1)
		go c.Start(&wg)
	}

	wg.Wait()

	stopStats <- struct{}{}
	statsWg.Wait()
}
