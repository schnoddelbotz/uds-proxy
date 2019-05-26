
FROM golang:1.12.5 as build-env
WORKDIR /src/github.com/schnoddelbotz/uds-proxy
COPY . .
RUN make clean test_server CGO_ENABLED=0

FROM alpine
COPY --from=build-env /src/github.com/schnoddelbotz/uds-proxy/test_server /bin/test_server

ENTRYPOINT []
ENV TEST_SERVER_PORT=25777
EXPOSE ${TEST_SERVER_PORT}

USER nobody
CMD test_server -port :${TEST_SERVER_PORT}
