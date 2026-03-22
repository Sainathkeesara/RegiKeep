package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/regikeep/rgk/internal/store"
)

// exportCmd returns the `rgk export` command.
func exportCmd() *cobra.Command {
	var format string
	var output string

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export tracked images to a file",
		Long: `Export all tracked images to oracle-json, ecr-json, or csv format.

If --output is given the result is written to that file, otherwise it is printed
to stdout.`,
		Example: `  rgk export --format oracle-json --output regikeep-export.json
  rgk export --format csv`,
		Run: func(cmd *cobra.Command, args []string) {
			db := mustGetDB()
			defer db.Close()

			imgs, err := db.ListImages(store.ImageFilter{})
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}

			// Load group names
			groups, _ := db.ListGroups()
			groupNames := make(map[string]string)
			for _, g := range groups {
				groupNames[g.ID] = g.Name
			}

			switch format {
			case "oracle-json", "ecr-json":
				exportJSON(imgs, groupNames, format, output)
			case "csv":
				exportCSV(imgs, groupNames, output)
			default:
				fmt.Fprintf(os.Stderr, "Error: unknown format '%s'. Use oracle-json, ecr-json, or csv.\n", format)
				os.Exit(1)
			}
		},
	}

	cmd.Flags().StringVar(&format, "format", "oracle-json", "Export format: oracle-json, ecr-json, or csv")
	cmd.Flags().StringVar(&output, "output", "", "Output file path (default: stdout)")
	return cmd
}

func exportJSON(imgs []store.ImageRef, groupNames map[string]string, format, output string) {
	type imageEntry struct {
		Repo          string `json:"repo"`
		Tag           string `json:"tag"`
		Digest        string `json:"digest"`
		Pinned        bool   `json:"pinned"`
		Group         string `json:"group"`
		ExpiresInDays int    `json:"expires_in_days"`
		LastKeepalive string `json:"last_keepalive"`
	}

	entries := make([]imageEntry, 0, len(imgs))
	for _, img := range imgs {
		grpName := ""
		if img.GroupID != nil {
			grpName = groupNames[*img.GroupID]
		}
		lastKA := ""
		if img.LastKeepaliveAt != nil {
			lastKA = img.LastKeepaliveAt.Format(time.RFC3339)
		}
		entries = append(entries, imageEntry{
			Repo:          img.Repo,
			Tag:           img.Tag,
			Digest:        img.Digest,
			Pinned:        img.Pinned,
			Group:         grpName,
			ExpiresInDays: img.ExpiresInDays,
			LastKeepalive: lastKA,
		})
	}

	registryName := "ocir-fra"
	if format == "ecr-json" {
		registryName = "ecr"
	}
	payload := map[string]interface{}{
		"version":     "rgk/v1",
		"registry":    registryName,
		"exported_at": time.Now().UTC().Format(time.RFC3339),
		"images":      entries,
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	if output != "" {
		if err := os.WriteFile(output, data, 0644); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		fmt.Printf("Exported %d images to %s\n", len(imgs), output)
	} else {
		fmt.Println(string(data))
	}
}

func exportCSV(imgs []store.ImageRef, groupNames map[string]string, output string) {
	header := []string{"repo", "tag", "digest", "pinned", "group", "expires_in_days", "last_keepalive"}

	writeRows := func(w *csv.Writer) {
		_ = w.Write(header)
		for _, img := range imgs {
			grpName := ""
			if img.GroupID != nil {
				grpName = groupNames[*img.GroupID]
			}
			lastKA := ""
			if img.LastKeepaliveAt != nil {
				lastKA = img.LastKeepaliveAt.Format(time.RFC3339)
			}
			_ = w.Write([]string{
				img.Repo, img.Tag, img.Digest,
				strconv.FormatBool(img.Pinned),
				grpName,
				strconv.Itoa(img.ExpiresInDays),
				lastKA,
			})
		}
		w.Flush()
	}

	if output != "" {
		f, err := os.Create(output)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		defer f.Close()
		w := csv.NewWriter(f)
		writeRows(w)
		if err := w.Error(); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		fmt.Printf("Exported %d images to %s\n", len(imgs), output)
		return
	}

	w := csv.NewWriter(os.Stdout)
	writeRows(w)
	if err := w.Error(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
