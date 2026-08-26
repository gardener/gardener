// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package admissionplugins_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestAdmissionPlugins(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Utils Validation AdmissionPlugins Suite")
}
