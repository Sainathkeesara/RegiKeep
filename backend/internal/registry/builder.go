package registry

import (
	"github.com/regikeep/rgk/internal/config"
	"github.com/regikeep/rgk/internal/store"
)

// BuildFromDB creates a Manager populated with adapters instantiated from
// DB-registered registries. Credentials come from the DB record first,
// falling back to environment variables (via config.Config) for backward compat.
func BuildFromDB(regs []store.RegistryConfig, cfg *config.Config) *Manager {
	mgr := NewManager()

	for _, r := range regs {
		var adapter Adapter

		switch r.RegistryType {
		case "ocir":
			adapter = NewOCIRAdapter(
				r.Name,
				r.Endpoint,
				envOr(r.Tenancy, cfg.OCIRTenancy),
				envOr(r.AuthUsername, cfg.OCIRUsername),
				envOr(r.AuthToken, cfg.OCIRAuthToken),
				envOr(r.Region, cfg.OCIRRegion),
				envOr(r.AuthExtra, cfg.OCIRCompartmentOCID),
			)
		case "ecr":
			adapter = NewECRAdapterWithCreds(
				r.Name,
				envOr(r.AuthExtra, cfg.ECRAccountID),
				envOr(r.Region, cfg.AWSRegion),
				envOr(r.AuthUsername, cfg.AWSAccessKeyID),
				envOr(r.AuthToken, cfg.AWSSecretAccessKey),
			)
		case "dockerhub":
			adapter = NewDockerHubAdapterWithCreds(
				r.Name,
				envOr(r.Tenancy, cfg.DockerHubNamespace),
				envOr(r.AuthUsername, cfg.DockerHubUsername),
				envOr(r.AuthToken, cfg.DockerHubAccessToken),
			)
		default:
			continue
		}

		mgr.Register(adapter)
	}

	return mgr
}

// envOr returns val if non-empty, otherwise fallback.
func envOr(val, fallback string) string {
	if val != "" {
		return val
	}
	return fallback
}
