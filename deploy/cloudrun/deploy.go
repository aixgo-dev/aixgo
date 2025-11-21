// deploy.go - Deploy aixgo to Google Cloud Run
// Run with: go run deploy.go
package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

// ANSI colors
const (
	red    = "\033[0;31m"
	green  = "\033[0;32m"
	yellow = "\033[1;33m"
	reset  = "\033[0m"
)

// Configuration
var (
	projectID   = getEnv("GCP_PROJECT_ID", "your-project-id")
	region      = getEnv("GCP_REGION", "us-central1")
	serviceName = "aixgo-mcp"
	imageName   = "mcp-server"
	repository  = "aixgo"
)

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func logInfo(msg string)  { fmt.Printf("%s[INFO]%s %s\n", green, reset, msg) }
func logWarn(msg string)  { fmt.Printf("%s[WARN]%s %s\n", yellow, reset, msg) }
func logError(msg string) { fmt.Printf("%s[ERROR]%s %s\n", red, reset, msg) }

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runQuiet(name string, args ...string) error {
	return exec.Command(name, args...).Run()
}

func runOutput(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).Output()
	return strings.TrimSpace(string(out)), err
}

func checkPrerequisites() {
	logInfo("Checking prerequisites...")
	if _, err := exec.LookPath("gcloud"); err != nil {
		logError("gcloud CLI not found. Please install it first.")
		os.Exit(1)
	}
	if _, err := exec.LookPath("docker"); err != nil {
		logError("docker not found. Please install it first.")
		os.Exit(1)
	}
	logInfo("Prerequisites check passed")
}

func setProject() {
	logInfo(fmt.Sprintf("Setting GCP project to %s...", projectID))
	if err := run("gcloud", "config", "set", "project", projectID); err != nil {
		logError("Failed to set project")
		os.Exit(1)
	}
}

func enableAPIs() {
	logInfo("Enabling required GCP APIs...")
	if err := run("gcloud", "services", "enable",
		"run.googleapis.com",
		"artifactregistry.googleapis.com",
		"secretmanager.googleapis.com",
		"cloudresourcemanager.googleapis.com"); err != nil {
		logError("Failed to enable APIs")
		os.Exit(1)
	}
}

func createRepository() {
	logInfo("Creating Artifact Registry repository...")
	if err := runQuiet("gcloud", "artifacts", "repositories", "describe", repository, "--location="+region); err == nil {
		logWarn(fmt.Sprintf("Repository %s already exists", repository))
		return
	}
	if err := run("gcloud", "artifacts", "repositories", "create", repository,
		"--repository-format=docker",
		"--location="+region,
		"--description=Aixgo container images"); err != nil {
		logError("Failed to create repository")
		os.Exit(1)
	}
	logInfo("Repository created successfully")
}

func buildAndPush() {
	logInfo("Building Docker image...")
	imageTag := fmt.Sprintf("%s-docker.pkg.dev/%s/%s/%s:latest", region, projectID, repository, imageName)

	if err := run("gcloud", "auth", "configure-docker", region+"-docker.pkg.dev", "--quiet"); err != nil {
		logError("Failed to configure Docker")
		os.Exit(1)
	}

	if err := run("docker", "build", "--platform", "linux/amd64", "-t", imageTag, "-f", "docker/aixgo.Dockerfile", "."); err != nil {
		logError("Failed to build image")
		os.Exit(1)
	}

	logInfo("Pushing image to Artifact Registry...")
	if err := run("docker", "push", imageTag); err != nil {
		logError("Failed to push image")
		os.Exit(1)
	}
	logInfo(fmt.Sprintf("Image pushed successfully: %s", imageTag))
}

func createSecret(name, value string) {
	if value == "" {
		return
	}
	cmd := exec.Command("gcloud", "secrets", "create", name, "--data-file=-", "--replication-policy=automatic")
	cmd.Stdin = strings.NewReader(value)
	if err := cmd.Run(); err != nil {
		cmd = exec.Command("gcloud", "secrets", "versions", "add", name, "--data-file=-")
		cmd.Stdin = strings.NewReader(value)
		_ = cmd.Run()
	}
	logInfo(fmt.Sprintf("%s secret created/updated", name))
}

func createSecrets() {
	logInfo("Setting up secrets...")
	createSecret("xai-api-key", os.Getenv("XAI_API_KEY"))
	createSecret("openai-api-key", os.Getenv("OPENAI_API_KEY"))
	createSecret("huggingface-api-key", os.Getenv("HUGGINGFACE_API_KEY"))
}

func createServiceAccount() {
	logInfo("Creating service account...")
	serviceAccount := fmt.Sprintf("aixgo-mcp@%s.iam.gserviceaccount.com", projectID)

	if err := runQuiet("gcloud", "iam", "service-accounts", "describe", serviceAccount); err == nil {
		logWarn("Service account already exists")
	} else {
		if err := run("gcloud", "iam", "service-accounts", "create", "aixgo-mcp", "--display-name=Aixgo MCP Service Account"); err != nil {
			logError("Failed to create service account")
			os.Exit(1)
		}
		logInfo("Service account created")
	}

	logInfo("Granting IAM permissions...")
	roles := []string{"roles/secretmanager.secretAccessor", "roles/logging.logWriter", "roles/cloudtrace.agent"}
	for _, role := range roles {
		_ = run("gcloud", "projects", "add-iam-policy-binding", projectID,
			"--member=serviceAccount:"+serviceAccount, "--role="+role)
	}
}

func deployService() {
	logInfo("Deploying to Cloud Run...")
	imageTag := fmt.Sprintf("%s-docker.pkg.dev/%s/%s/%s:latest", region, projectID, repository, imageName)
	serviceAccount := fmt.Sprintf("aixgo-mcp@%s.iam.gserviceaccount.com", projectID)

	if err := run("gcloud", "run", "deploy", serviceName,
		"--image="+imageTag,
		"--platform=managed",
		"--region="+region,
		"--allow-unauthenticated",
		"--service-account="+serviceAccount,
		"--min-instances=0",
		"--max-instances=100",
		"--cpu=2",
		"--memory=2Gi",
		"--timeout=300",
		"--concurrency=80",
		"--port=8080",
		"--set-env-vars=PORT=8080,GRPC_PORT=9090,LOG_LEVEL=info,ENVIRONMENT=production",
		"--set-secrets=XAI_API_KEY=xai-api-key:latest,OPENAI_API_KEY=openai-api-key:latest,HUGGINGFACE_API_KEY=huggingface-api-key:latest",
		"--execution-environment=gen2",
		"--cpu-boost"); err != nil {
		logError("Failed to deploy")
		os.Exit(1)
	}
	logInfo("Deployment complete!")

	if url, err := runOutput("gcloud", "run", "services", "describe", serviceName,
		"--platform=managed", "--region="+region, "--format=value(status.url)"); err == nil {
		logInfo(fmt.Sprintf("Service URL: %s", url))
	}
}

func testDeployment() {
	logInfo("Testing deployment...")
	url, err := runOutput("gcloud", "run", "services", "describe", serviceName,
		"--platform=managed", "--region="+region, "--format=value(status.url)")
	if err != nil {
		logError("Failed to get service URL")
		os.Exit(1)
	}

	resp, err := http.Get(url + "/health/live")
	if err != nil || resp.StatusCode != 200 {
		logError("Health check failed!")
		os.Exit(1)
	}
	_ = resp.Body.Close()
	logInfo("Health check passed!")
	logInfo("Deployment test passed!")
}

func main() {
	logInfo("Starting Cloud Run deployment...")
	checkPrerequisites()
	setProject()
	enableAPIs()
	createRepository()
	createServiceAccount()
	createSecrets()
	buildAndPush()
	deployService()
	testDeployment()
	logInfo("Deployment completed successfully!")
}
