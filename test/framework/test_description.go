// Copyright 2020 Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package framework

import (
	"context"
	"fmt"
	"time"

	"github.com/onsi/ginkgo/v2"
	"k8s.io/apimachinery/pkg/util/sets"
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
// This kind of test should run with care.
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
func (t TestDescription) It(text string, body func(), opts ...TestOption) {
	testOptions := &TestOptions{}
	testOptions.ApplyOptions(opts)

	testOptions.Complete(func() {
		ginkgo.It(fmt.Sprintf("%s %s", t.String(), text), body)
	})
}

// FIt defines a ginkgo FIt block and enhances the test description with the provided labels
func (t TestDescription) FIt(text string, body func(), opts ...TestOption) {
	testOptions := &TestOptions{}
	testOptions.ApplyOptions(opts)

	testOptions.Complete(func() {
		ginkgo.FIt(fmt.Sprintf("%s %s", t.String(), text), body)
	})
}

// CIt defines a contextified ginkgo It block and enhances the test description with the provided labels
func (t TestDescription) CIt(text string, body func(context.Context), timeout time.Duration, opts ...TestOption) {
	testOptions := &TestOptions{}
	testOptions.ApplyOptions(opts)

	testOptions.Complete(func() {
		CIt(fmt.Sprintf("%s %s", t.String(), text), body, timeout)
	})
}

// FCIt defines a contextified ginkgo FIt block and enhances the test description with the provided labels
func (t TestDescription) FCIt(text string, body func(context.Context), timeout time.Duration, opts ...TestOption) {
	testOptions := &TestOptions{}
	testOptions.ApplyOptions(opts)

	testOptions.Complete(func() {
		FCIt(fmt.Sprintf("%s %s", t.String(), text), body, timeout)
	})
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
