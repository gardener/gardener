// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package adminaccess_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestAdminAccess(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Component Kubernetes AdminAccess Suite")
}
