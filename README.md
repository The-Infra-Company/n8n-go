# n8n-go
Go client for n8n API

## Usage

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	n8n "github.com/The-Infra-Company/n8n-go"
)

func main() {
	// Read credentials from env
	apiKey := os.Getenv("N8N_API_KEY")
	baseURL := os.Getenv("N8N_API_URL")
	if apiKey == "" || baseURL == "" {
		log.Fatal("N8N_API_KEY and N8N_API_URL must be set")
	}

	// Create the client
	client := n8n.NewClient(apiKey, baseURL)

	// Fetch all users
	ctx := context.Background()
	users, err := client.GetAllUsers(ctx)
	if err != nil {
		log.Fatalf("failed to list users: %v", err)
	}

	// Print the result
	for _, u := range users {
		fmt.Printf("%s (%s)\n", u.Name, u.ID)
	}
}
```
