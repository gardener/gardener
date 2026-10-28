// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package mutator_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener/pkg/apiserver/features"
)

func TestMutator(t *testing.T) {
	features.RegisterFeatureGates()

	RegisterFailHandler(Fail)
	RunSpecs(t, "AdmissionPlugin Shoot Mutator Suite")
}
