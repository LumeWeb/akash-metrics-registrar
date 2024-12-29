package registrar

import (
	"time"
)

// Config holds the application configuration
type Config struct {
	// Etcd configuration
	EtcdEndpoints []string
	EtcdPrefix    string
	EtcdUsername  string
	EtcdPassword  string
	EtcdTimeout   time.Duration

	// Target exporter configuration
	TargetURL      string
	ServiceName    string
	ExporterType   string
	CustomLabels   map[string]string
	RegistrationTTL time.Duration
	Password       string // Password for basic auth, used in ServiceGroupSpec

	// Optional settings
	RetryAttempts int
	RetryDelay    time.Duration

	// Proxy configuration
	DisableProxy bool              // Whether to disable proxy (default: false)
	ProxyPort    int               // Port for proxy server
}

// ServiceStatus represents the current state of the service
type ServiceStatus string

const (
	StatusStarting ServiceStatus = "starting"
	StatusHealthy  ServiceStatus = "healthy"
	StatusDegraded ServiceStatus = "degraded"
	StatusShutdown ServiceStatus = "shutdown"
)

// RegistrationInfo contains the information needed for service registration
type RegistrationInfo struct {
	ID           string
	URL          string
	Labels       map[string]string
	Status       ServiceStatus
	LastChecked  time.Time
	HealthStatus bool
}
