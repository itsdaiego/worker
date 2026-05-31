package model

import (
	"github.com/google/uuid"
)

type Job struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Payload string `json:"payload"`
	Status  string `json:"status"`
}

func (job *Job) EnsureID() {
	if job.ID == "" {
		job.ID = uuid.New().String()
	}
}
