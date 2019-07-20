package internal

type Dispatcher interface {
	Run()
	GetQPS() float64
}

func NewDispatcher(numOfWorkers, numOfJobs, numOfJobCache, numOfResCache int, url string) Dispatcher {
	return NewWorkerPool(numOfWorkers, numOfJobs, numOfJobCache, numOfResCache, url)
}
