package kv

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type FileDatasource struct {
	rootDir string
}

func NewFileDatasource(rootDir string) *FileDatasource {
	return &FileDatasource{rootDir: rootDir}
}

func (f *FileDatasource) Get(ctx context.Context, path string) (any, error) {
	var fullPath string
	if strings.HasPrefix(path, "/") {
		fullPath = filepath.Join(f.rootDir, strings.TrimPrefix(path, "/"))
	} else {
		fullPath = path
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("read file %s: %w", path, err)
	}

	var result any
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &result); err != nil {
			return nil, fmt.Errorf("parse yaml %s: %w", path, err)
		}
	case ".json":
		if err := json.Unmarshal(data, &result); err != nil {
			return nil, fmt.Errorf("parse json %s: %w", path, err)
		}
	default:
		return nil, fmt.Errorf("unsupported file format %s: use .json, .yaml, or .yml", path)
	}

	return result, nil
}
