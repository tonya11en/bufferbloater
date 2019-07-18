package main

import (
	"testing"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/stretchr/testify/assert"
)

var validYamlString = `client:
  workload:
    - rps: 100
      duration: 500us
    - rps: 500
      duration: 30ms
  rq_timeout: 100ms
  target_server:
    address: 0.0.0.0
    port: 9001
server:
  profile:
    - rq_latency: 1ms
      duration: 1s
    - rq_latency: 5ms
      duration: 5s
  listen_port: 9002
	threads: 1`

func TestServerParsing(t *testing.T) {
	var parsedConfig parsedYamlConfig
	err := yaml.UnmarshalStrict([]byte(validYamlString), &parsedConfig)
	assert.Equal(t, err, nil)

	ss, err := serverConfigParse(parsedConfig)
	assert.Equal(t, err, nil)
	assert.Equal(t, ss.ListenPort, uint(9002))
	assert.Equal(t, ss.Profile[0].RequestLatency, time.Millisecond)
	assert.Equal(t, ss.Profile[0].SegmentDuration, time.Second)
	assert.Equal(t, ss.Profile[1].RequestLatency, time.Millisecond*5)
	assert.Equal(t, ss.Profile[1].SegmentDuration, time.Second*5)
	assert.Equal(t, ss.Threads, uint(1))
}

func TestClientParsing(t *testing.T) {
	var parsedConfig parsedYamlConfig
	err := yaml.UnmarshalStrict([]byte(validYamlString), &parsedConfig)
	assert.Equal(t, err, nil)

	cc, err := clientConfigParse(parsedConfig)
	assert.Equal(t, err, nil)
	assert.Equal(t, cc.TargetServer.Address, "0.0.0.0")
	assert.Equal(t, cc.TargetServer.Port, uint(9001))
	assert.Equal(t, cc.RequestTimeout, time.Millisecond*100)
	assert.Equal(t, cc.Workload[0].RPS, uint(100))
	assert.Equal(t, cc.Workload[0].Duration, time.Microsecond*500)
	assert.Equal(t, cc.Workload[1].RPS, uint(500))
	assert.Equal(t, cc.Workload[1].Duration, time.Millisecond*30)
}
