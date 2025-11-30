package kv

import (
	"context"
	"embed"
	"encoding/json"
	"strconv"
	"testing"
)

type constFetcher struct {
	content []byte
}

func (f *constFetcher) Fetch(ctx context.Context, key string) ([]byte, error) {
	return f.content, nil
}

func mustMarshalJSON(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		panic(err)
	}
	return string(b)
}

func prettifyJson(j string) string {
	v := new(any)
	err := json.Unmarshal([]byte(j), v)
	if err != nil {
		panic(err)
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		panic(err)
	}
	return string(b)
}

//go:embed assets/*
var assetsFS embed.FS

func assetFSFile(filename string) []byte {
	data, err := assetsFS.ReadFile("assets/" + filename)
	if err != nil {
		panic(err)
	}
	return data
}

func TestRemoteTerraformState_Get(t *testing.T) {
	tests := []struct {
		content []byte
		want    string
		wantErr bool
	}{
		{
			content: assetFSFile("state.json"),
			want: `
{
	"cluster_id": 1,
	"cluster_name": "hoba",
	"k8s_version": "1.33.4",
	"kubeconfig": "kubeconfig_content",
	"kubeconfig_data": {
		"client_certificate": "client_certificate_content",
		"client_key": "client_key_content",
		"cluster_ca_certificate": "client_ca_content",
		"cluster_name": "hoba",
		"endpoint": "https://172.16.12.200:6443",
		"host": "172.16.12.200",
		"port": 6443
	}
}
`,
			wantErr: false,
		},
	}
	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			s := &RemoteTerraformState{
				fetcher: &constFetcher{content: tt.content},
			}
			got, err := s.Get(context.Background(), "it doesn't metter")
			if (err != nil) != tt.wantErr {
				t.Errorf("Get() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			gotJson := mustMarshalJSON(got)
			wantJson := prettifyJson(tt.want)
			if gotJson != wantJson {
				t.Errorf("Get() got = \n%s\n, want \n%s\n", gotJson, wantJson)
			}
		})
	}
}
