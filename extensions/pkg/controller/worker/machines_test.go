// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package worker_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	. "github.com/gardener/gardener/extensions/pkg/controller/worker"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

var _ = Describe("Machines", func() {
	Context("MachineDeployment", func() {
		DescribeTable("#HasDeployment",
			func(machineDeployments MachineDeployments, name string, expectation bool) {
				Expect(machineDeployments.HasDeployment(name)).To(Equal(expectation))
			},

			Entry("list is nil", nil, "foo", false),
			Entry("empty list", MachineDeployments{}, "foo", false),
			Entry("entry not found", MachineDeployments{{Name: "bar"}}, "foo", false),
			Entry("entry exists", MachineDeployments{{Name: "bar"}}, "bar", true),
		)

		DescribeTable("#FindByName",
			func(machineDeployments MachineDeployments, name string, expectedDeployment *MachineDeployment) {
				Expect(machineDeployments.FindByName(name)).To(Equal(expectedDeployment))
			},

			Entry("list is nil", nil, "foo", nil),
			Entry("empty list", MachineDeployments{}, "foo", nil),
			Entry("entry not found", MachineDeployments{{Name: "bar"}}, "foo", nil),
			Entry("entry exists", MachineDeployments{{Name: "bar"}}, "bar", &MachineDeployment{Name: "bar"}),
		)

		DescribeTable("#HasClass",
			func(machineDeployments MachineDeployments, class string, expectation bool) {
				Expect(machineDeployments.HasClass(class)).To(Equal(expectation))
			},

			Entry("list is nil", nil, "foo", false),
			Entry("empty list", MachineDeployments{}, "foo", false),
			Entry("entry not found", MachineDeployments{{ClassName: "bar"}}, "foo", false),
			Entry("entry exists", MachineDeployments{{ClassName: "bar"}}, "bar", true),
		)

		DescribeTable("#HasSecret",
			func(machineDeployments MachineDeployments, secret string, expectation bool) {
				Expect(machineDeployments.HasSecret(secret)).To(Equal(expectation))
			},

			Entry("list is nil", nil, "foo", false),
			Entry("empty list", MachineDeployments{}, "foo", false),
			Entry("entry not found", MachineDeployments{{SecretName: "bar"}}, "foo", false),
			Entry("entry exists", MachineDeployments{{SecretName: "bar"}}, "bar", true),
		)
	})

	Describe("#WorkerPoolHashV1", func() {
		var (
			p                           extensionsv1alpha1.WorkerPool
			c                           *extensionscontroller.Cluster
			hash                        string
			lastCARotationInitiation    = metav1.Time{Time: time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)}
			lastSAKeyRotationInitiation = metav1.Time{Time: time.Date(1, 1, 2, 0, 0, 0, 0, time.UTC)}
			additionalDataV1            []string
			additionalDataV2            []string
		)

		BeforeEach(func() {
			volumeType := "fast"
			p = extensionsv1alpha1.WorkerPool{
				Name:        "test-worker",
				MachineType: "foo",
				MachineImage: extensionsv1alpha1.MachineImage{
					Name:    "bar",
					Version: "baz",
				},
				ProviderConfig: &runtime.RawExtension{
					Raw: []byte("foo"),
				},
				Volume: &extensionsv1alpha1.Volume{
					Type: &volumeType,
					Size: "20Gi",
				},
			}
			c = &extensionscontroller.Cluster{
				Shoot: &gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Kubernetes: gardencorev1beta1.Kubernetes{
							Version: "1.2.3",
						},
					},
					Status: gardencorev1beta1.ShootStatus{
						Credentials: &gardencorev1beta1.ShootCredentials{
							Rotation: &gardencorev1beta1.ShootCredentialsRotation{
								CertificateAuthorities: &gardencorev1beta1.CARotation{
									LastInitiationTime: &lastCARotationInitiation,
								},
								ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
									LastInitiationTime: &lastSAKeyRotationInitiation,
								},
							},
						},
					},
				},
			}
			additionalDataV1 = []string{"sample"}
			additionalDataV2 = []string{"sample"}

			var err error
			hash, err = WorkerPoolHash(p, c, additionalDataV1, additionalDataV2)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("hash value should not change", func() {
			AfterEach(func() {
				actual, err := WorkerPoolHash(p, c, additionalDataV1, additionalDataV2)
				Expect(err).NotTo(HaveOccurred())
				Expect(actual).To(Equal(hash))
			})

			It("when changing minimum", func() {
				p.Minimum = 1
			})

			It("when changing maximum", func() {
				p.Maximum = 2
			})

			It("when changing priority", func() {
				p.Priority = ptr.To(int32(1337))
			})

			It("when changing max surge", func() {
				p.MaxSurge.StrVal = "new-val"
			})

			It("when changing max unavailable", func() {
				p.MaxUnavailable.StrVal = "new-val"
			})

			It("when changing annotations", func() {
				p.Annotations = map[string]string{"foo": "bar"}
			})

			It("when changing labels", func() {
				p.Labels = map[string]string{"foo": "bar"}
			})

			It("when changing taints", func() {
				p.Taints = []corev1.Taint{{Key: "foo"}}
			})

			It("when changing name", func() {
				p.Name = "different-name"
			})

			It("when changing user-data secret ref", func() {
				p.UserDataSecretRef = corev1.SecretKeySelector{Key: "foo"}
			})

			It("when changing zones", func() {
				p.Zones = []string{"1"}
			})

			It("when changing the kubernetes patch version of the worker pool version", func() {
				p.KubernetesVersion = ptr.To("1.2.4")
			})

			It("when changing the kubernetes patch version of the control plane version", func() {
				c.Shoot.Spec.Kubernetes.Version = "1.2.4"
			})

			It("when disabling node local dns via specification", func() {
				c.Shoot.Spec.SystemComponents = &gardencorev1beta1.SystemComponents{NodeLocalDNS: &gardencorev1beta1.NodeLocalDNS{Enabled: false}}
			})

			It("when changing additional data for V2", func() {
				additionalDataV2 = []string{"test"}
			})

			It("when a shoot CA rotation is triggered but worker pool rollout is pending", func() {
				newRotationTime := metav1.Time{Time: lastCARotationInitiation.Add(time.Hour)}
				c.Shoot.Status.Credentials.Rotation.CertificateAuthorities.LastInitiationTime = &newRotationTime
				c.Shoot.Status.Credentials.Rotation.CertificateAuthorities.PendingWorkersRollouts = []gardencorev1beta1.PendingWorkersRollout{{
					Name:               p.Name,
					LastInitiationTime: &lastCARotationInitiation,
				}}
			})

			It("when a shoot service account key rotation is triggered but worker pool rollout is pending", func() {
				newRotationTime := metav1.Time{Time: lastSAKeyRotationInitiation.Add(time.Hour)}
				c.Shoot.Status.Credentials.Rotation.ServiceAccountKey.LastInitiationTime = &newRotationTime
				c.Shoot.Status.Credentials.Rotation.ServiceAccountKey.PendingWorkersRollouts = []gardencorev1beta1.PendingWorkersRollout{{
					Name:               p.Name,
					LastInitiationTime: &lastSAKeyRotationInitiation,
				}}
			})
		})

		Context("hash value should change", func() {
			AfterEach(func() {
				actual, err := WorkerPoolHash(p, c, additionalDataV1, additionalDataV2)
				Expect(err).NotTo(HaveOccurred())
				Expect(actual).NotTo(Equal(hash))
			})

			It("when changing additional data", func() {
				additionalDataV1 = []string{"test"}
			})

			It("when changing machine type", func() {
				p.MachineType = "small"
			})

			It("when changing machine image name", func() {
				p.MachineImage.Name = "new-image"
			})

			It("when changing machine image version", func() {
				p.MachineImage.Version = "new-version"
			})

			It("when changing volume type", func() {
				t := "xl"
				p.Volume.Type = &t
			})

			It("when changing volume size", func() {
				p.Volume.Size = "100Mi"
			})

			It("when changing provider config", func() {
				p.ProviderConfig.Raw = nil
			})

			It("when changing the kubernetes major/minor version of the worker pool version", func() {
				p.KubernetesVersion = ptr.To("1.3.3")
			})

			It("when changing the kubernetes major/minor version of the control plane version", func() {
				c.Shoot.Spec.Kubernetes.Version = "1.3.3"
			})

			It("when changing the CRI configurations", func() {
				c.Shoot.Spec.Provider = gardencorev1beta1.Provider{Workers: []gardencorev1beta1.Worker{
					{Name: "test-worker", CRI: &gardencorev1beta1.CRI{Name: gardencorev1beta1.CRINameContainerD}}}}
			})

			It("when a shoot CA rotation is triggered", func() {
				newRotationTime := metav1.Time{Time: lastCARotationInitiation.Add(time.Hour)}
				c.Shoot.Status.Credentials.Rotation.CertificateAuthorities.LastInitiationTime = &newRotationTime
			})

			It("when a shoot CA rotation is triggered for the first time (lastInitiationTime was nil)", func() {
				var err error
				credentialStatusWithInitiatedRotation := c.Shoot.Status.Credentials.Rotation.CertificateAuthorities.DeepCopy()
				c.Shoot.Status.Credentials.Rotation.CertificateAuthorities = nil
				hash, err = WorkerPoolHash(p, c, additionalDataV1, additionalDataV2)
				Expect(err).ToNot(HaveOccurred())

				c.Shoot.Status.Credentials.Rotation.CertificateAuthorities = credentialStatusWithInitiatedRotation
			})

			It("when a shoot service account key rotation is triggered", func() {
				newRotationTime := metav1.Time{Time: lastSAKeyRotationInitiation.Add(time.Hour)}
				c.Shoot.Status.Credentials.Rotation.ServiceAccountKey.LastInitiationTime = &newRotationTime
			})

			It("when a shoot service account key rotation is triggered for the first time (lastInitiationTime was nil)", func() {
				var err error
				credentialStatusWithInitiatedRotation := c.Shoot.Status.Credentials.Rotation.ServiceAccountKey.DeepCopy()
				c.Shoot.Status.Credentials.Rotation.ServiceAccountKey = nil
				hash, err = WorkerPoolHash(p, c, additionalDataV1, additionalDataV2)
				Expect(err).ToNot(HaveOccurred())

				c.Shoot.Status.Credentials.Rotation.ServiceAccountKey = credentialStatusWithInitiatedRotation
			})

			It("when enabling node local dns via specification", func() {
				c.Shoot.Spec.SystemComponents = &gardencorev1beta1.SystemComponents{NodeLocalDNS: &gardencorev1beta1.NodeLocalDNS{Enabled: true}}
			})
		})
	})

	Describe("#WorkerPoolHashV2", func() {
		var (
			p                extensionsv1alpha1.WorkerPool
			hash             string
			additionalDataV1 []string
			additionalDataV2 []string
		)

		BeforeEach(func() {
			nodeAgentSecretName := "sample-hash"
			p = extensionsv1alpha1.WorkerPool{
				NodeAgentSecretName: &nodeAgentSecretName,
			}
			additionalDataV1 = []string{"sample"}
			additionalDataV2 = []string{"sample"}

			var err error
			hash, err = WorkerPoolHash(p, nil, additionalDataV1, additionalDataV2)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("hash value should not change", func() {
			AfterEach(func() {
				actual, err := WorkerPoolHash(p, nil, additionalDataV1, additionalDataV2)
				Expect(err).NotTo(HaveOccurred())
				Expect(actual).To(Equal(hash))
			})

			It("when changing priority", func() {
				p.Priority = ptr.To(int32(1337))
			})

			It("when changing annotations", func() {
				p.Annotations = map[string]string{"foo": "bar"}
			})

			It("when changing labels", func() {
				p.Labels = map[string]string{"foo": "bar"}
			})

			It("when changing taints", func() {
				p.Taints = []corev1.Taint{{Key: "foo"}}
			})

			It("when changing name", func() {
				p.Name = "different-name"
			})

			It("when changing zones", func() {
				p.Zones = []string{"1"}
			})

			It("when changing the kubernetes patch version of the worker pool version", func() {
				p.KubernetesVersion = ptr.To("1.2.4")
			})

			It("when changing additional data for V1", func() {
				additionalDataV1 = []string{"test"}
			})

			It("when changing machine type", func() {
				p.MachineType = "small"
			})

			It("when changing the kubernetes major/minor version of the worker pool version", func() {
				p.KubernetesVersion = ptr.To("1.3.3")
			})
		})

		Context("hash value should change", func() {
			AfterEach(func() {
				actual, err := WorkerPoolHash(p, nil, additionalDataV1, additionalDataV2)
				Expect(err).NotTo(HaveOccurred())
				Expect(actual).NotTo(Equal(hash))
			})

			It("when changing additional data", func() {
				additionalDataV2 = []string{"test"}
			})

			It("when changing nodeAgentSecretName", func() {
				p.NodeAgentSecretName = ptr.To("different")
			})
		})
	})

	DescribeTable("#DistributeOverZones",
		func(zoneIndex, size, zoneSize, expectation int) {
			Expect(DistributeOverZones(int32(zoneIndex), int32(size), int32(zoneSize))).To(Equal(int32(expectation)))
		},

		Entry("one zone, size 5", 0, 5, 1, 5),
		Entry("two zones, size 5, first index", 0, 5, 2, 3),
		Entry("two zones, size 5, second index", 1, 5, 2, 2),
		Entry("two zones, size 6, first index", 0, 6, 2, 3),
		Entry("two zones, size 6, second index", 1, 6, 2, 3),
		Entry("three zones, size 9, first index", 0, 9, 3, 3),
		Entry("three zones, size 9, second index", 1, 9, 3, 3),
		Entry("three zones, size 9, third index", 2, 9, 3, 3),
		Entry("three zones, size 10, first index", 0, 10, 3, 4),
		Entry("three zones, size 10, second index", 1, 10, 3, 3),
		Entry("three zones, size 10, third index", 2, 10, 3, 3),
	)

	DescribeTable("#DistributePercentOverZones",
		func(zoneIndex int, percent string, zoneSize, total int, expectation string) {
			Expect(DistributePercentOverZones(int32(zoneIndex), percent, int32(zoneSize), int32(total))).To(Equal(expectation))
		},

		Entry("even size, size 2", 0, "10%", 2, 8, "10%"),
		Entry("even size, size 2", 1, "50%", 2, 2, "50%"),
		Entry("uneven size, size 2", 0, "50%", 2, 5, "60%"),
		Entry("uneven size, size 2", 1, "50%", 2, 5, "40%"),
		Entry("uneven size, size 3", 0, "75%", 3, 5, "90%"),
		Entry("uneven size, size 3", 1, "75%", 3, 5, "90%"),
		Entry("uneven size, size 3", 2, "75%", 3, 5, "45%"),
	)

	DescribeTable("#DistributePositiveIntOrPercent",
		func(zoneIndex int, intOrPercent intstr.IntOrString, zoneSize, total int, expectation intstr.IntOrString) {
			Expect(DistributePositiveIntOrPercent(int32(zoneIndex), intOrPercent, int32(zoneSize), int32(total))).To(Equal(expectation))
		},

		Entry("percent", 2, intstr.FromString("75%"), 3, 5, intstr.FromString("45%")),
		Entry("positive int", 2, intstr.FromInt32(10), 3, 3, intstr.FromInt32(3)),
	)

	DescribeTable("#DiskSize",
		func(size string, expectation int, errMatcher types.GomegaMatcher) {
			val, err := DiskSize(size)

			Expect(val).To(Equal(expectation))
			Expect(err).To(errMatcher)
		},

		Entry("1-digit size", "2Gi", 2, BeNil()),
		Entry("2-digit size", "20Gi", 20, BeNil()),
		Entry("3-digit size", "200Gi", 200, BeNil()),
		Entry("4-digit size", "2000Gi", 2000, BeNil()),
		Entry("non-parseable size", "foo", -1, HaveOccurred()),
	)

	Describe("#FetchUserData", func() {
		var (
			ctx        = context.Background()
			fakeClient client.Client

			namespace = "some-namespace"
			pool      extensionsv1alpha1.WorkerPool
			userData  = []byte("some-user-data")

			secretName = "some-secret-name"
			key        = "the-key"
			secret     *corev1.Secret
		)

		BeforeEach(func() {
			pool = extensionsv1alpha1.WorkerPool{Name: "pool"}
			fakeClient = fakeclient.NewClientBuilder().Build()
		})

		BeforeEach(func() {
			pool.UserDataSecretRef = corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
				Key:                  key,
			}
			secret = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: namespace}}
		})

		It("should fail because the referenced secret is not found", func() {
			result, err := FetchUserData(ctx, fakeClient, namespace, pool)
			Expect(err).To(MatchError(ContainSubstring("failed fetching user data secret")))
			Expect(result).To(BeNil())
		})

		It("should fail because the referenced secret does not contain key", func() {
			Expect(fakeClient.Create(ctx, secret)).To(Succeed())

			result, err := FetchUserData(ctx, fakeClient, namespace, pool)
			Expect(err).To(MatchError(ContainSubstring("field or it's empty")))
			Expect(result).To(BeNil())
		})

		It("should fail because the referenced secret's key is empty", func() {
			secret.Data = map[string][]byte{key: nil}
			Expect(fakeClient.Create(ctx, secret)).To(Succeed())

			result, err := FetchUserData(ctx, fakeClient, namespace, pool)
			Expect(err).To(MatchError(ContainSubstring("field or it's empty")))
			Expect(result).To(BeNil())
		})

		It("should return the user data because the referenced secret's key is set", func() {
			secret.Data = map[string][]byte{key: userData}
			Expect(fakeClient.Create(ctx, secret)).To(Succeed())

			result, err := FetchUserData(ctx, fakeClient, namespace, pool)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(userData))
		})
	})
})
