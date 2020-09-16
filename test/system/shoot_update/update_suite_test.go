// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shootupdate_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestUpdateShoot(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Shoot Update Integration Test Suite")
}
