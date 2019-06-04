
FROM golang:1.12.5 as build-env
WORKDIR /src/github.com/schnoddelbotz/uds-proxy
COPY . .
RUN go get golang.org/x/lint/golint && \
    make test clean uds-proxy CGO_ENABLED=0

FROM alpine
RUN apk add ca-certificates
COPY --from=build-env /src/github.com/schnoddelbotz/uds-proxy/uds-proxy /bin/uds-proxy

ENTRYPOINT []
ENV UDS_PROXY_DOCKER_SOCKET_PATH=/tmp/uds-proxy-docker.sock
ENV UDS_PROXY_DOCKER_PROMETHEUS_PORT=28080
EXPOSE ${UDS_PROXY_DOCKER_PROMETHEUS_PORT}

USER nobody
CMD uds-proxy -socket ${UDS_PROXY_DOCKER_SOCKET_PATH} -prometheus-port :${UDS_PROXY_DOCKER_PROMETHEUS_PORT}
