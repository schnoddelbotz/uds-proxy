// +build functional

package proxy_test

import (
	"context"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"gotest.tools/assert"

	"github.com/schnoddelbotz/uds-proxy/proxy"
	"github.com/schnoddelbotz/uds-proxy/proxy_test_server"
)

var testProxy *proxy.ProxyInstance

const (
	fakeServerPort    = ":25777"
	fakeServerBaseURL = "http://localhost" + fakeServerPort
	metricsPort       = ":18081"
	metricsURL        = "http://localhost" + metricsPort + "/metrics"
)

func TestMain(m *testing.M) {
	// create a fake upstream server with well-known responses (a la mountebank)
	fakeServer := proxy_test_server.RunUpstreamFakeServer(fakeServerPort, true)
	// create a single uds-proxy instance and use it for all tests in here
	testProxy = newTestProxyInstance()

	// run all tests
	res := m.Run()

	testProxy.Shutdown(nil)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	if err := fakeServer.Shutdown(ctx); err != nil {
		log.Printf("fakeserver shutdown fail:s %s", err)
	}
	os.Exit(res)
}

func Test_RootIndexOK(t *testing.T) {
	body, _, responseCode, err := httpGet(fakeServerBaseURL+"/", testProxy)

	assert.NilError(t, err)
	assert.Equal(t, responseCode, 200, "root / returns 200 OK")
	assert.Equal(t, string(body), "ROOT-INDEX-OK", "root / body matches `ROOT-INDEX-OK`")
}

func Test_404Remains404(t *testing.T) {
	_, _, responseCode, err := httpGet(fakeServerBaseURL+"/code/404", testProxy)

	assert.NilError(t, err)
	assert.Equal(t, responseCode, 404, "proxy should forward 404s")
}

func Test_TimeoutRespectedAndReportedCorrectly(t *testing.T) {
	_, _, responseCode, err := httpGet(fakeServerBaseURL+"/slow/200/1100", testProxy)

	assert.NilError(t, err)
	assert.Equal(t, testProxy.Options.ClientTimeout, 1000)
	assert.Equal(t, responseCode, 504, "proxy should return 504 for timeout")
}

func Test_BadHostnameYields502(t *testing.T) {
	_, _, responseCode, err := httpGet("http://jghjghjgjgjtybmbknkj.jhgjhg/slow/200/1100", testProxy)

	assert.NilError(t, err)
	assert.Equal(t, testProxy.Options.ClientTimeout, 1000)
	assert.Equal(t, responseCode, 502, "proxy should return 502 for invalid hostname")
}

func Test_MetricsExported(t *testing.T) {
	_, headersNoProxy, responseCode, err := httpGet(metricsURL, nil)

	assert.NilError(t, err)
	assert.Equal(t, headersNoProxy.Get("X-Response-Via"), "")
	assert.Equal(t, responseCode, 200, "uds-proxy should provide /metrics on -prometheus-port")
	// tbd: match expected metrics
}

func Test_ProxyPreservesResponseHeaders(t *testing.T) {
	// get request headers for a public website - without proxy
	_, headersNoProxy, responseCode, err := httpGet("https://www.google.com/", nil)
	assert.NilError(t, err)
	assert.Equal(t, responseCode, 200)
	assert.Equal(t, headersNoProxy.Get("X-Response-Via"), "")
	// run same request through proxy instance
	args := proxy.CliArgs{
		SocketPath:  "uds-proxy-https.sock",
		RemoteHTTPS: true,
	}
	httpsEnforcingProxy := proxy.NewProxyInstance(args)
	go httpsEnforcingProxy.Run()
	time.Sleep(250 * time.Millisecond)
	_, headersWithProxy, responseCode, err := httpGet("http://www.google.com/", httpsEnforcingProxy)

	assert.NilError(t, err)
	assert.Equal(t, responseCode, 200)
	assert.Equal(t, headersWithProxy.Get("X-Response-Via"), "uds-proxy")
	for k, v := range headersNoProxy {
		if k == "Set-Cookie" || k == "Alt-Svc" || k == "Date" {
			continue
		}
		assert.Equal(t, v[0], headersWithProxy.Get(k))
	}

	httpsEnforcingProxy.Shutdown(nil)
}

// MultipleBlockingCallsDoNotBlockSocket -- 10 x go curl /slow/no-response/65000
// TimeoutRespectedAndReportedCorrectly
// PostDataIsPreserved
// NotMoreThanMaxConnsAfter1000Requests
// DoesNotLeakConnections
// TestAllMethodsWithEchoServer
// Test (via echo?) all headers passed /echo
// test [post request with] echo endpoint
// TestMetricsReportCorrectNumberOfRequests
// check behaviour with sticky/slow client ie sock read timeout etc

func httpGet(url string, proxyInstance *proxy.ProxyInstance) (body []byte, header http.Header, responseCode int, err error) {
	client := http.Client{}
	if proxyInstance != nil {
		client.Transport = &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", proxyInstance.Options.SocketPath)
			},
		}
	}
	response, err := client.Get(url)
	if err != nil {
		return
	}
	responseCode = response.StatusCode
	body, err = ioutil.ReadAll(response.Body)
	header = response.Header
	return
}

func newTestProxyInstance() *proxy.ProxyInstance {
	args := proxy.CliArgs{
		SocketPath:      "uds-proxy-functional_test.sock",
		PidFile:         "uds-proxy-test.pid",
		PrometheusPort:  metricsPort,
		NoLogTimeStamps: true,
		ClientTimeout:   1000,
	}
	e := proxy.NewProxyInstance(args)
	go e.Run()
	time.Sleep(250 * time.Millisecond)
	return e
}
