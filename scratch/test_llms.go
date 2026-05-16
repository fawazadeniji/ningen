package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"ningen/internal/llm"

	"github.com/joho/godotenv"
)

func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found, using existing environment")
	}

	ctx := context.Background()
	testMessage := []llm.Message{
		{Role: "user", Content: "Hello! Say 'Key works' if you can read this."},
	}

	providers, err := llm.Build()
	if err != nil {
		log.Fatalf("Failed to build providers: %v", err)
	}

	fmt.Println("=== LLM Provider Connectivity Test ===")
	
	for name, provider := range providers {
		fmt.Printf("[%s] Testing... ", name)
		
		start := time.Now()
		resp, err := provider.Complete(ctx, testMessage)
		duration := time.Since(start)

		if err != nil {
			fmt.Printf("FAILED ❌\n Error: %v\n", err)
		} else {
			fmt.Printf("SUCCESS ✅ (%v)\n Response: %q\n", duration.Round(time.Millisecond), resp)
		}
		fmt.Println("---------------------------------------")
	}
}
