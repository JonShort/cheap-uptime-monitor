package health

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

type mockS3Client struct {
	getResponse []byte
	getErr      error
	putInput    *s3.PutObjectInput
	putErr      error
}

func (m *mockS3Client) GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	return &s3.GetObjectOutput{
		Body: io.NopCloser(bytes.NewReader(m.getResponse)),
	}, nil
}

func (m *mockS3Client) PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	m.putInput = params
	if m.putErr != nil {
		return nil, m.putErr
	}
	return &s3.PutObjectOutput{}, nil
}

// noSuchKeyError implements smithy.APIError for testing NoSuchKey handling.
type noSuchKeyError struct{}

func (e *noSuchKeyError) Error() string   { return "NoSuchKey" }
func (e *noSuchKeyError) ErrorCode() string   { return "NoSuchKey" }
func (e *noSuchKeyError) ErrorMessage() string { return "The specified key does not exist." }
func (e *noSuchKeyError) ErrorFault() smithy.ErrorFault { return smithy.FaultClient }

func TestTracker_Load_Success(t *testing.T) {
	kh := KnownHealth{
		Endpoints: map[string]EndpointHealth{
			"Test API": {
				Status:      StatusUp,
				LastChecked: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			},
		},
	}
	data, _ := json.Marshal(kh)

	client := &mockS3Client{getResponse: data}
	tracker := NewTracker(client, "test-bucket", "known-health.json")

	result, err := tracker.Load(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.GetStatus("Test API") != StatusUp {
		t.Errorf("expected status up, got %s", result.GetStatus("Test API"))
	}

	ep := result.Endpoints["Test API"]
	if ep.LastChecked.IsZero() {
		t.Error("expected non-zero last_checked")
	}
}

func TestTracker_Load_MissingFile(t *testing.T) {
	client := &mockS3Client{getErr: &noSuchKeyError{}}
	tracker := NewTracker(client, "test-bucket", "known-health.json")

	result, err := tracker.Load(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.Endpoints) != 0 {
		t.Errorf("expected empty endpoints, got %d", len(result.Endpoints))
	}
}

func TestTracker_Load_S3Error(t *testing.T) {
	client := &mockS3Client{getErr: &smithy.GenericAPIError{Code: "AccessDenied", Message: "access denied"}}
	tracker := NewTracker(client, "test-bucket", "known-health.json")

	_, err := tracker.Load(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestTracker_Save(t *testing.T) {
	client := &mockS3Client{}
	tracker := NewTracker(client, "test-bucket", "known-health.json")

	kh := &KnownHealth{
		Endpoints: map[string]EndpointHealth{
			"Test API": {
				Status:      StatusDown,
				LastChecked: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			},
		},
	}

	err := tracker.Save(context.Background(), kh)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if client.putInput == nil {
		t.Fatal("expected PutObject to be called")
	}

	if *client.putInput.Bucket != "test-bucket" {
		t.Errorf("unexpected bucket: %s", *client.putInput.Bucket)
	}
	if *client.putInput.Key != "known-health.json" {
		t.Errorf("unexpected key: %s", *client.putInput.Key)
	}

	body, _ := io.ReadAll(client.putInput.Body)
	var saved KnownHealth
	if err := json.Unmarshal(body, &saved); err != nil {
		t.Fatalf("failed to parse saved JSON: %v", err)
	}
	if saved.GetStatus("Test API") != StatusDown {
		t.Errorf("expected saved status down, got %s", saved.GetStatus("Test API"))
	}
}

func TestTracker_Save_Error(t *testing.T) {
	client := &mockS3Client{putErr: &smithy.GenericAPIError{Code: "AccessDenied", Message: "access denied"}}
	tracker := NewTracker(client, "test-bucket", "known-health.json")

	err := tracker.Save(context.Background(), &KnownHealth{Endpoints: map[string]EndpointHealth{}})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestKnownHealth_GetStatus(t *testing.T) {
	kh := &KnownHealth{
		Endpoints: map[string]EndpointHealth{
			"Up API":   {Status: StatusUp},
			"Down API": {Status: StatusDown},
		},
	}

	if kh.GetStatus("Up API") != StatusUp {
		t.Errorf("expected up, got %s", kh.GetStatus("Up API"))
	}
	if kh.GetStatus("Down API") != StatusDown {
		t.Errorf("expected down, got %s", kh.GetStatus("Down API"))
	}
	if kh.GetStatus("Unknown API") != StatusUnknown {
		t.Errorf("expected unknown, got %s", kh.GetStatus("Unknown API"))
	}
}
