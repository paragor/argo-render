package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	appconfig "github.com/paragor/argo-render/pkg/config"
	"github.com/paragor/argo-render/pkg/kv"
	"github.com/paragor/argo-render/pkg/pipeline"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		appFile          string
		enableTerraform  bool
		enablePostRender bool
	)

	flag.StringVar(&appFile, "config", "app.yaml", "path to app.yaml config file")
	flag.BoolVar(&enableTerraform, "enable-terraform", false, "enable terraform datasource (requires AWS credentials)")
	flag.BoolVar(&enablePostRender, "enable-post-render", false, "enable post-render template processing")
	flag.Parse()

	ctx := context.Background()

	appFileAbs, err := filepath.Abs(appFile)
	if err != nil {
		return fmt.Errorf("resolve app file: %w", err)
	}

	cfg, err := appconfig.Load(appFileAbs)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	gitRoot, err := findGitRoot(filepath.Dir(appFileAbs))
	if err != nil {
		return fmt.Errorf("find git root: %w", err)
	}

	appFileRel, err := filepath.Rel(gitRoot, appFileAbs)
	if err != nil {
		return fmt.Errorf("relative app file path: %w", err)
	}

	var tfDatasource *kv.RemoteTerraformState
	if enableTerraform {
		tfDatasource, err = createTerraformDatasource(ctx)
		if err != nil {
			return fmt.Errorf("create terraform datasource: %w", err)
		}
	}

	p := pipeline.New(tfDatasource, enablePostRender)

	output, err := p.Run(ctx, cfg, gitRoot, appFileRel)
	if err != nil {
		return fmt.Errorf("run pipeline: %w", err)
	}

	fmt.Print(output)
	return nil
}

func createTerraformDatasource(ctx context.Context) (*kv.RemoteTerraformState, error) {
	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	s3Client := s3.NewFromConfig(awsCfg)
	fetcher := kv.NewTerraformS3Fetcher(s3Client)
	cachedFetcher := kv.NewTerraformStateCache(fetcher)

	return kv.NewRemoteTerraformState(cachedFetcher), nil
}

func findGitRoot(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}

	for {
		gitPath := filepath.Join(dir, ".git")
		if _, err := os.Stat(gitPath); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("git root not found")
		}
		dir = parent
	}
}
