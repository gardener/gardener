// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package publicip_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPublicIP(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Utils Public IP Suite")
}
