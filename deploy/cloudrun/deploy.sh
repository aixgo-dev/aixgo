#!/bin/bash
# Deploy aixgo to Google Cloud Run

set -e

# Configuration
PROJECT_ID="${GCP_PROJECT_ID:-your-project-id}"
REGION="${GCP_REGION:-us-central1}"
SERVICE_NAME="aixgo-mcp"
IMAGE_NAME="mcp-server"
REPOSITORY="aixgo"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."

    if ! command -v gcloud &> /dev/null; then
        log_error "gcloud CLI not found. Please install it first."
        exit 1
    fi

    if ! command -v docker &> /dev/null; then
        log_error "docker not found. Please install it first."
        exit 1
    fi

    log_info "Prerequisites check passed"
}

# Set GCP project
set_project() {
    log_info "Setting GCP project to $PROJECT_ID..."
    gcloud config set project "$PROJECT_ID"
}

# Enable required APIs
enable_apis() {
    log_info "Enabling required GCP APIs..."
    gcloud services enable \
        run.googleapis.com \
        artifactregistry.googleapis.com \
        secretmanager.googleapis.com \
        cloudresourcemanager.googleapis.com
}

# Create Artifact Registry repository
create_repository() {
    log_info "Creating Artifact Registry repository..."

    if gcloud artifacts repositories describe "$REPOSITORY" \
        --location="$REGION" &> /dev/null; then
        log_warn "Repository $REPOSITORY already exists"
    else
        gcloud artifacts repositories create "$REPOSITORY" \
            --repository-format=docker \
            --location="$REGION" \
            --description="Aixgo container images"
        log_info "Repository created successfully"
    fi
}

# Build and push Docker image
build_and_push() {
    log_info "Building Docker image..."

    IMAGE_TAG="${REGION}-docker.pkg.dev/${PROJECT_ID}/${REPOSITORY}/${IMAGE_NAME}:latest"

    # Configure Docker for Artifact Registry
    gcloud auth configure-docker "${REGION}-docker.pkg.dev" --quiet

    # Build multi-platform image
    docker build \
        --platform linux/amd64 \
        -t "$IMAGE_TAG" \
        -f docker/aixgo.Dockerfile \
        .

    log_info "Pushing image to Artifact Registry..."
    docker push "$IMAGE_TAG"

    log_info "Image pushed successfully: $IMAGE_TAG"
}

# Create secrets in Secret Manager
create_secrets() {
    log_info "Setting up secrets..."

    # XAI API Key
    if [ -n "${XAI_API_KEY}" ]; then
        echo -n "$XAI_API_KEY" | gcloud secrets create xai-api-key \
            --data-file=- \
            --replication-policy="automatic" \
            2>/dev/null || \
        echo -n "$XAI_API_KEY" | gcloud secrets versions add xai-api-key \
            --data-file=-
        log_info "XAI API key secret created/updated"
    fi

    # OpenAI API Key
    if [ -n "${OPENAI_API_KEY}" ]; then
        echo -n "$OPENAI_API_KEY" | gcloud secrets create openai-api-key \
            --data-file=- \
            --replication-policy="automatic" \
            2>/dev/null || \
        echo -n "$OPENAI_API_KEY" | gcloud secrets versions add openai-api-key \
            --data-file=-
        log_info "OpenAI API key secret created/updated"
    fi

    # HuggingFace API Key
    if [ -n "${HUGGINGFACE_API_KEY}" ]; then
        echo -n "$HUGGINGFACE_API_KEY" | gcloud secrets create huggingface-api-key \
            --data-file=- \
            --replication-policy="automatic" \
            2>/dev/null || \
        echo -n "$HUGGINGFACE_API_KEY" | gcloud secrets versions add huggingface-api-key \
            --data-file=-
        log_info "HuggingFace API key secret created/updated"
    fi
}

# Create service account
create_service_account() {
    log_info "Creating service account..."

    SERVICE_ACCOUNT="aixgo-mcp@${PROJECT_ID}.iam.gserviceaccount.com"

    if gcloud iam service-accounts describe "$SERVICE_ACCOUNT" &> /dev/null; then
        log_warn "Service account already exists"
    else
        gcloud iam service-accounts create aixgo-mcp \
            --display-name="Aixgo MCP Service Account"
        log_info "Service account created"
    fi

    # Grant permissions
    log_info "Granting IAM permissions..."
    gcloud projects add-iam-policy-binding "$PROJECT_ID" \
        --member="serviceAccount:${SERVICE_ACCOUNT}" \
        --role="roles/secretmanager.secretAccessor"

    gcloud projects add-iam-policy-binding "$PROJECT_ID" \
        --member="serviceAccount:${SERVICE_ACCOUNT}" \
        --role="roles/logging.logWriter"

    gcloud projects add-iam-policy-binding "$PROJECT_ID" \
        --member="serviceAccount:${SERVICE_ACCOUNT}" \
        --role="roles/cloudtrace.agent"
}

# Deploy to Cloud Run
deploy_service() {
    log_info "Deploying to Cloud Run..."

    IMAGE_TAG="${REGION}-docker.pkg.dev/${PROJECT_ID}/${REPOSITORY}/${IMAGE_NAME}:latest"

    gcloud run deploy "$SERVICE_NAME" \
        --image="$IMAGE_TAG" \
        --platform=managed \
        --region="$REGION" \
        --allow-unauthenticated \
        --service-account="aixgo-mcp@${PROJECT_ID}.iam.gserviceaccount.com" \
        --min-instances=0 \
        --max-instances=100 \
        --cpu=2 \
        --memory=2Gi \
        --timeout=300 \
        --concurrency=80 \
        --port=8080 \
        --set-env-vars="PORT=8080,GRPC_PORT=9090,LOG_LEVEL=info,ENVIRONMENT=production" \
        --set-secrets="XAI_API_KEY=xai-api-key:latest,OPENAI_API_KEY=openai-api-key:latest,HUGGINGFACE_API_KEY=huggingface-api-key:latest" \
        --execution-environment=gen2 \
        --cpu-boost

    log_info "Deployment complete!"

    # Get service URL
    SERVICE_URL=$(gcloud run services describe "$SERVICE_NAME" \
        --platform=managed \
        --region="$REGION" \
        --format='value(status.url)')

    log_info "Service URL: $SERVICE_URL"
}

# Test deployment
test_deployment() {
    log_info "Testing deployment..."

    SERVICE_URL=$(gcloud run services describe "$SERVICE_NAME" \
        --platform=managed \
        --region="$REGION" \
        --format='value(status.url)')

    # Test health endpoint
    if curl -sf "${SERVICE_URL}/health/live" > /dev/null; then
        log_info "Health check passed!"
    else
        log_error "Health check failed!"
        exit 1
    fi

    log_info "Deployment test passed!"
}

# Main deployment flow
main() {
    log_info "Starting Cloud Run deployment..."

    check_prerequisites
    set_project
    enable_apis
    create_repository
    create_service_account
    create_secrets
    build_and_push
    deploy_service
    test_deployment

    log_info "Deployment completed successfully!"
}

# Run main function
main "$@"
