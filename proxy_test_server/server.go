package proxy_test_server

import (
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type RandomReader struct {
	toRead  int
	written int
}

func RunUpstreamFakeServer(port string, fork bool) *http.Server {
	log.Printf("Starting fake upstream server for functional tests on %s", port)
	srv := &http.Server{Addr: port}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "ROOT-INDEX-OK")
	})

	http.HandleFunc("/slow/", func(w http.ResponseWriter, r *http.Request) {
		params := strings.Split(strings.Replace(r.URL.Path, "/slow/", "", 1), "/")
		if len(params) < 2 {
			http.Error(w, "Bad usage", http.StatusBadRequest)
			return
		}
		code, _ := strconv.Atoi(params[0])
		delay, _ := strconv.Atoi(params[1])
		message := http.StatusText(code)
		if message == "" {
			code = http.StatusBadRequest
			message = http.StatusText(code)
		}
		time.Sleep(time.Duration(delay) * time.Millisecond)
		http.Error(w, message, code)
	})

	http.HandleFunc("/slow/no-response/", func(w http.ResponseWriter, r *http.Request) {
		delay, _ := strconv.Atoi(strings.Replace(r.URL.Path, "/slow/no-response/", "", 1))
		time.Sleep(time.Duration(delay) * time.Millisecond)
		// find a nicer way...?
		panic("Believe I need to panic to create empty response")
	})

	http.HandleFunc("/code/", func(w http.ResponseWriter, r *http.Request) {
		code, _ := strconv.Atoi(strings.Replace(r.URL.Path, "/code/", "", 1))
		message := http.StatusText(code)
		if message == "" {
			code = http.StatusBadRequest
			message = http.StatusText(code)
		}
		http.Error(w, message, code)
	})

	http.HandleFunc("/size/", func(w http.ResponseWriter, r *http.Request) {
		size, _ := strconv.Atoi(strings.Replace(r.URL.Path, "/size/", "", 1))
		w.Header().Set("Content-Length", strconv.Itoa(size))
		io.Copy(w, io.LimitReader(NewRandomReaderWithSizeLimit(size), int64(size)))
	})

	http.HandleFunc("/shutdown", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "Fakeserver says ciao!")
		go func() {
			time.Sleep(250 * time.Millisecond)
			os.Exit(0)
		}()
	})

	if fork {
		go func() {
			if err := srv.ListenAndServe(); err != http.ErrServerClosed {
				log.Fatalf("ListenAndServe(): %s", err)
			}
		}()
		time.Sleep(250 * time.Millisecond)
	} else {
		err := srv.ListenAndServe()
		log.Fatalf("ListenAndServe(): %s", err)
	}

	return srv
}

func NewRandomReaderWithSizeLimit(limit int) *RandomReader {
	return &RandomReader{limit, 0}
}

func (r *RandomReader) Read(p []byte) (n int, err error) {
	bufSize := len(p)
	for i := 0; i < bufSize; i++ {
		p[i] = 'X' // not supposed to be random. just fill bandwidth :)
	}
	r.written = r.written + bufSize
	if r.written >= r.toRead {
		return bufSize, io.EOF
	}
	return bufSize, nil
}
