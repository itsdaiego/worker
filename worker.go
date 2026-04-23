package main

import (
	. "challenge/model"
	. "challenge/repository"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)


type Worker interface {
	ProcessBatch(batchSize int, jobChan chan Job) (JobSummaryResults, error)
}

type myWorker struct {
	repo JobRepository
}


func NewWorker(repo JobRepository) Worker {
	return &myWorker{repo:repo}
}

func (w *myWorker) ProcessBatch(batchSize int, jobChan chan Job) (JobSummaryResults, error) {
	queryOpts := QueryOptions{BatchSize: batchSize}
	jobs, err := w.repo.GetAllJobs(queryOpts)
	if err != nil {
		return JobSummaryResults{}, err
	}

	maxWorkers := 10

	jobsChunkSize :=  (len(jobs) + maxWorkers - 1) / maxWorkers

	var wg sync.WaitGroup
	var succeeded, failed atomic.Int32

	for i := 0; i < len(jobs); i += jobsChunkSize {
		jobsBatch := jobs[i:i+jobsChunkSize]

		wg.Add(1)
		go func(jobsBatch []Job) {
			defer wg.Done()
			for _, job := range jobsBatch {
				randChange := rand.Float64()
				if randChange > 0.7 {
					succeeded.Add(1)
					job.Status = "succeeded"
				} else {
					failed.Add(1)
					job.Status = "failed"
				}

				time.Sleep(300)

				if err != nil {
					failed.Add(1)
					continue
				}

				if err := w.repo.UpdateJobStatus(job.ID, job.Status); err != nil {
					failed.Add(1)
				} else {
					succeeded.Add(1)
				}
			}
		}(jobsBatch)
	}
	wg.Wait()

	fmt.Printf("Batch processing completed. Succeeded: %d, Failed: %d\n", succeeded.Load(), failed.Load())

	return JobSummaryResults{
		Succeeded: succeeded.Load(),
		Failed:    failed.Load(),
	}, nil
}
