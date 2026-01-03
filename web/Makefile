.PHONY: dev serve build clean lint lint-md lint-html lint-install

# Load environment variables from .env if it exists
ifneq (,$(wildcard ./.env))
    include .env
    export
endif

# Development server with fast render disabled for accurate previews
dev:
	hugo server --disableFastRender

# Alias for dev
serve: dev

# Build the site for production
build:
	hugo --minify --environment production

# Clean generated files
clean:
	rm -rf public/ resources/

# =============================================================================
# Linting
# =============================================================================

# Install linting dependencies (run once)
lint-install:
	npm install -g markdownlint-cli2 htmlhint

# Lint all (markdown + HTML if built)
lint: lint-md
	@if [ -d "public" ]; then \
		$(MAKE) lint-html; \
	else \
		echo "Tip: Run 'make build' first, then 'make lint-html' to lint generated HTML"; \
	fi

# Lint Markdown files in content directory
lint-md:
	@echo "Linting Markdown files..."
	markdownlint-cli2 "content/**/*.md"

# Lint generated HTML (requires build first)
lint-html:
	@echo "Linting HTML files..."
	htmlhint "public/**/*.html" --config .htmlhintrc
