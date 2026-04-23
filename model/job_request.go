package model

type CreateJobRequest struct {
	Type    string `json:"type"`
	Payload string `json:"payload"`
}


type BatchResult struct {
	Total     int
	Succeeded int
	Failed    int
}
