package main

import (
	. "challenge/model"
	. "challenge/repository"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

type Worker interface {
	ProcessBatch(batchSize int) (BatchResult, error)
}

type myWorker struct {
	repo JobRepository
}

func NewWorker(repo JobRepository) Worker {
	return &myWorker{repo: repo}
}

func (w *myWorker) ProcessBatch(batchSize int) (BatchResult, error) {
	queryOpts := QueryOptions{BatchSize: batchSize}
	jobs, err := w.repo.GetAllJobs(queryOpts)
	if err != nil {
		return BatchResult{}, err
	}

	if len(jobs) == 0 {
		return BatchResult{}, nil
	}

	jobChan := make(chan Job, len(jobs))

	maxWorkers := 10000
	jobsChunkSize := (len(jobs) + maxWorkers - 1) / maxWorkers

	var wg sync.WaitGroup
	var succeeded, failed atomic.Int32

	for i := 0; i < len(jobs); i += jobsChunkSize {
		end := min(i+jobsChunkSize, len(jobs))
		batch := jobs[i:end]

		wg.Add(1)
		go func(batch []Job) {
			defer wg.Done()
			for _, job := range batch {
				// fake processing time for each job
				// the goal for the project is to implement parallel processing only
				time.Sleep(300 * time.Millisecond)
				jobChan <- job
			}
		}(batch)
	}
	wg.Wait()
	close(jobChan)

	jobIds := make([]string, 0, len(jobChan))

	for job := range jobChan {
		jobIds = append(jobIds, job.ID)
	}

	for i := 0; i < len(jobIds); i += 1000 {
		end := min(i+1000, len(jobIds))
		err := w.repo.BulkUpdateJobStatus(jobIds[i:end], "done")
		if err != nil {
			failed.Add(int32(end - i))
		} else {
			succeeded.Add(int32(end - i))
		}
	}

	return BatchResult{
		Total:     int(succeeded.Load() + failed.Load()),
		Succeeded: int(succeeded.Load()),
		Failed:    int(failed.Load()),
	}, nil
}

type CreatePool struct {
	repo  JobRepository
	queue chan Job
	wg    sync.WaitGroup
}

func NewCreatePool(repo JobRepository, queueSize, workers int) *CreatePool {
	p := &CreatePool{
		repo:  repo,
		queue: make(chan Job, queueSize),
	}
	for i := 0; i < workers; i++ {
		p.wg.Add(1)
		go p.run()
	}
	return p
}

func (p *CreatePool) run() {
	defer p.wg.Done()
	for job := range p.queue {
		if err := p.repo.CreateJob(job); err != nil {
			log.Printf("create job %s failed: %v", job.ID, err)
		}
	}
}

func (p *CreatePool) Enqueue(job Job) bool {
	select {
	case p.queue <- job:
		return true
	default:
		return false
	}
}

func (p *CreatePool) Shutdown() {
	close(p.queue)
	p.wg.Wait()
}
