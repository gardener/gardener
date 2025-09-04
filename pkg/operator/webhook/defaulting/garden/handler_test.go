// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package garden_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	. "github.com/gardener/gardener/pkg/operator/webhook/defaulting/garden"
)

var _ = Describe("Handler", func() {
	var (
		ctx     context.Context
		handler *Handler
		garden  *operatorv1alpha1.Garden
	)

	BeforeEach(func() {
		ctx = context.Background()
		handler = &Handler{}
		garden = &operatorv1alpha1.Garden{}
	})

	Describe("#Default", func() {
		var defaultKubeAPIServerConfig *operatorv1alpha1.KubeAPIServerConfig

		BeforeEach(func() {
			defaultKubeAPIServerConfig = &operatorv1alpha1.KubeAPIServerConfig{
				KubeAPIServerConfig: &gardencorev1beta1.KubeAPIServerConfig{
					Requests: &gardencorev1beta1.APIServerRequests{
						MaxNonMutatingInflight: ptr.To[int32](400),
						MaxMutatingInflight:    ptr.To[int32](200),
					},
					EventTTL: &metav1.Duration{Duration: time.Hour},
					Logging: &gardencorev1beta1.APIServerLogging{
						Verbosity: ptr.To[int32](2),
					},
				},
			}
		})

		It("should default all expected fields", func() {
			Expect(handler.Default(ctx, garden)).To(Succeed())
			Expect(garden).To(Equal(&operatorv1alpha1.Garden{
				Spec: operatorv1alpha1.GardenSpec{
					RuntimeCluster: operatorv1alpha1.RuntimeCluster{
						Networking: operatorv1alpha1.RuntimeNetworking{
							IPFamilies: []gardencorev1beta1.IPFamily{"IPv4"},
						},
					},
					VirtualCluster: operatorv1alpha1.VirtualCluster{
						Kubernetes: operatorv1alpha1.Kubernetes{
							KubeAPIServer: defaultKubeAPIServerConfig,
							KubeControllerManager: &operatorv1alpha1.KubeControllerManagerConfig{
								KubeControllerManagerConfig: &gardencorev1beta1.KubeControllerManagerConfig{},
							},
						},
					},
				},
			}))
		})

		It("should not overwrite configured set fields in Kube API server config", func() {
			customRequests := &gardencorev1beta1.APIServerRequests{
				MaxNonMutatingInflight: ptr.To[int32](800),
				MaxMutatingInflight:    ptr.To[int32](400),
			}

			garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer = &operatorv1alpha1.KubeAPIServerConfig{
				KubeAPIServerConfig: &gardencorev1beta1.KubeAPIServerConfig{
					Requests: customRequests,
				},
			}
			Expect(handler.Default(ctx, garden)).To(Succeed())
			Expect(garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer).To(Equal(&operatorv1alpha1.KubeAPIServerConfig{
				KubeAPIServerConfig: &gardencorev1beta1.KubeAPIServerConfig{
					Requests: customRequests,
					EventTTL: &metav1.Duration{Duration: time.Hour},
					Logging: &gardencorev1beta1.APIServerLogging{
						Verbosity: ptr.To[int32](2),
					},
				},
			}))
		})

		It("should not overwrite configured fields in Kube Controller Manager", func() {
			customKubeControllerManagerConfig := &operatorv1alpha1.KubeControllerManagerConfig{
				CertificateSigningDuration: &metav1.Duration{Duration: 123 * time.Second},
				KubeControllerManagerConfig: &gardencorev1beta1.KubeControllerManagerConfig{
					NodeCIDRMaskSize: ptr.To[int32](10),
				},
			}

			garden.Spec.VirtualCluster.Kubernetes.KubeControllerManager = customKubeControllerManagerConfig
			Expect(handler.Default(ctx, garden)).To(Succeed())
			Expect(garden.Spec.VirtualCluster.Kubernetes.KubeControllerManager).To(Equal(customKubeControllerManagerConfig))
		})

		It("should not overwrite configured fields in IPFamilies", func() {
			garden.Spec.RuntimeCluster.Networking.IPFamilies = []gardencorev1beta1.IPFamily{"foo"}
			Expect(handler.Default(ctx, garden)).To(Succeed())
			Expect(garden.Spec.RuntimeCluster.Networking.IPFamilies).To(Equal([]gardencorev1beta1.IPFamily{"foo"}))
		})

		DescribeTable("#MigrateEncryptedResources",
			func(status operatorv1alpha1.GardenStatus, expected operatorv1alpha1.GardenStatus) {
				garden.Status = status

				Expect(handler.Default(ctx, garden)).To(Succeed())

				Expect(garden.Status).To(Equal(expected))
			},
			Entry("no encrypted resources", operatorv1alpha1.GardenStatus{}, operatorv1alpha1.GardenStatus{}),
			Entry("with encrypted resources",
				operatorv1alpha1.GardenStatus{
					EncryptedResources: []string{"configmaps", "shoots.core.gardener.cloud"},
				},
				operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						ETCDEncryption: operatorv1alpha1.ETCDEncryption{
							Resources: []string{"configmaps", "shoots.core.gardener.cloud"},
						},
					},
					EncryptedResources: []string{"configmaps", "shoots.core.gardener.cloud"},
				},
			),
			Entry("should not overwrite", operatorv1alpha1.GardenStatus{
				Credentials: &operatorv1alpha1.Credentials{
					ETCDEncryption: operatorv1alpha1.ETCDEncryption{
						Resources: []string{"configmaps", "shoots.core.gardener.cloud"},
					},
				},
				EncryptedResources: []string{"configmaps"},
			},
				operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						ETCDEncryption: operatorv1alpha1.ETCDEncryption{
							Resources: []string{"configmaps", "shoots.core.gardener.cloud"},
						},
					},
					EncryptedResources: []string{"configmaps"},
				},
			),
		)
	})
})
