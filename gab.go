package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"log"
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
	//
	const MAXSIZE int = 1<<31 - 1
	worker := NewWorkerPool(conc, num, min(num, MAXSIZE), min(num, MAXSIZE), rawurl)
	worker.Run()
	log.Printf("QPS=%.2f\n", worker.GetQPS())
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

type Result struct{}

type Job struct {
	id  int
	url string
}

// Process defines and replays the pipeline of httptrace
func (j Job) Process() (*Result, error) {
	log.Printf("start processing job %d\n", j.id)
	req, _ := http.NewRequest("GET", j.url, nil)
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         req.Host,
		},
	}
	tr.TLSNextProto = make(map[string]func(string, *tls.Conn) http.RoundTripper)
	var Timeout int = 20
	client := &http.Client{Transport: tr, Timeout: time.Duration(Timeout) * time.Second}
	// define the trace
	var connStart time.Duration = getNow()
	var connDuration time.Duration
	trace := &httptrace.ClientTrace{
		GotConn: func(connInfo httptrace.GotConnInfo) {
			if !connInfo.Reused {
				connDuration = getNow() - connStart
			}
		},
	}
	// replay the trace
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))
	_, err := client.Do(req)
	log.Printf("end processing job %d, cost %.2f s\n", j.id, connDuration.Seconds())
	if err == nil {
		return &Result{}, err
	}
	return nil, err
}

type WorkerPool struct {
	done         chan struct{}
	jobs         chan Job
	results      chan Result
	numOfWorkers int
	numOfJobs    int
	url          string
	numOfConn    int64
	timeUsed     time.Duration
}

func NewWorkerPool(numOfWorkers, numOfJobs, numOfJobCache, numOfResCache int, url string) *WorkerPool {
	pool := &WorkerPool{
		done:         make(chan struct{}),
		jobs:         make(chan Job, numOfJobCache),
		results:      make(chan Result, numOfResCache),
		numOfWorkers: numOfWorkers,
		numOfJobs:    numOfJobs,
		url:          url,
		numOfConn:    0,
	}
	return pool
}

// AllocateJobs is the producer of Chan Job
func (p *WorkerPool) AllocateJobs() {
	for i := 0; i < p.numOfJobs; i++ {
		p.jobs <- Job{id: i, url: p.url}
	}
	close(p.jobs)
}

// GatherResults is the consumer of Chan Result
func (p *WorkerPool) GatherResults() {
	for result := range p.results {
		_ = result
		p.numOfConn++
	}
	p.done <- struct{}{}
}

// Run defines the pipeline of the pool
func (p *WorkerPool) Run() {
	// starting
	s := getNow()
	// allocate jobs & gathering results
	go p.AllocateJobs()
	go p.GatherResults()
	// worker
	var wg sync.WaitGroup
	for i := 0; i < p.numOfWorkers; i++ {
		wg.Add(1)
		go p.Worker(&wg)
	}
	wg.Wait()
	close(p.results)
	// end
	<-p.done
	e := getNow()
	p.timeUsed = e - s
}

// Worker is the consumer of Chan Job and producer of Chan Result
func (p *WorkerPool) Worker(wg *sync.WaitGroup) {
	defer wg.Done()
	for job := range p.jobs {
		result, err := job.Process()
		if err == nil {
			p.results <- *result
		}
	}
}

// GetQPS returns numOfConn if timeUsed < 1 seconds
func (p *WorkerPool) GetQPS() float64 {
	if p.timeUsed.Seconds() < 1 {
		return float64(p.numOfConn)
	}
	return float64(p.numOfConn) / p.timeUsed.Seconds()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
