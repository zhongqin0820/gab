package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewWorkerPoolAdv(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "from test")
	}
	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()

	d := internal.NewDispatcherAdv(3, 10, 20, 20, server.URL)
	d.Run()
	log.Printf("QPS=%.2f\n", d.GetQPS())
}
