// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package general_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestGeneral(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Extensions Controller HealthCheck General Suite")
}
