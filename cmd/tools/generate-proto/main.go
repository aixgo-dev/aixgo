package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	colorReset  = "\033[0m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
)

var (
	verbose   = flag.Bool("verbose", false, "Enable verbose output")
	dryRun    = flag.Bool("dry-run", false, "Show commands without executing")
	protoFile = flag.String("proto", "proto/mcp/mcp.proto", "Path to proto file")
	install   = flag.Bool("install", true, "Install protoc plugins if missing")
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Generate Go code from protobuf definitions.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExample:\n")
		fmt.Fprintf(os.Stderr, "  %s\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -proto proto/mcp/custom.proto\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -verbose -dry-run\n", os.Args[0])
	}
	flag.Parse()

	if err := run(); err != nil {
		logError("Failed: %v", err)
		os.Exit(1)
	}

	logSuccess("Code generation complete!")
}

func run() error {
	// Get project root
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("finding project root: %w", err)
	}

	// Change to project root
	if err := os.Chdir(projectRoot); err != nil {
		return fmt.Errorf("changing to project root: %w", err)
	}

	logInfo("Working directory: %s", projectRoot)

	// Check prerequisites
	if err := checkPrerequisites(); err != nil {
		return err
	}

	// Install plugins if requested
	if *install {
		if err := installPlugins(); err != nil {
			return err
		}
	}

	// Verify proto file exists
	if _, err := os.Stat(*protoFile); err != nil {
		return fmt.Errorf("proto file not found: %s", *protoFile)
	}

	// Generate Go code
	if err := generateCode(); err != nil {
		return err
	}

	return nil
}

func findProjectRoot() (string, error) {
	// Start from current directory
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Walk up until we find go.mod
	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find project root (no go.mod found)")
		}
		dir = parent
	}
}

func checkPrerequisites() error {
	logInfo("Checking prerequisites...")

	// Check protoc
	if !commandExists("protoc") {
		return fmt.Errorf("protoc not found - please install Protocol Buffers compiler")
	}

	version, err := getProtocVersion()
	if err != nil {
		logWarn("Could not determine protoc version: %v", err)
	} else if *verbose {
		logInfo("Using protoc version: %s", version)
	}

	return nil
}

func commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

func getProtocVersion() (string, error) {
	cmd := exec.Command("protoc", "--version")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func installPlugins() error {
	plugins := map[string]string{
		"protoc-gen-go":      "google.golang.org/protobuf/cmd/protoc-gen-go@latest",
		"protoc-gen-go-grpc": "google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest",
	}

	for plugin, pkg := range plugins {
		if !commandExists(plugin) {
			logInfo("Installing %s...", plugin)
			if err := installGoTool(pkg); err != nil {
				return fmt.Errorf("installing %s: %w", plugin, err)
			}
			logSuccess("%s installed", plugin)
		} else if *verbose {
			logInfo("%s already installed", plugin)
		}
	}

	return nil
}

func installGoTool(pkg string) error {
	if *dryRun {
		logInfo("[DRY-RUN] Would run: go install %s", pkg)
		return nil
	}

	cmd := exec.Command("go", "install", pkg)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if *verbose {
		logInfo("Running: go install %s", pkg)
	}

	return cmd.Run()
}

func generateCode() error {
	logInfo("Generating Go code from protobuf definitions...")

	// Prepare protoc command
	args := []string{
		"--go_out=.",
		"--go_opt=paths=source_relative",
		"--go-grpc_out=.",
		"--go-grpc_opt=paths=source_relative",
		*protoFile,
	}

	if *dryRun {
		logInfo("[DRY-RUN] Would run: protoc %s", strings.Join(args, " "))
		return nil
	}

	cmd := exec.Command("protoc", args...)

	if *verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		logInfo("Running: protoc %s", strings.Join(args, " "))
	} else {
		// Capture output to show only on error
		output, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s", output)
			return fmt.Errorf("protoc failed: %w", err)
		}
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("running protoc: %w", err)
	}

	// Verify generated files
	expectedFiles := []string{
		strings.TrimSuffix(*protoFile, ".proto") + ".pb.go",
		strings.TrimSuffix(*protoFile, ".proto") + "_grpc.pb.go",
	}

	for _, file := range expectedFiles {
		if _, err := os.Stat(file); err != nil {
			logWarn("Expected file not found: %s", file)
		} else if *verbose {
			logSuccess("Generated: %s", file)
		}
	}

	return nil
}

func logInfo(format string, args ...interface{}) {
	fmt.Printf("%s[INFO]%s %s\n", colorGreen, colorReset, fmt.Sprintf(format, args...))
}

func logWarn(format string, args ...interface{}) {
	fmt.Printf("%s[WARN]%s %s\n", colorYellow, colorReset, fmt.Sprintf(format, args...))
}

func logError(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "%s[ERROR]%s %s\n", colorRed, colorReset, fmt.Sprintf(format, args...))
}

func logSuccess(format string, args ...interface{}) {
	fmt.Printf("%s[SUCCESS]%s %s\n", colorGreen, colorReset, fmt.Sprintf(format, args...))
}
