// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package nodeagentrestriction_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestNodeAgentRestriction(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ResourceManager Webhook NodeAgentRestriction Suite")
}
