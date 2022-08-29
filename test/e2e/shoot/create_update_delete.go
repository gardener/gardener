// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/test/e2e"
	"github.com/gardener/gardener/test/framework"
	"github.com/gardener/gardener/test/utils/shoots/access"
	shootupdatesuite "github.com/gardener/gardener/test/utils/shoots/update"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
)

var _ = Describe("Shoot Tests", Label("Shoot", "default"), func() {
	f := defaultShootCreationFramework()
	f.Shoot = e2e.DefaultShoot("e2e-default")

	// explicitly use one version below the latest supported minor version so that Kubernetes version update test can be
	// performed
	f.Shoot.Spec.Kubernetes.Version = "1.23.6"

	// create two additional worker pools which explicitly specify the kubernetes version
	pool1 := f.Shoot.Spec.Provider.Workers[0]
	pool2, pool3 := pool1.DeepCopy(), pool1.DeepCopy()
	pool2.Name += "2"
	pool2.Kubernetes = &gardencorev1beta1.WorkerKubernetes{Version: &f.Shoot.Spec.Kubernetes.Version}
	pool3.Name += "3"
	pool3.Kubernetes = &gardencorev1beta1.WorkerKubernetes{Version: pointer.String("1.22.0")}
	f.Shoot.Spec.Provider.Workers = append(f.Shoot.Spec.Provider.Workers, *pool2, *pool3)

	It("Create, Update, Delete", Label("simple"), func() {
		By("Create Shoot")
		ctx, cancel := context.WithTimeout(parentCtx, 20*time.Minute)
		defer cancel()
		Expect(f.CreateShootAndWaitForCreation(ctx, false)).To(Succeed())
		f.Verify()

		By("Verify shoot access using admin kubeconfig")
		Eventually(func(g Gomega) {
			shootClient, err := access.CreateShootClientFromAdminKubeconfig(ctx, f.GardenClient, f.Shoot)
			g.Expect(err).NotTo(HaveOccurred())

			g.Expect(shootClient.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())
		}).Should(Succeed())

		By("Update Shoot")
		ctx, cancel = context.WithTimeout(parentCtx, 20*time.Minute)
		defer cancel()
		shootupdatesuite.RunTest(ctx, &framework.ShootFramework{
			GardenerFramework: f.GardenerFramework,
			Shoot:             f.Shoot,
		}, nil, nil)

		By("Delete Shoot")
		ctx, cancel = context.WithTimeout(parentCtx, 20*time.Minute)
		defer cancel()
		Expect(f.DeleteShootAndWaitForDeletion(ctx, f.Shoot)).To(Succeed())
	})
})
