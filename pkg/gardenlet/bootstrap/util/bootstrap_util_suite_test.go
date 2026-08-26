// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package util_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestBootstrapUtils(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Gardenlet Bootstrap Utils Suite")
}
