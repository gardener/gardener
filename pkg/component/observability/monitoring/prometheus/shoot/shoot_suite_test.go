// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener/pkg/gardenlet/features"
)

func TestShoot(t *testing.T) {
	features.RegisterFeatureGates()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Component Observability Monitoring Prometheus Shoot Suite")
}
