// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reporter

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/ginkgo/v2/types"
)

// TestSuiteMetadata describes the metadata of a whole test suite with all tests.
type TestSuiteMetadata struct {
	Name     string      `json:"name"`
	Phase    ESSpecPhase `json:"phase"`
	Tests    int         `json:"tests"`
	Failures int         `json:"failures"`
	Errors   int         `json:"errors"`
	Duration float64     `json:"duration"`
}

// TestCase is one instance of a test execution
type TestCase struct {
	Metadata       *TestSuiteMetadata `json:"suite"`
	Name           string             `json:"name"`
	ShortName      string             `json:"shortName"`
	Labels         []string           `json:"labels,omitempty"`
	Phase          ESSpecPhase        `json:"phase"`
	FailureMessage *FailureMessage    `json:"failure,omitempty"`
	Duration       float64            `json:"duration"`
	SystemOut      string             `json:"system-out,omitempty"`
}

// FailureMessage describes the error and the log output if an error occurred.
type FailureMessage struct {
	Type    ESSpecPhase `json:"type"`
	Message string      `json:"message"`
}

// ESSpecPhase represents the phase of a test
type ESSpecPhase string

const (
	// SpecPhaseUnknown is a test that unknown or skipped
	SpecPhaseUnknown ESSpecPhase = "Unknown"
	// SpecPhasePending is a test that is still running
	SpecPhasePending ESSpecPhase = "Pending"
	// SpecPhaseSucceeded is a successfully completed test
	SpecPhaseSucceeded ESSpecPhase = "Succeeded"
	// SpecPhaseFailed is a failed test
	SpecPhaseFailed ESSpecPhase = "Failed"
	// SpecPhaseInterrupted is a test which execution time was longer than the specified timeout
	SpecPhaseInterrupted ESSpecPhase = "Interrupted"
)

// GardenerESReporter is a custom ginkgo exporter for gardener integration tests that write a summary of the tests in an
// elastic search json report.
type GardenerESReporter struct {
	suite         *TestSuiteMetadata
	testCases     []TestCase
	testSuiteName string

	filename string
	index    []byte
}

var matchLabel = regexp.MustCompile(`\\[(.*?)\\]`)

// newGardenerESReporter creates a new Gardener elasticsearch reporter.
// Any report will be encoded to json and stored to the passed filename in the given es index.
func newGardenerESReporter(filename, index string) *GardenerESReporter {
	reporter := &GardenerESReporter{
		filename:  filename,
		testCases: []TestCase{},
	}
	if index != "" {
		reporter.index = append([]byte(getESIndexString(index)), []byte("\n")...)
	}

	return reporter
}

// ReportResults implements reporting based on the ginkgo v2 Report type
// while maintaining the existing structure of the elastic index.
// ReportsResults is intended to be called once in an ReportAfterSuite node.
func ReportResults(filename, index string, report ginkgo.Report) {
	reporter := newGardenerESReporter(filename, index)
	reporter.processReport(report)
	reporter.storeResults()
}

func (reporter *GardenerESReporter) processReport(report ginkgo.Report) {
	reporter.suite = &TestSuiteMetadata{
		Name:  report.SuiteDescription,
		Phase: SpecPhaseSucceeded,
	}
	reporter.testSuiteName = report.SuiteDescription

	for _, spec := range report.SpecReports {
		// do not report skipped tests
		if spec.State == types.SpecStateSkipped || spec.State == types.SpecStatePending {
			continue
		}

		var componentTexts []string
		componentTexts = append(componentTexts, spec.ContainerHierarchyTexts...)
		componentTexts = append(componentTexts, spec.LeafNodeText)
		testCaseName := strings.Join(componentTexts[1:], " ")
		testCase := TestCase{
			Metadata:  reporter.suite,
			Name:      testCaseName,
			ShortName: getShortName(componentTexts[len(componentTexts)-1]),
			Phase:     PhaseForState(spec.State),
			Labels:    parseLabels(testCaseName),
		}

		if spec.State == types.SpecStateFailed || spec.State == types.SpecStateInterrupted || spec.State == types.SpecStatePanicked {
			if spec.State == types.SpecStateFailed {
				reporter.suite.Failures++
			} else {
				reporter.suite.Errors++
			}

			testCase.FailureMessage = &FailureMessage{
				Type:    PhaseForState(spec.State),
				Message: failureMessage(spec.Failure),
			}
			testCase.SystemOut = spec.CombinedOutput()
		}

		testCase.Duration = spec.RunTime.Seconds()
		reporter.testCases = append(reporter.testCases, testCase)
	}

	if reporter.suite.Failures != 0 || reporter.suite.Errors != 0 {
		reporter.suite.Phase = SpecPhaseFailed
	}

	reporter.suite.Tests = report.PreRunStats.SpecsThatWillRun
	reporter.suite.Duration = math.Trunc(report.RunTime.Seconds()*1000) / 1000
}

func (reporter *GardenerESReporter) storeResults() {
	dir := filepath.Dir(reporter.filename)
	if _, err := os.Stat(dir); err != nil {
		if !os.IsNotExist(err) {
			fmt.Printf("Failed to create report file: %s\n", err.Error())
			return
		}

		if err := os.MkdirAll(dir, 0750); err != nil {
			fmt.Printf("Failed to create report directory %s: %s\n", dir, err.Error())
			return
		}
	}

	file, err := os.Create(reporter.filename)
	if err != nil {
		fmt.Printf("Failed to create report file: %s\n\t%s", reporter.filename, err.Error())
	}

	defer func() {
		if err := file.Close(); err != nil {
			fmt.Printf("unable to close report file: %s", err.Error())
		}
	}()

	encoder := json.NewEncoder(file)
	for _, testCase := range reporter.testCases {
		if len(reporter.index) != 0 {
			if _, err := file.Write(reporter.index); err != nil {
				fmt.Printf("Failed to write index: %s", err.Error())
				return
			}
		}

		err = encoder.Encode(testCase)
		if err != nil {
			fmt.Printf("Failed to generate report\n\t%s", err.Error())
			return
		}
	}
}

func failureMessage(failure types.Failure) string {
	return fmt.Sprintf("%s\n%s\n%s", failure.FailureNodeLocation.String(), failure.Message, failure.Location.String())
}

// parseLabels returns all labels of a test that have the format [<label>]
func parseLabels(name string) []string {
	labels := matchLabel.FindAllString(name, -1)
	for i, label := range labels {
		labels[i] = strings.Trim(label, "[]")
	}
	return labels
}

// getShortName removes all labels from the test name
func getShortName(name string) string {
	short := matchLabel.ReplaceAllString(name, "")
	return strings.TrimSpace(short)
}

// getESIndexString returns a bulk index configuration string for an index.
func getESIndexString(index string) string {
	format := `{ "index": { "_index": "%s", "_type": "_doc" } }`
	return fmt.Sprintf(format, index)
}

// PhaseForState maps ginkgo spec states to internal elasticsearch used phases
func PhaseForState(state types.SpecState) ESSpecPhase {
	switch state {
	case types.SpecStatePending:
		return SpecPhasePending
	case types.SpecStatePassed:
		return SpecPhaseSucceeded
	case types.SpecStateFailed:
		return SpecPhaseFailed
	case types.SpecStateInterrupted:
		return SpecPhaseInterrupted
	case types.SpecStatePanicked:
		return SpecPhaseFailed
	default:
		return SpecPhaseUnknown
	}
}
