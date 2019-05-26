# monitoring uds-proxy

Exposure of performance metrics can be enabled by using uds-proxy's 
`-prometheus-port` command line argument. For example, using `-prometheus-port :12345` 
will make metrics available at `http://localhost:12345/metrics`.

This directory contains Grafana provisioning configuration and prometheus
configuration used by [docker-compose.yml](../docker-compose.yml) to
spin up an example uds-proxy dashboard, fed with live data.

From uds-proxy source root directory, run

```bash
make monitoring_test
```

to bring up uds-proxy, Prometheus and Grafana for experiments. This will...

- on Mac: build and run uds-proxy locally on your host (requires Go)
- on Linux: build and run uds-proxy using Docker 
- spin up Prometheus, configured to collect metrics from uds-proxy
- spin up Grafana, configured with prometheus as datasource and uds-proxy dashboard
- timeit: run 50 test requests directly and 50 via uds-proxy

Then...

- visit http://localhost:3000 and select one of the three dashboards available:
  - [uds-proxy stats](http://localhost:3000/d/ups/uds-proxy-stats?orgId=1)
  - [go process stats](http://localhost:3000/d/go/go-stats?orgId=1)
  - [prometheus stats](http://localhost:3000/d/prom/prometheus-stats?orgId=1)
- use `uds-proxy-test.socket` for further tests, eg. use `make run_some_requests`

Shut the test environment down and **lose all collected data** by hitting ctrl-c.

Why does `make monitoring_test` spin up two uds-proxy instances on Mac -- a local one and a dockerized one? 
[Because](https://github.com/docker/for-mac/issues/483) containers on Docker for 
Mac will never be able to share a socket with the host. So, for testing purposes,
a 2nd instance is started locally to enable test runs, while the dockerized one
is sitting idle.