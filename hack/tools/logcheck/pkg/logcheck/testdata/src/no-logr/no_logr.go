// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// This is a test file for executing unit tests for logcheck using golang.org/x/tools/go/analysis/analysistest.
// This package doesn't import logr, so there is no need to check any logging calls. There should be no errors.

package no_logr

import (
	"fmt"
)

func init() {
	fmt.Println("bar")
}
