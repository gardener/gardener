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

package project

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Project Tests", Label("Project", "default"), func() {
	var (
		project             *gardencorev1beta1.Project
		projectNamespaceKey client.ObjectKey
	)

	BeforeEach(func() {
		projectName := "test-" + utils.ComputeSHA256Hex([]byte(CurrentSpecReport().LeafNodeLocation.String()))[:5]

		project = &gardencorev1beta1.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name: projectName,
			},
			Spec: gardencorev1beta1.ProjectSpec{
				Namespace: ptr.To("garden-" + projectName),
			},
		}
		projectNamespaceKey = client.ObjectKey{Name: *project.Spec.Namespace}
	})

	JustBeforeEach(func() {
		By("Create Project")
		Expect(testClient.Create(ctx, project)).To(Succeed())
		log.Info("Created Project", "project", client.ObjectKeyFromObject(project))

		DeferCleanup(func() {
			By("Delete Project")
			Expect(client.IgnoreNotFound(gardenerutils.ConfirmDeletion(ctx, testClient, project))).To(Succeed())
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, project))).To(Succeed())

			By("Wait for Project to be gone")
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(project), project)
			}).
				WithTimeout(2 * time.Minute). // it might take a while for the project namespace to disappear
				Should(BeNotFoundError())
		})
	})

	waitForProjectPhase := func(phase gardencorev1beta1.ProjectPhase) {
		By("Wait for Project to be reconciled")
		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(project), project)).To(Succeed())
			g.Expect(project.Status.ObservedGeneration).To(Equal(project.Generation), "project controller should observe generation %d", project.Generation)
			g.Expect(project.Status.Phase).To(Equal(phase), "project should transition to phase %s", phase)
		}).Should(Succeed())
	}

	Describe("Project Member RBAC", func() {
		var (
			testUserName   string
			testUserClient client.Client
		)

		BeforeEach(func() {
			testUserName = project.Name
			testUserConfig := rest.CopyConfig(restConfig)
			// use impersonation to simulate different user
			// TODO: use a ServiceAccount instead
			testUserConfig.Impersonate = rest.ImpersonationConfig{
				UserName: testUserName,
			}

			var err error
			testUserClient, err = client.New(testUserConfig, client.Options{})
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			waitForProjectPhase(gardencorev1beta1.ProjectReady)
		})

		It("should create and bind extension roles", func() {
			By("Create test endpoints")
			testEndpoints := &corev1.Endpoints{ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-",
				Namespace:    projectNamespaceKey.Name,
			}}
			Expect(testClient.Create(ctx, testEndpoints)).To(Succeed())
			log.Info("Created Endpoints for test", "endpoints", client.ObjectKeyFromObject(testEndpoints))

			By("Ensure non-member doesn't have access to endpoints")
			Consistently(func(g Gomega) {
				g.Expect(testUserClient.Get(ctx, client.ObjectKeyFromObject(testEndpoints), testEndpoints)).To(BeForbiddenError())
			}).Should(Succeed())

			By("Create Extension Role")
			// use dedicated role name per test run
			extensionClusterRole := &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "gardener.cloud:extension:aggregate-to-test",
					Labels: map[string]string{
						"rbac.gardener.cloud/aggregate-to-extension-role": "e2e-test",
					},
				},
				Rules: []rbacv1.PolicyRule{{
					APIGroups: []string{""},
					Resources: []string{"endpoints"},
					Verbs:     []string{"get"},
				}},
			}
			Expect(testClient.Create(ctx, extensionClusterRole)).To(Succeed())
			log.Info("Created ClusterRole for test", "clusterRole", client.ObjectKeyFromObject(extensionClusterRole))

			DeferCleanup(func() {
				By("Delete Extension Role")
				Expect(testClient.Delete(ctx, extensionClusterRole)).To(Or(Succeed(), BeNotFoundError()))
			})

			By("Add new member with extension role")
			patch := client.MergeFrom(project.DeepCopy())
			project.Spec.Members = append(project.Spec.Members, gardencorev1beta1.ProjectMember{
				Subject: rbacv1.Subject{
					APIGroup: rbacv1.GroupName,
					Kind:     rbacv1.UserKind,
					Name:     testUserName,
				},
				Role: "extension:e2e-test",
			})
			Expect(testClient.Patch(ctx, project, patch)).To(Succeed())

			By("Ensure new member has access to endpoints")
			Eventually(func(g Gomega) {
				g.Expect(testUserClient.Get(ctx, client.ObjectKeyFromObject(testEndpoints), testEndpoints)).To(Succeed())
			}).Should(Succeed())
		})
	})
})
