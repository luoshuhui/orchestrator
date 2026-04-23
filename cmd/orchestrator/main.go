package main

import (
	"log"
	"os"

	"orchestrator/internal/demo"
)

func main() {
	// 如果提供了命令行参数，则视为 JSON 路径
	if len(os.Args) > 1 {
		jsonPath := os.Args[1]
		if err := demo.RunJSONDemo(jsonPath); err != nil {
			log.Fatalf("JSON Demo failed: %v", err)
		}
	} else {
		// 默认运行硬编码的 Demo
		if err := demo.RunDemo(); err != nil {
			log.Fatalf("Demo failed: %v", err)
		}
	}
}
