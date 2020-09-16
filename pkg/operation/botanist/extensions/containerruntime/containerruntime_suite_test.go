// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package containerruntime_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestContainerRuntime(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Botanist Extensions Container Runtime Suite")
}
