# bufferbloater
A configurable client/server bufferbloat simulation.

## Configuration

A basic configuration file looks something like this:
```
client:
  workload:
    - rps: 10
      duration: 10s
    - rps: 20
      duration: 50s
  rq_timeout: 5s
  target_server:
    address: 0.0.0.0
    port: 9002
server:
  profile:
    - rq_latency: 200ms
      duration: 60s
  listen_port: 9002
  threads: 4
```

### Client

The client configuration allows a user to specify stages of variable durations
at a specific request rate (RPS). In the example above, the client will send
HTTP `GET` requests to `0.0.0.0:9002` at a rate of 10 RPS for 10 seconds,
followed by a rate of 20 RPS for 50 seconds.

### Server

The server configuration, similar to the client, allows a user to specify stages
in the server's lifetime. The `workload` field contains an obvious `duration
field`; however, the `rq_latency` field controls how long the server
will spend "servicing" a request. One can simulate scenarios of service
degradation by including stages with an increased request latency.

The `threads` field controls how many worker threads are allowed to "service"
requests.

## Generating data

CSV files are created for each run and placed in a folder called `data`. If
files already exist in that folder, they will be overwritten. The bufferbloater
will only create files if there is data that needs to be written to it.

There is a lot of polish left to do in this area.
