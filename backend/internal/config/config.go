package config

import (
	"os"
	"strconv"
	"strings"
)

// Config holds all runtime configuration loaded from environment variables.
type Config struct {
	// Server
	ListenAddr     string
	AllowedOrigins []string

	// Database
	DBPath string

	// OCIR
	OCIREndpoint        string
	OCIRTenancy         string
	OCIRUsername        string
	OCIRAuthToken       string
	OCIRRegion          string
	OCIRCompartmentOCID string

	// AWS ECR / S3
	AWSAccessKeyID     string
	AWSSecretAccessKey string
	AWSRegion          string
	ECRAccountID       string

	// Docker Hub
	DockerHubUsername    string
	DockerHubAccessToken string
	DockerHubNamespace   string

	// Archive — S3
	ArchiveS3Bucket string
	ArchiveS3Region string
	ArchiveS3Prefix string

	// Archive — OCI Object Storage
	ArchiveOCIBucket    string
	ArchiveOCINamespace string
	ArchiveOCIRegion    string

	// Trivy
	TrivyServerURL string

	// Daemon
	DaemonWorkers   int
	DaemonAutoStart bool
}

// Load reads configuration from environment variables.
func Load() *Config {
	c := &Config{
		ListenAddr:      getEnv("LISTEN_ADDR", ":8080"),
		AllowedOrigins:  splitEnv("ALLOWED_ORIGINS", "*"),
		DBPath:          getEnv("DB_PATH", "/data/regikeep.db"),
		OCIREndpoint:    getEnv("OCIR_ENDPOINT", ""),
		OCIRTenancy:     getEnv("OCIR_TENANCY", ""),
		OCIRUsername:    getEnv("OCIR_USERNAME", ""),
		OCIRAuthToken:   getEnv("OCIR_AUTH_TOKEN", ""),
		OCIRRegion:      getEnv("OCIR_REGION", ""),
		OCIRCompartmentOCID: getEnv("OCIR_COMPARTMENT_OCID", ""),
		AWSAccessKeyID:     getEnv("AWS_ACCESS_KEY_ID", ""),
		AWSSecretAccessKey: getEnv("AWS_SECRET_ACCESS_KEY", ""),
		AWSRegion:          getEnv("AWS_REGION", ""),
		ECRAccountID:       getEnv("ECR_ACCOUNT_ID", ""),
		DockerHubUsername:    getEnv("DOCKERHUB_USERNAME", ""),
		DockerHubAccessToken: getEnv("DOCKERHUB_ACCESS_TOKEN", ""),
		DockerHubNamespace:   getEnv("DOCKERHUB_NAMESPACE", ""),
		ArchiveS3Bucket: getEnv("ARCHIVE_S3_BUCKET", ""),
		ArchiveS3Region: getEnv("ARCHIVE_S3_REGION", ""),
		ArchiveS3Prefix: getEnv("ARCHIVE_S3_PREFIX", "/archives"),
		ArchiveOCIBucket:    getEnv("ARCHIVE_OCI_BUCKET", ""),
		ArchiveOCINamespace: getEnv("ARCHIVE_OCI_NAMESPACE", ""),
		ArchiveOCIRegion:    getEnv("ARCHIVE_OCI_REGION", ""),
		TrivyServerURL:  getEnv("TRIVY_SERVER_URL", ""),
		DaemonWorkers:   getEnvInt("DAEMON_WORKERS", 4),
		DaemonAutoStart: getEnvBool("DAEMON_AUTO_START", false),
	}
	return c
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func splitEnv(key, fallback string) []string {
	v := os.Getenv(key)
	if v == "" {
		return []string{fallback}
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func getEnvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}
