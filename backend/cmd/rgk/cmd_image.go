package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/regikeep/rgk/internal/registry"
	"github.com/regikeep/rgk/internal/store"
)

// parseImageRef splits an image reference into (registry, repo, tag).
// If the reference contains a "." before the first "/" it is treated as the registry host.
// The tag is extracted after ":". Returns empty strings for missing parts.
func parseImageRef(ref string) (registryHost, repo, tag string) {
	// Extract tag
	colonIdx := strings.LastIndex(ref, ":")
	slashIdx := strings.Index(ref, "/")

	if colonIdx > 0 && (slashIdx < 0 || colonIdx > slashIdx) {
		tag = ref[colonIdx+1:]
		ref = ref[:colonIdx]
	}

	// Determine if there is a registry host prefix (contains a dot before first slash)
	slashIdx = strings.Index(ref, "/")
	if slashIdx > 0 {
		possibleHost := ref[:slashIdx]
		if strings.Contains(possibleHost, ".") || strings.Contains(possibleHost, ":") {
			registryHost = possibleHost
			repo = ref[slashIdx+1:]
			return
		}
	}
	repo = ref
	return
}

// addCmd returns the `rgk add` command.
func addCmd() *cobra.Command {
	var group string
	var tagPattern string
	var registryFlag string

	cmd := &cobra.Command{
		Use:   "add <image>",
		Short: "Register an image for keepalive tracking",
		Long: `Add a container image reference to RegiKeep for keepalive tracking.

The image format is registry/repo:tag (e.g. iad.ocir.io/mytenancy/myapp:v1.0.0) or repo:tag.
If a registry adapter is configured for the registry host, the digest will be resolved automatically.
Use --registry to explicitly name which configured registry this image belongs to.`,
		Example: `  rgk add iad.ocir.io/recvue/myapp:v1.0.0 --group production
  rgk add 123456789.dkr.ecr.us-east-1.amazonaws.com/myapp:latest --group prod --registry ecr
  rgk add myapp:latest --group staging --registry ocir`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			imageArg := args[0]
			if group == "" {
				fmt.Fprintln(os.Stderr, "Error: --group is required")
				os.Exit(1)
			}

			cfg := openConfig()
			db := mustGetDB()
			defer db.Close()

			grp, err := db.GetGroupByName(group)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}
			if grp == nil {
				fmt.Fprintf(os.Stderr, "Error: group '%s' not found. Create it first with 'rgk group create'\n", group)
				os.Exit(1)
			}

			registryHost, repo, tag := parseImageRef(imageArg)
			if repo == "" {
				fmt.Fprintln(os.Stderr, "Error: invalid image reference:", imageArg)
				os.Exit(1)
			}

			regMgr := buildRegMgr(cfg)

			if tagPattern != "" {
				fmt.Printf("Tag pattern filtering is noted (pattern: %s); adding specified tag.\n", tagPattern)
			}

			// Determine registry ID: explicit flag takes priority over endpoint auto-detection
			digest := ""
			registryID := registryFlag

			if registryID != "" {
				// Explicit registry specified — just resolve digest
				adapter, adErr := regMgr.Get(registryID)
				if adErr != nil {
					fmt.Fprintf(os.Stderr, "Warning: registry '%s' not found in configured registries — tracking without adapter\n", registryID)
				} else {
					d, dErr := adapter.ResolveDigest(repo, tag)
					if dErr == nil && d != "" {
						digest = d
					} else if dErr != nil {
						fmt.Fprintf(os.Stderr, "Warning: digest resolution failed — %s\n", dErr)
					}
				}
			} else if registryHost != "" {
				// Auto-detect from endpoint
				reg, _ := db.GetRegistryByEndpoint(registryHost)
				if reg != nil {
					adapter, adErr := regMgr.Get(reg.Name)
					if adErr == nil {
						registryID = adapter.ID()
						d, dErr := adapter.ResolveDigest(repo, tag)
						if dErr == nil && d != "" {
							digest = d
						} else if dErr != nil {
							fmt.Fprintf(os.Stderr, "Warning: digest resolution failed — %s\n", dErr)
						}
					} else {
						fmt.Fprintf(os.Stderr, "Warning: registry '%s' found but no adapter loaded (type=%s)\n", reg.Endpoint, reg.RegistryType)
					}
				} else {
					fmt.Fprintf(os.Stderr, "Warning: no registry configured for host '%s'. Use --registry <name> or add it with:\n  rgk config add-registry %s\n", registryHost, registryHost)
				}
			}

			if digest == "" && (registryHost != "" || registryFlag != "") {
				fmt.Fprintln(os.Stderr, "Tracking without digest.")
			}

			img := store.ImageRef{
				Registry:      registryID,
				Repo:          repo,
				Tag:           tag,
				Digest:        digest,
				GroupID:       &grp.ID,
				ExpiresInDays: -1,
			}

			created, err := db.CreateImage(img)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}

			digestStr := created.Digest
			if digestStr == "" {
				digestStr = "(none)"
			}
			fmt.Printf("Added %s:%s (digest: %s) to group '%s'\n", repo, tag, digestStr, group)
		},
	}

	cmd.Flags().StringVar(&group, "group", "", "Group name to assign the image to (required)")
	cmd.Flags().StringVar(&tagPattern, "tag-pattern", "", "Optional glob pattern to list matching tags from the registry")
	cmd.Flags().StringVar(&registryFlag, "registry", "", "Registry name to associate this image with (e.g. ecr, ocir)")
	_ = cmd.MarkFlagRequired("group")
	return cmd
}

// pushCmd returns the `rgk push` command.
func pushCmd() *cobra.Command {
	var group string
	var toRegistry string
	var ecrRepo string

	cmd := &cobra.Command{
		Use:   "push <image>",
		Short: "Pull an image from Docker Hub and push it to a target registry",
		Long: `Pull an image from Docker Hub and push it to a target registry (e.g. ECR).
The image is then registered in RegiKeep for keepalive tracking.`,
		Example: `  rgk push sainath2/myapp:latest --to ecr --group production
  rgk push nginx:latest --to ecr --group staging --ecr-repo mycompany/nginx`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			imageArg := args[0]
			if group == "" {
				fmt.Fprintln(os.Stderr, "Error: --group is required")
				os.Exit(1)
			}
			if toRegistry == "" {
				fmt.Fprintln(os.Stderr, "Error: --to is required (target registry name, e.g. 'ecr')")
				os.Exit(1)
			}

			cfg := openConfig()
			db := mustGetDB()
			defer db.Close()

			grp, err := db.GetGroupByName(group)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}
			if grp == nil {
				fmt.Fprintf(os.Stderr, "Error: group '%s' not found. Create it first with 'rgk group create'\n", group)
				os.Exit(1)
			}

			_, srcRepo, srcTag := parseImageRef(imageArg)
			if srcRepo == "" {
				fmt.Fprintln(os.Stderr, "Error: invalid image reference:", imageArg)
				os.Exit(1)
			}
			if srcTag == "" {
				srcTag = "latest"
			}

			// Determine destination repo name
			dstRepo := ecrRepo
			if dstRepo == "" {
				// Use the source repo name as-is
				dstRepo = srcRepo
			}

			// Build registry manager and get adapters
			regMgr := buildRegMgr(cfg)

			// Get or create DockerHub adapter (source)
			srcAdapter, srcErr := regMgr.Get("dockerhub")
			var dhAdapter *registry.DockerHubAdapter
			if srcErr != nil {
				// No dockerhub configured — create unauthenticated adapter for public images
				dhAdapter = registry.NewDockerHubAdapter("dockerhub", "")
			} else {
				var ok bool
				dhAdapter, ok = srcAdapter.(*registry.DockerHubAdapter)
				if !ok {
					fmt.Fprintln(os.Stderr, "Error: 'dockerhub' registry is not a DockerHub adapter")
					os.Exit(1)
				}
			}

			// Get destination adapter (must be ECR)
			dstAdapter, dstErr := regMgr.Get(toRegistry)
			if dstErr != nil {
				fmt.Fprintf(os.Stderr, "Error: target registry '%s' not found. Configure it first.\n", toRegistry)
				os.Exit(1)
			}
			ecrAdapter, ok := dstAdapter.(*registry.ECRAdapter)
			if !ok {
				fmt.Fprintf(os.Stderr, "Error: target registry '%s' is not an ECR registry\n", toRegistry)
				os.Exit(1)
			}

			// Authenticate ECR
			if err := ecrAdapter.Authenticate(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: ECR authentication failed: %v\n", err)
				os.Exit(1)
			}

			// Perform cross-registry copy
			fmt.Printf("Copying %s:%s → ECR %s:%s\n", srcRepo, srcTag, dstRepo, srcTag)
			result, err := registry.CopyDockerHubToECR(dhAdapter, ecrAdapter, srcRepo, srcTag, dstRepo, func(msg string) {
				fmt.Println(msg)
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			// Register in RegiKeep DB
			img := store.ImageRef{
				Registry:      toRegistry,
				Repo:          dstRepo,
				Tag:           srcTag,
				Digest:        result.Digest,
				GroupID:       &grp.ID,
				ExpiresInDays: -1,
			}

			created, err := db.CreateImage(img)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error registering image:", err)
				os.Exit(1)
			}

			digestStr := created.Digest
			if digestStr == "" {
				digestStr = "(none)"
			}
			fmt.Printf("\n%s Pushed %s:%s → ECR %s:%s\n", colorGreen("✓"), srcRepo, srcTag, dstRepo, srcTag)
			fmt.Printf("  Digest: %s\n", digestStr)
			fmt.Printf("  Blobs copied: %d, skipped: %d, total: %d bytes\n", result.BlobsCopied, result.BlobsSkipped, result.TotalBytes)
			fmt.Printf("  Tracked in group '%s'\n", group)
		},
	}

	cmd.Flags().StringVar(&group, "group", "", "Group name to assign the image to (required)")
	cmd.Flags().StringVar(&toRegistry, "to", "", "Target registry name (e.g. 'ecr')")
	cmd.Flags().StringVar(&ecrRepo, "ecr-repo", "", "Override destination repository name in ECR")
	_ = cmd.MarkFlagRequired("group")
	_ = cmd.MarkFlagRequired("to")
	return cmd
}

// searchCmd returns the `rgk search` command.
func searchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search Docker Hub for images",
		Example: `  rgk search nginx
  rgk search sainath2/myapp`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			query := args[0]

			adapter := registry.NewDockerHubAdapter("dockerhub", "")
			results, err := adapter.SearchImages(query)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}

			if len(results) == 0 {
				fmt.Println("No results found.")
				return
			}

			tw := tableWriter()
			fmt.Fprintf(tw, "NAME\tDESCRIPTION\tSTARS\tOFFICIAL\n")
			for _, r := range results {
				desc := r.Description
				if len(desc) > 60 {
					desc = desc[:57] + "..."
				}
				official := ""
				if r.IsOfficial {
					official = "[official]"
				}
				fmt.Fprintf(tw, "%s\t%s\t%d\t%s\n", r.Name, desc, r.StarCount, official)
			}
			tw.Flush()
		},
	}
	return cmd
}

// removeCmd returns the `rgk remove` command.
func removeCmd() *cobra.Command {
	var id string

	cmd := &cobra.Command{
		Use:   "remove <image>",
		Short: "Stop tracking an image",
		Long: `Remove an image from RegiKeep tracking.

The image argument can be repo:tag or a full image reference. If multiple images match,
all matches are listed and you must re-run with --id <id> to specify the exact one.`,
		Example: `  rgk remove myapp:v1.0.0
  rgk remove myapp:v1.0.0 --id <uuid>`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			imageArg := args[0]
			db := mustGetDB()
			defer db.Close()

			// If --id specified, delete directly
			if id != "" {
				img, err := db.GetImage(id)
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error:", err)
					os.Exit(1)
				}
				if img == nil {
					fmt.Fprintf(os.Stderr, "Error: image with id '%s' not found\n", id)
					os.Exit(1)
				}
				if err := db.DeleteImage(id); err != nil {
					fmt.Fprintln(os.Stderr, "Error:", err)
					os.Exit(1)
				}
				fmt.Printf("Removed %s:%s from tracking\n", img.Repo, img.Tag)
				return
			}

			// Search by repo:tag
			_, repo, tag := parseImageRef(imageArg)
			all, err := db.ListImages(store.ImageFilter{Search: repo})
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}

			var matches []store.ImageRef
			for _, img := range all {
				if img.Repo == repo && (tag == "" || img.Tag == tag) {
					matches = append(matches, img)
				}
			}

			if len(matches) == 0 {
				fmt.Fprintf(os.Stderr, "Error: no tracked image found matching '%s'\n", imageArg)
				os.Exit(1)
			}
			if len(matches) > 1 {
				fmt.Fprintf(os.Stderr, "Multiple images match '%s'. Re-run with --id <id>:\n", imageArg)
				for _, m := range matches {
					fmt.Fprintf(os.Stderr, "  id=%s  %s:%s\n", m.ID, m.Repo, m.Tag)
				}
				os.Exit(1)
			}

			target := matches[0]
			if err := db.DeleteImage(target.ID); err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}
			fmt.Printf("Removed %s:%s from tracking\n", target.Repo, target.Tag)
		},
	}

	cmd.Flags().StringVar(&id, "id", "", "Exact image UUID (required when multiple images match)")
	return cmd
}
