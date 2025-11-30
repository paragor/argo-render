package helm

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/paragor/argo-render/pkg/config"
)

type Renderer struct{}

func NewRenderer() *Renderer {
	return &Renderer{}
}

func (r *Renderer) Render(cfg *config.HelmConfig, templatedValues []string) error {
	releaseName := cfg.ReleaseName
	if releaseName == "" {
		releaseName = os.Getenv("ARGOCD_APP_NAME")
	}
	if releaseName == "" {
		return fmt.Errorf("releaseName is required (set in config or ARGOCD_APP_NAME env)")
	}

	namespace := cfg.Namespace
	if namespace == "" {
		namespace = os.Getenv("ARGOCD_APP_NAMESPACE")
	}

	args := []string{"template", releaseName}

	if cfg.Repo != "" {
		args = append(args, cfg.Chart, "--repo", cfg.Repo)
		if cfg.Version != "" {
			args = append(args, "--version", cfg.Version)
		}
	} else {
		args = append(args, cfg.Chart)
	}

	if namespace != "" {
		args = append(args, "--namespace", namespace)
	}

	for _, v := range templatedValues {
		args = append(args, "--values", v)
	}

	cmd := exec.Command("helm", args...)
	cmd.Stderr = os.Stderr

	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("helm template: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(cfg.Output), 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	if err := os.WriteFile(cfg.Output, output, 0644); err != nil {
		return fmt.Errorf("write output: %w", err)
	}

	return nil
}
