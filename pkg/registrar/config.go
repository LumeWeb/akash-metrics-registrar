package registrar

import "time"

const (
	defaultEtcdTimeout     = 120 * time.Second
	defaultRegistrationTTL = 30 * time.Second
	defaultRetryAttempts   = 3
	defaultRetryDelay     = 5 * time.Second
)
