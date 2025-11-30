package kv

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	ctyjson "github.com/zclconf/go-cty/cty/json"
)

type TerraformStateFetcher interface {
	Fetch(ctx context.Context, key string) ([]byte, error)
}

type terraformStateCache struct {
	cache  map[string][]byte
	origin TerraformStateFetcher
}

func NewTerraformStateCache(origin TerraformStateFetcher) *terraformStateCache {
	return &terraformStateCache{origin: origin, cache: make(map[string][]byte)}
}

func (c *terraformStateCache) Fetch(ctx context.Context, key string) ([]byte, error) {
	if c.cache[key] != nil {
		return c.cache[key], nil
	}
	resp, err := c.origin.Fetch(ctx, key)
	if err != nil {
		return nil, err
	}
	c.cache[key] = resp
	return resp, nil
}

type terraformS3Fetcher struct {
	s3client *s3.Client
}

func NewTerraformS3Fetcher(s3client *s3.Client) *terraformS3Fetcher {
	return &terraformS3Fetcher{s3client: s3client}
}

func (f *terraformS3Fetcher) Fetch(ctx context.Context, fullKey string) ([]byte, error) {
	keyPath := strings.Split(fullKey, "/")
	if len(keyPath) < 2 {
		return nil, fmt.Errorf("invalid key: %s", fullKey)
	}
	bucket := keyPath[0]
	if len(bucket) == 0 {
		return nil, fmt.Errorf("invalid bucket: %s", bucket)
	}
	key := strings.Join(keyPath[1:], "/")
	object, err := f.s3client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch object %s: %w", fullKey, err)
	}
	defer object.Body.Close()
	response, err := io.ReadAll(object.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read object %s: %w", fullKey, err)
	}
	return response, nil
}

type RemoteTerraformState struct {
	fetcher TerraformStateFetcher
}

func NewRemoteTerraformState(fetcher TerraformStateFetcher) *RemoteTerraformState {
	return &RemoteTerraformState{fetcher: fetcher}
}

func (s *RemoteTerraformState) Get(ctx context.Context, fullKey string) (any, error) {
	data, err := s.fetcher.Fetch(ctx, fullKey)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch object %s: %w", fullKey, err)
	}

	tfState := &terraformState{Outputs: make(map[string]*tfOutputValue)}
	if err := json.Unmarshal(data, tfState); err != nil {
		return nil, fmt.Errorf("failed to unmarshal object %s: %w", fullKey, err)
	}

	intermediateResult := map[string]any{}
	for key, rawValue := range tfState.Outputs {
		ctyType, err := ctyjson.UnmarshalType(rawValue.ValueType)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal ctyType %s: %w", fullKey, err)
		}
		ctyValue, err := ctyjson.Unmarshal(rawValue.Value, ctyType)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal ctyValue %s: %w", fullKey, err)
		}

		ctyValueJson, err := ctyjson.Marshal(ctyValue, ctyType)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal ctyValue %s: %w", fullKey, err)
		}

		resultValue := new(any)
		if err := json.Unmarshal(ctyValueJson, resultValue); err != nil {
			return nil, fmt.Errorf("failed to unmarshal marshaled ctyValue %s: %w", fullKey, err)
		}

		intermediateResult[key] = resultValue
	}

	result := map[string]any{}
	data, err = json.Marshal(intermediateResult)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal intermediateResult: %w", err)
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal intermediateResult: %w", err)
	}

	return result, nil
}

type terraformState struct {
	Outputs map[string]*tfOutputValue `json:"outputs"`
}

type tfOutputValue struct {
	Value     json.RawMessage `json:"value"`
	ValueType json.RawMessage `json:"type"`
}
