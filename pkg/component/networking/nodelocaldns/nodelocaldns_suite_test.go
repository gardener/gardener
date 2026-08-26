// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package nodelocaldns_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestNodeLocalDNS(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Component Networking NodeLocalDNS Suite")
}
