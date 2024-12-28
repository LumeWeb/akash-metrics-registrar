# Akash Metrics Registrar

A service registration manager for Akash Network Prometheus exporters that handles metrics endpoint proxying and etcd-based service discovery.

## Features

- Proxies metrics endpoints with basic authentication
- Registers services in etcd for service discovery
- Monitors target health and updates status
- Supports custom labels for service identification
- Handles Akash deployment-specific configuration
- Rate limiting for etcd operations
- Configurable retry mechanisms

## Installation

```bash
go install go.lumeweb.com/akash-metrics-registrar@latest
```

## Usage

The registrar can be configured through command line flags or environment variables:

### Required Configuration

- `--target-host` (env: TARGET_HOST): Host address of the target service
- `--service-name` (env: SERVICE_NAME): Name of the service for registration  
- `--etcd-endpoints` (env: ETCD_ENDPOINTS): Comma-separated etcd server addresses
- `--metrics-password` (env: METRICS_PASSWORD): Password for metrics basic auth

### Optional Configuration

- `--target-path` (env: TARGET_PATH): Path to metrics endpoint (default: "/metrics")
- `--target-port` (env: TARGET_PORT): Target service port (default: 9090)
- `--proxy-port` (env: PROXY_PORT): Proxy server port (default: 8080)
- `--etcd-prefix` (env: ETCD_PREFIX): Key prefix for etcd registration
- `--etcd-username` (env: ETCD_USERNAME): ETCD username
- `--etcd-password` (env: ETCD_PASSWORD): ETCD password
- `--etcd-timeout` (env: ETCD_TIMEOUT): ETCD timeout (default: 120s)
- `--registration-ttl` (env: REGISTRATION_TTL): Registration TTL (default: 30s)
- `--retry-attempts` (env: RETRY_ATTEMPTS): Number of retry attempts (default: 3)
- `--retry-delay` (env: RETRY_DELAY): Delay between retries (default: 5s)
- `--custom-labels` (env: CUSTOM_LABELS): JSON string of custom labels

### Example

```bash
akash-metrics-registrar \
  --target-host=localhost \
  --target-port=9100 \
  --service-name=node-exporter \
  --etcd-endpoints=localhost:2379 \
  --metrics-password=secret123 \
  --custom-labels='{"environment":"prod"}'
```

## Akash Deployment Support

The registrar automatically detects Akash deployment configuration:

- Uses `AKASH_EXTERNAL_PORT_*` for proper port mapping
- Configures using `AKASH_INGRESS_HOST` for correct addressing
- Adapts registration details to match Akash deployment structure

## Development

### Prerequisites

- Go 1.21+
- Access to an etcd cluster
- A metrics-producing service to proxy

### Building

```bash
go build -o akash-metrics-registrar ./cmd/metrics-registrar
```

### Testing

```bash
go test ./...
```

## License

MIT License - see [LICENSE](LICENSE) for details

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -am 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request
