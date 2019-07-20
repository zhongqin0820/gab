package internal

import (
	"log"
	"time"
)

type DispatcherAdv interface {
	Run()
	GetQPS() float64
}

func NewDispatcherAdv(numOfWorkers, numOfJobs, numOfJobCache, numOfResCache int, url string) DispatcherAdv {
	return NewWorkerPoolAdv(numOfWorkers, numOfJobs, numOfJobCache, numOfResCache, url)
}

type WorkerPoolAdv struct {
	done         chan struct{}
	workers      chan chan Job
	jobs         chan Job
	results      chan Result
	numOfWorkers int
	numOfJobs    int
	url          string
	numOfConn    int64
	timeUsed     time.Duration
}

func NewWorkerPoolAdv(numOfWorkers, numOfJobs, numOfJobCache, numOfResCache int, url string) *WorkerPoolAdv {
	return &WorkerPoolAdv{
		done:         make(chan struct{}),
		workers:      make(chan chan Job, numOfWorkers),
		jobs:         make(chan Job, numOfJobCache),
		results:      make(chan Result, numOfResCache),
		numOfWorkers: numOfWorkers,
		numOfJobs:    numOfJobs,
		url:          url,
		numOfConn:    0,
	}
}

func (p *WorkerPoolAdv) GenerateJobs() {
	for i := 0; i < p.numOfJobs; i++ {
		p.jobs <- Job{id: i, url: p.url}
	}
	close(p.jobs)
}

func (p *WorkerPoolAdv) GatherResults() {
	for result := range p.results {
		_ = result
		p.numOfConn++
	}
	p.done <- struct{}{}
}

func (p *WorkerPoolAdv) Run() {
	// starting
	s := getNow()
	// generate jobs & gathering results
	go p.GenerateJobs()
	go p.GatherResults()
	//
	p.Dispatch()
	//
	close(p.results)
	//
	<-p.done
	e := getNow()
	p.timeUsed = e - s
}

// gets a job from jobs
// gets a jobs chan from the pool
// adds the job to the jobs chan, then send it to the worker
func (p *WorkerPoolAdv) Dispatch() {
	for job := range p.jobs {
		jobs := <-p.workers
		jobs <- job
		go p.Worker(jobs)
	}
}

func (p *WorkerPoolAdv) Worker(jobs chan Job) {
	for {
		select {
		case job := <-jobs:
			result, err := job.Process()
			if err == nil {
				p.results <- *result
			}
		case <-time.After(time.Duration(time.Millisecond)):
			log.Println(".....")
			return
		}
		p.workers <- jobs
	}
}

// GetQPS returns numOfConn if timeUsed < 1 seconds
func (p *WorkerPoolAdv) GetQPS() float64 {
	if p.timeUsed.Seconds() < 1 {
		return float64(p.numOfConn)
	}
	return float64(p.numOfConn) / p.timeUsed.Seconds()
}
