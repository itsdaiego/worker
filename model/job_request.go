package model

type CreateJobRequest struct {
	Type    string `json:"type"`
	Payload string `json:"payload"`
}


type JobSummaryResults struct {
	Succeeded int32
	Failed    int32
}
