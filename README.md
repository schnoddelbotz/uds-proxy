# uds-proxy
uds-proxy provides a UNIX domain socket and forwards traffic to HTTP(S) remotes
through a customizable connection pool (i.e. using persistent connections).

## what for? why? how?
Interacting with microservices often involves communication overhead: Every contact
with another service may involve DNS lookups and establishment of a TCP connection
plus, most likely, a HTTPS handshake.

This overhead can be costly and especially hard to circumvent for legacy applications -- thus uds-proxy.

uds-proxy creates a UNIX domain socket and forwards communication to one or more
remote web servers. In a way, uds-proxy aims a bit at reducing application/API complexity by
providing a generic and simple solution for connection pooling.

uds-proxy is implemented in Go, so it runs as native application on any
OS supporting Go and UNIX domain sockets (i.e. not on Windows). Critical
performance metrics of uds-proxy (request latencies, response codes...)
and Go process statistics are exposed through Prometheus client library.

## building / installing uds-proxy

Building requires a local Go 1.11+ installation:

```bash
go get -v github.com/schnoddelbotz/uds-proxy/cmd/uds-proxy
```

... or just grab a [uds-proxy binary release](https://github.com/schnoddelbotz/uds-proxy/releases).

See [usage-example-for-an-https-endpoint](#usage-example-for-an-https-endpoint) for Docker usage.

To start uds-proxy at system boot, create e.g. a systemd unit.
Don't try to run uds-proxy as root. It won't start.

## usage

```
Usage of ./uds-proxy:
  -client-timeout int
      http client connection timeout [ms] for proxy requests (default 5000)
  -idle-timeout int
      connection timeout [ms] for idle backend connections (default 90000)
  -max-conns-per-host int
      maximum number of connections per backend host (default 20)
  -max-idle-conns int
      maximum number of idle HTTP(S) connections (default 100)
  -max-idle-conns-per-host int
      maximum number of idle conns per backend (default 25)
  -no-access-log
    	disable proxy access logging
  -no-log-timestamps
      disable timestamps in log messages
  -pid-file string
      pid file to use, none if empty
  -prometheus-port string
      Prometheus monitoring port, e.g. :18080
  -remote-https
      remote uses https://
  -socket string
      path of socket to create
  -socket-read-timeout int
      read timeout [ms] for -socket (default 5000)
  -socket-write-timeout int
      write timeout [ms] for -socket (default 5000)
  -version
      print uds-proxy version
```

## monitoring / testing / development

Clone this repository and check the [Makefile](Makefile) targets.

Most relevant `make` targets:

- `make monitoring_test` spins up Prometheus, grafana and uds-proxy using Docker and
  starts another uds-proxy instance locally (outside Docker, on Mac only). The uds-proxy instances will be
  scraped by dockerized Prometheus and Grafana will provide dashboards.
  See [monitoring/README.md](monitoring/README.md) for details.
- `make run_proxy` starts a local uds-proxy instance for testing purposes.
  `TEST_SOCKET` environment variable controls socket location, defaults
  to `uds-proxy-test.socket`.
- `make test` runs unit and functional tests from [proxy_test](proxy_test) directory.
- `make coverage` generates code test coverage statistics.
- `make test_integration` starts a local uds-proxy and runs some proxied-vs-non-proxied perf tests.
- `make realclean` removes leftovers from tests or builds.

### usage example for an HTTPS endpoint
Start the proxy:

```bash
uds-proxy -socket /tmp/proxied-svc.sock -prometheus-port :28080 -remote-https
```

Docker users:

```bash
mkdir -p /tmp/mysock_dir
docker run --rm -it -p28080:28080 -v/tmp/mysock_dir:/tmp schnoddelbotz/uds-proxy
```

For both cases, metrics should be available at http://localhost:28080/metrics while uds-proxy is running.

#### using bash / curl
```bash
# without uds-proxy, you would...
time curl -I https://www.google.com/

# with uds-proxy, always ...
# a) talk through socket and
# b) use http:// and let `-remote-https` ensure https is used to connect to remote hosts
time curl -I --unix-socket /tmp/proxied-svc.sock http://www.google.com/
# ... or using socket provided by dockerized uds-proxy:
time curl -I --unix-socket /tmp/mysock_dir/uds-proxy-docker.sock http://www.google.com/
```

#### using php / curl
```php
<?php
// without uds-proxy
$ch = curl_init();
curl_setopt($ch, CURLOPT_URL, "https://www.google.com/");
curl_exec($ch);

// with uds-proxy
$ch = curl_init();
curl_setopt($ch, CURLOPT_URL, "http://www.google.com/");
curl_setopt($ch, CURLOPT_UNIX_SOCKET_PATH, "/tmp/proxied-svc.sock");
curl_exec($ch);
```

### further socket testing

Mac's (i.e. BSD's) netcat allows to talk to unix domain sockets.
It can be used to e.g. ensure correct behaviour of uds-proxy's
`-socket-(read|write)-timeout` options. Try `nc -U /path/to/uds-proxy.sock`.

## todo ...

- fix/drop sudo nobody for dockerized tests
- fixme: add option [-dont-follow-redirects](https://stackoverflow.com/questions/23297520/how-can-i-make-the-go-http-client-not-follow-redirects-automatically)
- for http/s client:
  - wrap in circuit breaker?
  - wrap in retry /w exponential backoff? consider api consumer constraints (i.e. timeout - worth it?)
- travis-ci + github release push
- example systemd unit
- sock umask / cli opt
- support magic uds request headers...?
  - X-udsproxy-timeout: 250ms
  - X-udsproxy-debug: true

## links

- https://godoc.org/gotest.tools/assert
- https://golang.org/pkg/net/#hdr-Name_Resolution
- https://stackoverflow.com/questions/17948827/reusing-http-connections-in-golang
- https://medium.com/@povilasve/go-advanced-tips-tricks-a872503ac859
- https://github.com/bouk/monkey/blob/master/monkey_test.go
- https://github.com/prometheus/client_golang/blob/master/prometheus/examples_test.go
- https://github.com/prometheus/client_golang/blob/master/prometheus/promhttp/instrument_server.go

## alternatives

Obviously, uds-proxy is a kludge. Simply use connection pooling if available!

- for Python and HTTP, simply reuse [requests library's session objects](https://2.python-requests.org/en/master/user/advanced/#session-objects) and you're set
- for Python and Redis, use a [redis.py connection pool](https://github.com/andymccurdy/redis-py#connection-pools)
- for Redis and PHP, [phpredis](https://github.com/phpredis/phpredis) supports connection pooling since v4.2.1
- a potentially more sophisticated solution can be found in
  [this TCP vs UDS speed comparison stackoverflow thread](https://stackoverflow.com/questions/14973942/performance-tcp-loopback-connection-vs-unix-domain-socket):
  [Speedus](http://speedus.torusware.com/) intercepts relevant system calls, which avoids
  need for any code changes. However, if I understood correctly, Speedus only helps if
  services actually sit on the same host system (?).

You can also use NGINX [to create a UDS HTTP/S pooling forward proxy](https://serverfault.com/questions/899109/universal-persistent-connection-pool-proxy-with-nginx) like uds-proxy.
It seems that neither [Apache](https://bz.apache.org/bugzilla/show_bug.cgi?id=55898)
nor Squid (?) are able to do that.

## license

MIT
