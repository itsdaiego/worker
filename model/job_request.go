package model

type CreateJobRequest struct {
	Type    string `json:"type"`
	Payload string `json:"payload"`
}
