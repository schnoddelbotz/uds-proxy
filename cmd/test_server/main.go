package main

import (
	"flag"

	"github.com/schnoddelbotz/uds-proxy/proxy_test_server"
)

func main() {
	var port string
	flag.StringVar(&port, "port", ":25777", "fake webserver tcp port")
	// could add flag to exit after N requests?
	// could add flag to exit after N seconds?
	flag.Parse()

	proxy_test_server.RunUpstreamFakeServer(port, false)
}
