package repository

import (
	. "challenge/model"

	"gorm.io/gorm"
)


type JobRepository interface {
	GetAllJobs() ([]Job, error)
	GetJobById(id int) (Job, error)
	CreateJob(jobRequest CreateJobRequest) (Job, error)
}

type jobRepository struct {
	db *gorm.DB
}

func NewJobRepository(db *gorm.DB) JobRepository {
	return &jobRepository{db: db}
}

func (r *jobRepository) GetAllJobs() ([]Job, error) {
	var jobs []Job
	result := r.db.Find(&jobs)
	return jobs, result.Error
}

func (r *jobRepository) GetJobById(id int) (Job, error) {
	var job Job

	result := r.db.First(&job, id)
	return job, result.Error
}

func (r *jobRepository) CreateJob(jobRequest CreateJobRequest) (Job, error) {
	job := Job{
		Type:       jobRequest.Type,
		Payload:    jobRequest.Payload,
		Status:     "pending",
	}

	result := r.db.Create(&job)

	return job, result.Error
}
