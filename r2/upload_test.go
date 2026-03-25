package r2

import (
	"testing"
)

func TestNewClientRequiresConfig(t *testing.T) {
	_, err := NewClient(Config{
		Bucket:          "test-bucket",
		AccessKeyID:     "key",
		SecretAccessKey: "secret",
		Endpoint:        "",
	})
	if err == nil {
		t.Error("expected error for missing endpoint")
	}
}

func TestNewClientWithValidConfig(t *testing.T) {
	client, err := NewClient(Config{
		Bucket:          "test-bucket",
		AccessKeyID:     "key",
		SecretAccessKey: "secret",
		Endpoint:        "https://account.r2.cloudflarestorage.com",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Error("expected non-nil client")
	}
}
