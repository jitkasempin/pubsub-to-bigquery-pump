package main

import (
	"log"
	"net"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/mchmarny/gcputil/env"
	"github.com/mchmarny/gcputil/project"
)

var (
	logger    = log.New(os.Stdout, "[PUMP] ", 0)
	port      = env.MustGetEnvVar("PORT", "8080")
	release   = env.MustGetEnvVar("RELEASE", "v0.0.1-default")
	projectID = project.GetIDOrFail()
)

func main() {

	gin.SetMode(gin.ReleaseMode)

	// router
	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	// simple routes
	r.GET("/", defaultHandler)
	r.GET("/health", healthHandler)

	// api
	v1 := r.Group("/v1")
	{
		v1.POST("/pump", pumpHandler)
		v1.POST("/pump/:format", pumpHandler)
	}

	// server
	hostPort := net.JoinHostPort("0.0.0.0", port)
	logger.Printf("Server starting: %s \n", hostPort)
	if err := r.Run(hostPort); err != nil {
		logger.Fatal(err)
	}
}
