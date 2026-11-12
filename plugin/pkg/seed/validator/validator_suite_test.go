// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package validator_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener/pkg/apiserver/features"
)

func TestValidator(t *testing.T) {
	RegisterFailHandler(Fail)
	features.RegisterFeatureGates()
	RunSpecs(t, "AdmissionPlugin Seed Validator Suite")
}
