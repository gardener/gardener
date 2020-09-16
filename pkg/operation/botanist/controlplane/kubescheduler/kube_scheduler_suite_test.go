// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubescheduler_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestKubeScheduler(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ControlPlane KubeScheduler component Suite")
}
