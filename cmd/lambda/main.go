package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/cheap-uptime-monitor/internal/alert"
	"github.com/cheap-uptime-monitor/internal/checker"
	monitorconfig "github.com/cheap-uptime-monitor/internal/config"
	"github.com/cheap-uptime-monitor/internal/health"
)

type Handler struct {
	configLoader   *monitorconfig.Loader
	checker        *checker.Checker
	alerter        *alert.Alerter
	healthTracker  *health.Tracker
	logger         *slog.Logger
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

	healthKey := os.Getenv("HEALTH_KEY")
	if healthKey == "" {
		healthKey = "known-health.json"
	}

	return &Handler{
		configLoader:  monitorconfig.NewLoader(s3Client, configBucket, configKey),
		checker:       checker.NewChecker(logger),
		alerter:       alert.NewAlerter(snsClient, snsTopic, logger),
		healthTracker: health.NewTracker(s3Client, configBucket, healthKey),
		logger:        logger,
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

	knownHealth, err := h.healthTracker.Load(ctx)
	if err != nil {
		h.logger.Error("failed to load known health", "error", err)
		return err
	}

	changed := false

	for _, endpoint := range cfg.Endpoints {
		result := h.checker.Check(ctx, endpoint)
		previousStatus := knownHealth.GetStatus(endpoint.Name)

		var currentStatus health.Status
		if result.Success {
			currentStatus = health.StatusUp
		} else {
			currentStatus = health.StatusDown
		}

		if currentStatus == previousStatus {
			h.logger.Info("endpoint status unchanged",
				"name", endpoint.Name,
				"status", string(currentStatus),
			)
			continue
		}

		// Status changed — update known health
		knownHealth.Endpoints[endpoint.Name] = health.EndpointHealth{
			Status:      currentStatus,
			LastChecked: time.Now().UTC(),
		}
		changed = true

		switch {
		case currentStatus == health.StatusDown:
			// UNKNOWN→DOWN or UP→DOWN: send down alert
			h.logger.Warn("endpoint is DOWN",
				"name", endpoint.Name,
				"url", endpoint.URL,
				"reason", result.Reason,
				"previous_status", string(previousStatus),
			)
			if err := h.alerter.SendDownAlert(ctx, endpoint, result); err != nil {
				h.logger.Error("failed to send alert",
					"name", endpoint.Name,
					"error", err,
				)
			}

		case currentStatus == health.StatusUp && previousStatus == health.StatusDown:
			// DOWN→UP: send recovery alert
			h.logger.Info("endpoint RECOVERED",
				"name", endpoint.Name,
				"url", endpoint.URL,
			)
			if err := h.alerter.SendRecoveryAlert(ctx, endpoint, result); err != nil {
				h.logger.Error("failed to send recovery alert",
					"name", endpoint.Name,
					"error", err,
				)
			}

		case currentStatus == health.StatusUp && previousStatus == health.StatusUnknown:
			// UNKNOWN→UP: no alert, just record
			h.logger.Info("endpoint initial status recorded",
				"name", endpoint.Name,
				"url", endpoint.URL,
				"status", "up",
			)
		}
	}

	if changed {
		if err := h.healthTracker.Save(ctx, knownHealth); err != nil {
			h.logger.Error("failed to save known health", "error", err)
			return err
		}
		h.logger.Info("known health updated")
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
