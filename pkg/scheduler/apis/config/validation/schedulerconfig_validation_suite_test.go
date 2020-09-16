// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestSeedManagerValidation(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Gardener Scheduler Configuration Validation Suite")
}
