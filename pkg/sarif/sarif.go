package sarif

import (
	"encoding/json"
	"fmt"

	"github.com/kusaridev/kusari-cli/api"
)

// SARIF 2.1.0 structures
type SarifLog struct {
	Version string     `json:"version"`
	Schema  string     `json:"$schema"`
	Runs    []SarifRun `json:"runs"`
}

type SarifRun struct {
	Tool    SarifTool     `json:"tool"`
	Results []SarifResult `json:"results"`
}

type SarifTool struct {
	Driver SarifDriver `json:"driver"`
}

type SarifDriver struct {
	Name           string      `json:"name"`
	InformationUri string      `json:"informationUri,omitempty"`
	Version        string      `json:"version,omitempty"`
	Rules          []SarifRule `json:"rules,omitempty"`
}

type SarifRule struct {
	ID               string                        `json:"id"`
	ShortDescription SarifMultiformatMessageString `json:"shortDescription,omitempty"`
	FullDescription  SarifMultiformatMessageString `json:"fullDescription,omitempty"`
	Help             SarifMultiformatMessageString `json:"help,omitempty"`
	Properties       map[string]interface{}        `json:"properties,omitempty"`
}

type SarifResult struct {
	RuleID     string                 `json:"ruleId"`
	Level      string                 `json:"level,omitempty"` // "error", "warning", "note", "none"
	Message    SarifMessage           `json:"message"`
	Locations  []SarifLocation        `json:"locations,omitempty"`
	Properties map[string]interface{} `json:"properties,omitempty"`
}

type SarifMessage struct {
	Text     string `json:"text"`
	Markdown string `json:"markdown,omitempty"`
}

type SarifMultiformatMessageString struct {
	Text     string `json:"text"`
	Markdown string `json:"markdown,omitempty"`
}

type SarifLocation struct {
	PhysicalLocation SarifPhysicalLocation `json:"physicalLocation"`
}

type SarifPhysicalLocation struct {
	ArtifactLocation SarifArtifactLocation `json:"artifactLocation"`
	Region           SarifRegion           `json:"region,omitempty"`
}

type SarifArtifactLocation struct {
	URI       string `json:"uri"`
	URIBaseID string `json:"uriBaseId,omitempty"`
}

type SarifRegion struct {
	StartLine   int                   `json:"startLine,omitempty"`
	StartColumn int                   `json:"startColumn,omitempty"`
	EndLine     int                   `json:"endLine,omitempty"`
	EndColumn   int                   `json:"endColumn,omitempty"`
	Snippet     *SarifArtifactContent `json:"snippet,omitempty"`
}

type SarifArtifactContent struct {
	Text string `json:"text,omitempty"`
}

// ConvertToSARIF converts SecurityAnalysis to SARIF format
func ConvertToSARIF(analysis *api.SecurityAnalysis) (string, error) {
	sarifLog := SarifLog{
		Version: "2.1.0",
		Schema:  "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/master/Schemata/sarif-schema-2.1.0.json",
		Runs: []SarifRun{
			{
				Tool: SarifTool{
					Driver: SarifDriver{
						Name:           "Kusari Inspector",
						InformationUri: "https://www.kusari.dev/",
						Rules: []SarifRule{
							{
								ID: "security-analysis",
								ShortDescription: SarifMultiformatMessageString{
									Text: "Security Analysis Results",
								},
								FullDescription: SarifMultiformatMessageString{
									Text: "Comprehensive security analysis of code changes and dependencies",
								},
							},
							{
								ID: "code-mitigation",
								ShortDescription: SarifMultiformatMessageString{
									Text: "Required Code Mitigation",
								},
								FullDescription: SarifMultiformatMessageString{
									Text: "Code changes that must be addressed before proceeding",
								},
							},
							{
								ID: "dependency-mitigation",
								ShortDescription: SarifMultiformatMessageString{
									Text: "Required Dependency Mitigation",
								},
								FullDescription: SarifMultiformatMessageString{
									Text: "Dependency issues that must be addressed before proceeding",
								},
							},
						},
					},
				},
				Results: []SarifResult{},
			},
		},
	}

	// Determine the main message text and markdown
	messageText, messageMarkdown := buildMessage(analysis)

	// Add overall analysis result
	overallResult := SarifResult{
		RuleID: "security-analysis",
		Level:  getLevel(analysis.ShouldProceed, len(analysis.RequiredCodeMitigations), len(analysis.RequiredDependencyMitigations)),
		Message: SarifMessage{
			Text:     messageText,
			Markdown: messageMarkdown,
		},
		Properties: map[string]interface{}{
			"should_proceed":  analysis.ShouldProceed,
			"failed_analysis": analysis.FailedAnalysis,
			"health_score":    analysis.HealthScore,
			"justification":   analysis.Justification,
		},
	}

	// Only add recommendation to properties if it's not empty
	if analysis.Recommendation != "" {
		overallResult.Properties["recommendation"] = analysis.Recommendation
	}

	sarifLog.Runs[0].Results = append(sarifLog.Runs[0].Results, overallResult)

	// Add code mitigations as individual results
	for _, mitigation := range analysis.RequiredCodeMitigations {
		result := SarifResult{
			RuleID: "code-mitigation",
			Level:  "warning",
			Message: SarifMessage{
				Text: mitigation.Content,
			},
			Locations: []SarifLocation{
				{
					PhysicalLocation: SarifPhysicalLocation{
						ArtifactLocation: SarifArtifactLocation{
							URI: mitigation.Path,
						},
						Region: SarifRegion{
							StartLine: mitigation.LineNumber,
							Snippet: &SarifArtifactContent{
								Text: mitigation.Code,
							},
						},
					},
				},
			},
			Properties: map[string]interface{}{
				"line_number": mitigation.LineNumber,
			},
		}
		sarifLog.Runs[0].Results = append(sarifLog.Runs[0].Results, result)
	}

	// Add dependency mitigations as individual results
	for _, mitigation := range analysis.RequiredDependencyMitigations {
		result := SarifResult{
			RuleID: "dependency-mitigation",
			Level:  "warning",
			Message: SarifMessage{
				Text: mitigation.Content,
			},
			Properties: map[string]interface{}{
				"type": "dependency",
			},
		}
		sarifLog.Runs[0].Results = append(sarifLog.Runs[0].Results, result)
	}

	// Convert to JSON
	jsonBytes, err := json.MarshalIndent(sarifLog, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal SARIF: %w", err)
	}

	return string(jsonBytes), nil
}

// buildMessage creates the message text and markdown from the analysis
// Handles cases where recommendation might be empty
func buildMessage(analysis *api.SecurityAnalysis) (text string, markdown string) {
	// Determine what to use as the main message
	if analysis.Recommendation != "" && analysis.Justification != "" {
		text = analysis.Recommendation
		markdown = fmt.Sprintf("**Recommendation:** %s\n\n**Justification:** %s",
			analysis.Recommendation, analysis.Justification)
	} else if analysis.Recommendation != "" {
		text = analysis.Recommendation
		markdown = fmt.Sprintf("**Recommendation:** %s", analysis.Recommendation)
	} else if analysis.Justification != "" {
		text = analysis.Justification
		markdown = fmt.Sprintf("**Analysis:** %s", analysis.Justification)
	} else {
		// Fallback message if both are empty
		text = "Analysis completed"
		markdown = "**Analysis:** Completed"
	}

	return text, markdown
}

// getLevel determines the SARIF level based on the analysis
func getLevel(shouldProceed bool, codeMitigations, depMitigations int) string {
	if !shouldProceed {
		return "error"
	}
	if codeMitigations > 0 || depMitigations > 0 {
		return "warning"
	}
	return "note"
}
