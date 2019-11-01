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

// PumpRequest represents pump request
type PumpRequest struct {
	Subscription string `json:"subscription"`
	Table        string `json:"table"`
	MaxMessages  int    `json:"max_messages"`
	MaxSeconds   int    `json:"max_seconds"`
}

func pumpHandler(c *gin.Context) {

	var req PumpRequest
	err := c.BindJSON(&req)
	if err != nil {
		logger.Printf("error binding request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Invalid request format",
			"status":  "BadRequest",
		})
		return
	}

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
