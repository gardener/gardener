// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package keys_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestKeys(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Client Kubernetes ClientMap Keys Suite")
}
