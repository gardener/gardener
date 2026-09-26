// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package refallowlist_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestRefAllowlist(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ControllerManager Controller Seed RefAllowlist Suite")
}
