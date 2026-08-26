// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package victorialogs_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestVictoriaLogs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Component Observability Logging VictoriaLogs Suite")
}
