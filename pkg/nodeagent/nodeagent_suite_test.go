// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package nodeagent_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestNodeAgent(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "NodeAgent Suite")
}
