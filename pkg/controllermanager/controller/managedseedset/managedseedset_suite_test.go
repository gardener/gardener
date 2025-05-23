// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package managedseedset_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	controllermanagerfeatures "github.com/gardener/gardener/pkg/controllermanager/features"
)

func TestManagedSeed(t *testing.T) {
	controllermanagerfeatures.RegisterFeatureGates()
	RegisterFailHandler(Fail)
	RunSpecs(t, "ControllerManager Controller ManagedSeedSet Suite")
}
