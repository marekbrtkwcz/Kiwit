package llm

import (
	"fmt"
	"regexp"
	"strings"
)

// ParsedScenario is one Gherkin scenario extracted from an LLM response.
type ParsedScenario struct {
	Name  string   `json:"name"`
	Steps []string `json:"steps"`
}

var fenceRe = regexp.MustCompile("(?s)```(?:gherkin)?\\s*(.*?)\\s*```")

// ExtractGherkin pulls the first Gherkin code block (or the whole response if
// no fence) out of an LLM answer.
func ExtractGherkin(raw string) string {
	m := fenceRe.FindStringSubmatch(raw)
	if m != nil {
		return m[1]
	}
	return strings.TrimSpace(raw)
}

// ParseScenarios splits Gherkin text into scenarios with their steps.
// Lines starting with Given/When/Then/And/But belong to the current scenario.
func ParseScenarios(gherkin string) ([]ParsedScenario, error) {
	var scenarios []ParsedScenario
	var cur *ParsedScenario
	for _, line := range strings.Split(gherkin, "\n") {
		t := strings.TrimSpace(line)
		low := strings.ToLower(t)
		switch {
		case strings.HasPrefix(low, "scenario outline:"), strings.HasPrefix(low, "scenario:"):
			name := strings.TrimSpace(t[strings.Index(t, ":")+1:])
			scenarios = append(scenarios, ParsedScenario{Name: name})
			cur = &scenarios[len(scenarios)-1]
		case strings.HasPrefix(low, "given "), strings.HasPrefix(low, "when "), strings.HasPrefix(low, "then "),
			strings.HasPrefix(low, "and "), strings.HasPrefix(low, "but "):
			if cur == nil {
				// steps before any Scenario — treat as one implicit scenario
				scenarios = append(scenarios, ParsedScenario{Name: "Scenario"})
				cur = &scenarios[len(scenarios)-1]
			}
			cur.Steps = append(cur.Steps, t)
		}
	}
	if len(scenarios) == 0 {
		return nil, fmt.Errorf("no scenarios found in LLM output")
	}
	for i := range scenarios {
		if len(scenarios[i].Steps) == 0 {
			return nil, fmt.Errorf("scenario %q has no steps", scenarios[i].Name)
		}
	}
	return scenarios, nil
}

// ScenarioToGherkinBody renders one scenario back to a Gherkin body.
func ScenarioToGherkinBody(s ParsedScenario) string {
	var b strings.Builder
	b.WriteString("Scenario: " + s.Name + "\n")
	for _, st := range s.Steps {
		b.WriteString("  " + st + "\n")
	}
	return b.String()
}

var systemPrompt = `You are a test analyst. Given a requirement description, write Gherkin test scenarios.
Rules:
- Output ONLY a fenced code block with language "gherkin".
- Start with "Feature:" line, then one or more "Scenario:" blocks.
- Use only Given/When/Then/And steps, one action per step.
- Cover happy path and the most important edge/negative cases (2-5 scenarios).
- No prose outside the code block.`

// GeneratePrompt builds the user prompt for a requirement.
func GeneratePrompt(requirementText string) string {
	return "Requirement:\n" + requirementText + "\n\nWrite Gherkin scenarios for this requirement."
}

// SystemPrompt returns the system prompt for test generation.
func SystemPrompt() string { return systemPrompt }
