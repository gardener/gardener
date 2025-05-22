// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reference_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	. "github.com/gardener/gardener/pkg/operator/controller/garden/reference"
)

var _ = Describe("Add", func() {
	Describe("#Predicate", func() {
		var garden *operatorv1alpha1.Garden

		BeforeEach(func() {
			garden = &operatorv1alpha1.Garden{
				Spec: operatorv1alpha1.GardenSpec{
					DNS: &operatorv1alpha1.DNSManagement{
						Providers: []operatorv1alpha1.DNSProvider{
							{
								Name: "primary",
								Type: "test",
								SecretRef: corev1.LocalObjectReference{
									Name: "secret-name",
								},
							},
						},
					},
					VirtualCluster: operatorv1alpha1.VirtualCluster{
						Kubernetes: operatorv1alpha1.Kubernetes{
							KubeAPIServer: &operatorv1alpha1.KubeAPIServerConfig{
								KubeAPIServerConfig: &gardencorev1beta1.KubeAPIServerConfig{},
							},
						},
						Gardener: operatorv1alpha1.Gardener{
							APIServer: &operatorv1alpha1.GardenerAPIServerConfig{},
						},
					},
				},
			}
		})

		It("should return false because new object is no garden", func() {
			Expect(Predicate(nil, nil)).To(BeFalse())
		})

		It("should return false because old object is no garden", func() {
			Expect(Predicate(nil, garden)).To(BeFalse())
		})

		It("should return false because there is no ref change", func() {
			Expect(Predicate(garden, garden)).To(BeFalse())
		})

		It("should return true because the kube-apiserver audit policy field changed", func() {
			oldShoot := garden.DeepCopy()
			garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer.AuditConfig = &gardencorev1beta1.AuditConfig{AuditPolicy: &gardencorev1beta1.AuditPolicy{ConfigMapRef: &corev1.ObjectReference{Name: "audit-policy"}}}
			Expect(Predicate(oldShoot, garden)).To(BeTrue())
		})

		It("should return true because the gardener-apiserver audit policy field changed", func() {
			oldShoot := garden.DeepCopy()
			garden.Spec.VirtualCluster.Gardener.APIServer.AuditConfig = &gardencorev1beta1.AuditConfig{AuditPolicy: &gardencorev1beta1.AuditPolicy{ConfigMapRef: &corev1.ObjectReference{Name: "audit-policy"}}}
			Expect(Predicate(oldShoot, garden)).To(BeTrue())
		})

		It("should return true because the kube-apiserver audit webhook secret field changed", func() {
			oldShoot := garden.DeepCopy()
			garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer.AuditWebhook = &operatorv1alpha1.AuditWebhook{KubeconfigSecretName: "webhook-secret"}
			Expect(Predicate(oldShoot, garden)).To(BeTrue())
		})

		It("should return true because the gardener-apiserver audit webhook secret field changed", func() {
			oldShoot := garden.DeepCopy()
			garden.Spec.VirtualCluster.Gardener.APIServer.AuditWebhook = &operatorv1alpha1.AuditWebhook{KubeconfigSecretName: "webhook-secret"}
			Expect(Predicate(oldShoot, garden)).To(BeTrue())
		})

		It("should return true because the ETCD backup secret field changed", func() {
			oldShoot := garden.DeepCopy()
			garden.Spec.VirtualCluster.ETCD = &operatorv1alpha1.ETCD{Main: &operatorv1alpha1.ETCDMain{Backup: &operatorv1alpha1.Backup{SecretRef: corev1.LocalObjectReference{Name: "secret-name"}}}}
			Expect(Predicate(oldShoot, garden)).To(BeTrue())
		})

		It("should return true because the DNS secret field changed", func() {
			oldShoot := garden.DeepCopy()
			garden.Spec.DNS.Providers[0].SecretRef.Name = "secret-name2"
			Expect(Predicate(oldShoot, garden)).To(BeTrue())
		})

		It("should return true because the DNS section has been deleted", func() {
			oldShoot := garden.DeepCopy()
			garden.Spec.DNS = nil
			Expect(Predicate(oldShoot, garden)).To(BeTrue())
		})

		It("should return true because the SNI secret field changed", func() {
			oldShoot := garden.DeepCopy()
			garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer.SNI = &operatorv1alpha1.SNI{SecretName: ptr.To("secret-sni")}
			Expect(Predicate(oldShoot, garden)).To(BeTrue())
		})

		It("should return true because the authentication webhook secret field changed", func() {
			oldShoot := garden.DeepCopy()
			garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer.Authentication = &operatorv1alpha1.Authentication{Webhook: &operatorv1alpha1.AuthenticationWebhook{KubeconfigSecretName: "auth-secret"}}
			Expect(Predicate(oldShoot, garden)).To(BeTrue())
		})

		It("should return true because the kube-apiserver structured authentication config map changed", func() {
			oldShoot := garden.DeepCopy()
			garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer.StructuredAuthentication = &gardencorev1beta1.StructuredAuthentication{ConfigMapName: "foo"}
			Expect(Predicate(oldShoot, garden)).To(BeTrue())
		})

		It("should return true because the kube-apiserver structured authorization configmap changed", func() {
			oldShoot := garden.DeepCopy()
			garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer.StructuredAuthorization = &gardencorev1beta1.StructuredAuthorization{ConfigMapName: "bar"}
			Expect(Predicate(oldShoot, garden)).To(BeTrue())
		})

		It("should return true because the kube-apiserver structured authorization kubeconfig secret fields changed", func() {
			oldShoot := garden.DeepCopy()
			garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer.StructuredAuthorization = &gardencorev1beta1.StructuredAuthorization{Kubeconfigs: []gardencorev1beta1.AuthorizerKubeconfigReference{{SecretName: "foo"}}}
			Expect(Predicate(oldShoot, garden)).To(BeTrue())
		})

		It("should return true because the kube-apiserver admission plugin secret fields changed", func() {
			oldShoot := garden.DeepCopy()
			garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer.AdmissionPlugins = []gardencorev1beta1.AdmissionPlugin{{KubeconfigSecretName: ptr.To("foo")}}
			Expect(Predicate(oldShoot, garden)).To(BeTrue())
		})

		It("should return true because the gardener-apiserver admission plugin secret fields changed", func() {
			oldShoot := garden.DeepCopy()
			garden.Spec.VirtualCluster.Gardener.APIServer.AdmissionPlugins = []gardencorev1beta1.AdmissionPlugin{{KubeconfigSecretName: ptr.To("foo")}}
			Expect(Predicate(oldShoot, garden)).To(BeTrue())
		})
	})
})
