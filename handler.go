package main

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func healthHandler(c *gin.Context) {
	c.String(http.StatusOK, "OK")
}

func defaultHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"release":      release,
		"request_on":   time.Now(),
		"request_from": c.Request.RemoteAddr,
	})
}

// PumpJob represents pump job configuration s
type PumpJob struct {
	// ID is the unique ID for this job. Will be used in metrics to track the
	// counts across executions
	ID string `json:"id"`
	// Source is the PubSub configuration
	Source struct {
		// Subscription is the name of existing PubSub subscription
		Subscription string `json:"subscription"`
		// MaxStall represents the maximum amount of time (seconds) the service
		// will wait for new messages when the queue has been drained. Should
		// be greater than 5 seconds
		MaxStall int `json:"max_stall"`
	} `json:"source"`
	// is the BigQuery configuration
	Target      JobTarget `json:"target"`
	MaxDuration int       `json:"max_duration"`
}

// JobTarget represents the job target configuration
type JobTarget struct {
	// Dataset is the name of the existing BigQuery dataset
	Dataset string `json:"dataset"`
	// Table is the name of the existing BigQuery dataset table
	Table string `json:"table"`
	// BatchSize is the size of the insert batch every n number of messages the
	// service will insert batch into BigQuery. Should be lesser than the
	// maximum size of BigQuery batch insert limits
	BatchSize int `json:"batch_size"`
	// IgnoreUnknowns indicates whether the service should error when there are
	// fields in your JSON message on PubSun that are not represented in
	// BigQuery table
	IgnoreUnknowns bool `json:"ignore_unknowns"`
}

func pumpHandler(c *gin.Context) {

	// get PumpJob instance from HTTP POST request
	var req PumpJob
	var bindErr error

	format := c.Param("format")
	if format == "yaml" {
		bindErr = c.BindYAML(&req)
	} else {
		bindErr = c.BindJSON(&req)
	}
	if bindErr != nil {
		logger.Printf("error binding request: %v", bindErr)
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Invalid request format",
			"status":  "BadRequest",
		})
		return
	}

	// execute the pump job
	result, err := pump(&req)
	if err != nil {
		logger.Printf("Error on pump exec: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Error processing request, see logs",
			"status":  "InternalServerError",
		})
		return
	}
	logger.Printf("pump result: %v", result)
	c.JSON(http.StatusOK, result)
}
