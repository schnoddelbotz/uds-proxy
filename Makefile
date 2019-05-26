
BINARY := uds-proxy

VERSION := $(shell git describe --tags | cut -dv -f2)
DOCKER_IMAGE := schnoddelbotz/uds-proxy
LDFLAGS := -X github.com/schnoddelbotz/uds-proxy/proxy.AppVersion=$(VERSION) -w

TEST_SOCKET   := $(PWD)/uds-proxy-test.socket
TEST_PIDFILE  := uds-proxy.pid
TEST_PROMETHEUS_PORT := 18080
TEST_DOCKERIZED_SOCKET_DIR := $(PWD)/udsproxy_docker_test
TEST_DOCKERIZED_SOCKET := $(TEST_DOCKERIZED_SOCKET_DIR)/uds-proxy-docker.sock

IS_MAC := $(shell test "Darwin" = "`uname -s`" && echo 1)
USE_DOCKER := $(shell test $(IS_MAC) || echo _docker)


build: $(BINARY)

$(BINARY): cmd/uds-proxy/main.go proxy/*.go
	env go build -ldflags='-w -s $(LDFLAGS)' ./cmd/uds-proxy

test_server: cmd/test_server/main.go proxy_test_server/server.go
	go build ./cmd/test_server

zip: $(BINARY)
	zip $(BINARY)_$(GOOS)-$(GOARCH)_$(VERSION).zip $(BINARY)

release: test realclean
	env GOOS=linux  GOARCH=amd64 make clean zip
	env GOOS=darwin GOARCH=amd64 make clean zip


run_proxy: $(BINARY)
	-./$(BINARY) -socket $(TEST_SOCKET) -pid-file $(TEST_PIDFILE) \
		-prometheus-port :$(TEST_PROMETHEUS_PORT) -no-log-timestamps $(EXTRA_ARGS)

run_proxy_docker:
	@echo "run_proxy_docker: Wait for docker-composed uds-proxy to come up; or use 'make docker_run'"
	# preparing mount directory for dockerized uds-proxy socket
	mkdir -p $(TEST_DOCKERIZED_SOCKET_DIR)
	chmod 777 $(TEST_DOCKERIZED_SOCKET_DIR)

run_test_server: test_server
	./test_server

run_test_server_docker:
	echo "INFO: run_test_server_docker is a noop, it's always started via docker-compose"


test: test_go_unit test_go_functional

test_go_unit:
	go test -ldflags='-w -s $(LDFLAGS)' -tags=unit ./proxy_test

test_go_functional:
	go test -tags=functional ./proxy_test

test_integration:
	# test_integration: same as monitoring test, but without Prometheus and Grafana
	make -j4 compose_up_no_metrics run_proxy$(USE_DOCKER) run_test_server$(USE_DOCKER) run_some_requests$(USE_DOCKER)


coverage: clean
	go test -coverprofile=coverage.out -coverpkg="github.com/schnoddelbotz/uds-proxy/proxy" \
		-tags="functional unit" -ldflags='-w -s $(LDFLAGS)' ./proxy_test
	go tool cover -html=coverage.out

monitoring_test:
	# visit Grafana on http://localhost:3000/ ... (re-)run_some_requests ... and ctrl-c to quit
	make -j2 compose_build compose_pull
	make -j4 compose_up run_proxy$(USE_DOCKER) run_test_server$(USE_DOCKER) run_some_requests$(USE_DOCKER)

run_some_requests:
	sh monitoring/paint_graphs.sh $(TEST_SOCKET) http://localhost:25777

run_some_requests_docker:
	sh monitoring/paint_graphs.sh $(TEST_DOCKERIZED_SOCKET) http://testserver:44555 use-sudo


docker_image: clean
	docker build -f uds-proxy.Dockerfile -t $(DOCKER_IMAGE):$(VERSION) -t $(DOCKER_IMAGE):latest .

docker_image_push:
	docker push $(DOCKER_IMAGE)

docker_run:
	@if [ $(IS_MAC) ]; then echo "Mounting UDS does not work on Mac. Use 'make run_proxy' instead"; exit 1; fi
	docker run --rm -it -p28080:28080 -v$(TEST_DOCKERIZED_SOCKET_DIR):/tmp $(DOCKER_IMAGE) $(EXTRA_ARGS)


compose_build:
	docker-compose build

compose_pull:
	docker-compose pull

compose_up:
	docker-compose up --force-recreate

compose_up_no_metrics:
	docker-compose up --force-recreate udsproxy testserver


grafana_dump_udsproxy_dashboard:
	DASH_COPY_1_UID=$(shell curl -s 'localhost:3000/api/search' | jq -r '.[] | select(.id==5) | .uid'); \
	curl -s localhost:3000/api/dashboards/uid/$$DASH_COPY_1_UID | \
		jq '.dashboard|.id=null|.title="uds-proxy stats"|.uid="ups"' \
		> monitoring/grafana/dashboards/uds-proxy.json
	git diff monitoring/grafana/dashboards/uds-proxy.json


clean:
	rm -f $(BINARY) $(TEST_SOCKET) $(TEST_PIDFILE) test_server coverage*

realclean: clean
	-docker-compose down --volumes
	-docker-compose rm -f
	rm -rf $(BINARY)*.zip $(TEST_DOCKERIZED_SOCKET_DIR)
