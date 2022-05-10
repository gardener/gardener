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

	"github.com/gardener/gardener/pkg/apis/authentication/v1alpha1"
	"github.com/gardener/gardener/pkg/client/core/clientset/versioned"
	"github.com/gardener/gardener/pkg/client/kubernetes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Shoot Tests", Label("Shoot"), func() {
	var (
		f           = defaultShootCreationFramework()
		shootClient kubernetes.Interface
	)

	f.Shoot = defaultShoot("e2e-default", "")

	It("Create and Delete", Label("fast"), func() {
		By("Create Shoot")
		ctx, cancel := context.WithTimeout(parentCtx, 15*time.Minute)
		defer cancel()
		Expect(f.CreateShootAndWaitForCreation(ctx, false)).To(Succeed())
		f.Verify()

		restConfig := f.GardenerFramework.GardenClient.RESTConfig()
		versionedClient, err := versioned.NewForConfig(restConfig)
		Expect(err).NotTo(HaveOccurred())

		adminKubeconfigRequest := &v1alpha1.AdminKubeconfigRequest{
			Spec: v1alpha1.AdminKubeconfigRequestSpec{
				ExpirationSeconds: pointer.Int64(3600),
			},
		}

		adminKubeconfig, err := versionedClient.CoreV1beta1().Shoots(f.Shoot.GetNamespace()).CreateAdminKubeconfigRequest(ctx, f.Shoot.GetName(), adminKubeconfigRequest, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		By("Create shoot client using adminKubeconfig")
		Eventually(func(g Gomega) {
			shootClient, err = kubernetes.NewClientFromBytes(adminKubeconfig.Status.Kubeconfig, kubernetes.WithClientOptions(
				client.Options{
					Scheme: kubernetes.ShootScheme,
				}),
				kubernetes.WithDisabledCachedClient(),
			)
			g.Expect(err).NotTo(HaveOccurred())
		}).Should(Succeed())

		By("Verify cluster access")
		namespaceList := &corev1.NamespaceList{}
		Expect(shootClient.APIReader().List(ctx, namespaceList)).To(Succeed())

		By("Delete Shoot")
		ctx, cancel = context.WithTimeout(parentCtx, 15*time.Minute)
		defer cancel()
		Expect(f.DeleteShootAndWaitForDeletion(ctx, f.Shoot)).To(Succeed())
	})
})
