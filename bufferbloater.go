package main

import (
	"io/ioutil"
	"sync"
	"time"

	"go.uber.org/zap"
	"gopkg.in/yaml.v2"

	"github.com/tonya11en/bufferbloater/client"
	"github.com/tonya11en/bufferbloater/server"
	"github.com/tonya11en/bufferbloater/stats"
)

type Bufferbloater struct {
	log      *zap.SugaredLogger
	c1       *client.Client
	c2       *client.Client
	s        *server.Server
	statsMgr *stats.StatsMgr
}

// Basic representation of the parsed yaml file before the durations are parsed.
type parsedYamlConfig struct {
	Client []struct {
		Workload []struct {
			Rps      uint
			Duration string
		} `yaml:"workload"`
		RqTimeout    string `yaml:"rq_timeout"`
		TargetServer struct {
			Address string
			Port    uint
		} `yaml:"target_server"`
	} `yaml:"client"`

	Server struct {
		Profile []struct {
			LatencyDistribution []struct {
				Weight  uint   `yaml:"weight"`
				Latency string `yaml:"latency"`
			} `yaml:"latency_distribution"`
			Duration string
		} `yaml:"profile"`
		ListenPort   uint   `yaml:"listen_port"`
		Threads      uint   `yaml:"threads"`
		MaxQueueSize uint   `yaml:"max_queue_size"`
		MaxActiveRq  uint   `yaml:"max_active_rq"`
		QueueTimeout string `yaml:"queue_timeout"`
	}
}

// Creates a properly typed client config.
func clientConfigParse(parsedConfig parsedYamlConfig, idx uint) (client.Config, error) {
	// TODO: validate config

	clientConfig := client.Config{
		TargetServer: client.Target{
			Address: parsedConfig.Client[idx].TargetServer.Address,
			Port:    parsedConfig.Client[idx].TargetServer.Port,
		},
	}

	d, err := time.ParseDuration(parsedConfig.Client[idx].RqTimeout)
	if err != nil {
		return client.Config{}, err
	}
	clientConfig.RequestTimeout = d

	for _, stage := range parsedConfig.Client[idx].Workload {
		d, err := time.ParseDuration(stage.Duration)
		if err != nil {
			return client.Config{}, err
		}

		workloadStage := client.WorkloadStage{
			RPS:      stage.Rps,
			Duration: d,
		}
		clientConfig.Workload = append(clientConfig.Workload, workloadStage)
	}

	return clientConfig, nil
}

func serverConfigParse(parsedConfig parsedYamlConfig) (server.Config, error) {
	// TODO: validate config

	serverConfig := server.Config{
		ListenPort:   parsedConfig.Server.ListenPort,
		Threads:      parsedConfig.Server.Threads,
		MaxActiveRq:  parsedConfig.Server.MaxActiveRq,
		MaxQueueSize: parsedConfig.Server.MaxQueueSize,
	}

	qtimeout, err := time.ParseDuration(parsedConfig.Server.QueueTimeout)
	if err != nil {
		return server.Config{}, err
	}
	serverConfig.QueueTimeout = qtimeout

	for _, segment := range parsedConfig.Server.Profile {
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

	clientConfig1, err := clientConfigParse(parsedConfig, 0)
	if err != nil {
		bb.log.Fatalw("failed to create client config",
			"error", err)
	}
	clientConfig2, err := clientConfigParse(parsedConfig, 1)
	if err != nil {
		bb.log.Fatalw("failed to create client config",
			"error", err)
	}
	bb.c1 = client.NewClient(clientConfig1, logger, bb.statsMgr, "1")
	bb.c2 = client.NewClient(clientConfig2, logger, bb.statsMgr, "2")

	serverConfig, err := serverConfigParse(parsedConfig)
	if err != nil {
		bb.log.Fatalw("failed to create server config",
			"error", err)
	}
	bb.s = server.NewServer(serverConfig, logger, bb.statsMgr)

	return &bb, nil
}

func (bb *Bufferbloater) Run() {
	// TODO: make folder configurable.
	defer bb.statsMgr.DumpStatsToFolder("data")

	stopStats := make(chan struct{}, 1)
	var statsWg sync.WaitGroup
	statsWg.Add(1)
	go bb.statsMgr.PeriodicStatsCollection(100*time.Millisecond, stopStats, &statsWg)

	var wg sync.WaitGroup
	wg.Add(3)
	go bb.s.Start(&wg)
	go bb.c1.Start(&wg)
	go bb.c2.Start(&wg)
	wg.Wait()

	stopStats <- struct{}{}
	statsWg.Wait()
}
