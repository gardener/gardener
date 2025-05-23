// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	schedulerfeatures "github.com/gardener/gardener/pkg/scheduler/features"
)

func TestShoot(t *testing.T) {
	schedulerfeatures.RegisterFeatureGates()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Scheduler Controller Shoot Suite")
}
