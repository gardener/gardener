// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package certificatesigningrequest_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCSR(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Utils Kubernetes CertificateSigningRequest Suite")
}
