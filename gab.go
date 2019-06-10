package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"time"
)

var startTime = time.Now()

func getNow() time.Duration {
	return time.Since(startTime)
}

var (
	cpus = flag.Int("cpus", runtime.GOMAXPROCS(-1), "Number of supported cpu cores to perform")
	n    = flag.Int("n", 20, "Total number of requests to perform")
	c    = flag.Int("c", 5, "Number of multiple requests to make at a time")
)

var usage = `Usage: gab [options...] <url>

Options:
  -cpus Number of used cpu cores to perform.(Default for current machine is %d.)
  -n Total number of requests to perform.(Default is 200.)
  -c Concurrent number of multiple requests to make at a time.(Default is 50.)
`

func main() {
	flag.Usage = func() {
		fmt.Fprint(os.Stderr, fmt.Sprintf(usage, runtime.NumCPU()))
	}
	flag.Parse()
	if flag.NArg() < 1 {
		printUsageAndExit("")
	}
	// setup configuration
	runtime.GOMAXPROCS(*cpus)
	var num, conc int = *n, *c
	rawurl := flag.Args()[0]
	validOptions(conc, num)
	u, err := url.Parse(rawurl)
	if err != nil || u.Scheme != "http" {
		printUsageAndExit("url should be valid.")
	}
	fmt.Printf("cpus=%d,n=%d,c=%d,url=%s\n", *cpus, num, conc, rawurl)
	// quit from os event
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	go func() {
		<-sig
		os.Exit(130)
	}()
	// mock running the job
	Run(*cpus, num, conc, rawurl)
	//
	SequencialRun(num, rawurl)
}

func validOptions(conc, num int) {
	if num <= 0 || conc <= 0 {
		printUsageAndExit("-c and -n cannot be smaller than 1.")
	}
	if num < conc {
		printUsageAndExit("-c should be bigger than -n.")
	}
}

func printUsageAndExit(msg string) {
	if msg != "" {
		fmt.Fprintf(os.Stderr, msg)
		fmt.Fprintf(os.Stderr, "\n\n")
	}
	flag.Usage()
	fmt.Fprintf(os.Stderr, "\n")
	os.Exit(1)
}

//
type result struct{}

func doRequest(c *http.Client, url string, ch chan<- result) {
	// start of each request
	// s := getNow()
	var dnsStart, connStart, resStart, reqStart, delayStart time.Duration
	var dnsDuration, connDuration, reqDuration, delayDuration time.Duration //resDuration,
	req, _ := http.NewRequest("GET", url, nil)
	// do the request
	trace := &httptrace.ClientTrace{
		DNSStart: func(info httptrace.DNSStartInfo) {
			dnsStart = getNow()
		},
		DNSDone: func(dnsInfo httptrace.DNSDoneInfo) {
			dnsDuration = getNow() - dnsStart
		},
		GetConn: func(h string) {
			connStart = getNow()
		},
		GotConn: func(connInfo httptrace.GotConnInfo) {
			if !connInfo.Reused {
				connDuration = getNow() - connStart
			}
			reqStart = getNow()
		},
		WroteRequest: func(w httptrace.WroteRequestInfo) {
			reqDuration = getNow() - reqStart
			delayStart = getNow()
		},
		GotFirstResponseByte: func() {
			delayDuration = getNow() - delayStart
			resStart = getNow()
		},
	}
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))
	resp, err := c.Do(req)
	// end of each request
	if err == nil {
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}
	// need to calculate the result duration etc.
	// t := getNow()
	// send result
	if err == nil {
		ch <- result{}
	}
}

func dispatchRequests(c *http.Client, url string, ch chan<- result, n int) {
	for i := 0; i < n; i++ {
		select {
		default:
			doRequest(c, url, ch)
		}
	}

}

func Run(cpus, n, c int, url string) (res float64) {
	// init the required
	req, _ := http.NewRequest("GET", url, nil)
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         req.Host,
		},
	}
	tr.TLSNextProto = make(map[string]func(string, *tls.Conn) http.RoundTripper)
	var Timeout int = 20
	client := &http.Client{Transport: tr, Timeout: time.Duration(Timeout) * time.Second}
	const MAXSIZE int = 10000000
	ch := make(chan result, MAXSIZE)
	var wg sync.WaitGroup
	wg.Add(c)
	// start
	s := getNow()
	// during
	for i := 0; i < c; i++ {
		go func() {
			defer wg.Done()
			dispatchRequests(client, url, ch, n/c)
		}()
	}
	wg.Wait()
	// end
	total := getNow() - s
	close(ch)
	var cnt int = len(ch)
	fmt.Printf("total requests=%d, total seconds=%.2f\n", cnt, total.Seconds())
	res = float64(cnt) / total.Seconds()
	fmt.Printf("QPS=%.1f\n", res)
	return
}

func SequencialRun(n int, url string) (res float64) {
	// init the required
	req, _ := http.NewRequest("GET", url, nil)
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         req.Host,
		},
	}
	tr.TLSNextProto = make(map[string]func(string, *tls.Conn) http.RoundTripper)
	var Timeout int = 20
	client := &http.Client{Transport: tr, Timeout: time.Duration(Timeout) * time.Second}
	const MAXSIZE int = 10000000
	ch := make(chan result, MAXSIZE)
	s := getNow()
	// start
	dispatchRequests(client, url, ch, n)
	// end
	total := getNow() - s
	close(ch)
	var cnt int = len(ch)
	fmt.Printf("total requests=%d, total seconds=%.2f\n", cnt, total.Seconds())
	res = float64(cnt) / total.Seconds()
	fmt.Printf("QPS=%.1f\n", res)
	return
}
