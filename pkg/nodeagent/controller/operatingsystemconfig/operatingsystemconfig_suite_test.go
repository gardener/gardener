// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operatingsystemconfig_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestOperatingSystemConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "NodeAgent Controller OperatingSystemConfig Suite")
}
