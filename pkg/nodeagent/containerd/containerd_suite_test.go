// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package containerd_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener/pkg/nodeagent/features"
)

func TestContainerd(t *testing.T) {
	features.RegisterFeatureGates()
	RegisterFailHandler(Fail)
	RunSpecs(t, "NodeAgent Containerd Suite")
}
