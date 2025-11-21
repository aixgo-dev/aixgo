package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
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
	ServiceName    string
	ImageName      string
	Repository     string
	Environment    string
	XAIKey         string
	OpenAIKey      string
	HuggingFaceKey string
	SkipBuild      bool
	SkipSecrets    bool
	SkipDeploy     bool
	AllowUnauth    bool
	Verbose        bool
	DryRun         bool
	MinInstances   int
	MaxInstances   int
	CPU            int
	Memory         string
	Timeout        int
	Concurrency    int
}

func main() {
	cfg := &Config{}

	flag.StringVar(&cfg.ProjectID, "project", os.Getenv("GCP_PROJECT_ID"), "GCP Project ID")
	flag.StringVar(&cfg.Region, "region", getEnvDefault("GCP_REGION", "us-central1"), "GCP Region")
	flag.StringVar(&cfg.ServiceName, "service", "aixgo-mcp", "Cloud Run service name")
	flag.StringVar(&cfg.ImageName, "image", "mcp-server", "Image name")
	flag.StringVar(&cfg.Repository, "repository", "aixgo", "Artifact Registry repository")
	flag.StringVar(&cfg.Environment, "env", "production", "Environment (staging, production)")
	flag.StringVar(&cfg.XAIKey, "xai-key", os.Getenv("XAI_API_KEY"), "XAI API Key")
	flag.StringVar(&cfg.OpenAIKey, "openai-key", os.Getenv("OPENAI_API_KEY"), "OpenAI API Key")
	flag.StringVar(&cfg.HuggingFaceKey, "huggingface-key", os.Getenv("HUGGINGFACE_API_KEY"), "HuggingFace API Key")
	flag.BoolVar(&cfg.SkipBuild, "skip-build", false, "Skip building and pushing Docker image")
	flag.BoolVar(&cfg.SkipSecrets, "skip-secrets", false, "Skip creating secrets")
	flag.BoolVar(&cfg.SkipDeploy, "skip-deploy", false, "Skip deployment")
	flag.BoolVar(&cfg.AllowUnauth, "allow-unauthenticated", true, "Allow unauthenticated access")
	flag.BoolVar(&cfg.Verbose, "verbose", false, "Enable verbose output")
	flag.BoolVar(&cfg.DryRun, "dry-run", false, "Show commands without executing")
	flag.IntVar(&cfg.MinInstances, "min-instances", 0, "Minimum number of instances")
	flag.IntVar(&cfg.MaxInstances, "max-instances", 100, "Maximum number of instances")
	flag.IntVar(&cfg.CPU, "cpu", 2, "Number of CPUs")
	flag.StringVar(&cfg.Memory, "memory", "2Gi", "Memory allocation")
	flag.IntVar(&cfg.Timeout, "timeout", 300, "Request timeout in seconds")
	flag.IntVar(&cfg.Concurrency, "concurrency", 80, "Maximum concurrent requests per instance")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Deploy aixgo to Google Cloud Run.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nEnvironment Variables:\n")
		fmt.Fprintf(os.Stderr, "  GCP_PROJECT_ID      - GCP project ID\n")
		fmt.Fprintf(os.Stderr, "  GCP_REGION          - GCP region (default: us-central1)\n")
		fmt.Fprintf(os.Stderr, "  XAI_API_KEY         - xAI API key\n")
		fmt.Fprintf(os.Stderr, "  OPENAI_API_KEY      - OpenAI API key\n")
		fmt.Fprintf(os.Stderr, "  HUGGINGFACE_API_KEY - HuggingFace API key\n")
		fmt.Fprintf(os.Stderr, "\nExample:\n")
		fmt.Fprintf(os.Stderr, "  %s -project my-project\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -project my-project -skip-secrets\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -verbose -dry-run\n", os.Args[0])
	}

	flag.Parse()

	if cfg.ProjectID == "" {
		logError("Project ID is required. Set -project or GCP_PROJECT_ID environment variable")
		os.Exit(1)
	}

	ctx := context.Background()

	logInfo("Starting Cloud Run deployment...")
	if cfg.DryRun {
		logWarn("DRY-RUN MODE: No changes will be made")
	}

	if err := checkPrerequisites(); err != nil {
		logError("Prerequisites check failed: %v", err)
		os.Exit(1)
	}

	if err := setProjectWithConfig(ctx, cfg, cfg.ProjectID); err != nil {
		logError("Failed to set project: %v", err)
		os.Exit(1)
	}

	if err := enableAPIs(ctx); err != nil {
		logError("Failed to enable APIs: %v", err)
		os.Exit(1)
	}

	if err := createRepository(ctx, cfg); err != nil {
		logError("Failed to create repository: %v", err)
		os.Exit(1)
	}

	if err := createServiceAccount(ctx, cfg); err != nil {
		logError("Failed to create service account: %v", err)
		os.Exit(1)
	}

	if !cfg.SkipSecrets {
		if err := createSecrets(ctx, cfg); err != nil {
			logError("Failed to create secrets: %v", err)
			os.Exit(1)
		}
	}

	if !cfg.SkipBuild {
		if err := buildAndPush(ctx, cfg); err != nil {
			logError("Failed to build and push image: %v", err)
			os.Exit(1)
		}
	}

	if !cfg.SkipDeploy {
		if err := deployService(ctx, cfg); err != nil {
			logError("Failed to deploy service: %v", err)
			os.Exit(1)
		}

		if err := testDeployment(ctx, cfg); err != nil {
			logError("Deployment test failed: %v", err)
			os.Exit(1)
		}
	}

	logInfo("Deployment completed successfully!")
}

func checkPrerequisites() error {
	logInfo("Checking prerequisites...")

	if err := checkCommand("gcloud"); err != nil {
		return fmt.Errorf("gcloud CLI not found. Please install it first")
	}

	if err := checkCommand("docker"); err != nil {
		return fmt.Errorf("docker not found. Please install it first")
	}

	logInfo("Prerequisites check passed")
	return nil
}


func setProjectWithConfig(ctx context.Context, cfg *Config, projectID string) error {
	logInfo("Setting GCP project to %s...", projectID)
	return runCommandWithConfig(cfg, "gcloud", "config", "set", "project", projectID)
}

func enableAPIs(ctx context.Context) error {
	logInfo("Enabling required GCP APIs...")
	return runCommand("gcloud", "services", "enable",
		"run.googleapis.com",
		"artifactregistry.googleapis.com",
		"secretmanager.googleapis.com",
		"cloudresourcemanager.googleapis.com")
}

func createRepository(ctx context.Context, cfg *Config) error {
	logInfo("Creating Artifact Registry repository...")

	// Check if repository exists
	cmd := exec.Command("gcloud", "artifacts", "repositories", "describe", cfg.Repository,
		"--location="+cfg.Region)
	if err := cmd.Run(); err == nil {
		logWarn("Repository %s already exists", cfg.Repository)
		return nil
	}

	return runCommand("gcloud", "artifacts", "repositories", "create", cfg.Repository,
		"--repository-format=docker",
		"--location="+cfg.Region,
		"--description=Aixgo container images")
}

func createServiceAccount(ctx context.Context, cfg *Config) error {
	logInfo("Creating service account...")

	serviceAccount := fmt.Sprintf("aixgo-mcp@%s.iam.gserviceaccount.com", cfg.ProjectID)

	// Check if service account exists
	cmd := exec.Command("gcloud", "iam", "service-accounts", "describe", serviceAccount)
	if err := cmd.Run(); err == nil {
		logWarn("Service account already exists")
	} else {
		if err := runCommand("gcloud", "iam", "service-accounts", "create", "aixgo-mcp",
			"--display-name=Aixgo MCP Service Account"); err != nil {
			return err
		}
		logInfo("Service account created")
	}

	// Grant permissions
	logInfo("Granting IAM permissions...")
	roles := []string{
		"roles/secretmanager.secretAccessor",
		"roles/logging.logWriter",
		"roles/cloudtrace.agent",
	}

	for _, role := range roles {
		if err := runCommand("gcloud", "projects", "add-iam-policy-binding", cfg.ProjectID,
			"--member=serviceAccount:"+serviceAccount,
			"--role="+role,
			"--condition=None"); err != nil {
			logWarn("Failed to grant role %s: %v", role, err)
		}
	}

	return nil
}

func createSecrets(ctx context.Context, cfg *Config) error {
	logInfo("Setting up secrets...")

	secrets := map[string]string{
		"xai-api-key":         cfg.XAIKey,
		"openai-api-key":      cfg.OpenAIKey,
		"huggingface-api-key": cfg.HuggingFaceKey,
	}

	for name, value := range secrets {
		if value == "" {
			logWarn("Skipping secret %s (not provided)", name)
			continue
		}

		// Try to create secret
		cmd := exec.Command("gcloud", "secrets", "create", name,
			"--replication-policy=automatic")
		cmd.Stdin = strings.NewReader(value)

		if err := cmd.Run(); err != nil {
			// Secret might exist, try to add a new version
			cmd = exec.Command("gcloud", "secrets", "versions", "add", name,
				"--data-file=-")
			cmd.Stdin = strings.NewReader(value)
			if err := cmd.Run(); err != nil {
				logWarn("Failed to create/update secret %s: %v", name, err)
				continue
			}
		}
		logInfo("Secret %s created/updated", name)
	}

	return nil
}

func buildAndPush(ctx context.Context, cfg *Config) error {
	logInfo("Building Docker image...")

	imageTag := fmt.Sprintf("%s-docker.pkg.dev/%s/%s/%s:latest",
		cfg.Region, cfg.ProjectID, cfg.Repository, cfg.ImageName)

	// Configure Docker for Artifact Registry
	if err := runCommand("gcloud", "auth", "configure-docker",
		fmt.Sprintf("%s-docker.pkg.dev", cfg.Region), "--quiet"); err != nil {
		return err
	}

	// Build image
	if err := runCommand("docker", "build",
		"--platform", "linux/amd64",
		"-t", imageTag,
		"-f", "docker/aixgo.Dockerfile",
		"."); err != nil {
		return err
	}

	logInfo("Pushing image to Artifact Registry...")
	if err := runCommand("docker", "push", imageTag); err != nil {
		return err
	}

	logInfo("Image pushed successfully: %s", imageTag)
	return nil
}

func deployService(ctx context.Context, cfg *Config) error {
	logInfo("Deploying to Cloud Run...")

	imageTag := fmt.Sprintf("%s-docker.pkg.dev/%s/%s/%s:latest",
		cfg.Region, cfg.ProjectID, cfg.Repository, cfg.ImageName)

	args := []string{
		"run", "deploy", cfg.ServiceName,
		"--image=" + imageTag,
		"--platform=managed",
		"--region=" + cfg.Region,
		"--service-account=aixgo-mcp@" + cfg.ProjectID + ".iam.gserviceaccount.com",
		fmt.Sprintf("--min-instances=%d", cfg.MinInstances),
		fmt.Sprintf("--max-instances=%d", cfg.MaxInstances),
		fmt.Sprintf("--cpu=%d", cfg.CPU),
		"--memory=" + cfg.Memory,
		fmt.Sprintf("--timeout=%d", cfg.Timeout),
		fmt.Sprintf("--concurrency=%d", cfg.Concurrency),
		"--port=8080",
		"--set-env-vars=PORT=8080,GRPC_PORT=9090,LOG_LEVEL=info,ENVIRONMENT=" + cfg.Environment,
		"--set-secrets=XAI_API_KEY=xai-api-key:latest,OPENAI_API_KEY=openai-api-key:latest,HUGGINGFACE_API_KEY=huggingface-api-key:latest",
		"--execution-environment=gen2",
		"--cpu-boost",
	}

	if cfg.AllowUnauth {
		args = append(args, "--allow-unauthenticated")
	} else {
		args = append(args, "--no-allow-unauthenticated")
	}

	if err := runCommand("gcloud", args...); err != nil {
		return err
	}

	logInfo("Deployment complete!")

	// Get service URL
	cmd := exec.Command("gcloud", "run", "services", "describe", cfg.ServiceName,
		"--platform=managed",
		"--region="+cfg.Region,
		"--format=value(status.url)")
	output, err := cmd.Output()
	if err != nil {
		return err
	}

	serviceURL := strings.TrimSpace(string(output))
	logInfo("Service URL: %s", serviceURL)

	return nil
}

func testDeployment(ctx context.Context, cfg *Config) error {
	logInfo("Testing deployment...")

	// Get service URL
	cmd := exec.Command("gcloud", "run", "services", "describe", cfg.ServiceName,
		"--platform=managed",
		"--region="+cfg.Region,
		"--format=value(status.url)")
	output, err := cmd.Output()
	if err != nil {
		return err
	}

	serviceURL := strings.TrimSpace(string(output)) + "/health/live"

	// Test health endpoint
	if err := runCommand("curl", "-sf", serviceURL); err != nil {
		return fmt.Errorf("health check failed")
	}

	logInfo("Health check passed!")
	logInfo("Deployment test passed!")
	return nil
}

func checkCommand(name string) error {
	_, err := exec.LookPath(name)
	return err
}

func runCommand(name string, args ...string) error {
	return runCommandWithConfig(nil, name, args...)
}

func runCommandWithConfig(cfg *Config, name string, args ...string) error {
	if cfg != nil && cfg.DryRun {
		logInfo("[DRY-RUN] Would run: %s %s", name, strings.Join(args, " "))
		return nil
	}

	cmd := exec.Command(name, args...)

	if cfg != nil && cfg.Verbose {
		logInfo("Running: %s %s", name, strings.Join(args, " "))
	}

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
