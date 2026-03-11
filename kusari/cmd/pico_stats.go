// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/kusaridev/kusari-cli/pkg/pico"
	"github.com/spf13/cobra"
)

func stats() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Get vulnerability statistics",
		Long:  "Get aggregate statistics about vulnerabilities including counts by severity",
		RunE: func(cmd *cobra.Command, args []string) error {
			if platformTenant == "" {
				return fmt.Errorf("no tenant configured. Use --tenant flag or run `kusari auth login` to select a tenant")
			}

			client := pico.NewClient(platformTenant)

			ctx := context.Background()
			result, err := client.GetStats(ctx)
			if err != nil {
				return fmt.Errorf("failed to fetch stats: %w", err)
			}

			// Pretty print JSON
			var formatted interface{}
			if err := json.Unmarshal(result, &formatted); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			output, err := json.MarshalIndent(formatted, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to format output: %w", err)
			}

			fmt.Println(string(output))
			return nil
		},
	}

	return cmd
}
