package main

import (
	"testing"
	"time"

	"gopkg.in/yaml.v2"

	"allen.gg/bufferbloater/server"
	"github.com/stretchr/testify/assert"
)

var validYamlString = `
clients:
  - workload:
      - rps: 100
        duration: 500us
      - rps: 500
        duration: 30ms
    rq_timeout: 100ms
    target_server:
      address: 0.0.0.0
      port: 9001
servers:
  - profile:
    - duration: 20s
      latency_distribution:
      - weight: 49
        latency: 1ms
      - weight: 51
        latency: 2ms
    - duration: 5s
      latency_distribution:
      - weight: 48
        latency: 3ms
      - weight: 50
        latency: 4ms
    listen_port: 9002
    threads: 1`

func TestServerParsing(t *testing.T) {
	var parsedConfig parsedYamlConfig
	err := yaml.UnmarshalStrict([]byte(validYamlString), &parsedConfig)
	assert.Equal(t, err, nil)

	expected := server.Config{
		Profile: []server.Segment{
			{
				SegmentDuration: time.Second * 20,
			},
			{
				SegmentDuration: time.Second * 5,
			},
		},
		ListenPort: 9002,
		Threads:    1,
	}

	ss, err := serverConfigParse(parsedConfig.Servers[0])
	assert.Equal(t, expected, ss)
	assert.Nil(t, err)
}

func TestClientParsing(t *testing.T) {
	var parsedConfig parsedYamlConfig
	err := yaml.UnmarshalStrict([]byte(validYamlString), &parsedConfig)
	assert.Equal(t, err, nil)

	cc, err := clientConfigParse(parsedConfig.Clients[0])
	assert.Equal(t, err, nil)
	assert.Equal(t, cc.TargetServer.Address, "0.0.0.0")
	assert.Equal(t, cc.TargetServer.Port, uint(9001))
	assert.Equal(t, cc.RequestTimeout, time.Millisecond*100)
	assert.Equal(t, cc.Workload[0].RPS, uint(100))
	assert.Equal(t, cc.Workload[0].Duration, time.Microsecond*500)
	assert.Equal(t, cc.Workload[1].RPS, uint(500))
	assert.Equal(t, cc.Workload[1].Duration, time.Millisecond*30)
}
