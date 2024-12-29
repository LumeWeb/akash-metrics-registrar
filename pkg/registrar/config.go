package registrar

import "time"

const (
	defaultEtcdTimeout     = 10 * time.Minute  // Increased timeout to reduce connection churn
	defaultRegistrationTTL = 15 * time.Minute  // Further increased TTL to reduce lease operations
	defaultRetryAttempts   = 3
	defaultRetryDelay     = 5 * time.Second
)
