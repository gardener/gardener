// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package cidr_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCIDR(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Utils Validation CIDR Suite")
}
