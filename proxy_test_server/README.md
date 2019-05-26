# proxy_test_server

... is instantiated by [functional tests](../proxy_test/functional_test.go) to 
simulate a backend server for uds-proxy tests.
In a way, it's this project's 'embedded' [mountebank](http://www.mbtest.org).

To build a binary, run `make test_server`.