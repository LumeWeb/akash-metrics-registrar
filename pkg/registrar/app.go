package registrar

import (
	"context"
	"fmt"
	"go.lumeweb.com/akash-metrics-registrar/pkg/logger"
	"go.lumeweb.com/akash-metrics-registrar/pkg/util"
	etcdregistry "go.lumeweb.com/etcd-registry"
	"go.lumeweb.com/etcd-registry/types"
	"golang.org/x/time/rate"
	"net/http"
	"sync"
	"time"
)

// App represents the registration service
type App struct {
	cfg         *Config
	registry    *etcdregistry.EtcdRegistry
	group       *types.ServiceGroup
	currentInfo RegistrationInfo
	etcdLimiter *rate.Limiter
	httpClient  *http.Client
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewApp creates a new registration service instance
func NewApp(cfg *Config) (*App, error) {
	ctx, cancel := context.WithCancel(context.Background())

	app := &App{
		cfg:         cfg,
		etcdLimiter: rate.NewLimiter(rate.Every(5*time.Second), 3),
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		ctx:    ctx,
		cancel: cancel,
	}

	return app, nil
}

// Start initializes and starts the registration service
func (a *App) Start(ctx context.Context) error {
	_, err := util.RetryOperation(func() (bool, error) {
		err := a.setupEtcd()
		return true, err
	}, util.ConnectionRetry, 3)
	if err != nil {
		return fmt.Errorf("etcd setup failed after retries: %w", err)
	}

	if err := a.setupServiceGroup(); err != nil {
		return fmt.Errorf("service group setup failed: %w", err)
	}

	// Start health checking
	a.startHealthCheck()

	return nil
}

func (a *App) setupEtcd() error {
	var err error
	a.registry, err = etcdregistry.NewEtcdRegistry(
		a.cfg.EtcdEndpoints,
		a.cfg.EtcdPrefix,
		a.cfg.EtcdUsername,
		a.cfg.EtcdPassword,
		a.cfg.EtcdTimeout,
		3,
	)
	if err != nil {
		return fmt.Errorf("failed to create etcd registry: %w", err)
	}
	return nil
}

func (a *App) setupServiceGroup() error {
	group, err := a.registry.CreateOrJoinServiceGroup(a.ctx, a.cfg.ServiceName)
	if err != nil {
		return fmt.Errorf("failed to create/join service group: %w", err)
	}

	spec := types.ServiceGroupSpec{
		CommonLabels: a.cfg.CustomLabels,
		Password:     a.cfg.Password,
	}

	if err := group.Configure(spec); err != nil {
		return fmt.Errorf("failed to configure service group: %w", err)
	}

	a.group = group
	return nil
}

func (a *App) startHealthCheck() {
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()

		ticker := time.NewTicker(a.cfg.RegistrationTTL / 2)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := a.checkHealth(); err != nil {
					logger.Log.Errorf("Health check failed: %v", err)
					a.updateStatus(StatusDegraded)
				} else {
					a.updateStatus(StatusHealthy)
				}
			case <-a.ctx.Done():
				return
			}
		}
	}()
}

func (a *App) checkHealth() error {
	req, err := http.NewRequest("GET", a.cfg.TargetURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}


	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed with status: %d", resp.StatusCode)
	}

	return nil
}

func (a *App) updateStatus(status ServiceStatus) error {
	if err := a.etcdLimiter.Wait(a.ctx); err != nil {
		return fmt.Errorf("rate limit exceeded: %w", err)
	}

	a.currentInfo.Status = status
	a.currentInfo.LastChecked = time.Now()

	node := types.Node{
		ID:           a.currentInfo.ID,
		ExporterType: "metrics_exporter",
		Labels:       a.currentInfo.Labels,
		Status:       string(status),
		LastSeen:     time.Now(),
	}

	operation := func() error {
		_, errChan, err := a.group.RegisterNode(a.ctx, node, a.cfg.RegistrationTTL)
		if err != nil {
			return fmt.Errorf("failed to register node: %w", err)
		}

		select {
		case err := <-errChan:
			if err != nil {
				return err
			}
		case <-a.ctx.Done():
			return a.ctx.Err()
		default:
		}

		return nil
	}

	retryConfig := util.RetryConfig{
		InitialInterval:     a.cfg.RetryDelay,
		MaxInterval:         a.cfg.RetryDelay * 2,
		MaxElapsedTime:      a.cfg.RetryDelay * time.Duration(a.cfg.RetryAttempts),
		RandomizationFactor: 0.2,
	}
	_, err := util.RetryOperation(func() (bool, error) {
		return true, operation()
	}, retryConfig, uint(a.cfg.RetryAttempts))
	return err
}

// Shutdown gracefully stops the registration service
func (a *App) Shutdown(ctx context.Context) error {
	// Cancel context to stop health checks
	a.cancel()

	// Update status to shutdown
	if err := a.updateStatus(StatusShutdown); err != nil {
		logger.Log.Errorf("Failed to update shutdown status: %v", err)
	}

	// Wait for goroutines with timeout
	done := make(chan struct{})
	go func() {
		a.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logger.Log.Info("All goroutines completed")
	case <-ctx.Done():
		return fmt.Errorf("shutdown timed out")
	}

	// Close etcd registry
	if err := a.registry.Close(); err != nil {
		return fmt.Errorf("failed to close etcd registry: %w", err)
	}

	return nil
}
