// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package tolerationrestriction_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestTolerationRestriction(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Admission TolerationRestriction Suite")
}
