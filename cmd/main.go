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
		printUsage()
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
		fmt.Println("This should be your OpenRouter API key")
		fmt.Println("Get your API key at: https://openrouter.ai/keys")
		os.Exit(1)
	}

	agent, err := amazon_agent.NewAgent(cfg, apiKey)
	if err != nil {
		fmt.Printf("Error initializing agent: %v\n", err)
		os.Exit(1)
	}
	defer agent.Close()

	fmt.Printf("\n🤖 Advanced Browser Agent Starting...\n")
	fmt.Printf("📋 Task: %s\n\n", taskDescription)
	fmt.Printf("⚙️  Configuration:\n")
	fmt.Printf("   Max Steps: %d\n", cfg.MaxSteps)
	fmt.Printf("   Total Timeout: %v\n", cfg.TotalTimeout)
	fmt.Printf("   Headless: %v\n", cfg.Headless)
	fmt.Printf("   Recovery: %v\n\n", cfg.EnableRecovery)

	fmt.Println("🚀 Starting execution...\n")

	result, err := agent.ExecuteTask(taskDescription)
	if err != nil {
		fmt.Printf("\n❌ Task failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n" + strings.Repeat("=", 60) + "\n")
	if result.Success {
		fmt.Printf("✅ Task completed successfully!\n")
	} else {
		fmt.Printf("⚠️  Task completed with warnings\n")
	}
	fmt.Printf(strings.Repeat("=", 60) + "\n\n")

	fmt.Printf("📊 Execution Summary:\n")
	fmt.Printf("   Steps executed: %d\n", result.StepsExecuted)
	fmt.Printf("   Duration: %v\n", result.Duration)
	
	if result.FinalState != "" {
		fmt.Printf("   Final state: %s\n", result.FinalState)
	}
	
	if result.Error != nil {
		fmt.Printf("   Error: %v\n", result.Error)
	}

	if result.Memory != nil {
		fmt.Printf("\n🧠 Memory Summary:\n")
		fmt.Printf("   Products viewed: %d\n", len(result.Memory.ProductURLs))
		if result.Memory.SelectedProduct != "" {
			fmt.Printf("   Selected product: %s\n", result.Memory.SelectedProduct)
		}
		fmt.Printf("   Cart items: %d\n", len(result.Memory.CartItems))
		if result.Memory.UserCredentials["email"] != "" {
			fmt.Printf("   User authenticated: Yes\n")
		}
	}
	
	fmt.Println()
}

func printUsage() {
	fmt.Println("Advanced Browser Agent - Complex E-commerce Automation")
	fmt.Println("\nUsage: agent run \"<task description>\"")
	fmt.Println("\nExamples:")
	fmt.Println("  Simple:")
	fmt.Println("    agent run \"Go to amazon.in and search for laptops\"")
	fmt.Println("\n  Medium Complexity:")
	fmt.Println("    agent run \"Search for wireless mouse on amazon.in, select the cheapest one and add to cart\"")
	fmt.Println("\n  High Complexity (Full Checkout):")
	fmt.Println("    agent run \"Go to amazon.in, search for headphones, select a product with good ratings, add to cart, and proceed to checkout\"")
	fmt.Println("    agent run \"Buy a smartphone case from amazon.in, add to cart and go to payment screen\"")
	fmt.Println("\nNote: The agent will:")
	fmt.Println("  - Execute 30-50+ steps for complex tasks")
	fmt.Println("  - Handle login when required (will prompt for credentials)")
	fmt.Println("  - Fill address forms (will prompt for details)")
	fmt.Println("  - Stop at payment screen (won't place actual orders)")
	fmt.Println("  - Auto-recover from errors")
	fmt.Println("\nEnvironment Variables:")
	fmt.Println("  GEMINI_API_KEY - Your OpenRouter API key (required)")
}