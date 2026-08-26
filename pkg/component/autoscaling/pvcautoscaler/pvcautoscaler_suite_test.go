// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package pvcautoscaler_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPVCAutoscaler(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Component Autoscaling PVC Autoscaler Suite")
}
