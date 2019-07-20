package internal

import (
	"crypto/tls"
	"log"
	"net/http"
	"net/http/httptrace"
	"time"
)

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
