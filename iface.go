package main

import (
	"github.com/gorilla/mux"
	"github.com/heptiolabs/healthcheck"
)

type Server interface {
	healthcheck.Handler

	App() *mux.Router

	// Init the application
	Init() error

	// Start the application
	Start() error

	// Stop the application
	Stop() error
	//AddLivenessCheck(name string, check healthcheck.Check)
	//AddReadinessCheck(name string, check healthcheck.Check)
}
