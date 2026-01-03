# Aixgo Website

Hugo-based static website for [aixgo.dev](https://aixgo.dev).

## Development

```bash
# Install Hugo (macOS)
brew install hugo

# Install linting tools (one-time)
make lint-install

# Start development server
make dev
# Access at http://localhost:1313/

# Build for production
make build

# Lint content
make lint
```

## Structure

- `content/` - Markdown content (guides, blog, examples)
- `layouts/` - Hugo templates and shortcodes
- `static/` - Static assets (CSS, JS, images)
- `data/` - YAML data files (features.yaml, milestones.yaml)
- `config/` - Hugo configuration

## Deployment

Automatically deployed via Google Cloud Build on push to main:

1. Cloud Build detects changes in `web/**`
2. Builds Hugo site with `--minify`
3. Deploys to Firebase Hosting

Manual deployment:

```bash
make build
firebase deploy --only hosting --project aixgo-dev
```

## Content Management

### Adding/Updating Features

Edit `data/features.yaml` to update the feature matrix on the Features page.

### Creating Guides

1. Create `content/guides/my-guide.md`
2. Add front matter (title, description, weight)
3. Write content in Markdown

### Blog Posts

1. Create `content/blog/my-post.md`
2. Add front matter with date and tags
3. Write content

## Related

- [Main Aixgo Documentation](../docs/)
- [Production Examples](../examples/)
