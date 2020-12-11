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

package managedseed_test

import (
	"context"

	"github.com/gardener/gardener/pkg/apis/seedmanagement"
	. "github.com/gardener/gardener/pkg/registry/seedmanagement/managedseed"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Strategy", func() {
	var (
		ctx      = context.TODO()
		strategy = Strategy{}
	)

	Describe("#PrepareForUpdate", func() {
		var oldManagedSeed, newManagedSeed *seedmanagement.ManagedSeed

		BeforeEach(func() {
			oldManagedSeed = &seedmanagement.ManagedSeed{}
			newManagedSeed = &seedmanagement.ManagedSeed{}
		})

		It("should bump the generation if the spec changes", func() {
			newManagedSeed.Spec.Shoot = &seedmanagement.Shoot{Name: "foo"}
			strategy.PrepareForUpdate(ctx, newManagedSeed, oldManagedSeed)
			Expect(newManagedSeed.Generation).To(Equal(oldManagedSeed.Generation + 1))
		})
	})
})
