// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
	. "github.com/gardener/gardener/test/e2e"
	. "github.com/gardener/gardener/test/e2e/gardener"
	. "github.com/gardener/gardener/test/e2e/gardener/project/internal"
)

var _ = Describe("Project Tests", Ordered, Label("Project", "default"), func() {
	var s *ProjectContext

	var (
		testUserName         string
		testUserClient       client.Client
		testEndpoint         *corev1.Endpoints
		extensionClusterRole *rbacv1.ClusterRole
	)

	BeforeTestSetup(func() {
		projectName := "test-" + utils.ComputeSHA256Hex([]byte(CurrentSpecReport().LeafNodeLocation.String()))[:5]

		project := &gardencorev1beta1.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name: projectName,
			},
			Spec: gardencorev1beta1.ProjectSpec{
				Namespace: ptr.To("garden-" + projectName),
			},
		}

		s = NewTestContext().ForProject(project)
	})

	BeforeAll(func() {
		DeferCleanup(func(ctx SpecContext) {
			Eventually(func(g Gomega) {
				if testEndpoint != nil {
					g.Expect(client.IgnoreNotFound(s.GardenClient.Delete(ctx, testEndpoint))).To(Succeed())
				}

				if extensionClusterRole != nil {
					g.Expect(client.IgnoreNotFound(s.GardenClient.Delete(ctx, extensionClusterRole))).To(Succeed())
				}

				g.Expect(client.IgnoreNotFound(gardenerutils.ConfirmDeletion(ctx, s.GardenClient, s.Project))).To(Succeed())
				g.Expect(client.IgnoreNotFound(s.GardenClient.Delete(ctx, s.Project))).To(Succeed())
			}).Should(Succeed())
		}, NodeTimeout(time.Minute))
	})

	ItShouldCreateProject(s)
	ItShouldWaitForProjectToBeReconciledAndReady(s)

	It("Initialize test user", func(ctx SpecContext) {
		testUserName = s.Project.Name
		testUserConfig := rest.CopyConfig(s.GardenClientSet.RESTConfig())
		// use impersonation to simulate different user
		// TODO: use a ServiceAccount instead
		testUserConfig.Impersonate = rest.ImpersonationConfig{
			UserName: testUserName,
		}

		Eventually(ctx, func(g Gomega) {
			var err error
			testUserClient, err = client.New(testUserConfig, client.Options{})
			g.Expect(err).NotTo(HaveOccurred())
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))

	It("Create test Endpoint", func(ctx SpecContext) {
		testEndpoint = &corev1.Endpoints{ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-",
			Namespace:    *s.Project.Spec.Namespace,
		}}

		Eventually(ctx, func() error {
			return s.GardenClient.Create(ctx, testEndpoint)
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))

	It("Verify non-member doesn't have access to Endpoints", func(ctx SpecContext) {
		Consistently(func(g Gomega) {
			g.Expect(testUserClient.Get(ctx, client.ObjectKeyFromObject(testEndpoint), testEndpoint)).To(BeForbiddenError())
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))

	It("Create Extension Role", func(ctx SpecContext) {
		// use dedicated role name per test run
		extensionClusterRole = &rbacv1.ClusterRole{
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

		Eventually(ctx, func() error {
			return s.GardenClient.Create(ctx, extensionClusterRole)
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))

	It("Add new member with extension role", func(ctx SpecContext) {
		Eventually(ctx, s.GardenKomega.Update(s.Project, func() {
			s.Project.Spec.Members = append(s.Project.Spec.Members, gardencorev1beta1.ProjectMember{
				Subject: rbacv1.Subject{
					APIGroup: rbacv1.GroupName,
					Kind:     rbacv1.UserKind,
					Name:     testUserName,
				},
				Role: "extension:e2e-test",
			})
		})).Should(Succeed())
	}, SpecTimeout(time.Minute))

	It("Verify new member has access to Endpoints", func(ctx SpecContext) {
		Eventually(func(g Gomega) {
			g.Expect(testUserClient.Get(ctx, client.ObjectKeyFromObject(testEndpoint), testEndpoint)).To(Succeed())
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))

	ItShouldDeleteProject(s)
	ItShouldWaitForProjectToBeDeleted(s)
})
