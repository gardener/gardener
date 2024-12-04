// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardenadm

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("gardenadm Tests", Label("gardenadm", "default"), func() {
	Describe("Single-node control plane", Ordered, Label("single"), func() {
		It("should initialize the control plane node", func(ctx SpecContext) {
			stdout, _, err := runtimeClient.PodExecutor().Execute(ctx, machinePod.Namespace, machinePod.Name, containerName, "gardenadm", "init")
			Expect(err).NotTo(HaveOccurred())

			Eventually(ctx, gbytes.BufferReader(stdout)).Should(gbytes.Say("not implemented"))
		}, SpecTimeout(5*time.Second))
	})
})
