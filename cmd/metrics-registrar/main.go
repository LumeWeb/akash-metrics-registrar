package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/urfave/cli/v3"
	"go.lumeweb.com/akash-metrics-registrar/pkg/build"
	"go.lumeweb.com/akash-metrics-registrar/pkg/logger"
	"go.lumeweb.com/akash-metrics-registrar/pkg/proxy"
	"go.lumeweb.com/akash-metrics-registrar/pkg/registrar"
	"net/http"
	"os"
	"time"
)

const (
	shutdownTimeout = 10 * time.Second
)

func main() {
	logger.Log.Infof("Starting metrics-registrar\n%s", build.GetVersionInfo())
	cmd := &cli.Command{
		Name:  "akash-metrics-registrar",
		Usage: "Service registration manager for Akash Network Prometheus exporters",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "target-host",
				Usage:    "Host address of the target service",
				Required: true,
				Sources:  cli.EnvVars("TARGET_HOST"),
			},
			&cli.StringFlag{
				Name:    "target-path",
				Usage:   "Path to the metrics endpoint",
				Value:   "/metrics",
				Sources: cli.EnvVars("TARGET_PATH"),
			},
			&cli.IntFlag{
				Name:    "target-port",
				Value:   9090,
				Usage:   "Port of the target service",
				Sources: cli.EnvVars("TARGET_PORT"),
			},
			&cli.IntFlag{
				Name:    "proxy-port",
				Value:   8080,
				Usage:   "Port for the proxy server",
				Sources: cli.EnvVars("PROXY_PORT"),
			},
			&cli.StringFlag{
				Name:     "service-name",
				Usage:    "Name of the service for registration",
				Required: true,
				Sources:  cli.EnvVars("SERVICE_NAME"),
			},
			&cli.StringFlag{
				Name:     "etcd-endpoints",
				Usage:    "Comma-separated etcd server addresses",
				Required: true,
				Sources:  cli.EnvVars("ETCD_ENDPOINTS"),
			},
			&cli.StringFlag{
				Name:    "etcd-prefix",
				Usage:   "Key prefix for etcd registration",
				Sources: cli.EnvVars("ETCD_PREFIX"),
			},
			&cli.StringFlag{
				Name:    "etcd-username",
				Usage:   "ETCD username",
				Sources: cli.EnvVars("ETCD_USERNAME"),
			},
			&cli.StringFlag{
				Name:    "etcd-password",
				Usage:   "ETCD password",
				Sources: cli.EnvVars("ETCD_PASSWORD"),
			},
			&cli.DurationFlag{
				Name:    "etcd-timeout",
				Value:   120 * time.Second,
				Usage:   "ETCD timeout",
				Sources: cli.EnvVars("ETCD_TIMEOUT"),
			},
			&cli.DurationFlag{
				Name:    "registration-ttl",
				Value:   30 * time.Second,
				Usage:   "Registration TTL duration",
				Sources: cli.EnvVars("REGISTRATION_TTL"),
			},
			&cli.IntFlag{
				Name:    "retry-attempts",
				Value:   3,
				Usage:   "Number of retry attempts",
				Sources: cli.EnvVars("RETRY_ATTEMPTS"),
			},
			&cli.DurationFlag{
				Name:    "retry-delay",
				Value:   5 * time.Second,
				Usage:   "Delay between retry attempts",
				Sources: cli.EnvVars("RETRY_DELAY"),
			},
			&cli.StringFlag{
				Name:     "metrics-password",
				Usage:    "Password for metrics basic auth",
				Required: true,
				Sources:  cli.EnvVars("METRICS_PASSWORD"),
			},
			&cli.StringFlag{
				Name:     "exporter-type",
				Usage:    "Type of metrics exporter",
				Value:    "metrics_exporter",
				Sources:  cli.EnvVars("EXPORTER_TYPE"),
			},
			&cli.StringFlag{
				Name:    "custom-labels",
				Usage:   "JSON string of custom labels",
				Sources: cli.EnvVars("CUSTOM_LABELS"),
			},
		},
		Action: run,
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		logger.Log.Fatal(err)
	}
}

func run(ctx context.Context, cmd *cli.Command) error {
	// Set log level
	logger.SetLevel(cmd.String("loglevel"))

	// Parse custom labels if provided
	var customLabels map[string]string
	if labelJSON := cmd.String("custom-labels"); labelJSON != "" {
		if err := json.Unmarshal([]byte(labelJSON), &customLabels); err != nil {
			return fmt.Errorf("failed to parse custom labels: %w", err)
		}
	}

	targetURL := fmt.Sprintf("http://%s:%d%s",
		cmd.String("target-host"),
		cmd.Int("target-port"),
		cmd.String("target-path"),
	)

	cfg := &registrar.Config{
		TargetURL:       targetURL,
		ServiceName:     cmd.String("service-name"),
		ExporterType:    cmd.String("exporter-type"),
		EtcdEndpoints:   []string{cmd.String("etcd-endpoints")},
		EtcdPrefix:      cmd.String("etcd-prefix"),
		EtcdUsername:    cmd.String("etcd-username"),
		EtcdPassword:    cmd.String("etcd-password"),
		EtcdTimeout:     cmd.Duration("etcd-timeout"),
		RegistrationTTL: cmd.Duration("registration-ttl"),
		RetryAttempts:   int(cmd.Int("retry-attempts")),
		RetryDelay:      cmd.Duration("retry-delay"),
		CustomLabels:    customLabels,
		Password:        cmd.String("metrics-password"),
	}

	// Create new registrar app
	app, err := registrar.NewApp(cfg)
	if err != nil {
		return fmt.Errorf("failed to create app: %w", err)
	}

	// Setup proxy
	proxyConfig := proxy.Config{
		TargetURL:     targetURL,
		FlushInterval: 5 * time.Second,
	}

	reverseProxy, err := proxy.NewMetricsProxy(proxyConfig)
	if err != nil {
		return fmt.Errorf("failed to create proxy: %w", err)
	}

	// Setup HTTP server with proxy
	proxyAddr := fmt.Sprintf(":%d", cmd.Int("proxy-port"))
	server := &http.Server{
		Addr:    proxyAddr,
		Handler: proxy.WithBasicAuth(reverseProxy, cfg.Password),
	}

	// Handle Akash port mapping for registration
	registrationPort := fmt.Sprintf("%d", cmd.Int("proxy-port"))
	akashPortVar := fmt.Sprintf("AKASH_EXTERNAL_PORT_%d", cmd.Int("proxy-port"))
	if akashPort := os.Getenv(akashPortVar); akashPort != "" {
		logger.Log.Infof("Found Akash external port mapping: %s - will use for registration", akashPort)
		registrationPort = akashPort
	}

	// Get Akash ingress host if available
	akashIngressHost := os.Getenv("AKASH_INGRESS_HOST")
	if akashIngressHost == "" {
		akashIngressHost = "localhost"
	}

	// Update registration URL to use proxy address with Akash configuration
	cfg.TargetURL = fmt.Sprintf("http://%s:%s%s",
		akashIngressHost,
		registrationPort,
		cmd.String("target-path"),
	)

	// Start HTTP server
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Log.Errorf("HTTP server error: %v", err)
		}
	}()

	// Start the registration service
	if err := app.Start(ctx); err != nil {
		return fmt.Errorf("failed to start app: %w", err)
	}

	// Create shutdown context
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()

	// Wait for context cancellation
	<-ctx.Done()

	// Shutdown HTTP server
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Log.Errorf("HTTP server shutdown error: %v", err)
	}

	// Initiate graceful shutdown of registrar
	if err := app.Shutdown(shutdownCtx); err != nil {
		if !errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("shutdown error: %w", err)
		}
		logger.Log.Warn("Shutdown timed out")
	}

	return nil
}
