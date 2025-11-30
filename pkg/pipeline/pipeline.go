package pipeline

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/paragor/argo-render/pkg/config"
	"github.com/paragor/argo-render/pkg/helm"
	"github.com/paragor/argo-render/pkg/kustomize"
	"github.com/paragor/argo-render/pkg/kv"
	"github.com/paragor/argo-render/pkg/template"
)

type Pipeline struct {
	tfDatasource     *kv.RemoteTerraformState
	helmRenderer     *helm.Renderer
	enablePostRender bool
}

func New(tfDatasource *kv.RemoteTerraformState, enablePostRender bool) *Pipeline {
	return &Pipeline{
		tfDatasource:     tfDatasource,
		helmRenderer:     helm.NewRenderer(),
		enablePostRender: enablePostRender,
	}
}

func (p *Pipeline) Run(ctx context.Context, cfg *config.Config, gitRoot, appFileRel string) (string, error) {
	workDir, err := os.MkdirTemp("", "argo-render-*")
	if err != nil {
		return "", fmt.Errorf("create work dir: %w", err)
	}
	defer os.RemoveAll(workDir)

	if err := copyDir(gitRoot, workDir); err != nil {
		return "", fmt.Errorf("copy git root: %w", err)
	}

	appDir := filepath.Dir(filepath.Join(workDir, appFileRel))
	if err := os.Chdir(appDir); err != nil {
		return "", fmt.Errorf("chdir to app dir: %w", err)
	}

	engine := template.NewEngine()
	engine.RegisterDatasource("file", kv.NewFileDatasource(workDir))
	if p.tfDatasource != nil {
		engine.RegisterDatasource("terraform", p.tfDatasource)
	}

	kustomizeBuilder := kustomize.NewBuilder(engine)

	if cfg.Helm != nil {
		resolvedValues := make([]string, len(cfg.Helm.Values))
		for i, v := range cfg.Helm.Values {
			resolvedValues[i] = resolvePath(v, workDir)
		}

		if err := p.templateValuesInPlace(ctx, engine, resolvedValues); err != nil {
			return "", fmt.Errorf("template values: %w", err)
		}

		helmCfg := *cfg.Helm
		if cfg.Helm.Repo == "" {
			helmCfg.Chart = resolvePath(cfg.Helm.Chart, workDir)
		}
		helmCfg.Output = resolvePath(cfg.Helm.Output, workDir)

		if err := p.helmRenderer.Render(&helmCfg, resolvedValues); err != nil {
			return "", fmt.Errorf("helm render: %w", err)
		}
	}

	kustomizePath := resolvePath(cfg.Kustomize.Path, workDir)
	output, err := kustomizeBuilder.Build(ctx, kustomizePath)
	if err != nil {
		return "", fmt.Errorf("kustomize build: %w", err)
	}

	if p.enablePostRender {
		output, err = p.postRender(ctx, engine, output)
		if err != nil {
			return "", fmt.Errorf("post render: %w", err)
		}
	}

	return output, nil
}

func (p *Pipeline) templateValuesInPlace(ctx context.Context, engine *template.Engine, valueFiles []string) error {
	for _, vf := range valueFiles {
		content, err := os.ReadFile(vf)
		if err != nil {
			return fmt.Errorf("read %s: %w", vf, err)
		}

		rendered, err := engine.Render(ctx, vf, string(content))
		if err != nil {
			return fmt.Errorf("render %s: %w", vf, err)
		}

		if err := os.WriteFile(vf, []byte(rendered), 0644); err != nil {
			return fmt.Errorf("write %s: %w", vf, err)
		}
	}
	return nil
}

func (p *Pipeline) postRender(ctx context.Context, engine *template.Engine, content string) (string, error) {
	return engine.Render(ctx, "postrender", content)
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(dstPath, 0755)
		}

		return copyFile(path, dstPath)
	})
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

func resolvePath(path, rootDir string) string {
	if strings.HasPrefix(path, "/") {
		return filepath.Join(rootDir, strings.TrimPrefix(path, "/"))
	}
	return path
}
