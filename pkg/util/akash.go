package util

import (
	"os"
	"strings"
)

// NodeIdentifiers contains Akash deployment identification info
type NodeIdentifiers struct {
	HashID       string // From ingress hostname
	DeploymentID string // From AKASH_DEPLOYMENT_SEQUENCE
}

// GetNodeIdentifiers extracts node identifiers from Akash environment
func GetNodeIdentifiers(host string) NodeIdentifiers {
	hashID := extractHashFromHost(host)
	deploymentID := os.Getenv("AKASH_DEPLOYMENT_SEQUENCE")

	return NodeIdentifiers{
		HashID:       hashID,
		DeploymentID: deploymentID,
	}
}

// ExtractHashFromHost gets the hash ID from an Akash hostname
func extractHashFromHost(host string) string {
	parts := strings.Split(host, ".")
	if len(parts) > 0 {
		return parts[0]
	}
	return host
}
