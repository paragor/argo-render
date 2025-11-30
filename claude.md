# argo-render

ArgoCD Config Management Plugin for rendering Kubernetes manifests with Terraform state integration.

## Project Rules

- All files in this project are small - read them entirely when needed
- Write documentation in English
- Write code and comments in English
- Use minimal comments - only where truly necessary
- Do not create README.md or other documentation files unless explicitly requested

## Overview

This plugin provides a pipeline for rendering Kubernetes manifests:

1. **Pre-render templating** - Template `values.yaml` files using gomplate with `@<<` `>>@` markers
2. **Helm rendering** (optional) - Render helm chart to output file
3. **Kustomize build** (required) - Build manifests using kustomize
4. **Post-render interpolation** - Interpolate `@<<` `>>@` blocks in final manifests

All templating uses `@<<` and `>>@` as delimiters (instead of standard `{{` `}}`).

## app.yaml Specification

The `app.yaml` file is the main configuration file placed in the application root directory.

```yaml
# app.yaml

helm:
  # Helm chart configuration (optional section)
  # Chart source - local path OR remote chart
  # Option 1: Local chart
  chart: ./charts/my-app

  # Option 2: Remote chart
  # chart: ingress-nginx
  # repo: https://kubernetes.github.io/ingress-nginx
  # version: 4.0.0

  # Release name (optional, defaults to ARGOCD_APP_NAME env var)
  # releaseName: my-release

  # Namespace for helm template (optional, defaults to ARGOCD_APP_NAMESPACE env var)
  # namespace: default

  # Values files (processed in order)
  # Files are templated with gomplate before being passed to helm
  values:
    - values.yaml
    - /shared/values-common.yaml

  # Output file path for rendered helm template
  # This file will be included in kustomize resources
  output: ./base/helm-output.yaml

kustomize:
  # Path to kustomization directory
  # Kustomize is always required
  path: ./overlays/production

  # Files with suffix .tmpl.yaml or .tmpl.yml in kustomize directory
  # are pre-processed as gomplate templates before kustomize build

# Post-render interpolation is optional (enabled via -enable-post-render flag)
# Use @<< and >>@ markers in final manifests for late-binding values
```

### Path Resolution

All paths in `app.yaml` (`helm.chart`, `helm.values`, `helm.output`, `kustomize.path`) support two modes:
- `/path/to/file` — absolute path from repository root (where `.git` is located)
- `path/to/file` or `./path/to/file` — relative path from app.yaml directory

## Processing Pipeline

### Step 1: Pre-render Templating (values.yaml)

Values files specified in `helm.values` are processed through gomplate before helm templating.

**Datasource:** Terraform outputs are available via `terraform` datasource using full S3 path.

```yaml
# values.yaml (before templating)
database:
  host: '@<< (datasource "terraform" "my-bucket/terraform/infra/terraform.tfstate").rds_endpoint >>@'
  port: 5432

cluster:
  name: '@<< (datasource "terraform" "my-bucket/terraform/infra/terraform.tfstate").cluster_name >>@'
```

The datasource argument is the full S3 path: `bucket-name/path/to/terraform.tfstate`.

### Step 2: Helm Rendering (optional)

If `helm` section is specified, the chart is rendered using `helm template`:

- Local charts are used directly from the specified path
- Remote charts are fetched from the repository
- Templated values files are passed in order
- Output is written to the file specified in `helm.output`

### Step 3: Kustomize Processing (required)

Kustomize is always executed:

1. Files matching `*.tmpl.yaml` or `*.tmpl.yml` in the kustomize directory are templated with gomplate (using `@<<` `>>@` markers)
2. `kustomize build` is executed on the directory

If helm is used, include the helm output file in kustomization.yaml resources.

### Step 4: Post-render Interpolation (optional)

When `-enable-post-render` flag is set, final manifests are scanned for `@<<` and `>>@` markers. Content between markers is evaluated as gomplate expressions.

```yaml
# Final manifest with interpolation markers
apiVersion: v1
kind: Secret
metadata:
  name: db-credentials
stringData:
  password: '@<< (datasource "terraform" "my-bucket/terraform/infra/terraform.tfstate").db_password >>@'
```

This allows injecting sensitive values at the last stage without exposing them in intermediate files.

## Templating Functions

All gomplate functions are available: https://docs.gomplate.ca/functions/

### Terraform Datasource

Access Terraform state outputs using full S3 path:

```
@<< (datasource "terraform" "bucket/path/to/terraform.tfstate").<output-name> >>@
```

Example with nested outputs:

```yaml
# Terraform output: kubeconfig_data.endpoint
endpoint: '@<< (datasource "terraform" "my-bucket/terraform/infra/terraform.tfstate").kubeconfig_data.endpoint >>@'
```

### File Datasource

Read JSON or YAML files. Supported extensions: `.json`, `.yaml`, `.yml`.

Path resolution:
- `/path/to/file.json` — absolute path from repository root (where `.git` is located)
- `path/to/file.json` — relative path from current working directory

```
@<< (datasource "file" "/path/to/file.json").<key> >>@
@<< (datasource "file" "relative/file.yaml").<key> >>@
```

Examples:

```yaml
# Absolute path from repo root: /config/settings.json
replicas: '@<< (datasource "file" "/config/settings.json").app.replicas >>@'

# Relative path from current directory: ./local/config.yaml
port: '@<< (datasource "file" "local/config.yaml").port >>@'
```

## Directory Structure Example

```
my-app/
├── app.yaml                    # Main configuration
├── values.yaml                 # Helm values (templated)
├── values-production.yaml      # Additional values (templated)
├── charts/
│   └── my-app/                 # Local helm chart
│       ├── Chart.yaml
│       ├── values.yaml
│       └── templates/
├── base/
│   └── helm-output.yaml        # Generated by helm (specified in helm.output)
└── overlays/
    └── production/
        ├── kustomization.yaml
        ├── patch.yaml
        └── secrets.tmpl.yaml   # Templated before kustomize
```

Example kustomization.yaml with helm output:
```yaml
resources:
  - ../../base/helm-output.yaml
patches:
  - path: patch.yaml
```

## CLI Usage

```bash
argo-render -config ./app.yaml                                        # basic usage
argo-render -config ./app.yaml -enable-terraform                      # with terraform datasource
argo-render -config ./app.yaml -enable-terraform -enable-post-render  # full pipeline
```

### Flags

- `-config` - path to app.yaml (default: "app.yaml")
- `-enable-terraform` - enable terraform datasource (requires AWS credentials)
- `-enable-post-render` - enable post-render template processing for late-binding values

## ArgoCD Environment Variables

When running in ArgoCD, these environment variables are used as defaults:

- `ARGOCD_APP_NAME` → `helm.releaseName`
- `ARGOCD_APP_NAMESPACE` → `helm.namespace`

## Execution Modes

### Kustomize Only

```yaml
kustomize:
  path: ./base
```

### Helm + Kustomize

```yaml
helm:
  chart: ./chart
  # releaseName and namespace from ARGOCD_APP_NAME / ARGOCD_APP_NAMESPACE
  values:
    - values.yaml
  output: ./base/helm-output.yaml

kustomize:
  path: ./overlays/production
```

## Template Syntax Notes

Template markers can be used with or without quotes depending on YAML context:

```yaml
# With quotes (for strings)
host: '@<< (datasource "file" "config.json").host >>@'

# Without quotes (for numbers, when value is purely the template)
replicas: @<< (datasource "file" "config.json").replicas >>@
port: @<< (datasource "file" "config.json").port >>@
```

## Architecture

### Work Directory

The pipeline copies the entire git repository (excluding `.git` directory) to a temporary directory before processing. This ensures source files are never modified. The pipeline then uses `os.Chdir` to the app directory in the temp copy, making all relative paths in `app.yaml` work correctly.

### Code Structure

```
cmd/argo-render/main.go    - CLI entry point with flag parsing
pkg/config/config.go       - app.yaml configuration parsing
pkg/template/engine.go     - gomplate wrapper with @<< >>@ delimiters
pkg/helm/renderer.go       - helm template execution (uses ARGOCD_* env vars)
pkg/kustomize/builder.go   - kustomize build with .tmpl.yaml preprocessing
pkg/pipeline/pipeline.go   - orchestrates the full rendering pipeline
pkg/kv/terraform.go        - S3 terraform state fetcher
pkg/kv/file.go             - file datasource for JSON/YAML files
pkg/kv/store.go            - datasource interface
```

### Template Engine

Uses Go's `text/template` with sprig functions. Custom delimiters `@<<` and `>>@` avoid conflicts with helm templating (`{{` `}}`). Datasources are registered via `engine.RegisterDatasource(name, datasource)` interface.
