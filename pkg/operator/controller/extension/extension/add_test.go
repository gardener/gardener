// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extension_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	. "github.com/gardener/gardener/pkg/operator/controller/extension/extension"
)

var _ = Describe("Add", func() {
	var (
		ctx        = context.Background()
		log        logr.Logger
		fakeClient client.Client
		extension1 *operatorv1alpha1.Extension
		extension2 *operatorv1alpha1.Extension
		extension3 *operatorv1alpha1.Extension
		secret1    *corev1.Secret
		secret2    *corev1.Secret
	)

	BeforeEach(func() {
		log = logr.Discard()
		fakeClient = fakeclient.NewClientBuilder().WithScheme(operatorclient.RuntimeScheme).Build()

		secret1 = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "secret1",
				Namespace: v1beta1constants.GardenNamespace,
				Labels:    map[string]string{"gardener.cloud/role": "helm-pull-secret"},
			},
		}
		secret2 = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "secret2",
				Namespace: v1beta1constants.GardenNamespace,
				Labels:    map[string]string{"gardener.cloud/role": "helm-pull-secret"},
			},
		}

		extension1 = &operatorv1alpha1.Extension{ObjectMeta: metav1.ObjectMeta{Name: "extension1"}}
		extension2 = &operatorv1alpha1.Extension{
			ObjectMeta: metav1.ObjectMeta{Name: "extension2"},
			Spec: operatorv1alpha1.ExtensionSpec{
				Deployment: &operatorv1alpha1.Deployment{
					AdmissionDeployment: &operatorv1alpha1.AdmissionDeploymentSpec{
						VirtualCluster: &operatorv1alpha1.DeploymentSpec{
							Helm: &operatorv1alpha1.ExtensionHelm{
								OCIRepository: &gardencorev1.OCIRepository{
									PullSecretRef: &corev1.LocalObjectReference{
										Name: secret2.Name,
									},
								},
							},
						},
						RuntimeCluster: &operatorv1alpha1.DeploymentSpec{
							Helm: &operatorv1alpha1.ExtensionHelm{
								OCIRepository: &gardencorev1.OCIRepository{
									PullSecretRef: &corev1.LocalObjectReference{
										Name: secret2.Name,
									},
								},
							},
						},
					},
				},
			},
		}
		extension3 = &operatorv1alpha1.Extension{
			ObjectMeta: metav1.ObjectMeta{Name: "extension3"},
			Spec: operatorv1alpha1.ExtensionSpec{
				Deployment: &operatorv1alpha1.Deployment{
					ExtensionDeployment: &operatorv1alpha1.ExtensionDeploymentSpec{
						DeploymentSpec: operatorv1alpha1.DeploymentSpec{
							Helm: &operatorv1alpha1.ExtensionHelm{
								OCIRepository: &gardencorev1.OCIRepository{
									PullSecretRef: &corev1.LocalObjectReference{
										Name: secret1.Name,
									},
								},
							},
						},
					},
				},
			},
		}

		Expect(fakeClient.Create(ctx, extension1)).To(Succeed())
		Expect(fakeClient.Create(ctx, extension2)).To(Succeed())
		Expect(fakeClient.Create(ctx, extension3)).To(Succeed())
	})

	Describe("#MapToAllExtensions", func() {
		It("should map to all extensions", func() {
			Expect((&Reconciler{RuntimeClientSet: fakekubernetes.NewClientSetBuilder().WithClient(fakeClient).Build()}).MapToAllExtensions(log)(ctx, nil)).To(ConsistOf(
				reconcile.Request{NamespacedName: types.NamespacedName{Name: "extension1"}},
				reconcile.Request{NamespacedName: types.NamespacedName{Name: "extension2"}},
				reconcile.Request{NamespacedName: types.NamespacedName{Name: "extension3"}},
			))
		})
	})

	Describe("#MapSecretToExtensions", func() {
		It("should map to extension referencing pull secret for extension deployment", func() {
			Expect((&Reconciler{RuntimeClientSet: kubernetesfake.NewClientSetBuilder().WithClient(fakeClient).Build()}).MapSecretToExtensions(log)(ctx, secret1)).To(ConsistOf(
				reconcile.Request{NamespacedName: types.NamespacedName{Name: "extension3"}},
			))
		})

		It("should ignore pull secrets for admission deployment", func() {
			Expect((&Reconciler{RuntimeClientSet: kubernetesfake.NewClientSetBuilder().WithClient(fakeClient).Build()}).MapSecretToExtensions(log)(ctx, secret2)).To(BeEmpty())
		})
	})
})
