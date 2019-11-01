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

func pumpHandler(c *gin.Context) {

	subArg := c.Param("sub")
	logger.Printf("sub == %s", subArg)
	if subArg == "" {
		logger.Println("Null subscription parameter")
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Null subscription parameter",
			"status":  "BadRequest",
		})
		return
	}

	tableArg := c.Param("table")
	logger.Printf("table == %s", tableArg)
	if tableArg == "" {
		logger.Println("Null table parameter")
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Null table parameter",
			"status":  "BadRequest",
		})
		return
	}

	request := &PumpRequest{
		Subscription: subArg,
		Table:        tableArg,
	}

	result, err := pump(request)
	if err != nil {
		logger.Printf("Error on pump exec: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Error processing request, see logs",
			"status":  "InternalServerError",
		})
		return
	}

	c.JSON(http.StatusOK, result)
}
