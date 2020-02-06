package framework

import (
	"context"
	"fmt"
	"github.com/onsi/ginkgo"
	"k8s.io/apimachinery/pkg/util/sets"
	"time"
)

// TestDescription labels tests according to the provided labels in the expected order.
type TestDescription struct {
	labels sets.String
}

// NewTestDescription creates a new test description
func NewTestDescription(baseLabel string) TestDescription {
	return TestDescription{
		labels: sets.NewString(baseLabel),
	}
}

// Beta labels a test as beta test
func (t TestDescription) Beta() TestDescription {
	return t.newLabel("BETA")
}

// Default labels a test as default test
func (t TestDescription) Default() TestDescription {
	return t.newLabel("DEFAULT")
}

// Release labels a test as release relevant test
func (t TestDescription) Release() TestDescription {
	return t.newLabel("RELEASE")
}

// Serial labels a test to be run as serial step
func (t TestDescription) Serial() TestDescription {
	return t.newLabel("SERIAL")
}

// Disruptive labels a test as disruptive.
// Tis kind of test should not run on a productive landscape.
func (t TestDescription) Disruptive() TestDescription {
	return t.newLabel("DISRUPTIVE")
}

func (t TestDescription) newLabel(label string) TestDescription {
	labels := t.labels.Union(nil)
	labels.Insert(label)
	return TestDescription{
		labels: labels,
	}
}

// It defines a ginkgo It block and enhances the test description with the provided labels
func (t TestDescription) It(text string, body func()) {
	ginkgo.It(fmt.Sprintf("%s %s", t.String(), text), body)
}

// FIt defines a ginkgo FIt block and enhances the test description with the provided labels
func (t TestDescription) FIt(text string, body func()) {
	ginkgo.FIt(fmt.Sprintf("%s %s", t.String(), text), body)
}

// CIt defines a contextified ginkgo It block and enhances the test description with the provided labels
func (t TestDescription) CIt(text string, body func(context.Context), timeout time.Duration) {
	CIt(fmt.Sprintf("%s %s", t.String(), text), body, timeout)
}

// FCIt defines a contextified ginkgo FIt block and enhances the test description with the provided labels
func (t TestDescription) FCIt(text string, body func(context.Context), timeout time.Duration) {
	FCIt(fmt.Sprintf("%s %s", t.String(), text), body, timeout)
}

// String returns the test description labels
func (t TestDescription) String() string {
	labelsList := t.labels.List()
	testText := fmt.Sprintf("[%s]", labelsList[0])
	for i := 1; i < len(labelsList); i++ {
		testText = fmt.Sprintf("%s [%s]", testText, labelsList[i])
	}
	return testText
}
