package template

import (
	"bytes"
	"context"
	"fmt"
	"text/template"

	"github.com/Masterminds/sprig/v3"
)

const (
	LeftDelim  = "@<<"
	RightDelim = ">>@"
)

type Engine struct {
	datasources map[string]Datasource
}

type Datasource interface {
	Get(ctx context.Context, key string) (any, error)
}

func NewEngine() *Engine {
	return &Engine{
		datasources: make(map[string]Datasource),
	}
}

func (e *Engine) RegisterDatasource(name string, ds Datasource) {
	e.datasources[name] = ds
}

func (e *Engine) Render(ctx context.Context, name string, content string) (string, error) {
	funcMap := sprig.TxtFuncMap()
	funcMap["datasource"] = e.datasourceFunc(ctx)

	tmpl, err := template.New(name).
		Delims(LeftDelim, RightDelim).
		Funcs(funcMap).
		Parse(content)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, nil); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	return buf.String(), nil
}

func (e *Engine) datasourceFunc(ctx context.Context) func(string, string) (any, error) {
	return func(dsName, key string) (any, error) {
		ds, ok := e.datasources[dsName]
		if !ok {
			return nil, fmt.Errorf("datasource not found: %s", dsName)
		}
		return ds.Get(ctx, key)
	}
}
