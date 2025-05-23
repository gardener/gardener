// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shootstate_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/utils/gardener/shootstate"
)

var _ = Describe("Machines", func() {
	Describe("#DecompressMachineState", func() {
		It("should do nothing because state is empty", func() {
			state, err := DecompressMachineState(nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(state).To(BeNil())
		})

		It("should fail because the state cannot be unmarshalled", func() {
			state, err := DecompressMachineState([]byte("{foo"))
			Expect(err).To(MatchError(ContainSubstring("failed unmarshalling JSON to compressed machine state structure")))
			Expect(state).To(BeNil())
		})

		It("should fail because the gzip reader cannot be created", func() {
			state, err := DecompressMachineState([]byte(`{"state":"eW91LXNob3VsZC1ub3QtaGF2ZS1yZWFkLXRoaXM="}`))
			Expect(err).To(MatchError(ContainSubstring("failed creating gzip reader for decompressing machine state data")))
			Expect(state).To(BeNil())
		})

		It("should successfully decompress the data", func() {
			state, err := DecompressMachineState([]byte(`{"state":"H4sIAAAAAAAAAyvJyCzWLc/MydHNSCxL1U3OzytOLSxNzUtOLQYA3w65lxsAAAA="}`))
			Expect(err).NotTo(HaveOccurred())
			Expect(state).To(Equal([]byte("this-will-have-consequences")))
		})
	})
})
