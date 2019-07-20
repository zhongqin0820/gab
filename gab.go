package main

import (
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"runtime"

	"github.com/zhongqin0820/gab/internal"
)

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
	const MAXSIZE int = 1<<31 - 1
	//
	d := internal.NewDispatcher(conc, num, internal.Min(num, MAXSIZE), internal.Min(num, MAXSIZE), rawurl)
	d.Run()
	log.Printf("QPS=%.2f\n", d.GetQPS())
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
