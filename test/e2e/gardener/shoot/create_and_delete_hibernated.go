// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package shoot

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	e2e "github.com/gardener/gardener/test/e2e/gardener"
	"github.com/gardener/gardener/test/framework"
)

var _ = Describe("Shoot Tests", Label("Shoot", "default"), func() {
	test := func(f *framework.ShootCreationFramework) {
		f.Shoot.Spec.Hibernation = &gardencorev1beta1.Hibernation{
			Enabled: pointer.Bool(true),
		}

		It("Create and Delete Hibernated Shoot", Offset(1), Label("hibernated"), func() {
			By("Create Shoot")
			ctx, cancel := context.WithTimeout(parentCtx, 15*time.Minute)
			defer cancel()
			Expect(f.CreateShootAndWaitForCreation(ctx, false)).To(Succeed())
			f.Verify()

			if !v1beta1helper.IsWorkerless(f.Shoot) {
				Expect(f.GardenerFramework.VerifyNoRunningPods(ctx, f.Shoot)).To(Succeed())
			}

			By("Delete Shoot")
			ctx, cancel = context.WithTimeout(parentCtx, 15*time.Minute)
			defer cancel()
			Expect(f.DeleteShootAndWaitForDeletion(ctx, f.Shoot)).To(Succeed())
		})
	}

	Context("Shoot with workers", func() {
		f := defaultShootCreationFramework()
		f.Shoot = e2e.DefaultShoot("e2e-hib")

		test(f)
	})

	Context("Workerless Shoot", Label("workerless"), func() {
		f := defaultShootCreationFramework()
		f.Shoot = e2e.DefaultWorkerlessShoot("e2e-hib")

		test(f)
	})
})
