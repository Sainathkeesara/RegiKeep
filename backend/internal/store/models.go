package store

import "time"

// Group represents a named set of images with a shared keepalive policy.
type Group struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Interval  string    `json:"interval"`  // e.g. "7d", "24h"
	Strategy  string    `json:"strategy"`  // "pull" | "retag" | "native"
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"createdAt"`
}

// ImageRef is a container image tracked by RegiKeep.
type ImageRef struct {
	ID              string     `json:"id"`
	Registry        string     `json:"registry"`  // registry ID, e.g. "ocir-fra"
	Repo            string     `json:"repo"`
	Tag             string     `json:"tag"`
	Digest          string     `json:"digest"`
	Size            string     `json:"size"`
	GroupID         *string    `json:"groupId"`
	AddedAt         time.Time  `json:"addedAt"`
	LastKeepaliveAt *time.Time `json:"lastKeepaliveAt"`
	LastStatus      string     `json:"lastStatus"` // "safe"|"warning"|"critical"|"unpinned"
	LastError       *string    `json:"lastError"`
	ExpiresInDays   int        `json:"expiresInDays"` // -1 = unknown
	Pinned          bool       `json:"pinned"`
}

// KeepaliveLog records a single keepalive attempt.
type KeepaliveLog struct {
	ID           string    `json:"id"`
	ImageRefID   string    `json:"imageRefId"`
	RanAt        time.Time `json:"ranAt"`
	Status       string    `json:"status"` // "success" | "failure"
	DurationMs   int64     `json:"durationMs"`
	Error        *string   `json:"error"`
}

// ArchiveManifest tracks a cold-archived image in object storage.
type ArchiveManifest struct {
	ID              string     `json:"id"`
	ImageRefID      string     `json:"imageRefId"`
	Repo            string     `json:"repo"`
	Tag             string     `json:"tag"`
	Digest          string     `json:"digest"`
	Bucket          string     `json:"bucket"`
	Key             string     `json:"key"`
	LayersCount     int        `json:"layersCount"`
	OriginalBytes   int64      `json:"originalBytes"`
	CompressedBytes int64      `json:"compressedBytes"`
	ArchivedAt      time.Time  `json:"archivedAt"`
	RestoredAt      *time.Time `json:"restoredAt"`
	RestoreStatus   string     `json:"restoreStatus"` // "idle"|"restoring"|"restored"
	StorageBackend  string     `json:"storageBackend"` // "s3"|"oci-os"
}

// RegistryConfig is a registry endpoint registered with RegiKeep.
type RegistryConfig struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	RegistryType     string    `json:"registryType"`     // "ocir"|"ecr"|"dockerhub"
	Endpoint         string    `json:"endpoint"`
	Region           string    `json:"region"`
	Tenancy          string    `json:"tenancy"`
	CredentialSource string    `json:"credentialSource"` // "db"|"env"
	AuthUsername     string    `json:"authUsername,omitempty"`
	AuthToken        string    `json:"authToken,omitempty"`
	AuthExtra        string    `json:"authExtra,omitempty"` // compartment OCID (ocir) or account ID (ecr)
	CreatedAt        time.Time `json:"createdAt"`
}

// RegistryImageView is the shape the UI expects for each image in the list.
type RegistryImageView struct {
	ID            string `json:"id"`
	Repo          string `json:"repo"`
	Tag           string `json:"tag"`
	Digest        string `json:"digest"`
	Region        string `json:"region"`
	Size          string `json:"size"`
	Group         string `json:"group"`
	Pinned        bool   `json:"pinned"`
	ExpiresIn     int    `json:"expiresIn"`
	LastKeepalive string `json:"lastKeepalive"`
	Status        string `json:"status"`
	Registry      string `json:"registry"`
	LastError     string `json:"lastError,omitempty"`
}

// ArchivedImageView is the shape the UI expects for each archive entry.
type ArchivedImageView struct {
	ID              string  `json:"id"`
	Repo            string  `json:"repo"`
	Tag             string  `json:"tag"`
	CompressedSize  string  `json:"compressedSize"`
	OriginalSize    string  `json:"originalSize"`
	ArchivedAt      string  `json:"archivedAt"`
	Restorable      bool    `json:"restorable"`
	RestoreStatus   string  `json:"restoreStatus"`
	StorageBackend  string  `json:"storageBackend"`
}
