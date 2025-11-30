package kustomize

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/paragor/argo-render/pkg/template"
)

type Builder struct {
	engine *template.Engine
}

func NewBuilder(engine *template.Engine) *Builder {
	return &Builder{engine: engine}
}

func (b *Builder) Build(ctx context.Context, path string) (string, error) {
	if err := b.preprocessTemplates(ctx, path); err != nil {
		return "", fmt.Errorf("preprocess templates: %w", err)
	}

	cmd := exec.Command("kustomize", "build", path)
	cmd.Stderr = os.Stderr

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("kustomize build: %w", err)
	}

	return string(output), nil
}

func (b *Builder) preprocessTemplates(ctx context.Context, path string) error {
	return filepath.WalkDir(path, func(filePath string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		if !isTemplateFile(filePath) {
			return nil
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("read %s: %w", filePath, err)
		}

		rendered, err := b.engine.Render(ctx, filePath, string(content))
		if err != nil {
			return fmt.Errorf("render %s: %w", filePath, err)
		}

		if err := os.WriteFile(filePath, []byte(rendered), 0644); err != nil {
			return fmt.Errorf("write %s: %w", filePath, err)
		}

		return nil
	})
}

func isTemplateFile(path string) bool {
	return strings.HasSuffix(path, ".tmpl.yaml") || strings.HasSuffix(path, ".tmpl.yml")
}
