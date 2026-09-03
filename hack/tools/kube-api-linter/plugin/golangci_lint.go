// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"golang.org/x/tools/go/analysis"
	pluginbase "sigs.k8s.io/kube-api-linter/pkg/plugin/base"
	_ "sigs.k8s.io/kube-api-linter/pkg/registration"
)

// New is the golangci-lint plugin entry point for kube-api-linter.
func New(settings any) ([]*analysis.Analyzer, error) {
	plugin, err := pluginbase.New(settings)
	if err != nil {
		return nil, err
	}

	return plugin.BuildAnalyzers()
}
