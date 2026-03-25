package r2

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Config struct {
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	Endpoint        string
}

type Client struct {
	s3     *s3.Client
	bucket string
}

func NewClient(cfg Config) (*Client, error) {
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("R2 endpoint is required")
	}
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("R2 bucket name is required")
	}

	s3Client := s3.New(s3.Options{
		Region:       "auto",
		BaseEndpoint: aws.String(cfg.Endpoint),
		Credentials: credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID,
			cfg.SecretAccessKey,
			"",
		),
	})

	return &Client{s3: s3Client, bucket: cfg.Bucket}, nil
}

func (c *Client) UploadJSON(ctx context.Context, key string, data interface{}) error {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}

	contentType := "application/json"
	_, err = c.s3.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(c.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(jsonBytes),
		ContentType: &contentType,
	})
	if err != nil {
		return fmt.Errorf("upload to R2: %w", err)
	}

	return nil
}
