// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	shootextensionactuator "github.com/gardener/gardener/pkg/provider-local/controller/extension/shoot"
	e2e "github.com/gardener/gardener/test/e2e/gardener"
)

var _ = Describe("Shoot Tests", Label("Shoot", "default"), func() {
	test := func(shoot *gardencorev1beta1.Shoot) {
		f := defaultShootCreationFramework()
		metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, shootextensionactuator.AnnotationTestForceDeleteShoot, "true")
		f.Shoot = shoot

		It("Create and Force Delete Shoot", Label("force-delete"), func() {
			By("Create Shoot")
			ctx, cancel := context.WithTimeout(parentCtx, 15*time.Minute)
			defer cancel()
			Expect(f.CreateShootAndWaitForCreation(ctx, false)).To(Succeed())
			f.Verify()

			By("Wait for Shoot to be force-deleted")
			ctx, cancel = context.WithTimeout(parentCtx, 10*time.Minute)
			defer cancel()
			Expect(f.ForceDeleteShootAndWaitForDeletion(ctx, f.Shoot)).To(Succeed())
		})
	}

	Context("Shoot", func() {
		test(e2e.DefaultShoot("e2e-force-delete"))
	})

	Context("Hibernated Shoot", func() {
		shoot := e2e.DefaultShoot("e2e-fd-hib")
		shoot.Spec.Hibernation = &gardencorev1beta1.Hibernation{
			Enabled: ptr.To(true),
		}

		test(shoot)
	})
})
