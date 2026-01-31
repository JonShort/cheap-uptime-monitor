package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/cheap-uptime-monitor/internal/alert"
	"github.com/cheap-uptime-monitor/internal/checker"
	monitorconfig "github.com/cheap-uptime-monitor/internal/config"
)

type Handler struct {
	configLoader *monitorconfig.Loader
	checker      *checker.Checker
	alerter      *alert.Alerter
	logger       *slog.Logger
}

func NewHandler(ctx context.Context) (*Handler, error) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}

	s3Client := s3.NewFromConfig(cfg)
	snsClient := sns.NewFromConfig(cfg)

	configBucket := os.Getenv("CONFIG_BUCKET")
	configKey := os.Getenv("CONFIG_KEY")
	snsTopic := os.Getenv("SNS_TOPIC_ARN")

	return &Handler{
		configLoader: monitorconfig.NewLoader(s3Client, configBucket, configKey),
		checker:      checker.NewChecker(logger),
		alerter:      alert.NewAlerter(snsClient, snsTopic, logger),
		logger:       logger,
	}, nil
}

func (h *Handler) Handle(ctx context.Context) error {
	h.logger.Info("starting uptime check")

	cfg, err := h.configLoader.Load(ctx)
	if err != nil {
		h.logger.Error("failed to load config", "error", err)
		return err
	}

	h.logger.Info("loaded configuration", "endpoint_count", len(cfg.Endpoints))

	for _, endpoint := range cfg.Endpoints {
		result := h.checker.Check(ctx, endpoint)

		if !result.Success {
			h.logger.Warn("endpoint check failed",
				"name", endpoint.Name,
				"url", endpoint.URL,
				"reason", result.Reason,
			)

			if err := h.alerter.SendAlert(ctx, endpoint, result); err != nil {
				h.logger.Error("failed to send alert",
					"name", endpoint.Name,
					"error", err,
				)
			}
		} else {
			h.logger.Info("endpoint check passed",
				"name", endpoint.Name,
				"url", endpoint.URL,
			)
		}
	}

	h.logger.Info("uptime check complete")
	return nil
}

func main() {
	ctx := context.Background()

	handler, err := NewHandler(ctx)
	if err != nil {
		slog.Error("failed to initialize handler", "error", err)
		os.Exit(1)
	}

	lambda.Start(handler.Handle)
}
