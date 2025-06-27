// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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
