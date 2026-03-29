package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if flagJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{
					"version":    version,
					"commit":     commit,
					"build_time": buildTime,
				})
			}
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "dynamoctl %s (commit %s, built %s)\n",
				version, commit, buildTime)
			return err
		},
	}
}
