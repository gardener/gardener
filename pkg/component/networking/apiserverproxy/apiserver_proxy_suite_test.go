// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apiserverproxy_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener/pkg/gardenlet/features"
)

func TestAPIServerProxy(t *testing.T) {
	RegisterFailHandler(Fail)
	features.RegisterFeatureGates()
	RunSpecs(t, "Component Networking APIServerProxy Suite")
}
