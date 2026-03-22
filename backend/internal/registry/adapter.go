package registry

import "github.com/regikeep/rgk/internal/store"

// Strategy represents a keepalive strategy.
type Strategy string

const (
	StrategyPull   Strategy = "pull"
	StrategyRetag  Strategy = "retag"
	StrategyNative Strategy = "native"
)

// KeepaliveResult holds the outcome of a single keepalive attempt.
type KeepaliveResult struct {
	Success   bool
	NewExpiry string // ISO8601 or empty
	Error     error
}

// Adapter is the interface every registry backend must implement.
type Adapter interface {
	// ID returns the registry identifier (e.g. "ocir-fra").
	ID() string

	// Authenticate validates credentials and prepares the client.
	Authenticate() error

	// ResolveDigest returns the current digest for the given repo+tag.
	ResolveDigest(repo, tag string) (string, error)

	// Keepalive performs the keepalive action for the given image.
	Keepalive(img store.ImageRef, strategy Strategy) (KeepaliveResult, error)

	// ApplyNativeProtection applies a registry-native lock/exempt/immutability.
	ApplyNativeProtection(img store.ImageRef) error

	// ListImages lists all images (repo+tag+digest) in the registry.
	ListImages(repo string) ([]store.ImageRef, error)

	// SupportsNativeProtection returns true if this adapter can apply native protection.
	SupportsNativeProtection() bool
}

// Manifest represents a pulled OCI/Docker manifest with its content type.
type Manifest struct {
	MediaType string
	Body      []byte
	Digest    string
}

// BlobDescriptor identifies a single blob (layer or config) within a manifest.
type BlobDescriptor struct {
	MediaType string
	Digest    string
	Size      int64
}

// SearchResult holds a single result from a Docker Hub image search.
type SearchResult struct {
	Name        string
	Description string
	StarCount   int
	IsOfficial  bool
}

// ErrNotImplemented is returned by stub adapters for unimplemented methods.
type ErrNotImplemented struct {
	Method   string
	Registry string
}

func (e ErrNotImplemented) Error() string {
	return e.Registry + ": " + e.Method + " not yet implemented"
}
