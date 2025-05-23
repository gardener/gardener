// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cp_migration_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCPMigrationApplications(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CP Migration Test Suite")
}
