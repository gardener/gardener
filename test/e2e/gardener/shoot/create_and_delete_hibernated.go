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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	e2e "github.com/gardener/gardener/test/e2e/gardener"
	"github.com/gardener/gardener/test/framework"
)

var _ = Describe("Shoot Tests", Label("Shoot", "default"), func() {
	f := defaultShootCreationFramework()
	f.Shoot = e2e.DefaultShoot("e2e-hibernated")
	f.Shoot.Spec.Hibernation = &gardencorev1beta1.Hibernation{
		Enabled: pointer.Bool(true),
	}

	It("Create and Delete Hibernated Shoot", Label("hibernated"), func() {
		By("Create Shoot")
		ctx, cancel := context.WithTimeout(parentCtx, 15*time.Minute)
		defer cancel()
		Expect(f.CreateShootAndWaitForCreation(ctx, false)).To(Succeed())
		f.Verify()

		verifyNoPodsRunning(ctx, f)

		By("Delete Shoot")
		ctx, cancel = context.WithTimeout(parentCtx, 15*time.Minute)
		defer cancel()
		Expect(f.DeleteShootAndWaitForDeletion(ctx, f.Shoot)).To(Succeed())
	})
})

func verifyNoPodsRunning(ctx context.Context, f *framework.ShootCreationFramework) {
	podList := &metav1.PartialObjectMetadataList{}
	podList.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("PodList"))
	ExpectWithOffset(1, f.ShootFramework.SeedClient.Client().List(ctx, podList, client.InNamespace(f.Shoot.Status.TechnicalID))).To(Succeed())
	ExpectWithOffset(1, podList.Items).To(BeEmpty())
}
