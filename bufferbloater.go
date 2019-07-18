package main

import (
	"io/ioutil"
	"time"

	"go.uber.org/zap"
	"gopkg.in/yaml.v2"

	"github.com/tonya11en/bufferbloater/client"
	"github.com/tonya11en/bufferbloater/server"
)

type Bufferbloater struct {
	log *zap.SugaredLogger
	c   *client.Client
	s   *server.Server
}

// Basic representation of the parsed yaml file before the durations are parsed.
type parsedYamlConfig struct {
	Client struct {
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
			RqLatency string `yaml:"rq_latency"`
			Duration  string
		} `yaml:"profile"`
		ListenPort uint `yaml:"listen_port"`
	}
}

// Creates a properly typed client config.
func clientConfigParse(parsedConfig parsedYamlConfig) (client.Config, error) {
	clientConfig := client.Config{
		TargetServer: client.Target{
			Address: parsedConfig.Client.TargetServer.Address,
			Port:    parsedConfig.Client.TargetServer.Port,
		},
	}

	d, err := time.ParseDuration(parsedConfig.Client.RqTimeout)
	if err != nil {
		return client.Config{}, err
	}
	clientConfig.RequestTimeout = d

	for _, stage := range parsedConfig.Client.Workload {
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
	serverConfig := server.Config{
		ListenPort: parsedConfig.Server.ListenPort,
	}

	for _, segment := range parsedConfig.Server.Profile {
		s := server.LatencySegment{}

		d, err := time.ParseDuration(segment.RqLatency)
		if err != nil {
			return server.Config{}, err
		}
		s.RequestLatency = d

		d, err = time.ParseDuration(segment.Duration)
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
		log: logger,
	}

	parsedConfig, err := parseConfigFromFile(configFilename)
	if err != nil {
		bb.log.Fatalw("failed to parse yaml file",
			"error", err)
	}

	clientConfig, err := clientConfigParse(parsedConfig)
	if err != nil {
		bb.log.Fatalw("failed to create server config",
			"error", err)
	}
	bb.c = client.NewClient(clientConfig, logger)

	serverConfig, err := serverConfigParse(parsedConfig)
	if err != nil {
		bb.log.Fatalw("failed to create server config",
			"error", err)
	}
	bb.s = server.NewServer(serverConfig, logger)

	return &bb, nil
}
