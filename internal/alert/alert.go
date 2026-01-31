package alert

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/cheap-uptime-monitor/internal/checker"
	"github.com/cheap-uptime-monitor/internal/config"
)

type SNSClient interface {
	Publish(ctx context.Context, params *sns.PublishInput, optFns ...func(*sns.Options)) (*sns.PublishOutput, error)
}

type Alerter struct {
	snsClient SNSClient
	topicARN  string
	logger    *slog.Logger
}

func NewAlerter(snsClient SNSClient, topicARN string, logger *slog.Logger) *Alerter {
	return &Alerter{
		snsClient: snsClient,
		topicARN:  topicARN,
		logger:    logger,
	}
}

func (a *Alerter) SendAlert(ctx context.Context, endpoint config.Endpoint, result checker.Result) error {
	subject := fmt.Sprintf("ALERT: %s is DOWN", endpoint.Name)

	message := fmt.Sprintf(`Uptime Monitor Alert

Endpoint: %s
URL: %s
Status: DOWN
Reason: %s
Timestamp: %s

--
Cheap Uptime Monitor`, endpoint.Name, endpoint.URL, result.Reason, result.Timestamp.Format("2006-01-02 15:04:05 UTC"))

	_, err := a.snsClient.Publish(ctx, &sns.PublishInput{
		TopicArn: aws.String(a.topicARN),
		Subject:  aws.String(subject),
		Message:  aws.String(message),
	})
	if err != nil {
		return fmt.Errorf("failed to publish to SNS: %w", err)
	}

	a.logger.Info("alert sent",
		"endpoint", endpoint.Name,
		"topic", a.topicARN,
	)

	return nil
}
