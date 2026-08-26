// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package shoot_deletion_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestShootApplications(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Shoot Deletion Test Suite")
}
