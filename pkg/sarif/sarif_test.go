package sarif

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/kusaridev/kusari-cli/api"
)

func TestConvertToSARIF(t *testing.T) {
	tests := []struct {
		name          string
		analysis      *api.SecurityAnalysis
		wantErr       bool
		validateFn    func(*testing.T, string)
		expectedLevel string
	}{
		{
			name: "empty analysis with should proceed",
			analysis: &api.SecurityAnalysis{
				Recommendation: "Code looks good",
				Justification:  "No issues found",
				ShouldProceed:  true,
				HealthScore:    5,
			},
			wantErr:       false,
			expectedLevel: "note",
			validateFn: func(t *testing.T, output string) {
				var sarif SarifLog
				if err := json.Unmarshal([]byte(output), &sarif); err != nil {
					t.Fatalf("Failed to parse SARIF output: %v", err)
				}

				if sarif.Version != "2.1.0" {
					t.Errorf("Expected version 2.1.0, got %s", sarif.Version)
				}

				if len(sarif.Runs) != 1 {
					t.Fatalf("Expected 1 run, got %d", len(sarif.Runs))
				}

				if len(sarif.Runs[0].Results) != 1 {
					t.Errorf("Expected 1 result, got %d", len(sarif.Runs[0].Results))
				}

				result := sarif.Runs[0].Results[0]
				if result.RuleID != "security-analysis" {
					t.Errorf("Expected ruleId 'security-analysis', got %s", result.RuleID)
				}

				if result.Message.Text != "Code looks good" {
					t.Errorf("Expected message 'Code looks good', got %s", result.Message.Text)
				}
			},
		},
		{
			name: "analysis with code mitigations",
			analysis: &api.SecurityAnalysis{
				Recommendation: "Fix security issues",
				Justification:  "Found vulnerabilities",
				ShouldProceed:  false,
				HealthScore:    2,
				RequiredCodeMitigations: []api.CodeMitigationItem{
					{
						LineNumber: 42,
						Path:       "src/main.go",
						Content:    "SQL injection vulnerability",
						Code:       "db.Query(userInput)",
					},
					{
						LineNumber: 105,
						Path:       "src/auth.go",
						Content:    "Weak password validation",
						Code:       "if len(password) > 0",
					},
				},
			},
			wantErr:       false,
			expectedLevel: "error",
			validateFn: func(t *testing.T, output string) {
				var sarif SarifLog
				if err := json.Unmarshal([]byte(output), &sarif); err != nil {
					t.Fatalf("Failed to parse SARIF output: %v", err)
				}

				// Should have 1 overall + 2 code mitigation results
				if len(sarif.Runs[0].Results) != 3 {
					t.Errorf("Expected 3 results (1 overall + 2 mitigations), got %d", len(sarif.Runs[0].Results))
				}

				// Check code mitigation results
				codeMitigationCount := 0
				for _, result := range sarif.Runs[0].Results {
					if result.RuleID == "code-mitigation" {
						codeMitigationCount++
						if result.Level != "warning" {
							t.Errorf("Expected code mitigation level 'warning', got %s", result.Level)
						}
						if len(result.Locations) != 1 {
							t.Errorf("Expected 1 location for code mitigation, got %d", len(result.Locations))
						}
					}
				}

				if codeMitigationCount != 2 {
					t.Errorf("Expected 2 code mitigation results, got %d", codeMitigationCount)
				}
			},
		},
		{
			name: "analysis with dependency mitigations",
			analysis: &api.SecurityAnalysis{
				Recommendation: "Update dependencies",
				Justification:  "Outdated packages found",
				ShouldProceed:  true,
				HealthScore:    3,
				RequiredDependencyMitigations: []api.DependencyMitigationItem{
					{Content: "Update lodash to 4.17.21"},
					{Content: "Update express to 4.18.0"},
					{Content: "Remove deprecated package xyz"},
				},
			},
			wantErr:       false,
			expectedLevel: "warning",
			validateFn: func(t *testing.T, output string) {
				var sarif SarifLog
				if err := json.Unmarshal([]byte(output), &sarif); err != nil {
					t.Fatalf("Failed to parse SARIF output: %v", err)
				}

				// Should have 1 overall + 3 dependency mitigation results
				if len(sarif.Runs[0].Results) != 4 {
					t.Errorf("Expected 4 results (1 overall + 3 mitigations), got %d", len(sarif.Runs[0].Results))
				}

				// Check dependency mitigation results
				depMitigationCount := 0
				for _, result := range sarif.Runs[0].Results {
					if result.RuleID == "dependency-mitigation" {
						depMitigationCount++
						if result.Level != "warning" {
							t.Errorf("Expected dependency mitigation level 'warning', got %s", result.Level)
						}
						if len(result.Locations) != 0 {
							t.Errorf("Expected 0 locations for dependency mitigation, got %d", len(result.Locations))
						}
						if result.Properties["type"] != "dependency" {
							t.Errorf("Expected type property 'dependency', got %v", result.Properties["type"])
						}
					}
				}

				if depMitigationCount != 3 {
					t.Errorf("Expected 3 dependency mitigation results, got %d", depMitigationCount)
				}
			},
		},
		{
			name: "analysis with both code and dependency mitigations",
			analysis: &api.SecurityAnalysis{
				Recommendation: "Multiple issues found",
				Justification:  "Both code and dependency issues detected",
				ShouldProceed:  false,
				HealthScore:    1,
				RequiredCodeMitigations: []api.CodeMitigationItem{
					{
						LineNumber: 10,
						Path:       "src/api.go",
						Content:    "Insecure random number generator",
						Code:       "rand.Seed(time.Now().Unix())",
					},
				},
				RequiredDependencyMitigations: []api.DependencyMitigationItem{
					{Content: "Update vulnerable package"},
				},
			},
			wantErr:       false,
			expectedLevel: "error",
			validateFn: func(t *testing.T, output string) {
				var sarif SarifLog
				if err := json.Unmarshal([]byte(output), &sarif); err != nil {
					t.Fatalf("Failed to parse SARIF output: %v", err)
				}

				// Should have 1 overall + 1 code + 1 dependency = 3 results
				if len(sarif.Runs[0].Results) != 3 {
					t.Errorf("Expected 3 results, got %d", len(sarif.Runs[0].Results))
				}

				// Verify we have one of each type
				ruleIDCounts := make(map[string]int)
				for _, result := range sarif.Runs[0].Results {
					ruleIDCounts[result.RuleID]++
				}

				if ruleIDCounts["security-analysis"] != 1 {
					t.Errorf("Expected 1 security-analysis result, got %d", ruleIDCounts["security-analysis"])
				}
				if ruleIDCounts["code-mitigation"] != 1 {
					t.Errorf("Expected 1 code-mitigation result, got %d", ruleIDCounts["code-mitigation"])
				}
				if ruleIDCounts["dependency-mitigation"] != 1 {
					t.Errorf("Expected 1 dependency-mitigation result, got %d", ruleIDCounts["dependency-mitigation"])
				}
			},
		},
		{
			name: "failed analysis",
			analysis: &api.SecurityAnalysis{
				Recommendation: "Analysis could not complete",
				Justification:  "Internal error",
				ShouldProceed:  false,
				FailedAnalysis: true,
				HealthScore:    0,
			},
			wantErr:       false,
			expectedLevel: "error",
			validateFn: func(t *testing.T, output string) {
				var sarif SarifLog
				if err := json.Unmarshal([]byte(output), &sarif); err != nil {
					t.Fatalf("Failed to parse SARIF output: %v", err)
				}

				result := sarif.Runs[0].Results[0]
				if !result.Properties["failed_analysis"].(bool) {
					t.Error("Expected failed_analysis property to be true")
				}
			},
		},
		{
			name: "null recommendation and mitigations with justification only",
			analysis: &api.SecurityAnalysis{
				Recommendation:                "", // NULL/empty
				Justification:                 "No pinned version dependency changes, code issues or exposed secrets detected!",
				RequiredCodeMitigations:       nil, // NULL
				RequiredDependencyMitigations: nil, // NULL
				ShouldProceed:                 true,
				FailedAnalysis:                false,
				HealthScore:                   0,
			},
			wantErr:       false,
			expectedLevel: "note",
			validateFn: func(t *testing.T, output string) {
				var sarif SarifLog
				if err := json.Unmarshal([]byte(output), &sarif); err != nil {
					t.Fatalf("Failed to parse SARIF output: %v", err)
				}

				// Should have only 1 result (overall analysis, no mitigations)
				if len(sarif.Runs[0].Results) != 1 {
					t.Errorf("Expected 1 result (no mitigations), got %d", len(sarif.Runs[0].Results))
				}

				result := sarif.Runs[0].Results[0]

				// Message should use justification when recommendation is empty
				if result.Message.Text == "" {
					t.Error("Expected non-empty message text")
				}

				if !strings.Contains(result.Message.Text, "No pinned version dependency changes") {
					t.Errorf("Expected message to contain justification text, got: %s", result.Message.Text)
				}

				// Should still be marked as note level (no issues)
				if result.Level != "note" {
					t.Errorf("Expected level 'note', got %s", result.Level)
				}

				// Verify properties - handle JSON unmarshaling float64 conversion
				if shouldProceed, ok := result.Properties["should_proceed"].(bool); !ok || shouldProceed != true {
					t.Errorf("Expected should_proceed to be true, got %v (type: %T)", result.Properties["should_proceed"], result.Properties["should_proceed"])
				}

				// JSON unmarshals numbers as float64
				if healthScore, ok := result.Properties["health_score"].(float64); !ok || healthScore != 0.0 {
					t.Errorf("Expected health_score to be 0, got %v (type: %T)", result.Properties["health_score"], result.Properties["health_score"])
				}

				if failedAnalysis, ok := result.Properties["failed_analysis"].(bool); !ok || failedAnalysis != false {
					t.Errorf("Expected failed_analysis to be false, got %v (type: %T)", result.Properties["failed_analysis"], result.Properties["failed_analysis"])
				}
			},
		},
		{
			name: "tool metadata validation",
			analysis: &api.SecurityAnalysis{
				Recommendation: "Test",
				Justification:  "Test",
				ShouldProceed:  true,
			},
			wantErr: false,
			validateFn: func(t *testing.T, output string) {
				var sarif SarifLog
				if err := json.Unmarshal([]byte(output), &sarif); err != nil {
					t.Fatalf("Failed to parse SARIF output: %v", err)
				}

				driver := sarif.Runs[0].Tool.Driver
				if driver.Name != "Kusari Inspector" {
					t.Errorf("Expected tool name 'Kusari Inspector', got %s", driver.Name)
				}

				if driver.InformationUri != "https://www.kusari.dev/" {
					t.Errorf("Expected information URI 'https://www.kusari.dev/', got %s", driver.InformationUri)
				}

				if len(driver.Rules) != 3 {
					t.Errorf("Expected 3 rules, got %d", len(driver.Rules))
				}

				// Check rule IDs
				expectedRuleIDs := map[string]bool{
					"security-analysis":     false,
					"code-mitigation":       false,
					"dependency-mitigation": false,
				}

				for _, rule := range driver.Rules {
					if _, exists := expectedRuleIDs[rule.ID]; exists {
						expectedRuleIDs[rule.ID] = true
					}
				}

				for ruleID, found := range expectedRuleIDs {
					if !found {
						t.Errorf("Expected rule %s not found", ruleID)
					}
				}
			},
		},
		{
			name: "markdown message formatting",
			analysis: &api.SecurityAnalysis{
				Recommendation: "Proceed with caution",
				Justification:  "Minor issues detected",
				ShouldProceed:  true,
				HealthScore:    4,
			},
			wantErr: false,
			validateFn: func(t *testing.T, output string) {
				var sarif SarifLog
				if err := json.Unmarshal([]byte(output), &sarif); err != nil {
					t.Fatalf("Failed to parse SARIF output: %v", err)
				}

				result := sarif.Runs[0].Results[0]
				expectedMarkdown := "**Recommendation:** Proceed with caution\n\n**Justification:** Minor issues detected"
				if result.Message.Markdown != expectedMarkdown {
					t.Errorf("Expected markdown:\n%s\n\nGot:\n%s", expectedMarkdown, result.Message.Markdown)
				}
			},
		},
		{
			name: "code mitigation with snippet",
			analysis: &api.SecurityAnalysis{
				Recommendation: "Fix code",
				Justification:  "Issues found",
				ShouldProceed:  false,
				RequiredCodeMitigations: []api.CodeMitigationItem{
					{
						LineNumber: 25,
						Path:       "pkg/handler.go",
						Content:    "XSS vulnerability",
						Code:       `fmt.Fprintf(w, "<html>%s</html>", userInput)`,
					},
				},
			},
			wantErr: false,
			validateFn: func(t *testing.T, output string) {
				var sarif SarifLog
				if err := json.Unmarshal([]byte(output), &sarif); err != nil {
					t.Fatalf("Failed to parse SARIF output: %v", err)
				}

				// Find code mitigation result
				var codeMitigation *SarifResult
				for i, result := range sarif.Runs[0].Results {
					if result.RuleID == "code-mitigation" {
						codeMitigation = &sarif.Runs[0].Results[i]
						break
					}
				}

				if codeMitigation == nil {
					t.Fatal("Code mitigation result not found")
				}

				if codeMitigation.Locations[0].PhysicalLocation.Region.StartLine != 25 {
					t.Errorf("Expected line 25, got %d", codeMitigation.Locations[0].PhysicalLocation.Region.StartLine)
				}

				if codeMitigation.Locations[0].PhysicalLocation.ArtifactLocation.URI != "pkg/handler.go" {
					t.Errorf("Expected path 'pkg/handler.go', got %s", codeMitigation.Locations[0].PhysicalLocation.ArtifactLocation.URI)
				}

				if codeMitigation.Locations[0].PhysicalLocation.Region.Snippet == nil {
					t.Error("Expected snippet to be present")
				} else {
					expectedCode := `fmt.Fprintf(w, "<html>%s</html>", userInput)`
					if codeMitigation.Locations[0].PhysicalLocation.Region.Snippet.Text != expectedCode {
						t.Errorf("Expected code snippet:\n%s\n\nGot:\n%s", expectedCode, codeMitigation.Locations[0].PhysicalLocation.Region.Snippet.Text)
					}
				}
			},
		},
		{
			name: "valid JSON output",
			analysis: &api.SecurityAnalysis{
				Recommendation: "All clear",
				Justification:  "No issues",
				ShouldProceed:  true,
			},
			wantErr: false,
			validateFn: func(t *testing.T, output string) {
				// Verify it's valid JSON
				var jsonObj interface{}
				if err := json.Unmarshal([]byte(output), &jsonObj); err != nil {
					t.Errorf("Output is not valid JSON: %v", err)
				}

				// Verify it's properly indented
				if !strings.Contains(output, "\n") {
					t.Error("Expected formatted JSON with newlines")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := ConvertToSARIF(tt.analysis)

			if (err != nil) != tt.wantErr {
				t.Errorf("ConvertToSARIF() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil && tt.validateFn != nil {
				tt.validateFn(t, output)
			}

			// Validate expected level if specified
			if tt.expectedLevel != "" && err == nil {
				var sarif SarifLog
				if err := json.Unmarshal([]byte(output), &sarif); err == nil {
					overallResult := sarif.Runs[0].Results[0]
					if overallResult.Level != tt.expectedLevel {
						t.Errorf("Expected level %s, got %s", tt.expectedLevel, overallResult.Level)
					}
				}
			}
		})
	}
}

func TestBuildMessage(t *testing.T) {
	tests := []struct {
		name             string
		analysis         *api.SecurityAnalysis
		expectedText     string
		expectedMarkdown string
	}{
		{
			name: "both recommendation and justification present",
			analysis: &api.SecurityAnalysis{
				Recommendation: "Fix the issues",
				Justification:  "Security vulnerabilities found",
			},
			expectedText:     "Fix the issues",
			expectedMarkdown: "**Recommendation:** Fix the issues\n\n**Justification:** Security vulnerabilities found",
		},
		{
			name: "only recommendation present",
			analysis: &api.SecurityAnalysis{
				Recommendation: "Proceed with deployment",
				Justification:  "",
			},
			expectedText:     "Proceed with deployment",
			expectedMarkdown: "**Recommendation:** Proceed with deployment",
		},
		{
			name: "only justification present",
			analysis: &api.SecurityAnalysis{
				Recommendation: "",
				Justification:  "No pinned version dependency changes, code issues or exposed secrets detected!",
			},
			expectedText:     "No pinned version dependency changes, code issues or exposed secrets detected!",
			expectedMarkdown: "**Analysis:** No pinned version dependency changes, code issues or exposed secrets detected!",
		},
		{
			name: "both empty - fallback message",
			analysis: &api.SecurityAnalysis{
				Recommendation: "",
				Justification:  "",
			},
			expectedText:     "Analysis completed",
			expectedMarkdown: "**Analysis:** Completed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, markdown := buildMessage(tt.analysis)

			if text != tt.expectedText {
				t.Errorf("Expected text:\n%s\n\nGot:\n%s", tt.expectedText, text)
			}

			if markdown != tt.expectedMarkdown {
				t.Errorf("Expected markdown:\n%s\n\nGot:\n%s", tt.expectedMarkdown, markdown)
			}
		})
	}
}

func TestGetLevel(t *testing.T) {
	tests := []struct {
		name            string
		shouldProceed   bool
		codeMitigations int
		depMitigations  int
		want            string
	}{
		{
			name:            "should not proceed returns error",
			shouldProceed:   false,
			codeMitigations: 0,
			depMitigations:  0,
			want:            "error",
		},
		{
			name:            "should not proceed with mitigations returns error",
			shouldProceed:   false,
			codeMitigations: 2,
			depMitigations:  1,
			want:            "error",
		},
		{
			name:            "should proceed with code mitigations returns warning",
			shouldProceed:   true,
			codeMitigations: 1,
			depMitigations:  0,
			want:            "warning",
		},
		{
			name:            "should proceed with dependency mitigations returns warning",
			shouldProceed:   true,
			codeMitigations: 0,
			depMitigations:  3,
			want:            "warning",
		},
		{
			name:            "should proceed with both mitigations returns warning",
			shouldProceed:   true,
			codeMitigations: 5,
			depMitigations:  2,
			want:            "warning",
		},
		{
			name:            "should proceed with no mitigations returns note",
			shouldProceed:   true,
			codeMitigations: 0,
			depMitigations:  0,
			want:            "note",
		},
		{
			name:            "edge case: negative mitigations treated as zero",
			shouldProceed:   true,
			codeMitigations: -1,
			depMitigations:  -1,
			want:            "note",
		},
		{
			name:            "large number of mitigations",
			shouldProceed:   true,
			codeMitigations: 100,
			depMitigations:  50,
			want:            "warning",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getLevel(tt.shouldProceed, tt.codeMitigations, tt.depMitigations)
			if got != tt.want {
				t.Errorf("getLevel(%v, %d, %d) = %v, want %v",
					tt.shouldProceed, tt.codeMitigations, tt.depMitigations, got, tt.want)
			}
		})
	}
}

func TestSARIFStructureCompliance(t *testing.T) {
	// Test that the output structure complies with SARIF 2.1.0 schema
	analysis := &api.SecurityAnalysis{
		Recommendation: "Test recommendation",
		Justification:  "Test justification",
		ShouldProceed:  true,
		HealthScore:    5,
		RequiredCodeMitigations: []api.CodeMitigationItem{
			{
				LineNumber: 1,
				Path:       "test.go",
				Content:    "Test content",
				Code:       "test code",
			},
		},
	}

	output, err := ConvertToSARIF(analysis)
	if err != nil {
		t.Fatalf("ConvertToSARIF() failed: %v", err)
	}

	var sarif SarifLog
	if err := json.Unmarshal([]byte(output), &sarif); err != nil {
		t.Fatalf("Failed to unmarshal SARIF: %v", err)
	}

	// Test required SARIF properties
	t.Run("has required version", func(t *testing.T) {
		if sarif.Version != "2.1.0" {
			t.Errorf("Expected version 2.1.0, got %s", sarif.Version)
		}
	})

	t.Run("has schema URL", func(t *testing.T) {
		if sarif.Schema == "" {
			t.Error("Schema URL should not be empty")
		}
		if !strings.HasPrefix(sarif.Schema, "http") {
			t.Error("Schema should be a valid URL")
		}
	})

	t.Run("has at least one run", func(t *testing.T) {
		if len(sarif.Runs) == 0 {
			t.Error("Expected at least one run")
		}
	})

	t.Run("run has tool with driver", func(t *testing.T) {
		if sarif.Runs[0].Tool.Driver.Name == "" {
			t.Error("Tool driver name should not be empty")
		}
	})

	t.Run("results have required properties", func(t *testing.T) {
		for i, result := range sarif.Runs[0].Results {
			if result.RuleID == "" {
				t.Errorf("Result %d has empty ruleId", i)
			}
			if result.Message.Text == "" {
				t.Errorf("Result %d has empty message text", i)
			}
		}
	})
}
