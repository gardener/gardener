// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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
