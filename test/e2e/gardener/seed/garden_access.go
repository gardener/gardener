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

package seed

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("Seed Tests", Label("Seed", "default"), func() {
	Describe("Garden Cluster Access For Seed Components", func() {
		var (
			seedNamespace    string
			gardenAccessName string
		)

		BeforeEach(func() {
			seedNamespace = gardenerutils.ComputeGardenNamespace("local")

			gardenAccessName = "test-" + utils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:8]
		})

		It("should request tokens for garden access secrets", func() {
			By("Create garden access secret")
			accessSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      gardenAccessName,
					Namespace: v1beta1constants.GardenNamespace,
					Labels: map[string]string{
						resourcesv1alpha1.ResourceManagerPurpose: resourcesv1alpha1.LabelPurposeTokenRequest,
						resourcesv1alpha1.ResourceManagerClass:   resourcesv1alpha1.ResourceManagerClassGarden,
					},
					Annotations: map[string]string{
						resourcesv1alpha1.ServiceAccountName: gardenAccessName,
					},
				},
			}
			Expect(testClient.Create(ctx, accessSecret)).To(Succeed())
			log.Info("Created garden access secret for test", "secret", client.ObjectKeyFromObject(accessSecret))

			DeferCleanup(func() {
				By("Delete garden access secret")
				Expect(testClient.Delete(ctx, accessSecret)).To(Succeed())
			})

			createRBACForGardenAccessServiceAccount(gardenAccessName, seedNamespace)

			By("Wait for token to be populated in garden access secret")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(accessSecret), accessSecret)).To(Succeed())
				g.Expect(accessSecret.Data).To(HaveKeyWithValue(resourcesv1alpha1.DataKeyToken, Not(BeEmpty())))
			}).Should(Succeed())

			By("Use token to access garden")
			gardenAccessConfig := rest.CopyConfig(restConfig)
			// drop kind admin client certificate so that we can test other credentials
			gardenAccessConfig.TLSClientConfig.CertData = nil
			gardenAccessConfig.TLSClientConfig.KeyData = nil
			// use the requested token and create a client
			gardenAccessConfig.BearerToken = string(accessSecret.Data[resourcesv1alpha1.DataKeyToken])
			gardenAccessClient, err := client.New(gardenAccessConfig, client.Options{})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func(g Gomega) {
				g.Expect(gardenAccessClient.Get(ctx, client.ObjectKey{Name: gardenAccessName, Namespace: seedNamespace}, &corev1.ServiceAccount{})).To(Succeed())
			}).Should(Succeed())
		})
	})
})

func createRBACForGardenAccessServiceAccount(name, namespace string) {
	By("Create RBAC resources for ServiceAccount")
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Rules: []rbacv1.PolicyRule{{
			APIGroups:     []string{""},
			Resources:     []string{"serviceaccounts"},
			Verbs:         []string{"get"},
			ResourceNames: []string{name},
		}},
	}
	Expect(testClient.Create(ctx, role)).To(Succeed())
	log.Info("Created role for test", "role", client.ObjectKeyFromObject(role))

	DeferCleanup(func() {
		Expect(testClient.Delete(ctx, role)).To(Succeed())
	})

	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Subjects: []rbacv1.Subject{{
			Kind: rbacv1.ServiceAccountKind,
			Name: name,
		}},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     name,
		},
	}
	Expect(testClient.Create(ctx, roleBinding)).To(Succeed())
	log.Info("Created role binding for test", "roleBinding", client.ObjectKeyFromObject(roleBinding))

	DeferCleanup(func() {
		Expect(testClient.Delete(ctx, roleBinding)).To(Succeed())
	})
}
