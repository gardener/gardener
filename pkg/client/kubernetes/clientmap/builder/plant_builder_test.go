// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package builder

import (
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("PlantClientMapBuilder", func() {
	var fakeReader client.Reader

	BeforeEach(func() {
		fakeReader = fakeclient.NewClientBuilder().Build()
	})

	Context("#gardenReader", func() {
		It("should be correctly set by WithGardenReader", func() {
			builder := NewPlantClientMapBuilder().WithGardenReader(fakeReader)
			Expect(builder.gardenReader).To(BeEquivalentTo(fakeReader))
		})
	})

	Context("#Build", func() {
		It("should fail if garden reader was not set", func() {
			clientMap, err := NewPlantClientMapBuilder().Build(logr.Discard())
			Expect(err).To(MatchError("garden reader is required but not set"))
			Expect(clientMap).To(BeNil())
		})

		It("should succeed to build ClientMap", func() {
			clientSet, err := NewPlantClientMapBuilder().
				WithGardenReader(fakeReader).
				Build(logr.Discard())
			Expect(err).NotTo(HaveOccurred())
			Expect(clientSet).NotTo(BeNil())
		})
	})

})
