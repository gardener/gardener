// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
