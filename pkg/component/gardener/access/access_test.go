// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package access_test

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
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/gardener/access"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Access", func() {
	var (
		fakeClient client.Client
		sm         secretsmanager.Interface
		access     component.DeployWaiter
		consistOf  func(...client.Object) types.GomegaMatcher

		ctx       = context.Background()
		namespace = "shoot--foo--bar"

		clusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:system:gardener",
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "cluster-admin",
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      "gardener",
					Namespace: "kube-system",
				},
				{
					Kind:      "ServiceAccount",
					Name:      "gardener-internal",
					Namespace: "kube-system",
				},
			},
		}

		gardenerSecretName         = "gardener"
		gardenerInternalSecretName = "gardener-internal"
		managedResourceName        = "shoot-core-gardeneraccess"
		managedResourceSecretName  = "managedresource-shoot-core-gardeneraccess"
		managedResourceSecret      *corev1.Secret

		serverOutOfCluster = "out-of-cluster"
		serverInCluster    = "in-cluster"

		expectedGardenerSecret         *corev1.Secret
		expectedGardenerInternalSecret *corev1.Secret
		expectedManagedResource        *resourcesv1alpha1.ManagedResource
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		sm = fakesecretsmanager.New(fakeClient, namespace)
		consistOf = NewManagedResourceConsistOfObjectsMatcher(fakeClient)

		By("Create secrets managed outside of this package for whose secretsmanager.Get() will be called")
		Expect(fakeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca", Namespace: namespace}})).To(Succeed())

		access = New(fakeClient, namespace, sm, Values{
			ServerOutOfCluster:    serverOutOfCluster,
			ServerInCluster:       serverInCluster,
			ManagedResourceLabels: map[string]string{"foo": "bar"},
		})

		expectedGardenerSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:            gardenerSecretName,
				Namespace:       namespace,
				ResourceVersion: "1",
				Annotations: map[string]string{
					"serviceaccount.resources.gardener.cloud/name":      "gardener",
					"serviceaccount.resources.gardener.cloud/namespace": "kube-system",
				},
				Labels: map[string]string{
					"resources.gardener.cloud/purpose": "token-requestor",
					"resources.gardener.cloud/class":   "shoot",
				},
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{"kubeconfig": []byte(`apiVersion: v1
clusters:
- cluster:
    server: https://` + serverOutOfCluster + `
  name: ` + namespace + `
contexts:
- context:
    cluster: ` + namespace + `
    user: ` + namespace + `
  name: ` + namespace + `
current-context: ` + namespace + `
kind: Config
preferences: {}
users:
- name: ` + namespace + `
  user: {}
`)},
		}

		expectedGardenerInternalSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:            gardenerInternalSecretName,
				Namespace:       namespace,
				ResourceVersion: "1",
				Annotations: map[string]string{
					"serviceaccount.resources.gardener.cloud/name":      "gardener-internal",
					"serviceaccount.resources.gardener.cloud/namespace": "kube-system",
				},
				Labels: map[string]string{
					"resources.gardener.cloud/purpose": "token-requestor",
					"resources.gardener.cloud/class":   "shoot",
				},
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{"kubeconfig": []byte(`apiVersion: v1
clusters:
- cluster:
    server: https://` + serverInCluster + `
  name: ` + namespace + `
contexts:
- context:
    cluster: ` + namespace + `
    user: ` + namespace + `
  name: ` + namespace + `
current-context: ` + namespace + `
kind: Config
preferences: {}
users:
- name: ` + namespace + `
  user: {}
`)},
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
					"foo":    "bar",
				},
			},
			Spec: resourcesv1alpha1.ManagedResourceSpec{
				SecretRefs:   []corev1.LocalObjectReference{},
				InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
				KeepObjects:  ptr.To(true),
			},
		}
	})

	AfterEach(func() {
		Expect(fakeClient.Delete(ctx, expectedGardenerSecret)).To(Or(Succeed(), BeNotFoundError()))
		Expect(fakeClient.Delete(ctx, expectedGardenerInternalSecret)).To(Or(Succeed(), BeNotFoundError()))
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
			Expect(reconciledManagedResource).To(consistOf(clusterRoleBinding))

			reconciledGardenerSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: gardenerSecretName, Namespace: namespace}}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(reconciledGardenerSecret), reconciledGardenerSecret)).To(Succeed())
			Expect(reconciledGardenerSecret).To(DeepEqual(expectedGardenerSecret))

			reconciledGardenerInternalSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: gardenerInternalSecretName, Namespace: namespace}}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(reconciledGardenerInternalSecret), reconciledGardenerInternalSecret)).To(Succeed())
			Expect(reconciledGardenerInternalSecret).To(DeepEqual(expectedGardenerInternalSecret))
		})

		It("should remove legacy secret data", func() {
			oldGardenerSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      expectedGardenerSecret.Name,
					Namespace: expectedGardenerSecret.Namespace,
				},
			}
			Expect(fakeClient.Create(ctx, oldGardenerSecret)).To(Succeed())
			expectedGardenerSecret.ResourceVersion = "2"

			oldGardenerInternalSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      expectedGardenerInternalSecret.Name,
					Namespace: expectedGardenerInternalSecret.Namespace,
				},
			}
			Expect(fakeClient.Create(ctx, oldGardenerInternalSecret)).To(Succeed())
			expectedGardenerInternalSecret.ResourceVersion = "2"

			Expect(access.Deploy(ctx)).To(Succeed())

			reconciledGardenerSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: gardenerSecretName, Namespace: namespace}}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(reconciledGardenerSecret), reconciledGardenerSecret)).To(Succeed())
			Expect(reconciledGardenerSecret).To(DeepEqual(expectedGardenerSecret))

			reconciledGardenerInternalSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: gardenerInternalSecretName, Namespace: namespace}}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(reconciledGardenerInternalSecret), reconciledGardenerInternalSecret)).To(Succeed())
			Expect(reconciledGardenerInternalSecret).To(DeepEqual(expectedGardenerInternalSecret))
		})
	})

	Describe("#Destroy", func() {
		It("should successfully delete all the resources", func() {
			expectedGardenerSecret.ResourceVersion = ""
			expectedGardenerInternalSecret.ResourceVersion = ""
			managedResourceSecret.ResourceVersion = ""
			expectedManagedResource.ResourceVersion = ""

			Expect(fakeClient.Create(ctx, expectedGardenerSecret)).To(Succeed())
			Expect(fakeClient.Create(ctx, expectedGardenerInternalSecret)).To(Succeed())
			Expect(fakeClient.Create(ctx, managedResourceSecret)).To(Succeed())
			Expect(fakeClient.Create(ctx, expectedManagedResource)).To(Succeed())

			Expect(access.Destroy(ctx)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(expectedGardenerSecret), expectedGardenerSecret)).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(expectedGardenerInternalSecret), expectedGardenerInternalSecret)).To(BeNotFoundError())
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
