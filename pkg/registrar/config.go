package registrar

import "time"

const (
	defaultEtcdTimeout     = 10 * time.Minute  // Increased timeout for stability
	defaultRegistrationTTL = 15 * time.Minute  // Increased TTL to reduce lease operations
	defaultRetryAttempts   = 3
	defaultRetryDelay     = 5 * time.Second
)
