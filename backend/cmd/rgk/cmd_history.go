package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/regikeep/rgk/internal/store"
)

// historyCmd returns the `rgk history` command.
func historyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history <image>",
		Short: "Show keepalive history for an image",
		Long: `Display the keepalive log for an image (last 7 days).

The image argument can be a repo:tag reference or an image UUID. UUID lookup is
attempted first; if no match is found the argument is treated as repo:tag.`,
		Example: `  rgk history myapp:v1.0.0
  rgk history iad.ocir.io/recvue/myapp:v1.0.0`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			imageArg := args[0]
			db := mustGetDB()
			defer db.Close()

			imageID, imageLabel, err := resolveImageID(db, imageArg)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}

			logs, err := db.ListKeepaliveLogs(imageID)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}

			if len(logs) == 0 {
				fmt.Printf("No keepalive history for %s.\n", imageLabel)
				return
			}

			tw := tableWriter()
			fmt.Fprintln(tw, "TIME\tSTATUS\tDURATION\tERROR")
			for _, l := range logs {
				errStr := ""
				if l.Error != nil {
					errStr = *l.Error
				}
				statusStr := l.Status
				if l.Status == "success" {
					statusStr = colorGreen(l.Status)
				} else {
					statusStr = colorRed(l.Status)
				}
				fmt.Fprintf(tw, "%s\t%s\t%dms\t%s\n",
					l.RanAt.Format("2006-01-02 15:04:05"),
					statusStr,
					l.DurationMs,
					errStr,
				)
			}
			tw.Flush()
		},
	}
	return cmd
}

// resolveImageID finds an image by UUID or repo:tag and returns (id, display label, error).
func resolveImageID(db *store.DB, ref string) (string, string, error) {
	// Try exact ID lookup first
	img, err := db.GetImage(ref)
	if err != nil {
		return "", "", err
	}
	if img != nil {
		return img.ID, img.Repo + ":" + img.Tag, nil
	}

	// Fall back to repo:tag search
	_, repo, tag := parseImageRef(ref)
	all, err := db.ListImages(store.ImageFilter{Search: repo})
	if err != nil {
		return "", "", err
	}

	var matches []store.ImageRef
	for _, i := range all {
		if i.Repo == repo && (tag == "" || i.Tag == tag) {
			matches = append(matches, i)
		}
	}

	if len(matches) == 0 {
		return "", "", fmt.Errorf("no tracked image found matching '%s'", ref)
	}
	if len(matches) > 1 {
		msg := fmt.Sprintf("multiple images match '%s'; use the image UUID instead:\n", ref)
		for _, m := range matches {
			msg += fmt.Sprintf("  %s  %s:%s\n", m.ID, m.Repo, m.Tag)
		}
		return "", "", fmt.Errorf("%s", msg)
	}
	m := matches[0]
	return m.ID, m.Repo + ":" + m.Tag, nil
}
