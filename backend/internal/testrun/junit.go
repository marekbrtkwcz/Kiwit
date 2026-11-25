package testrun

import (
	"encoding/xml"
	"fmt"
)

type JUnitTestSuites struct {
	Suites []JUnitTestSuite `xml:"testsuite"`
}

type JUnitTestSuite struct {
	Name  string          `xml:"name,attr"`
	Cases []JUnitTestCase `xml:"testcase"`
}

type JUnitTestCase struct {
	Name      string        `xml:"name,attr"`
	Classname string        `xml:"classname,attr"`
	Failure   *JUnitMessage `xml:"failure"`
	Error     *JUnitMessage `xml:"error"`
	Skipped   *JUnitMessage `xml:"skipped"`
}

type JUnitMessage struct {
	Message string `xml:"message,attr"`
	Body    string `xml:",chardata"`
}

type ParsedCaseResult struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // passed | failed | skipped
	Comment string `json:"comment,omitempty"`
}

// ParseJUnitXML parses standard JUnit XML (either <testsuites> or single <testsuite>).
func ParseJUnitXML(data []byte) ([]ParsedCaseResult, error) {
	var suites JUnitTestSuites
	if err := xml.Unmarshal(data, &suites); err == nil && len(suites.Suites) > 0 {
		return flattenSuites(suites.Suites), nil
	}

	var single JUnitTestSuite
	if err := xml.Unmarshal(data, &single); err != nil {
		return nil, fmt.Errorf("parse junit xml: %w", err)
	}
	return flattenSuites([]JUnitTestSuite{single}), nil
}

func flattenSuites(suites []JUnitTestSuite) []ParsedCaseResult {
	results := make([]ParsedCaseResult, 0)
	for _, s := range suites {
		for _, tc := range s.Cases {
			status := "passed"
			comment := ""
			switch {
			case tc.Failure != nil:
				status = "failed"
				comment = firstNonEmpty(tc.Failure.Message, tc.Failure.Body)
			case tc.Error != nil:
				status = "failed"
				comment = firstNonEmpty(tc.Error.Message, tc.Error.Body)
			case tc.Skipped != nil:
				status = "skipped"
				comment = firstNonEmpty(tc.Skipped.Message, tc.Skipped.Body)
			}
			results = append(results, ParsedCaseResult{
				Name:    tc.Name,
				Status:  status,
				Comment: comment,
			})
		}
	}
	return results
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
