package sarif

import (
	"encoding/json"
	"fmt"

	"github.com/kusaridev/kusari-cli/v2/api"
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
	Properties       map[string]any                `json:"properties,omitempty"`
}

type SarifResult struct {
	RuleID     string                        `json:"ruleId"`
	Level      string                        `json:"level,omitempty"` // "error", "warning", "note", "none"
	Message    SarifMessage                  `json:"message"`
	Help       SarifMultiformatMessageString `json:"help,omitempty"`
	HelpUri    string                        `json:"helpUri,omitempty"`
	Locations  []SarifLocation               `json:"locations,omitempty"`
	Properties map[string]any                `json:"properties,omitempty"`
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
func ConvertToSARIF(analysis *api.SecurityAnalysis, consoleUrl string) (string, error) {
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
	messageText, messageMarkdown := buildMessage(analysis, consoleUrl)

	// Mitigations are only reported when the verdict blocks.
	//
	// should_proceed is the backend's verdict and it has already weighed the
	// mitigations it attached. When it says proceed, those items are advisory --
	// listing them as "required" beside a "safe to proceed" recommendation reads
	// as a contradiction and sends people hunting for a blocker that does not
	// exist.
	emitMitigations := !analysis.ShouldProceed

	codeCount, depCount := 0, 0
	if emitMitigations {
		codeCount = len(analysis.RequiredCodeMitigations)
		depCount = len(analysis.RequiredDependencyMitigations)
	}

	// Add overall analysis result
	overallResult := SarifResult{
		RuleID: "security-analysis",
		Level:  getLevel(analysis.ShouldProceed, codeCount, depCount),
		Message: SarifMessage{
			Text:     messageText,
			Markdown: messageMarkdown,
		},
		Help: SarifMultiformatMessageString{
			Text: "View your full detailed results here",
		},
		HelpUri: consoleUrl,
		// health_score is deliberately absent. It is a 0-5 rating produced only by
		// full repository risk checks (see api.SecurityAnalysis.HealthScore), and
		// this function is only ever reached for diff scans -- the full-scan path
		// returns earlier via printFullScanResults. So the value was always the
		// unpopulated zero, which reads as "health: 0 out of 5" rather than "not
		// applicable" and contradicted the accompanying recommendation.
		Properties: map[string]interface{}{
			"should_proceed":  analysis.ShouldProceed,
			"failed_analysis": analysis.FailedAnalysis,
			"justification":   analysis.Justification,
		},
	}

	// Only add recommendation to properties if it's not empty
	if analysis.Recommendation != "" {
		overallResult.Properties["recommendation"] = analysis.Recommendation
	}

	sarifLog.Runs[0].Results = append(sarifLog.Runs[0].Results, overallResult)

	if !emitMitigations {
		return marshalSarif(sarifLog)
	}

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
			Properties: map[string]any{
				"type":        "code",
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
			Properties: map[string]any{
				"type": "dependency",
			},
		}
		sarifLog.Runs[0].Results = append(sarifLog.Runs[0].Results, result)
	}

	return marshalSarif(sarifLog)
}

// marshalSarif renders the SARIF document.
func marshalSarif(sarifLog SarifLog) (string, error) {
	jsonBytes, err := json.MarshalIndent(sarifLog, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal sarif: %w", err)
	}

	return string(jsonBytes), nil
}

// buildMessage creates the message text and markdown from the analysis
// Handles cases where recommendation might be empty
func buildMessage(analysis *api.SecurityAnalysis, consoleUrl string) (text string, markdown string) {
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

	// Add console URL link if provided
	if consoleUrl != "" {
		markdown = fmt.Sprintf("%s\n\n[View your full detailed results here](%s)", markdown, consoleUrl)
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
