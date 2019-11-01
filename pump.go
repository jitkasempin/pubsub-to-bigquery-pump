package main

import "time"

// PumpRequest represents pump request
type PumpRequest struct {
	Subscription string `json:"subscription"`
	Table        string `json:"table"`
}

// PumpResult represents the result of pump process
type PumpResult struct {
	ExecutedOn time.Time    `json:"executed_on"`
	Duration   string       `json:"duration"`
	Release    string       `json:"release"`
	EventCount int          `json:"event_count"`
	Request    *PumpRequest `json:"request"`
}

func pump(in *PumpRequest) (out *PumpResult, err error) {

	start := time.Now()

	r := &PumpResult{
		ExecutedOn: start,
		Request:    in,
		Release:    release,
	}

	return r, nil
}
