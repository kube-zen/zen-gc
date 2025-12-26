# Helm Repository Setup

This repository hosts a Helm chart repository via GitHub Pages, served from the `main` branch.

## Repository URL

The Helm repository is available at:
```
https://kube-zen.github.io/zen-gc
```

## Setup Instructions

### Initial Setup (One-time)

1. **Enable GitHub Pages** in repository settings:
   - Go to Settings → Pages
   - Source: Deploy from a branch
   - Branch: `main` / `/docs`
   - Save

2. **The GitHub Actions workflow** will automatically:
   - Package the Helm chart when changes are pushed to `main` or on releases
   - Generate/update `index.yaml` in `docs/`
   - Commit and push to the `main` branch
   - GitHub Pages will serve the repository from the `docs/` folder

### Manual Setup (if needed)

If you need to set up the repository manually:

```bash
# Package the chart
make helm-package

# Create docs directory (if it doesn't exist)
mkdir -p docs

# Copy packaged chart
cp .helm-packages/*.tgz docs/

# Generate index
cd docs
helm repo index . --url https://kube-zen.github.io/zen-gc
cd ..

# Commit and push
git add docs/*.tgz docs/index.yaml
git commit -m "chore: Initial Helm repository setup"
git push origin main
```

## Using the Repository

Users can add and use the repository:

```bash
# Add repository
helm repo add zen-gc https://kube-zen.github.io/zen-gc
helm repo update

# Install chart (specify version for now)
helm install gc-controller zen-gc/gc-controller --version 0.0.1-alpha --namespace gc-system --create-namespace

# Note: Once multiple versions are available, you can install without --version to use latest:
# helm install gc-controller zen-gc/gc-controller --namespace gc-system --create-namespace
```

## Local Development

To test packaging locally:

```bash
# Lint chart
make helm-lint

# Test chart rendering
make helm-test

# Package chart
make helm-package

# Generate repository index
make helm-repo-index

# Or run all Helm tasks
make helm-all
```

## Workflow

The GitHub Actions workflow (`.github/workflows/publish-helm-chart.yml`) automatically:
- Triggers on pushes to `main` branch (when `charts/` changes)
- Triggers on releases
- Can be manually triggered via `workflow_dispatch`
- Packages the chart
- Updates the `gh-pages` branch with new chart packages and index

## Troubleshooting

### GitHub Pages not working

1. Check that GitHub Pages is enabled in repository settings (should be set to `main` branch, `/docs` folder)
2. Verify `docs/` directory exists and contains `index.yaml`
3. Check GitHub Actions workflow runs successfully
4. Wait a few minutes for GitHub Pages to update

### Chart not appearing

1. Verify the workflow ran successfully
2. Check `docs/` directory contains the packaged `.tgz` file
3. Verify `docs/index.yaml` includes the chart entry
4. Run `helm repo update` on the client side
5. Verify GitHub Pages is configured to serve from `/docs` folder

