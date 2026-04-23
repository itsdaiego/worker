package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

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
		var jobs []Job

		jobRepo := NewJobRepository(s.db)

		queryOpts := QueryOptions{
			BatchSize: 10000,
		}

		jobs, err := jobRepo.GetAllJobs(queryOpts)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch jobs"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"jobs": jobs})
	})

	s.router.GET("/jobs/:id", func(c *gin.Context) {
		id := c.Param("id")

		jobRepo := NewJobRepository(s.db)

		intId, err := strconv.Atoi(id)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid job ID"})
			return
		}

		job, err := jobRepo.GetJobById(intId)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch job"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"job": job})
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

		jobRepo := NewJobRepository(s.db)

		createdJob, err := jobRepo.CreateJob(jobRequest)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create job"})
			return
		}

		c.JSON(http.StatusOK, createdJob)
	})

	s.router.POST("/jobs/batch", func(c *gin.Context) {
		batchSize := 10000

		worker := NewWorker(NewJobRepository(s.db))

		result, err := worker.ProcessBatch(batchSize, nil)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process batch"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Batch processing completed",
			"succeeded":  result.Succeeded,
			"failed":     result.Failed,
		})
	})



	if err := s.router.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}
