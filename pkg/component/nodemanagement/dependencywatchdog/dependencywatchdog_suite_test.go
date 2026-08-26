// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package dependencywatchdog_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestDependencyWatchdog(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Component NodeManagement DependencyWatchdog Suite")
}
