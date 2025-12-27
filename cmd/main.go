package main

import (
	"fmt"
	"os"
	"strings"

	"browser-agent/internal/amazon_agent"
	"browser-agent/internal/config"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: agent run \"<task description>\"")
		fmt.Println("\nExample:")
		fmt.Println("  agent run \"Go to amazon.com and search for laptops\"")
		os.Exit(1)
	}

	command := os.Args[1]
	if command != "run" {
		fmt.Printf("Unknown command: %s\n", command)
		fmt.Println("Only 'run' command is supported")
		os.Exit(1)
	}

	taskDescription := strings.Join(os.Args[2:], " ")
	if taskDescription == "" {
		fmt.Println("Error: Task description cannot be empty")
		os.Exit(1)
	}

	cfg := config.NewConfig()
	
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		fmt.Println("Error: GEMINI_API_KEY environment variable not set")
		fmt.Println("Get your free API key at: https://ai.google.dev")
		os.Exit(1)
	}

	agent, err := amazon_agent.NewAgent(cfg, apiKey)
	if err != nil {
		fmt.Printf("Error initializing agent: %v\n", err)
		os.Exit(1)
	}
	defer agent.Close()

	fmt.Printf("\n🤖 Starting task: %s\n\n", taskDescription)

	result, err := agent.ExecuteTask(taskDescription)
	if err != nil {
		fmt.Printf("\n❌ Task failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n✅ Task completed successfully!\n")
	fmt.Printf("\nSummary:\n")
	fmt.Printf("  Steps executed: %d\n", result.StepsExecuted)
	fmt.Printf("  Duration: %v\n", result.Duration)
	if result.FinalState != "" {
		fmt.Printf("  Final state: %s\n", result.FinalState)
	}
	fmt.Println()
}