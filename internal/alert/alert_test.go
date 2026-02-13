package alert

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/cheap-uptime-monitor/internal/checker"
	"github.com/cheap-uptime-monitor/internal/config"
)

type mockSNSClient struct {
	publishInput *sns.PublishInput
	err          error
}

func (m *mockSNSClient) Publish(ctx context.Context, params *sns.PublishInput, optFns ...func(*sns.Options)) (*sns.PublishOutput, error) {
	m.publishInput = params
	if m.err != nil {
		return nil, m.err
	}
	return &sns.PublishOutput{}, nil
}

func newTestAlerter(client *mockSNSClient) *Alerter {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	return NewAlerter(client, "arn:aws:sns:us-east-1:123456789:test-topic", logger)
}

func TestAlerter_SendAlert_Success(t *testing.T) {
	client := &mockSNSClient{}
	alerter := newTestAlerter(client)

	endpoint := config.Endpoint{
		Name: "Production API",
		URL:  "https://api.example.com/health",
	}

	result := checker.Result{
		Success:   false,
		Reason:    "unexpected status code: 500",
		Timestamp: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
	}

	err := alerter.SendAlert(context.Background(), endpoint, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if client.publishInput == nil {
		t.Fatal("expected Publish to be called")
	}

	if *client.publishInput.TopicArn != "arn:aws:sns:us-east-1:123456789:test-topic" {
		t.Errorf("unexpected topic ARN: %s", *client.publishInput.TopicArn)
	}

	subject := *client.publishInput.Subject
	if !strings.Contains(subject, "Production API") {
		t.Errorf("subject should contain endpoint name: %s", subject)
	}
	if !strings.Contains(subject, "DOWN") {
		t.Errorf("subject should contain DOWN: %s", subject)
	}

	message := *client.publishInput.Message
	if !strings.Contains(message, "Production API") {
		t.Errorf("message should contain endpoint name: %s", message)
	}
	if !strings.Contains(message, "https://api.example.com/health") {
		t.Errorf("message should contain URL: %s", message)
	}
	if !strings.Contains(message, "unexpected status code: 500") {
		t.Errorf("message should contain reason: %s", message)
	}
	if !strings.Contains(message, "2024-01-15 10:30:00") {
		t.Errorf("message should contain timestamp: %s", message)
	}
}

func TestAlerter_SendAlert_SNSError(t *testing.T) {
	client := &mockSNSClient{err: errors.New("SNS publish failed")}
	alerter := newTestAlerter(client)

	endpoint := config.Endpoint{
		Name: "Test",
		URL:  "https://example.com",
	}

	result := checker.Result{
		Success:   false,
		Reason:    "timeout",
		Timestamp: time.Now().UTC(),
	}

	err := alerter.SendAlert(context.Background(), endpoint, result)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestAlerter_SendAlert_MessageFormat(t *testing.T) {
	client := &mockSNSClient{}
	alerter := newTestAlerter(client)

	endpoint := config.Endpoint{
		Name: "My Service",
		URL:  "https://service.example.com/healthz",
	}

	result := checker.Result{
		Success:    false,
		StatusCode: 503,
		Reason:     "unexpected status code: 503",
		Timestamp:  time.Date(2024, 6, 20, 14, 45, 30, 0, time.UTC),
	}

	err := alerter.SendAlert(context.Background(), endpoint, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	message := *client.publishInput.Message

	expectedParts := []string{
		"Uptime Monitor Alert",
		"Endpoint: My Service",
		"URL: https://service.example.com/healthz",
		"Status: DOWN",
		"Reason: unexpected status code: 503",
		"Timestamp: 2024-06-20 14:45:30 UTC",
		"Cheap Uptime Monitor",
	}

	for _, part := range expectedParts {
		if !strings.Contains(message, part) {
			t.Errorf("message should contain '%s', got:\n%s", part, message)
		}
	}
}

func TestAlerter_SendRecoveryAlert_Success(t *testing.T) {
	client := &mockSNSClient{}
	alerter := newTestAlerter(client)

	endpoint := config.Endpoint{
		Name: "Production API",
		URL:  "https://api.example.com/health",
	}

	result := checker.Result{
		Success:    true,
		StatusCode: 200,
		Timestamp:  time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
	}

	err := alerter.SendRecoveryAlert(context.Background(), endpoint, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if client.publishInput == nil {
		t.Fatal("expected Publish to be called")
	}

	subject := *client.publishInput.Subject
	if !strings.Contains(subject, "RECOVERED") {
		t.Errorf("subject should contain RECOVERED: %s", subject)
	}
	if !strings.Contains(subject, "Production API") {
		t.Errorf("subject should contain endpoint name: %s", subject)
	}

	message := *client.publishInput.Message
	expectedParts := []string{
		"Uptime Monitor Recovery",
		"Endpoint: Production API",
		"URL: https://api.example.com/health",
		"Status: UP",
		"Timestamp: 2024-01-15 10:30:00 UTC",
		"Cheap Uptime Monitor",
	}

	for _, part := range expectedParts {
		if !strings.Contains(message, part) {
			t.Errorf("message should contain '%s', got:\n%s", part, message)
		}
	}
}

func TestAlerter_SendRecoveryAlert_SNSError(t *testing.T) {
	client := &mockSNSClient{err: errors.New("SNS publish failed")}
	alerter := newTestAlerter(client)

	endpoint := config.Endpoint{
		Name: "Test",
		URL:  "https://example.com",
	}

	result := checker.Result{
		Success:   true,
		Timestamp: time.Now().UTC(),
	}

	err := alerter.SendRecoveryAlert(context.Background(), endpoint, result)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
