// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package namespacedcloudprofile_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	cmfeatures "github.com/gardener/gardener/pkg/controllermanager/features"
)

func TestCloudProfile(t *testing.T) {
	cmfeatures.RegisterFeatureGates()
	RegisterFailHandler(Fail)
	RunSpecs(t, "ControllerManager Controller NamespacedCloudProfile Suite")
}
