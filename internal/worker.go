package internal

import (
	"sync"
	"time"
)

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
	return &WorkerPool{
		done:         make(chan struct{}),
		jobs:         make(chan Job, numOfJobCache),
		results:      make(chan Result, numOfResCache),
		numOfWorkers: numOfWorkers,
		numOfJobs:    numOfJobs,
		url:          url,
		numOfConn:    0,
	}
}

// AllocateJobs is the producer of Chan Job
func (p *WorkerPool) GenerateJobs() {
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
	// generate jobs & gathering results
	go p.GenerateJobs()
	go p.GatherResults()
	// dispatch
	go p.Dispatch()
	// end
	<-p.done
	e := getNow()
	p.timeUsed = e - s
}

// Dispatch jobs to worker
func (p *WorkerPool) Dispatch() {
	var wg sync.WaitGroup
	for i := 0; i < p.numOfWorkers; i++ {
		wg.Add(1)
		// worker process jobs
		go p.Worker(&wg)
	}
	wg.Wait()
	close(p.results)
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
