//go:build ignore

// generate.go - Generate Go code from protobuf definitions
// Run with: go run generate.go
package main

import (
	"fmt"
	"os"
	"os/exec"
)

func main() {
	// Install protoc plugins if not already installed
	if _, err := exec.LookPath("protoc-gen-go"); err != nil {
		fmt.Println("Installing protoc-gen-go...")
		cmd := exec.Command("go", "install", "google.golang.org/protobuf/cmd/protoc-gen-go@latest")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to install protoc-gen-go: %v\n", err)
			os.Exit(1)
		}
	}

	if _, err := exec.LookPath("protoc-gen-go-grpc"); err != nil {
		fmt.Println("Installing protoc-gen-go-grpc...")
		cmd := exec.Command("go", "install", "google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to install protoc-gen-go-grpc: %v\n", err)
			os.Exit(1)
		}
	}

	// Generate Go code
	fmt.Println("Generating Go code from protobuf definitions...")
	cmd := exec.Command("protoc",
		"--go_out=.", "--go_opt=paths=source_relative",
		"--go-grpc_out=.", "--go-grpc_opt=paths=source_relative",
		"proto/mcp/mcp.proto")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate code: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Code generation complete!")
}
