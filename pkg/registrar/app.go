package registrar

import (
	"context"
	"fmt"
	"go.lumeweb.com/akash-metrics-registrar/pkg/build"
	"go.lumeweb.com/akash-metrics-registrar/pkg/logger"
	"go.lumeweb.com/akash-metrics-registrar/pkg/util"
	etcdregistry "go.lumeweb.com/etcd-registry"
	"go.lumeweb.com/etcd-registry/types"
	"golang.org/x/time/rate"
	"net/http"
	"os"
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
	regDone     <-chan struct{}
	regErrChan  <-chan error
}

// NewApp creates a new registration service instance
func NewApp(cfg *Config) (*App, error) {
	ctx, cancel := context.WithCancel(context.Background())

	app := &App{
		cfg:         cfg,
		etcdLimiter: rate.NewLimiter(rate.Every(60*time.Second), 1), // Much more conservative rate limiting
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
	// Health check disabled to reduce lease operations
	logger.Log.Info("Health check disabled - relying on TTL for status management")
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

func (a *App) performInitialRegistration() error {
	if err := a.etcdLimiter.Wait(a.ctx); err != nil {
		return fmt.Errorf("rate limit exceeded: %w", err)
	}

	// Get Akash identifiers
	ingressHost := os.Getenv("AKASH_INGRESS_HOST")
	identifiers := util.GetNodeIdentifiers(ingressHost)

	// Add standard Akash and version labels
	labels := make(map[string]string)
	for k, v := range a.currentInfo.Labels {
		labels[k] = v
	}
	labels["version"] = build.Version
	labels["git_commit"] = build.GitCommit
	labels["deployment_id"] = identifiers.DeploymentID
	labels["hash_id"] = identifiers.HashID
	labels["ingress_host"] = ingressHost

	node := types.Node{
		ID:           a.currentInfo.ID,
		ExporterType: "metrics_exporter",
		Labels:       labels,
		Status:       string(a.currentInfo.Status),
		LastSeen:     time.Now(),
	}

	done, errChan, err := a.group.RegisterNode(a.ctx, node, a.cfg.RegistrationTTL)
	if err != nil {
		return fmt.Errorf("failed to register node: %w", err)
	}

	a.regDone = done
	a.regErrChan = errChan
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

	done, errChan, err := a.group.RegisterNode(a.ctx, node, a.cfg.RegistrationTTL)
	if err != nil {
		return fmt.Errorf("failed to update node status: %w", err)
	}

	a.regDone = done
	a.regErrChan = errChan
	return nil
}

// Shutdown gracefully stops the registration service
func (a *App) Shutdown(ctx context.Context) error {
	// Cancel context to stop health checks
	a.cancel()

	// Simply close registry without cleanup
	if a.registry != nil {
		a.registry.Close()
	}

	// Wait for goroutines with short timeout
	waitCh := make(chan struct{})
	go func() {
		a.wg.Wait()
		close(waitCh)
	}()

	select {
	case <-waitCh:
		logger.Log.Info("Clean shutdown completed")
	case <-time.After(5 * time.Second):
		logger.Log.Warn("Shutdown timed out waiting for goroutines")
	}

	return nil
}
