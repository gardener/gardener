// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package apigroups_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestAPIGroups(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Utils Validation APIGroups Suite")
}
