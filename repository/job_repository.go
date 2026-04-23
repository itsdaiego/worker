package repository

import (
	. "challenge/model"

	"gorm.io/gorm"
)


type JobRepository interface {
	GetAllJobs(queryOpts QueryOptions) ([]Job, error)
	GetJobById(id int) (Job, error)
	CreateJob(jobRequest CreateJobRequest) (Job, error)
	UpdateJobStatus(id string, status string) error
}

type QueryOptions struct {
	BatchSize int
}

type jobRepository struct {
	db *gorm.DB
}

func NewJobRepository(db *gorm.DB) JobRepository {
	return &jobRepository{db: db}
}

func (r *jobRepository,) GetAllJobs(queryOpts QueryOptions) ([]Job, error) {
	var jobs []Job


	if queryOpts.BatchSize > 0 {
		result := r.db.Where("status = ?", "pending").Limit(queryOpts.BatchSize).Find(&jobs)
		return jobs, result.Error
	}

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

func (r *jobRepository) UpdateJobStatus(id string, status string) error {
	result := r.db.Model(&Job{}).Where("id = ?", id).Update("status", status)
	return result.Error
}
