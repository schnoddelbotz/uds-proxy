package main

import (
	"flag"
	"os"

	"github.com/schnoddelbotz/uds-proxy/proxy"
)

func main() {
	var args proxy.Settings

	if os.Getuid() == 0 {
		println("uds-proxy is refusing to run as root user")
		os.Exit(1)
	}

	flag.BoolVar(&args.NoLogTimeStamps, "no-log-timestamps", false, "disable timestamps in log messages")
	flag.BoolVar(&args.NoAccessLog, "no-access-log", false, "disable proxy access logging")
	flag.BoolVar(&args.PrintVersion, "version", false, "print uds-proxy version")
	flag.BoolVar(&args.RemoteHTTPS, "remote-https", false, "remote uses https://")

	flag.IntVar(&args.MaxConnsPerHost, "max-conns-per-host", 20, "maximum number of connections per backend host")
	flag.IntVar(&args.MaxIdleConns, "max-idle-conns", 100, "maximum number of idle HTTP(S) connections")
	flag.IntVar(&args.MaxIdleConnsPerHost, "max-idle-conns-per-host", 15, "maximum number of idle conns per backend")
	flag.IntVar(&args.ClientTimeout, "client-timeout", 5000, "http client connection timeout [ms] for proxy requests")
	flag.IntVar(&args.IdleConnTimeout, "idle-timeout", 90000, "connection timeout [ms] for idle backend connections")
	flag.IntVar(&args.SocketReadTimeout, "socket-read-timeout", 5500, "read timeout [ms] for -socket")
	flag.IntVar(&args.SocketWriteTimeout, "socket-write-timeout", 5500, "write timeout [ms] for -socket")

	flag.StringVar(&args.PidFile, "pid-file", "", "pid file to use, none if empty")
	flag.StringVar(&args.SocketPath, "socket", os.Getenv("UDS_PROXY_SOCKET"), "path of socket to create")
	flag.StringVar(&args.PrometheusPort, "prometheus-port", "", "Prometheus monitoring port, e.g. :18080")

	flag.Parse()

	proxy.NewProxyInstance(args).Run()
}
