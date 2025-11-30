# argo-render

ArgoCD Config Management Plugin for rendering Kubernetes manifests with Terraform state integration.

## Features

- Helm chart rendering (local or remote)
- Kustomize build with template preprocessing
- Terraform outputs from S3 state files
- File datasource (JSON/YAML)
- Post-render interpolation for sensitive values

## Installation

```bash
go install github.com/paragor/argo-render@latest
```

Requirements:
- `helm` in PATH (if using helm)
- `kustomize` in PATH
- AWS credentials (if using `-enable-terraform`)

## Usage

```bash
argo-render -config ./app.yaml
argo-render -config ./app.yaml -enable-terraform
argo-render -config ./app.yaml -enable-terraform -enable-post-render
```

Flags:
- `-config` — path to app.yaml (default: `app.yaml`)
- `-enable-terraform` — enable terraform datasource
- `-enable-post-render` — enable post-render interpolation

Output is written to stdout.

## Configuration

Create `app.yaml` in your application directory:

```yaml
helm:
  chart: ./charts/my-app
  # Remote chart:
  # chart: ingress-nginx
  # repo: https://kubernetes.github.io/ingress-nginx
  # version: 4.0.0
  values:
    - values.yaml
    - /shared/values-common.yaml   # absolute from repo root
  output: ./base/helm-output.yaml

kustomize:
  path: ./overlays/production
```

### Path Resolution

All paths in `app.yaml` support two modes:
- `/path` — absolute from repository root (`.git` location)
- `path` or `./path` — relative from app.yaml directory

### ArgoCD Environment Variables

- `ARGOCD_APP_NAME` → `helm.releaseName` (if not set)
- `ARGOCD_APP_NAMESPACE` → `helm.namespace` (if not set)

## Templating

All templating uses `@<<` and `>>@` delimiters.

### Terraform Datasource

```yaml
host: '@<< (datasource "terraform" "bucket/path/terraform.tfstate").rds_endpoint >>@'
```

Path format: `bucket-name/key/path/terraform.tfstate`

### File Datasource

Read JSON or YAML files. Supported extensions: `.json`, `.yaml`, `.yml`.

```yaml
# Absolute from repo root
replicas: '@<< (datasource "file" "/config/settings.yaml").app.replicas >>@'

# Relative from current directory
port: '@<< (datasource "file" "local/config.json").port >>@'
```

### Template Files

Files with `.tmpl.yaml` or `.tmpl.yml` suffix in kustomize directory are templated before build.

### Post-render Interpolation

With `-enable-post-render`, markers in final output are interpolated:

```yaml
apiVersion: v1
kind: Secret
stringData:
  password: '@<< (datasource "terraform" "bucket/infra/terraform.tfstate").db_password >>@'
```

## Pipeline

1. Template `values.yaml` files (if helm configured)
2. Run `helm template` (if helm configured)
3. Template `.tmpl.yaml` files in kustomize directory
4. Run `kustomize build`
5. Post-render interpolation (if `-enable-post-render`)

Source files are never modified — all operations run in a temp directory.

## Directory Structure

```
my-app/
├── app.yaml
├── values.yaml
├── kustomization.yaml
├── helm-output.yaml        # generated
├── chart/
│   ├── Chart.yaml
│   └── templates/
└── base/
    ├── kustomization.yaml
    ├── namespace.yaml
    └── secrets.tmpl.yaml
```

See `examples/complete` for a working example.

## ArgoCD Integration

### Plugin ConfigMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argo-render-plugin
  namespace: argocd
data:
  plugin.yaml: |
    apiVersion: argoproj.io/v1alpha1
    kind: ConfigManagementPlugin
    metadata:
      name: argo-render
    spec:
      generate:
        command:
          - argo-render
          - -enable-terraform
          - -enable-post-render
```

### Sidecar Container

Add to `argocd-repo-server`:

```yaml
containers:
  - name: argo-render
    image: ghcr.io/paragor/argo-render:latest
    command: ["/var/run/argocd/argocd-cmp-server"]
    securityContext:
      runAsNonRoot: true
      runAsUser: 999
    volumeMounts:
      - name: var-files
        mountPath: /var/run/argocd
      - name: plugins
        mountPath: /home/argocd/cmp-server/plugins
      - name: argo-render-plugin
        mountPath: /home/argocd/cmp-server/config/plugin.yaml
        subPath: plugin.yaml
      - name: cmp-tmp
        mountPath: /tmp
volumes:
  - name: argo-render-plugin
    configMap:
      name: argo-render-plugin
  - name: cmp-tmp
    emptyDir: {}
```

### Application

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-app
spec:
  source:
    repoURL: https://github.com/example/repo.git
    path: apps/my-app
    plugin:
      name: argo-render
  destination:
    server: https://kubernetes.default.svc
    namespace: my-namespace
```

## License

MIT
