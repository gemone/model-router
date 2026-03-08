package main

import (
	"github.com/gemone/model-router/internal/cli"
	"github.com/joho/godotenv"

	// Import adapters to register them
	_ "github.com/gemone/model-router/internal/adapter"
	_ "github.com/gemone/model-router/internal/adapter/anthropic"
	_ "github.com/gemone/model-router/internal/adapter/deepseek"
	_ "github.com/gemone/model-router/internal/adapter/ollama"
	_ "github.com/gemone/model-router/internal/adapter/openai_compatible"
)

func main() {
	// Load .env file (optional - ignore errors if file doesn't exist)
	_ = godotenv.Load()

	// Execute the CLI
	cli.Execute()
}
