package main

import (
	"log"

	"orchestrator/internal/demo"
)

func main() {
	if err := demo.RunDemo(); err != nil {
		log.Fatalf("Demo failed: %v", err)
	}
}
