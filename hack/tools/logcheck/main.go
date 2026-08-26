// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"golang.org/x/tools/go/analysis/singlechecker"

	"github.com/gardener/gardener/hack/tools/logcheck/pkg/logcheck"
)

func main() {
	// make a callable binary with a single check out of Analyzer
	singlechecker.Main(logcheck.Analyzer)
}
