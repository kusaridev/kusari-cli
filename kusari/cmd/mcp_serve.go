// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/kusaridev/kusari-cli/pkg/mcp"
	"github.com/spf13/cobra"
)

func serve() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the MCP server",
		Long: `Start the MCP server process.

This command is designed to be spawned by MCP clients (like Claude Code),
not run directly by users. The server communicates via stdio transport.

For debugging, you can run it directly with --verbose to see detailed logging.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := mcp.LoadConfig()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Override verbose from flag if set
			if cmd.Flags().Changed("verbose") {
				cfg.Verbose = verbose
			}

			server, err := mcp.NewServer(cfg)
			if err != nil {
				return fmt.Errorf("failed to create server: %w", err)
			}

			ctx := context.Background()
			if err := server.Run(ctx); err != nil {
				// Don't print error if it's just EOF (client disconnected)
				if err.Error() != "EOF" {
					fmt.Fprintln(os.Stderr, "Server error:", err)
				}
				return err
			}

			return nil
		},
	}

	return cmd
}
