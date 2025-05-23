// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reporter

import (
	"fmt"
	"math"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/ginkgo/v2/types"
	. "github.com/onsi/gomega"
)

func TestGardenerESReporter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Gardener ES Reporter Test Suite")
}

const (
	reportFileName             = "/tmp/report_test.json"
	indexName                  = "test-index"
	mockReportSuiteDescription = "mock report suite"
	testCaseName               = "[DEFAULT] [REPORT] Should complete successfully"
)

var _ = Describe("processReport tests", func() {

	var (
		reporter                    *GardenerESReporter
		mockReport                  Report
		mockContainerHierarchyTexts []string
		suiteDuration               float64
		testCaseDuration            float64
	)

	BeforeEach(func() {
		reporter = newGardenerESReporter(reportFileName, indexName)
		mockReport.SuiteDescription = mockReportSuiteDescription
		mockContainerHierarchyTexts = []string{"DESCRIBE"}
		mockReport.RunTime = time.Duration(2000000000)
		mockReport.SpecReports = []SpecReport{
			{
				ContainerHierarchyTexts:    mockContainerHierarchyTexts,
				LeafNodeText:               testCaseName,
				RunTime:                    time.Duration(1000000000),
				Failure:                    types.Failure{},
				CapturedGinkgoWriterOutput: "",
				CapturedStdOutErr:          "",
			},
		}
		suiteDuration = math.Trunc(mockReport.RunTime.Seconds()*1000) / 1000
		testCaseDuration = time.Duration(1000000000).Seconds()
	})

	It("should setup test suite metadata correctly", func() {
		expectedIndex := append([]byte(fmt.Sprintf(`{ "index": { "_index": "%s", "_type": "_doc" } }`, indexName)), []byte("\n")...)
		mockReport.PreRunStats.SpecsThatWillRun = 0

		reporter.processReport(mockReport)

		Expect(reporter.filename).To(Equal(reportFileName))
		Expect(reporter.index).To(Equal(expectedIndex))
		Expect(reporter.testSuiteName).To(Equal(mockReportSuiteDescription))
		Expect(reporter.suite.Name).To(Equal(mockReportSuiteDescription))
		Expect(reporter.suite.Failures).To(Equal(0))
		Expect(reporter.suite.Phase).To(Equal(SpecPhaseSucceeded))
		Expect(reporter.suite.Tests).To(Equal(0))
		Expect(reporter.suite.Duration).To(Equal(suiteDuration))
		Expect(reporter.suite.Errors).To(Equal(0))
	})

	It("should process one successful test correctly", func() {
		mockReport.PreRunStats.SpecsThatWillRun = 1
		mockReport.SpecReports[0].State = types.SpecStatePassed

		reporter.processReport(mockReport)

		Expect(reporter.suite.Tests).To(Equal(1))
		Expect(reporter.suite.Failures).To(Equal(0))
		Expect(reporter.suite.Phase).To(Equal(SpecPhaseSucceeded))

		Expect(len(reporter.testCases)).To(HaveLen(1))
		Expect(reporter.testCases[0].Metadata.Name).To(Equal(mockReportSuiteDescription))
		Expect(reporter.testCases[0].Name).To(Equal(testCaseName))
		Expect(reporter.testCases[0].ShortName).To(Equal(testCaseName))
		Expect(reporter.testCases[0].Phase).To(Equal(SpecPhaseSucceeded))
		Expect(reporter.testCases[0].Duration).To(Equal(testCaseDuration))
	})

	It("should process one failed test correctly", func() {
		stderr := "stderr - something failed"
		failureMessage := "something went wrong"
		location := types.CodeLocation{
			FileName:       "test.go",
			LineNumber:     10,
			FullStackTrace: "some text",
		}
		failureLocation := types.CodeLocation{
			FileName:       "error.go",
			LineNumber:     20,
			FullStackTrace: "some error",
		}
		mockReport.PreRunStats.SpecsThatWillRun = 1
		mockReport.SpecReports[0].State = types.SpecStateFailed
		mockReport.SpecReports[0].Failure = types.Failure{
			Message:             failureMessage,
			Location:            location,
			FailureNodeLocation: failureLocation,
		}
		mockReport.SpecReports[0].CapturedStdOutErr = stderr

		reporter.processReport(mockReport)

		Expect(reporter.suite.Tests).To(Equal(1))
		Expect(reporter.suite.Failures).To(Equal(1))
		Expect(reporter.suite.Errors).To(Equal(0))
		Expect(reporter.suite.Phase).To(Equal(SpecPhaseFailed))

		Expect(len(reporter.testCases)).To(HaveLen(1))
		Expect(reporter.testCases[0].Metadata.Name).To(Equal(mockReportSuiteDescription))
		Expect(reporter.testCases[0].Name).To(Equal(testCaseName))
		Expect(reporter.testCases[0].ShortName).To(Equal(testCaseName))
		Expect(reporter.testCases[0].Phase).To(Equal(SpecPhaseFailed))
		Expect(reporter.testCases[0].Duration).To(Equal(testCaseDuration))
		Expect(reporter.testCases[0].FailureMessage).NotTo(BeNil())
		Expect(reporter.testCases[0].FailureMessage.Type).To(Equal(SpecPhaseFailed))
		Expect(reporter.testCases[0].FailureMessage.Message).To(Equal(fmt.Sprintf("%s\n%s\n%s", failureLocation.String(), failureMessage, location.String())))
		Expect(reporter.testCases[0].SystemOut).To(Equal(stderr))
	})

	It("should process one panicked test correctly", func() {
		stderr := "stderr - something panicked"
		failureMessage := "something went utterly wrong"
		location := types.CodeLocation{
			FileName:       "test.go",
			LineNumber:     10,
			FullStackTrace: "some text",
		}
		failureLocation := types.CodeLocation{
			FileName:       "error.go",
			LineNumber:     20,
			FullStackTrace: "some error",
		}
		mockReport.PreRunStats.SpecsThatWillRun = 1
		mockReport.SpecReports[0].State = types.SpecStatePanicked
		mockReport.SpecReports[0].Failure = types.Failure{
			Message:             failureMessage,
			Location:            location,
			FailureNodeLocation: failureLocation,
		}
		mockReport.SpecReports[0].CapturedStdOutErr = stderr

		reporter.processReport(mockReport)

		Expect(reporter.suite.Tests).To(Equal(1))
		Expect(reporter.suite.Failures).To(Equal(0))
		Expect(reporter.suite.Errors).To(Equal(1))
		Expect(reporter.suite.Phase).To(Equal(SpecPhaseFailed))

		Expect(len(reporter.testCases)).To(HaveLen(1))
		Expect(reporter.testCases[0].Metadata.Name).To(Equal(mockReportSuiteDescription))
		Expect(reporter.testCases[0].Name).To(Equal(testCaseName))
		Expect(reporter.testCases[0].ShortName).To(Equal(testCaseName))
		Expect(reporter.testCases[0].Phase).To(Equal(SpecPhaseFailed))
		Expect(reporter.testCases[0].Duration).To(Equal(testCaseDuration))
		Expect(reporter.testCases[0].FailureMessage).NotTo(BeNil())
		Expect(reporter.testCases[0].FailureMessage.Type).To(Equal(SpecPhaseFailed))
		Expect(reporter.testCases[0].FailureMessage.Message).To(Equal(fmt.Sprintf("%s\n%s\n%s", failureLocation.String(), failureMessage, location.String())))
		Expect(reporter.testCases[0].SystemOut).To(Equal(stderr))
	})

	It("should process one skipped test correctly", func() {
		mockReport.PreRunStats.SpecsThatWillRun = 0
		mockReport.SpecReports[0].State = types.SpecStateSkipped

		reporter.processReport(mockReport)

		Expect(reporter.suite.Tests).To(Equal(0))
		Expect(reporter.suite.Failures).To(Equal(0))
		Expect(reporter.suite.Phase).To(Equal(SpecPhaseSucceeded))
		Expect(reporter.testCases).To(BeEmpty())
	})
})
