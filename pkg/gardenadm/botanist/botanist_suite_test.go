// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apiserverfeatures "github.com/gardener/gardener/pkg/apiserver/features"
	"github.com/gardener/gardener/pkg/gardenlet/features"
)

func TestBotanist(t *testing.T) {
	features.RegisterFeatureGates()
	apiserverfeatures.RegisterFeatureGates()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Gardenadm Botanist Suite")
}
