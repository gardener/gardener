// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package logcheck_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	. "github.com/gardener/gardener/hack/tools/logcheck/pkg/logcheck"
)

func TestLogcheck(t *testing.T) {
	for _, test := range []string{
		"use-logr",
		"no-logr",
	} {
		t.Run(test, func(t *testing.T) {
			analysistest.Run(t, analysistest.TestData(), Analyzer, test)
		})
	}
}
