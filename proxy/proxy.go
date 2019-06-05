/*
Package proxy implements an HTTP forward proxy that exclusively listens on a UNIX domain socket for
client requests. It uses a single http.Client to proxy requests, enabling connection
pooling. Optionally, the proxy can expose metrics via prometheus client library.
*/
package proxy

import (
	"fmt"
	"github.com/PuerkitoBio/rehttp"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// AppVersion is set at compile time via make / ldflags
var AppVersion = "0.8.x-dev"

// Instance provides state storage for a single proxy instance.
type Instance struct {
	Options    Settings
	HTTPClient *http.Client
	metrics    appMetrics
}

// Settings configure a Instance and need to be passed to NewProxyInstance().
type Settings struct {
	SocketPath          string
	PidFile             string
	PrometheusPort      string
	ClientTimeout       int
	ClientMaxRetries    int
	MaxConnsPerHost     int
	MaxIdleConns        int
	MaxIdleConnsPerHost int
	IdleConnTimeout     int
	SocketReadTimeout   int
	SocketWriteTimeout  int
	PrintVersion        bool
	NoLogTimeStamps     bool
	NoAccessLog         bool
	RemoteHTTPS         bool
}

// NewProxyInstance validates supplied Settings and returns a ready-to-run proxy instance.
func NewProxyInstance(args Settings) *Instance {
	if args.PrintVersion {
		println("uds-proxy", AppVersion, runtime.Version())
		os.Exit(0)
	}
	if args.SocketPath == "" {
		println("Error: -socket must be provided, use -h for help")
		os.Exit(1)
	}
	if args.NoLogTimeStamps {
		log.SetFlags(0)
	}
	log.Printf("👋 uds-proxy %s, pid %d starting...", AppVersion, os.Getpid())

	writePidFile(args.PidFile)

	proxyInstance := Instance{}
	proxyInstance.Options = args
	if args.PrometheusPort != "" {
		proxyInstance.setupMetrics()
	}
	proxyInstance.HTTPClient = newHTTPClient(&proxyInstance.Options, proxyInstance.metrics.enabled)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go sigHandler(c, &proxyInstance)

	return &proxyInstance
}

// Run starts the proxy's socket server accept loop, which will run until Shutdown() is called.
func (proxy *Instance) Run() {
	if proxy.metrics.enabled {
		go proxy.startPrometheusMetricsServer()
	}
	proxy.startSocketServerAcceptLoop()
}

// Shutdown cleanly terminates a proxy instance (and is invoked by signal handlers or during tests).
func (proxy *Instance) Shutdown(sig os.Signal) {
	if sig == nil {
		sig = os.Interrupt
	}
	log.Printf("%v -- cleaning up", sig)
	proxy.HTTPClient.CloseIdleConnections()
	os.Remove(proxy.Options.SocketPath)
	os.Remove(proxy.Options.PidFile)
	log.Print("uds-proxy shut down cleanly. nice. good bye 👋")
}

func (proxy *Instance) startSocketServerAcceptLoop() {
	if _, err := os.Stat(proxy.Options.SocketPath); err == nil {
		err := os.Remove(proxy.Options.SocketPath)
		if err != nil {
			panic(err)
		}
	}

	server := http.Server{
		ReadTimeout:  time.Duration(proxy.Options.SocketReadTimeout) * time.Millisecond,
		WriteTimeout: time.Duration(proxy.Options.SocketWriteTimeout) * time.Millisecond,
		Handler:      http.HandlerFunc(proxy.handleProxyRequest)}

	if proxy.metrics.enabled {
		server.Handler = promhttp.InstrumentHandlerInFlight(proxy.metrics.RequestsInflight,
			promhttp.InstrumentHandlerCounter(proxy.metrics.RequestsCounter,
				promhttp.InstrumentHandlerDuration(proxy.metrics.RequestsDuration,
					promhttp.InstrumentHandlerResponseSize(proxy.metrics.RequestsSize,
						http.HandlerFunc(proxy.handleProxyRequest)))))
	}

	if !proxy.Options.NoAccessLog {
		server.Handler = accessLogHandler(server.Handler)
	}

	unixListener, err := net.Listen("unix", proxy.Options.SocketPath)
	if err != nil {
		panic(err)
	}
	server.Serve(unixListener)
}

func (proxy *Instance) handleProxyRequest(clientResponseWriter http.ResponseWriter, clientRequest *http.Request) {
	scheme := "http"
	if proxy.Options.RemoteHTTPS {
		scheme = "https"

	}
	targetURL := fmt.Sprintf("%s://%s%s", scheme, clientRequest.Host, clientRequest.URL)

	backendRequest, err := http.NewRequest(clientRequest.Method, targetURL, clientRequest.Body)
	if err != nil {
		http.Error(clientResponseWriter, err.Error(), http.StatusInternalServerError)
		return
	}
	backendRequest.Header = clientRequest.Header
	backendRequest.Header.Set("X-Request-Via", "uds-proxy")

	backendResponse, err := proxy.HTTPClient.Do(backendRequest)
	if err != nil {
		if err.(*url.Error).Timeout() {
			http.Error(clientResponseWriter, err.Error(), http.StatusGatewayTimeout)
		} else {
			http.Error(clientResponseWriter, err.Error(), http.StatusBadGateway)
		}
		return
	}

	for k, v := range backendResponse.Header {
		clientResponseWriter.Header().Set(k, v[0])
	}
	clientResponseWriter.Header().Set("X-Response-Via", "uds-proxy")
	clientResponseWriter.WriteHeader(backendResponse.StatusCode)
	io.Copy(clientResponseWriter, backendResponse.Body)
	backendResponse.Body.Close()
}

func newHTTPClient(opt *Settings, metricsEnabled bool) (client *http.Client) {
	transport := &http.Transport{
		MaxConnsPerHost:       opt.MaxConnsPerHost,
		MaxIdleConns:          opt.MaxIdleConns,
		MaxIdleConnsPerHost:   opt.MaxIdleConnsPerHost,
		IdleConnTimeout:       time.Duration(opt.IdleConnTimeout) * time.Millisecond,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: 5 * time.Second,
	}
	client = &http.Client{
		Timeout:   time.Duration(opt.ClientTimeout) * time.Millisecond,
		Transport: transport,
	}

	if opt.ClientMaxRetries > 0 {
		client.Transport = rehttp.NewTransport(transport,
			rehttp.RetryMaxRetries(opt.ClientMaxRetries),
			rehttp.ExpJitterDelay(100*time.Millisecond, time.Duration(opt.ClientTimeout)*time.Millisecond))
	}
	if metricsEnabled {
		client.Transport = getTracingRoundTripper(transport)
	}
	return
}
