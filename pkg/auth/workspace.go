// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package auth

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// SelectWorkspace prompts the user to select a workspace from a list
func SelectWorkspace(workspaces []WorkspaceInfo) (*WorkspaceInfo, error) {
	if len(workspaces) == 0 {
		return nil, fmt.Errorf("no workspaces available")
	}

	// If there's only one workspace, auto-select it
	if len(workspaces) == 1 {
		fmt.Printf("You only have one workspace available: %s\n", workspaces[0].Description)
		fmt.Println("Auto-selecting this workspace.")
		return &workspaces[0], nil
	}

	// Display available workspaces
	fmt.Println("\nAvailable workspaces:")
	for i, ws := range workspaces {
		fmt.Printf("  [%d] %s\n", i+1, ws.Description)
	}

	// Prompt user for selection
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("\nSelect a workspace (1-%d): ", len(workspaces))
		input, err := reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("failed to read input: %w", err)
		}

		input = strings.TrimSpace(input)
		selection, err := strconv.Atoi(input)
		if err != nil || selection < 1 || selection > len(workspaces) {
			fmt.Printf("Invalid selection. Please enter a number between 1 and %d.\n", len(workspaces))
			continue
		}

		selected := &workspaces[selection-1]
		fmt.Printf("Selected workspace: %s\n", selected.Description)
		return selected, nil
	}
}

// SelectTenant prompts the user to select a tenant from a list
func SelectTenant(tenants []string) (string, error) {
	if len(tenants) == 0 {
		return "", fmt.Errorf("no tenants available")
	}

	// If there's only one tenant, auto-select it
	if len(tenants) == 1 {
		fmt.Printf("You only have one tenant available: %s\n", tenants[0])
		fmt.Println("Auto-selecting this tenant.")
		return tenants[0], nil
	}

	// Display available tenants
	fmt.Println("\nAvailable tenants:")
	for i, tenant := range tenants {
		fmt.Printf("  [%d] %s\n", i+1, tenant)
	}

	// Prompt user for selection
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("\nSelect a tenant (1-%d): ", len(tenants))
		input, err := reader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("failed to read input: %w", err)
		}

		input = strings.TrimSpace(input)
		selection, err := strconv.Atoi(input)
		if err != nil || selection < 1 || selection > len(tenants) {
			fmt.Printf("Invalid selection. Please enter a number between 1 and %d.\n", len(tenants))
			continue
		}

		selected := tenants[selection-1]
		fmt.Printf("Selected tenant: %s\n", selected)
		return selected, nil
	}
}
