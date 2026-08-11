// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package adminaccess_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/kubernetes/adminaccess"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("AdminAccess", func() {
	var (
		fakeClient client.Client
		access     component.DeployWaiter
		consistOf  func(...client.Object) types.GomegaMatcher

		ctx       = context.Background()
		namespace = "shoot--foo--bar"

		clusterAdminClusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:system:cluster-admin",
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "cluster-admin",
			},
			Subjects: []rbacv1.Subject{{
				Kind:      "ServiceAccount",
				Name:      "cluster-admin",
				Namespace: "kube-system",
			}},
		}

		shootAccessSecretName     = "shoot-access-cluster-admin"
		managedResourceName       = "shoot-core-adminaccess"
		managedResourceSecretName = "managedresource-shoot-core-adminaccess"
		managedResourceSecret     *corev1.Secret

		expectedShootAccessSecret *corev1.Secret
		expectedManagedResource   *resourcesv1alpha1.ManagedResource
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		consistOf = NewManagedResourceConsistOfObjectsMatcher(fakeClient)

		access = New(fakeClient, namespace)

		expectedShootAccessSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:            shootAccessSecretName,
				Namespace:       namespace,
				ResourceVersion: "1",
				Annotations: map[string]string{
					"serviceaccount.resources.gardener.cloud/name":      "cluster-admin",
					"serviceaccount.resources.gardener.cloud/namespace": "kube-system",
				},
				Labels: map[string]string{
					"resources.gardener.cloud/purpose": "token-requestor",
					"resources.gardener.cloud/class":   "shoot",
				},
			},
			Type: corev1.SecretTypeOpaque,
		}

		managedResourceSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceSecretName,
				Namespace: namespace,
			},
		}
		expectedManagedResource = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:            managedResourceName,
				Namespace:       namespace,
				ResourceVersion: "1",
				Labels: map[string]string{
					"origin": "gardener",
				},
			},
			Spec: resourcesv1alpha1.ManagedResourceSpec{
				SecretRefs:   []corev1.LocalObjectReference{},
				InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
				KeepObjects:  new(true),
			},
		}
	})

	AfterEach(func() {
		Expect(fakeClient.Delete(ctx, expectedShootAccessSecret)).To(Or(Succeed(), BeNotFoundError()))
		Expect(fakeClient.Delete(ctx, managedResourceSecret)).To(Or(Succeed(), BeNotFoundError()))
		Expect(fakeClient.Delete(ctx, expectedManagedResource)).To(Or(Succeed(), BeNotFoundError()))
	})

	Describe("#Deploy", func() {
		It("should successfully deploy all resources", func() {
			Expect(access.Deploy(ctx)).To(Succeed())

			reconciledManagedResource := &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: managedResourceName, Namespace: namespace}}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(reconciledManagedResource), reconciledManagedResource)).To(Succeed())
			expectedManagedResource.Spec.SecretRefs = []corev1.LocalObjectReference{{Name: reconciledManagedResource.Spec.SecretRefs[0].Name}}
			utilruntime.Must(references.InjectAnnotations(expectedManagedResource))
			Expect(reconciledManagedResource).To(DeepEqual(expectedManagedResource))
			Expect(reconciledManagedResource).To(consistOf(clusterAdminClusterRoleBinding))

			reconciledShootAccessSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: shootAccessSecretName, Namespace: namespace}}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(reconciledShootAccessSecret), reconciledShootAccessSecret)).To(Succeed())
			Expect(reconciledShootAccessSecret).To(DeepEqual(expectedShootAccessSecret))
		})
	})

	Describe("#Destroy", func() {
		It("should successfully delete all the resources", func() {
			expectedShootAccessSecret.ResourceVersion = ""
			managedResourceSecret.ResourceVersion = ""
			expectedManagedResource.ResourceVersion = ""

			Expect(fakeClient.Create(ctx, expectedShootAccessSecret)).To(Succeed())
			Expect(fakeClient.Create(ctx, managedResourceSecret)).To(Succeed())
			Expect(fakeClient.Create(ctx, expectedManagedResource)).To(Succeed())

			Expect(access.Destroy(ctx)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(expectedShootAccessSecret), expectedShootAccessSecret)).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(expectedManagedResource), expectedManagedResource)).To(BeNotFoundError())
		})
	})

	Context("waiting functions", func() {
		var (
			fakeOps *retryfake.Ops
		)

		BeforeEach(func() {
			fakeOps = &retryfake.Ops{MaxAttempts: 1}
			DeferCleanup(test.WithVars(
				&TimeoutWaitForManagedResource, 500*time.Millisecond,
				&retry.Until, fakeOps.Until,
				&retry.UntilTimeout, fakeOps.UntilTimeout,
			))
		})

		Describe("#Wait", func() {
			It("should fail because reading the ManagedResource fails", func() {
				Expect(access.Wait(ctx)).To(MatchError(ContainSubstring("not found")))
			})

			It("should fail because the ManagedResource doesn't become healthy", func() {
				fakeOps.MaxAttempts = 2

				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceName,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 1,
						Conditions: []gardencorev1beta1.Condition{
							{
								Type:   resourcesv1alpha1.ResourcesApplied,
								Status: gardencorev1beta1.ConditionFalse,
							},
							{
								Type:   resourcesv1alpha1.ResourcesHealthy,
								Status: gardencorev1beta1.ConditionFalse,
							},
						},
					},
				})).To(Succeed())

				Expect(access.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
			})

			It("should successfully wait for the managed resource to become healthy", func() {
				fakeOps.MaxAttempts = 2

				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceName,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 1,
						Conditions: []gardencorev1beta1.Condition{
							{
								Type:   resourcesv1alpha1.ResourcesApplied,
								Status: gardencorev1beta1.ConditionTrue,
							},
							{
								Type:   resourcesv1alpha1.ResourcesHealthy,
								Status: gardencorev1beta1.ConditionTrue,
							},
						},
					},
				})).To(Succeed())

				Expect(access.Wait(ctx)).To(Succeed())
			})
		})

		Describe("#WaitCleanup", func() {
			It("should fail when the wait for the managed resource deletion times out", func() {
				fakeOps.MaxAttempts = 2

				Expect(access.Deploy(ctx)).To(Succeed())

				Expect(access.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
			})

			It("should not return an error when it's already removed", func() {
				Expect(access.WaitCleanup(ctx)).To(Succeed())
			})
		})
	})
})
