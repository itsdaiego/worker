package model

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Job struct {
	ID      string `json:"id" gorm:"type:uuid;primaryKey"`
	Type    string `json:"type"`
	Payload string `json:"payload"`
	Status  string `json:"status"`
}

func (job *Job) BeforeCreate(tx *gorm.DB) error {
	job.ID = uuid.New().String()
	return nil
}
