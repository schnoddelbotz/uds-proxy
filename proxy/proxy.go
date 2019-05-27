package proxy

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// AppVersion is set at compile time via make / ldflags
	AppVersion = "0.0.0-dev"
)

type ProxyInstance struct {
	Options    CliArgs
	HttpClient *http.Client
	metrics    AppMetrics
}

type CliArgs struct {
	SocketPath          string
	PidFile             string
	PrometheusPort      string
	ClientTimeout       int
	MaxConnsPerHost     int
	MaxIdleConns        int
	MaxIdleConnsPerHost int
	IdleConnTimeout     int
	SocketReadTimeout   int
	SocketWriteTimeout  int
	PrintVersion        bool
	NoLogTimeStamps     bool
	RemoteHTTPS         bool
}

func NewProxyInstance(args CliArgs) *ProxyInstance {
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

	proxyInstance := ProxyInstance{}
	proxyInstance.Options = args
	if args.PrometheusPort != "" {
		proxyInstance.setupMetrics()
	}
	proxyInstance.HttpClient = newHTTPClient(&proxyInstance.Options, proxyInstance.metrics.enabled)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go sigHandler(c, &proxyInstance)

	return &proxyInstance
}

func (p *ProxyInstance) Run() {
	if p.metrics.enabled {
		go p.startPrometheusMetricsServer()
	}
	p.startSocketServerAcceptLoop()
}

func (p *ProxyInstance) Shutdown(sig os.Signal) {
	if sig == nil {
		sig = os.Interrupt
	}
	log.Printf("%v -- cleaning up", sig)
	p.HttpClient.CloseIdleConnections()
	os.Remove(p.Options.SocketPath)
	os.Remove(p.Options.PidFile)
	log.Print("uds-proxy shut down cleanly. nice. good bye 👋")
}

func (p *ProxyInstance) startSocketServerAcceptLoop() {
	if _, err := os.Stat(p.Options.SocketPath); err == nil {
		err := os.Remove(p.Options.SocketPath)
		if err != nil {
			panic(err)
		}
	}

	server := http.Server{
		ReadTimeout:  time.Duration(p.Options.SocketReadTimeout) * time.Millisecond,
		WriteTimeout: time.Duration(p.Options.SocketWriteTimeout) * time.Millisecond,
		Handler: func() http.HandlerFunc {
			return p.handleProxyRequest
		}(),
	}

	if p.metrics.enabled {
		server.Handler = promhttp.InstrumentHandlerInFlight(p.metrics.RequestsInflight,
			promhttp.InstrumentHandlerCounter(p.metrics.RequestsCounter,
				promhttp.InstrumentHandlerDuration(p.metrics.RequestsDuration,
					promhttp.InstrumentHandlerResponseSize(p.metrics.RequestsSize,
						func() http.HandlerFunc {
							return p.handleProxyRequest
						}()))))
	}

	unixListener, err := net.Listen("unix", p.Options.SocketPath)
	if err != nil {
		panic(err)
	}
	server.Serve(unixListener)
}

func (p *ProxyInstance) handleProxyRequest(clientResponseWriter http.ResponseWriter, clientRequest *http.Request) {
	requestLogFormat := "%03d %d %s %s '%s'"
	httpsS := ""
	if p.Options.RemoteHTTPS {
		httpsS = "s"

	}
	targetURL := fmt.Sprintf("http%s://%s%s", httpsS, clientRequest.Host, clientRequest.URL)

	backendRequest, err := http.NewRequest(clientRequest.Method, targetURL, clientRequest.Body)
	if err != nil {
		log.Printf(requestLogFormat, 0, 0, clientRequest.Method, targetURL, err)
		http.Error(clientResponseWriter, "500 Internal Server Error", http.StatusInternalServerError)
		return
	}
	backendRequest.Header = clientRequest.Header
	backendRequest.Header.Set("X-Request-Via", "uds-proxy")

	backendResponse, err := p.HttpClient.Do(backendRequest)
	if err != nil {
		log.Printf(requestLogFormat, 0, 0, clientRequest.Method, targetURL, err)
		http.Error(clientResponseWriter, "504 Gateway Timeout", http.StatusGatewayTimeout)
	} else {
		for k, v := range backendResponse.Header {
			clientResponseWriter.Header().Set(k, v[0])
		}
		clientResponseWriter.Header().Set("X-Response-Via", "uds-proxy")
		clientResponseWriter.WriteHeader(backendResponse.StatusCode)
		bytesWritten, _ := io.Copy(clientResponseWriter, backendResponse.Body)
		log.Printf(requestLogFormat, backendResponse.StatusCode, bytesWritten, clientRequest.Method, targetURL, "")
		backendResponse.Body.Close()
	}
}

func newHTTPClient(opt *CliArgs, metricsEnabled bool) (client *http.Client) {
	transport := http.Transport{
		MaxConnsPerHost:       opt.MaxConnsPerHost,
		MaxIdleConns:          opt.MaxIdleConns,
		MaxIdleConnsPerHost:   opt.MaxIdleConnsPerHost,
		IdleConnTimeout:       time.Duration(opt.IdleConnTimeout) * time.Millisecond,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: 5 * time.Second,
	}
	client = &http.Client{
		Timeout:   time.Duration(opt.ClientTimeout) * time.Millisecond,
		Transport: &transport,
	}
	if metricsEnabled {
		client.Transport = getTracingTransport(&transport)
	}
	return
}
