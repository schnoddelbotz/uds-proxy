// +build unit

package proxy_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/schnoddelbotz/uds-proxy/proxy"
)

const testSocketFilename = "uds-proxy-unit-test.sock"

func Test_Create(t *testing.T) {
	e := proxy.NewProxyInstance(proxy.Settings{SocketPath: testSocketFilename})

	assert.False(t, e.Options.RemoteHTTPS)
	assert.Equal(t, e.Options.SocketPath, testSocketFilename)
	e.Shutdown(nil)
}

func Test_HttpClientUsesCliArgs(t *testing.T) {
	testTimeout := 500
	e := proxy.NewProxyInstance(proxy.Settings{SocketPath: testSocketFilename, ClientTimeout: testTimeout, MaxIdleConns: 11})

	assert.Equal(t, e.HTTPClient.Timeout, time.Duration(testTimeout)*time.Millisecond)
	e.Shutdown(nil)
}

func Test_EnvArgumentRemoteHTTPSArrivesInOptions(t *testing.T) {
	e := proxy.NewProxyInstance(proxy.Settings{SocketPath: testSocketFilename, RemoteHTTPS: true})

	assert.Equal(t, e.Options.RemoteHTTPS, true)
	e.Shutdown(nil)
}

func Test_AppVersionDefined(t *testing.T) {
	assert.NotEqual(t, proxy.AppVersion, "0.0.0-dev")
}

func Test_PanicsIfDirectoryProvidedAsFilenameForSocket(t *testing.T) {
	e := proxy.NewProxyInstance(proxy.Settings{SocketPath: "/tmp"})

	assert.Panics(t, e.Run, "-socket must be a filename, panics if  undeleteable")
}
