package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	. "challenge/model"
	. "challenge/repository"
)

const (
	createQueueSize = 1000
	createWorkers   = 16
)

type Server struct {
	db     *sqlx.DB
	router *gin.Engine
	pool   *CreatePool
}

func NewServer() (*Server, error) {
	db, err := initDB()
	if err != nil {
		return nil, err
	}

	return &Server{
		db:     db,
		router: gin.Default(),
		pool:   NewCreatePool(NewJobRepository(db), createQueueSize, createWorkers),
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
			if errors.Is(err, sql.ErrNoRows) {
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

		job := Job{
			ID:      uuid.New().String(),
			Type:    jobRequest.Type,
			Payload: jobRequest.Payload,
			Status:  "pending",
		}

		if !s.pool.Enqueue(job) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Queue full"})
			return
		}

		c.JSON(http.StatusAccepted, gin.H{"id": job.ID})
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

	server := &http.Server{
		Addr:    ":8080",
		Handler: s.router,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("shutting down")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("http shutdown: %v", err)
	}

	s.pool.Shutdown()
	log.Println("done")
}
