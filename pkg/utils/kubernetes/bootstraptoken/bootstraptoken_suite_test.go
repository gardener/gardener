// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bootstraptoken_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestNet(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Utils Kubernetes Bootstraptoken Suite")
}
