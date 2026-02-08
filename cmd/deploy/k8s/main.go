package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/aixgo-dev/aixgo/pkg/security"
)

const (
	colorRed    = "\033[0;31m"
	colorGreen  = "\033[0;32m"
	colorYellow = "\033[1;33m"
	colorReset  = "\033[0m"
)

type Config struct {
	ProjectID      string
	Region         string
	Zone           string
	Cluster        string
	Environment    string
	Registry       string
	ImageTag       string
	XAIKey         string
	OpenAIKey      string
	HuggingFaceKey string
	SkipBuild      bool
	SkipSecrets    bool
	SkipDeploy     bool
	SkipTests      bool
	Namespace      string
	Overlay        string
}

func main() {
	cfg := &Config{}

	flag.StringVar(&cfg.ProjectID, "project", os.Getenv("GCP_PROJECT_ID"), "GCP Project ID")
	flag.StringVar(&cfg.Region, "region", getEnvDefault("GCP_REGION", "us-central1"), "GCP Region")
	flag.StringVar(&cfg.Zone, "zone", getEnvDefault("GKE_ZONE", "us-central1"), "GKE Zone")
	flag.StringVar(&cfg.Cluster, "cluster", getEnvDefault("GKE_CLUSTER", "aixgo-cluster"), "GKE Cluster name")
	flag.StringVar(&cfg.Environment, "env", "staging", "Environment (staging, production)")
	flag.StringVar(&cfg.Registry, "registry", "", "Container registry (auto-detected if empty)")
	flag.StringVar(&cfg.ImageTag, "tag", getEnvDefault("IMAGE_TAG", "latest"), "Image tag")
	flag.StringVar(&cfg.XAIKey, "xai-key", os.Getenv("XAI_API_KEY"), "XAI API Key")
	flag.StringVar(&cfg.OpenAIKey, "openai-key", os.Getenv("OPENAI_API_KEY"), "OpenAI API Key")
	flag.StringVar(&cfg.HuggingFaceKey, "huggingface-key", os.Getenv("HUGGINGFACE_API_KEY"), "HuggingFace API Key")
	flag.BoolVar(&cfg.SkipBuild, "skip-build", false, "Skip building and pushing Docker images")
	flag.BoolVar(&cfg.SkipSecrets, "skip-secrets", false, "Skip creating secrets")
	flag.BoolVar(&cfg.SkipDeploy, "skip-deploy", false, "Skip deployment")
	flag.BoolVar(&cfg.SkipTests, "skip-tests", false, "Skip smoke tests")

	flag.Parse()

	// Set defaults based on environment
	if cfg.Environment == "production" {
		cfg.Namespace = "aixgo"
		cfg.Overlay = "production"
	} else {
		cfg.Namespace = "aixgo-staging"
		cfg.Overlay = "staging"
	}

	if cfg.Registry == "" {
		cfg.Registry = fmt.Sprintf("%s-docker.pkg.dev", cfg.Region)
	}

	if cfg.ProjectID == "" {
		logError("Project ID is required. Set -project or GCP_PROJECT_ID environment variable")
		os.Exit(1)
	}

	// Validate deployment inputs to prevent command injection
	// For K8s deployment, we validate critical parameters that are passed to shell commands
	imageName := "orchestrator" // Using a representative image name for validation
	if err := security.ValidateDeploymentInputs(
		cfg.ProjectID,
		cfg.Region,
		cfg.Cluster,      // Using cluster name as service name for validation
		"aixgo",          // Repository name
		imageName,        // Image name
		cfg.Environment,
	); err != nil {
		logError("Invalid deployment configuration: %v", err)
		os.Exit(1)
	}

	// Additional validation for K8s-specific fields
	if err := security.ValidateRegion(cfg.Zone); err != nil {
		logError("Invalid zone: %v", err)
		os.Exit(1)
	}

	// Validate namespace
	if err := security.ValidateNamespace(cfg.Namespace); err != nil {
		logError("Invalid namespace: %v", err)
		os.Exit(1)
	}

	ctx := context.Background()

	logInfo("Starting Kubernetes deployment to %s environment...", cfg.Environment)

	if err := checkPrerequisites(); err != nil {
		logError("Prerequisites check failed: %v", err)
		os.Exit(1)
	}

	if err := authenticateGCP(ctx, cfg); err != nil {
		logError("Failed to authenticate to GCP: %v", err)
		os.Exit(1)
	}

	if err := getClusterCredentials(ctx, cfg); err != nil {
		logError("Failed to get cluster credentials: %v", err)
		os.Exit(1)
	}

	if !cfg.SkipBuild {
		if err := buildAndPushImages(ctx, cfg); err != nil {
			logError("Failed to build and push images: %v", err)
			os.Exit(1)
		}
	}

	if !cfg.SkipSecrets {
		if err := createSecrets(ctx, cfg); err != nil {
			logError("Failed to create secrets: %v", err)
			os.Exit(1)
		}
	}

	if !cfg.SkipDeploy {
		if err := deployToKubernetes(ctx, cfg); err != nil {
			logError("Failed to deploy to Kubernetes: %v", err)
			os.Exit(1)
		}

		if err := waitForRollout(ctx, cfg); err != nil {
			logError("Rollout failed: %v", err)
			os.Exit(1)
		}

		if err := verifyDeployment(ctx, cfg); err != nil {
			logError("Deployment verification failed: %v", err)
			os.Exit(1)
		}

		if !cfg.SkipTests {
			if err := runSmokeTests(ctx, cfg); err != nil {
				logError("Smoke tests failed: %v", err)
				os.Exit(1)
			}
		}
	}

	logInfo("Deployment completed successfully!")
}

func checkPrerequisites() error {
	logInfo("Checking prerequisites...")

	commands := []string{"gcloud", "kubectl", "docker"}
	for _, cmd := range commands {
		if err := checkCommand(cmd); err != nil {
			return fmt.Errorf("%s not found. Please install it first", cmd)
		}
	}

	logInfo("Prerequisites check passed")
	return nil
}

func authenticateGCP(ctx context.Context, cfg *Config) error {
	logInfo("Authenticating to Google Cloud...")

	if err := runCommand("gcloud", "config", "set", "project", cfg.ProjectID); err != nil {
		return err
	}

	// Configure Docker for Artifact Registry
	return runCommand("gcloud", "auth", "configure-docker", cfg.Registry)
}

func getClusterCredentials(ctx context.Context, cfg *Config) error {
	logInfo("Getting GKE credentials...")

	return runCommand("gcloud", "container", "clusters", "get-credentials", cfg.Cluster,
		"--zone="+cfg.Zone,
		"--project="+cfg.ProjectID)
}

func buildAndPushImages(ctx context.Context, cfg *Config) error {
	logInfo("Building and pushing Docker images...")

	images := []struct {
		name       string
		dockerfile string
	}{
		{"orchestrator", "docker/aixgo.Dockerfile"},
		{"mcp-server", "docker/aixgo.Dockerfile"},
	}

	for _, img := range images {
		imageTag := fmt.Sprintf("%s/%s/aixgo/%s:%s", cfg.Registry, cfg.ProjectID, img.name, cfg.ImageTag)
		imageLatest := fmt.Sprintf("%s/%s/aixgo/%s:latest", cfg.Registry, cfg.ProjectID, img.name)

		logInfo("Building %s...", img.name)

		if err := runCommand("docker", "build",
			"--platform", "linux/amd64",
			"-t", imageTag,
			"-t", imageLatest,
			"-f", img.dockerfile,
			"."); err != nil {
			return err
		}

		logInfo("Pushing %s...", img.name)

		if err := runCommand("docker", "push", imageTag); err != nil {
			return err
		}

		if err := runCommand("docker", "push", imageLatest); err != nil {
			return err
		}

		logInfo("Image %s pushed successfully", img.name)
	}

	return nil
}

func createSecrets(ctx context.Context, cfg *Config) error {
	logInfo("Creating Kubernetes secrets...")

	// Create namespace if it doesn't exist
	// SECURITY: Namespace validated at startup
	cmd := exec.Command("kubectl", "get", "namespace", cfg.Namespace) // #nosec G204 -- namespace validated at startup
	if err := cmd.Run(); err != nil {
		logInfo("Creating namespace %s...", cfg.Namespace)
		if err := runCommand("kubectl", "create", "namespace", cfg.Namespace); err != nil {
			return err
		}
	}

	// Check if all required API keys are provided
	if cfg.XAIKey == "" && cfg.OpenAIKey == "" && cfg.HuggingFaceKey == "" {
		logWarn("No API keys provided, skipping secret creation")
		return nil
	}

	// SECURITY: Validate secret name before using in command
	secretName := "api-keys"
	if err := security.ValidateSecretName(secretName); err != nil {
		return fmt.Errorf("invalid secret name: %w", err)
	}

	// Delete existing secret if it exists
	// SECURITY: Secret name and namespace validated above/at startup
	cmd = exec.Command("kubectl", "delete", "secret", secretName, "-n", cfg.Namespace) // #nosec G204 -- inputs validated
	_ = cmd.Run() // Ignore error if secret doesn't exist

	// Build the create secret command
	args := []string{"create", "secret", "generic", secretName, "-n", cfg.Namespace}

	if cfg.XAIKey != "" {
		args = append(args, "--from-literal=xai-api-key="+cfg.XAIKey)
	}
	if cfg.OpenAIKey != "" {
		args = append(args, "--from-literal=openai-api-key="+cfg.OpenAIKey)
	}
	if cfg.HuggingFaceKey != "" {
		args = append(args, "--from-literal=huggingface-api-key="+cfg.HuggingFaceKey)
	}

	if err := runCommand("kubectl", args...); err != nil {
		return err
	}

	logInfo("Secrets created successfully")
	return nil
}

func deployToKubernetes(ctx context.Context, cfg *Config) error {
	logInfo("Deploying to Kubernetes using kustomize overlay: %s", cfg.Overlay)

	overlayPath := fmt.Sprintf("deploy/k8s/overlays/%s", cfg.Overlay)

	// Check if kustomize overlay exists
	if _, err := os.Stat(overlayPath); os.IsNotExist(err) {
		return fmt.Errorf("kustomize overlay not found: %s", overlayPath)
	}

	// Update image tags in kustomization
	if err := updateKustomizeImages(cfg); err != nil {
		return err
	}

	// Apply the kustomization
	return runCommand("kubectl", "apply", "-k", overlayPath)
}

func updateKustomizeImages(cfg *Config) error {
	logInfo("Updating image tags in kustomization...")

	overlayPath := fmt.Sprintf("deploy/k8s/overlays/%s", cfg.Overlay)

	orchestratorImage := fmt.Sprintf("REGION-docker.pkg.dev/PROJECT_ID/aixgo/orchestrator=%s/%s/aixgo/orchestrator:%s",
		cfg.Registry, cfg.ProjectID, cfg.ImageTag)
	mcpServerImage := fmt.Sprintf("REGION-docker.pkg.dev/PROJECT_ID/aixgo/mcp-server=%s/%s/aixgo/mcp-server:%s",
		cfg.Registry, cfg.ProjectID, cfg.ImageTag)

	// Change to overlay directory
	originalDir, err := os.Getwd()
	if err != nil {
		return err
	}
	defer func() { _ = os.Chdir(originalDir) }()

	if err := os.Chdir(overlayPath); err != nil {
		return err
	}

	// Use kustomize to set images
	if err := runCommand("kubectl", "kustomize", "edit", "set", "image",
		orchestratorImage, mcpServerImage); err != nil {
		// If kustomize command fails, try using kubectl kustomize
		logWarn("kustomize edit failed, images may need manual update")
	}

	return os.Chdir(originalDir)
}

func waitForRollout(ctx context.Context, cfg *Config) error {
	logInfo("Waiting for rollout to complete...")

	deployments := []string{"aixgo-orchestrator", "mcp-server"}

	if cfg.Environment == "staging" {
		deployments = []string{"staging-aixgo-orchestrator", "staging-mcp-server"}
	}

	for _, deployment := range deployments {
		logInfo("Waiting for %s rollout...", deployment)
		if err := runCommand("kubectl", "rollout", "status", "deployment/"+deployment,
			"-n", cfg.Namespace, "--timeout=5m"); err != nil {
			return err
		}
	}

	logInfo("Rollout completed successfully")
	return nil
}

func verifyDeployment(ctx context.Context, cfg *Config) error {
	logInfo("Verifying deployment...")

	// Get pods
	logInfo("Pods status:")
	if err := runCommand("kubectl", "get", "pods", "-n", cfg.Namespace); err != nil {
		return err
	}

	// Get services
	logInfo("Services:")
	if err := runCommand("kubectl", "get", "services", "-n", cfg.Namespace); err != nil {
		return err
	}

	logInfo("Deployment verified successfully")
	return nil
}

func runSmokeTests(ctx context.Context, cfg *Config) error {
	logInfo("Running smoke tests...")

	// SECURITY: Validate service name to prevent command injection
	serviceName := "aixgo-service"
	if err := security.ValidateServiceName(serviceName); err != nil {
		return fmt.Errorf("invalid service name: %w", err)
	}

	// Port forward for testing
	// SECURITY: Namespace validated at startup, service name validated above
	portForwardCmd := exec.Command("kubectl", "port-forward", // #nosec G204 -- inputs validated
		"-n", cfg.Namespace,
		"svc/"+serviceName, "8080:8080")

	if err := portForwardCmd.Start(); err != nil {
		return fmt.Errorf("failed to start port forward: %v", err)
	}
	defer func() { _ = portForwardCmd.Process.Kill() }()

	// Wait for port forward to be ready
	time.Sleep(5 * time.Second)

	// Test health endpoints
	// SECURITY: Using hardcoded localhost URLs - safe
	endpoints := []string{
		"http://localhost:8080/health/live",
		"http://localhost:8080/health/ready",
	}

	for _, endpoint := range endpoints {
		logInfo("Testing endpoint: %s", endpoint)
		cmd := exec.Command("curl", "-f", endpoint) // #nosec G204 -- hardcoded localhost URLs
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("health check failed for %s", endpoint)
		}
	}

	logInfo("Smoke tests passed!")
	return nil
}

func checkCommand(name string) error {
	_, err := exec.LookPath(name)
	return err
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func getEnvDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func logInfo(format string, args ...interface{}) {
	log.Printf("%s[INFO]%s %s\n", colorGreen, colorReset, fmt.Sprintf(format, args...))
}

func logWarn(format string, args ...interface{}) {
	log.Printf("%s[WARN]%s %s\n", colorYellow, colorReset, fmt.Sprintf(format, args...))
}

func logError(format string, args ...interface{}) {
	log.Printf("%s[ERROR]%s %s\n", colorRed, colorReset, fmt.Sprintf(format, args...))
}
