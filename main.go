package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	. "challenge/model"
	. "challenge/repository"
)

type Server struct {
	db     *gorm.DB
	router *gin.Engine
}

func NewServer() (*Server, error) {
	db, err := initDB()
	if err != nil {
		return nil, err
	}

	return &Server{
		db:     db,
		router: gin.Default(),
	}, nil
}

func main() {
	s, err := NewServer()
	if err != nil {
		log.Fatal(err)
	}

	s.router.GET("/jobs", func(c *gin.Context) {
		jobRepo := NewJobRepository(s.db)

		jobs, err := jobRepo.GetAllJobs(QueryOptions{})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch jobs"})
			return
		}

		c.JSON(http.StatusOK, jobs)
	})

	s.router.GET("/jobs/:id", func(c *gin.Context) {
		id := c.Param("id")
		jobRepo := NewJobRepository(s.db)

		job, err := jobRepo.GetJobById(id)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch job"})
			return
		}

		c.JSON(http.StatusOK, job)
	})

	s.router.POST("/jobs", func(c *gin.Context) {
		var jobRequest CreateJobRequest
		jsonData, err := c.GetRawData()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		err = json.Unmarshal(jsonData, &jobRequest)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse job data"})
			return
		}

		validTypes := map[string]bool{"send_email": true, "resize_image": true, "generate_report": true}
		if !validTypes[jobRequest.Type] {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Invalid job type"})
			return
		}
		if len(jobRequest.Payload) == 0 {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Payload cannot be empty"})
			return
		}
		if len(jobRequest.Payload) > 500 {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Payload exceeds 500 characters"})
			return
		}

		jobRepo := NewJobRepository(s.db)

		createdJob, err := jobRepo.CreateJob(jobRequest)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create job"})
			return
		}

		c.JSON(http.StatusCreated, createdJob)
	})

	s.router.POST("/jobs/batch", func(c *gin.Context) {
		worker := NewWorker(NewJobRepository(s.db))

		result, err := worker.ProcessBatch(10000)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process batch"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"total":     result.Total,
			"succeeded": result.Succeeded,
			"failed":    result.Failed,
		})
	})

	if err := s.router.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}
