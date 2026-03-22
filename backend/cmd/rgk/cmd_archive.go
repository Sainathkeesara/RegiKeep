package main

import (
	"fmt"
	"io"
	"os"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/regikeep/rgk/internal/core"
	"github.com/regikeep/rgk/internal/store"
)

// archiveCmd returns the `rgk archive` command subtree.
func archiveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "archive",
		Short: "Archive images to cold storage",
		Long:  "Archive container images to S3 or OCI Object Storage for long-term cold storage.",
	}
	cmd.AddCommand(
		archiveRunCmd(),
		archiveListCmd(),
	)
	return cmd
}

func archiveRunCmd() *cobra.Command {
	var group string
	var image string
	var to string

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Archive a group or specific image",
		Long: `Archive container images to cold storage.

Destination format: s3://bucket/prefix or oci://namespace/bucket.
Specify either --group or --image.`,
		Example: `  rgk archive run --group production --to s3://my-bucket/archives
  rgk archive run --image myapp:v1.0.0 --to oci://mynamespace/my-bucket`,
		Run: func(cmd *cobra.Command, args []string) {
			if group == "" && image == "" {
				fmt.Fprintln(os.Stderr, "Error: either --group or --image is required")
				os.Exit(1)
			}
			if to == "" {
				fmt.Fprintln(os.Stderr, "Error: --to is required")
				os.Exit(1)
			}

			cfg := openConfig()
			db := mustGetDB()
			defer db.Close()

			// Determine storage backend from destination
			storage := "s3"
			if len(to) > 4 && to[:4] == "oci:" {
				storage = "oci-os"
			}

			silentLog := zerolog.New(io.Discard)
			archSvc := core.NewArchiveService(db, cfg, silentLog)
			regMgr := buildRegMgr(cfg)
			archSvc.SetRegistryManager(regMgr)

			req := core.ArchiveRequest{
				TargetStorage: storage,
			}

			if group != "" {
				req.Group = group
			} else {
				// Look up image by repo:tag
				_, repo, tag := parseImageRef(image)
				imgs, err := db.ListImages(store.ImageFilter{Search: repo})
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error:", err)
					os.Exit(1)
				}
				var found string
				for _, img := range imgs {
					if img.Repo == repo && img.Tag == tag {
						found = img.ID
						break
					}
				}
				if found == "" {
					fmt.Fprintf(os.Stderr, "Error: image '%s' not found in tracking\n", image)
					os.Exit(1)
				}
				req.ImageIDs = []string{found}
			}

			fmt.Printf("Archiving to %s...\n", to)
			resp, err := archSvc.Archive(req)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}

			for _, step := range resp.Steps {
				icon := colorGreen("✓")
				if step.Status != "complete" {
					icon = colorRed("✗")
				}
				detail := ""
				if step.Detail != "" {
					detail = " (" + step.Detail + ")"
				}
				fmt.Printf("  %s %s [%s]%s\n", icon, step.Step, step.Duration, detail)
			}

			if resp.Success {
				fmt.Println(colorGreen("Archive complete."))
			} else {
				fmt.Fprintln(os.Stderr, colorRed("Archive failed."))
				os.Exit(1)
			}
		},
	}

	cmd.Flags().StringVar(&group, "group", "", "Group name to archive")
	cmd.Flags().StringVar(&image, "image", "", "Image repo:tag to archive")
	cmd.Flags().StringVar(&to, "to", "", "Destination (s3://bucket/prefix or oci://namespace/bucket) (required)")
	return cmd
}

func archiveListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "List all archives",
		Long:    "Display all cold-archived images with size and status information.",
		Example: `  rgk archive list`,
		Run: func(cmd *cobra.Command, args []string) {
			db := mustGetDB()
			defer db.Close()

			archives, err := db.ListArchives()
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}

			stats, _ := db.GetArchiveStats()

			if len(archives) == 0 {
				fmt.Println("No archives found.")
				return
			}

			// Build a map from image_ref_id to repo:tag
			imgLabels := make(map[string]string)
			allImgs, err := db.ListImages(store.ImageFilter{})
			if err == nil {
				for _, img := range allImgs {
					imgLabels[img.ID] = img.Repo + ":" + img.Tag
				}
			}

			tw := tableWriter()
			fmt.Fprintln(tw, "IMAGE\tSIZE (ORIG→COMPRESSED)\tARCHIVED\tSTATUS\tBACKEND")
			for _, a := range archives {
				label := imgLabels[a.ImageRefID]
				if label == "" {
					label = a.ImageRefID[:8]
				}
				sizeStr := fmt.Sprintf("%s → %s",
					humanBytes(a.OriginalBytes),
					humanBytes(a.CompressedBytes))
				archivedAt := a.ArchivedAt.Format("2006-01-02")
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
					label, sizeStr, archivedAt, a.RestoreStatus, a.StorageBackend)
			}
			tw.Flush()

			savings := 0.0
			if stats.TotalOriginalBytes > 0 {
				savings = (1 - float64(stats.TotalCompressedBytes)/float64(stats.TotalOriginalBytes)) * 100
			}
			fmt.Printf("\nTotal: %d archives, %s → %s (%.0f%% savings)\n",
				len(archives),
				humanBytes(stats.TotalOriginalBytes),
				humanBytes(stats.TotalCompressedBytes),
				savings,
			)
		},
	}
}

// humanBytes formats a byte count in a human-readable form.
func humanBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
