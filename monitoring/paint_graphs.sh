#!/bin/sh -e

# run `make monitoring_test` in another shell before running this...
# watch -n0.5 'ps aux|grep curl|egrep -v "(watch|grep)"'

[ -n "$1" ] || exit 1
TEST_SOCKET="$1"

[ -n "$2" ] || exit 1
FAKES_BASEURL="$2"

CURL_WRAP=""
if [ "$3" = "use-sudo" ]; then
  CURL_WRAP="sudo -u nobody"
fi


while ! test -S "${TEST_SOCKET}"; do echo Waiting for ${TEST_SOCKET} ...; sleep 5; done

paintCurl() {
  $CURL_WRAP curl -s --unix-socket "${TEST_SOCKET}" -o /dev/null "${FAKES_BASEURL}/${1}"
}

echo
echo "Firing some requests through uds-proxy socket towards test server in 15s...!"
echo "Watch Grafana / http://localhost:3000/ while waiting for completion."
echo
sleep 15

paintCurl "code/301" &
paintCurl "code/404" &
paintCurl "code/201" &

# fixme: bring back https endpoint perf tests
# requests via uds-proxy; requires sudo to switch to nonody account ... ugly. fix? -set-uid option?
# python -m timeit -n 1 -r 10 -s 'import os' 'os.system("curl --unix-socket $(TEST_SOCKET) -s -o /dev/null http://hacker.ch/")'
# requests without proxy:
# sudo -u nobody python -m timeit -n 1 -r 10 -s 'import os' 'os.system("curl -Lso /dev/null http://hacker.ch/")'

for t in $(seq 0 500 5000); do
  paintCurl "slow/200/$t" &
done

paintCurl "code/301"
sleep 5
paintCurl "code/301"
sleep 5
paintCurl "code/302"

sleep 10

# default timeout is 5000, so trigger some 504s as well
for t in $(seq 0 100 5100); do
  paintCurl "slow/200/$t" &
  paintCurl "slow/200/$t" &
  paintCurl "slow/404/$t" &
  # sleep 5-$t ?
  sleep 5
done

echo "Waiting for background curls to complete..."
wait

sleep 5
echo
echo "Test graph painting completed. Feel free to fire further requests against socket, e.g."
echo "  time $CURL_WRAP curl -v --unix-socket ${TEST_SOCKET} ${FAKES_BASEURL}/slow/200/1234"
echo "  time $CURL_WRAP curl -v --unix-socket ${TEST_SOCKET} -o /dev/null ${FAKES_BASEURL}/size/25000"
echo "  time $CURL_WRAP curl -v --unix-socket ${TEST_SOCKET} http://example.com/"
echo
