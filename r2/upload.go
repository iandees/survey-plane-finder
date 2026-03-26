package r2

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
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

// DownloadJSON downloads a JSON file from the bucket and unmarshals it into dest.
// Returns nil error and leaves dest unchanged if the key does not exist.
func (c *Client) DownloadJSON(ctx context.Context, key string, dest interface{}) error {
	resp, err := c.s3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			return nil
		}
		// R2 may return a generic error for missing keys; check the message
		if errors.As(err, new(*types.NotFound)) {
			return nil
		}
		return fmt.Errorf("download from R2: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if err := json.Unmarshal(body, dest); err != nil {
		return fmt.Errorf("unmarshal JSON: %w", err)
	}

	return nil
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
