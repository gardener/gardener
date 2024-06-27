// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package certmanagement_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCertManagement(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Component CertManagement Suite")
}
