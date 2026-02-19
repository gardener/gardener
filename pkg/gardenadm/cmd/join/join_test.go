// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package join_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/gardenadm/botanist"
	. "github.com/gardener/gardener/pkg/gardenadm/cmd/join"
	operationpkg "github.com/gardener/gardener/pkg/gardenlet/operation"
	botanistpkg "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
)

var _ = Describe("Join", func() {
	var (
		ctx = context.Background()

		fakeClient client.Client
		b          *botanist.GardenadmBotanist
		options    *Options

		shoot   *gardencorev1beta1.Shoot
		cluster *extensionsv1alpha1.Cluster
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		b = &botanist.GardenadmBotanist{
			Botanist: &botanistpkg.Botanist{
				Operation: &operationpkg.Operation{
					ShootClientSet: fakekubernetes.NewClientSetBuilder().WithClient(fakeClient).Build(),
					Shoot:          &shootpkg.Shoot{ControlPlaneNamespace: "kube-system"},
				},
			},
		}
		options = &Options{}

		shoot = &gardencorev1beta1.Shoot{}
	})

	createCluster := func() {
		shootRaw, err := runtime.Encode(&json.Serializer{}, shoot)
		Expect(err).NotTo(HaveOccurred())

		cluster = &extensionsv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: "kube-system"},
			Spec: extensionsv1alpha1.ClusterSpec{
				Shoot: runtime.RawExtension{Raw: shootRaw},
			},
		}
		Expect(fakeClient.Create(ctx, cluster)).To(Succeed())
	}

	Describe("#GetGardenerNodeAgentSecretName", func() {
		BeforeEach(func() {
			shootRaw, err := runtime.Encode(&json.Serializer{}, shoot)
			Expect(err).NotTo(HaveOccurred())

			cluster = &extensionsv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "kube-system"},
				Spec: extensionsv1alpha1.ClusterSpec{
					Shoot: runtime.RawExtension{Raw: shootRaw},
				},
			}
		})

		When("Cluster object does not exist", func() {
			It("should fail when worker pool name is not set", func() {
				secretName, err := GetGardenerNodeAgentSecretName(ctx, options, b)
				Expect(err).To(MatchError(ContainSubstring(`clusters.extensions.gardener.cloud "kube-system" not found`)))
				Expect(secretName).To(BeEmpty())
			})

			When("worker pool name is set", func() {
				BeforeEach(func() {
					options.WorkerPoolName = "some-pool-name"
				})

				It("should fail because there are no gardener-node-agent secrets", func() {
					secretName, err := GetGardenerNodeAgentSecretName(ctx, options, b)
					Expect(err).To(MatchError(ContainSubstring("no gardener-node-agent secrets found")))
					Expect(secretName).To(BeEmpty())
				})

				It("should succeed when there are gardener-node-agent secrets", func() {
					secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
						Name:      "gardener-node-agent-test-pool",
						Namespace: "kube-system",
						Labels: map[string]string{
							"gardener.cloud/role":        "operating-system-config",
							"worker.gardener.cloud/pool": options.WorkerPoolName,
						},
					}}

					Expect(fakeClient.Create(ctx, secret)).To(Succeed())

					secretName, err := GetGardenerNodeAgentSecretName(ctx, options, b)
					Expect(err).NotTo(HaveOccurred())
					Expect(secretName).To(Equal(secret.Name))
				})
			})
		})

		When("Cluster object exists", func() {
			BeforeEach(func() {
				Expect(fakeClient.Create(ctx, cluster)).To(Succeed())
			})

			When("control plane node should be joined", func() {
				BeforeEach(func() {
					options.ControlPlane = true
				})

				It("should fail when there is no control plane pool in the Shoot spec", func() {
					secretName, err := GetGardenerNodeAgentSecretName(ctx, options, b)
					Expect(err).To(MatchError(ContainSubstring("no control plane worker pool found in Shoot manifest")))
					Expect(secretName).To(BeEmpty())
				})

				When("control plane pool is in Shoot spec", func() {
					BeforeEach(func() {
						shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, gardencorev1beta1.Worker{Name: "cp", ControlPlane: &gardencorev1beta1.WorkerControlPlane{}})

						var err error
						cluster.Spec.Shoot.Raw, err = runtime.Encode(&json.Serializer{}, shoot)
						Expect(err).NotTo(HaveOccurred())

						Expect(fakeClient.Update(ctx, cluster)).To(Succeed())
					})

					It("should fail because there are no gardener-node-agent secrets", func() {
						secretName, err := GetGardenerNodeAgentSecretName(ctx, options, b)
						Expect(err).To(MatchError(ContainSubstring("no gardener-node-agent secrets found")))
						Expect(secretName).To(BeEmpty())
					})

					It("should succeed when there are gardener-node-agent secrets", func() {
						secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
							Name:      "gardener-node-agent-cp",
							Namespace: "kube-system",
							Labels: map[string]string{
								"gardener.cloud/role":        "operating-system-config",
								"worker.gardener.cloud/pool": "cp",
							},
						}}

						Expect(fakeClient.Create(ctx, secret)).To(Succeed())

						secretName, err := GetGardenerNodeAgentSecretName(ctx, options, b)
						Expect(err).NotTo(HaveOccurred())
						Expect(secretName).To(Equal(secret.Name))
					})
				})
			})

			When("worker node should be joined", func() {
				It("should fail when there is no non-control-plane pool in Shoot manifest", func() {
					secretName, err := GetGardenerNodeAgentSecretName(ctx, options, b)
					Expect(err).To(MatchError(ContainSubstring("no non-control-plane pool found in Shoot manifest")))
					Expect(secretName).To(BeEmpty())
				})

				When("there are non-control-plane pools in Shoot manifest", func() {
					BeforeEach(func() {
						shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers,
							gardencorev1beta1.Worker{Name: "worker1"},
							gardencorev1beta1.Worker{Name: "worker2"},
						)

						var err error
						cluster.Spec.Shoot.Raw, err = runtime.Encode(&json.Serializer{}, shoot)
						Expect(err).NotTo(HaveOccurred())

						Expect(fakeClient.Update(ctx, cluster)).To(Succeed())
					})

					It("should fail because there are no gardener-node-agent secrets", func() {
						secretName, err := GetGardenerNodeAgentSecretName(ctx, options, b)
						Expect(err).To(MatchError(ContainSubstring("no gardener-node-agent secrets found")))
						Expect(secretName).To(BeEmpty())
					})

					It("should succeed when there are gardener-node-agent secrets", func() {
						secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
							Name:      "gardener-node-agent-worker1",
							Namespace: "kube-system",
							Labels: map[string]string{
								"gardener.cloud/role":        "operating-system-config",
								"worker.gardener.cloud/pool": "worker1",
							},
						}}

						Expect(fakeClient.Create(ctx, secret)).To(Succeed())

						secretName, err := GetGardenerNodeAgentSecretName(ctx, options, b)
						Expect(err).NotTo(HaveOccurred())
						Expect(secretName).To(Equal(secret.Name))
					})
				})
			})
		})
	})

	Describe("#validateZone", func() {
		When("cluster object does not exist", func() {
			It("should fail", func() {
				effectiveZone, err := ValidateZone(ctx, options, b)
				Expect(err).To(MatchError(ContainSubstring(`clusters.extensions.gardener.cloud "kube-system" not found`)))
				Expect(effectiveZone).To(BeEmpty())
			})
		})

		Context("zone validation with managed infrastructure", func() {
			BeforeEach(func() {
				shoot.Spec.CredentialsBindingName = ptr.To("test-credentials")
				shoot.Spec.Provider.Workers = []gardencorev1beta1.Worker{
					{
						Name:         "control-plane",
						Minimum:      1,
						Maximum:      1,
						ControlPlane: &gardencorev1beta1.WorkerControlPlane{},
					},
				}
				createCluster()
			})

			It("should reject zone when provided for managed infrastructure", func() {
				options.Zone = "us-east-1a"

				effectiveZone, err := ValidateZone(ctx, options, b)
				Expect(err).To(MatchError(ContainSubstring("zone can't be configured for shoot with managed infrastrcture")))
				Expect(effectiveZone).To(BeEmpty())
			})

			It("should allow empty zone for managed infrastructure", func() {
				options.Zone = ""

				effectiveZone, err := ValidateZone(ctx, options, b)
				Expect(err).NotTo(HaveOccurred())
				Expect(effectiveZone).To(BeEmpty())
			})
		})

		Context("zone validation with unmanaged infrastructure", func() {
			Context("worker with no zones configured", func() {
				BeforeEach(func() {
					shoot.Spec.Provider.Workers = []gardencorev1beta1.Worker{
						{
							Name:    "worker1",
							Minimum: 1,
							Maximum: 1,
						},
					}
					createCluster()
				})

				It("should reject zone when worker has no zones configured", func() {
					options.Zone = "custom-zone"

					effectiveZone, err := ValidateZone(ctx, options, b)
					Expect(err).To(MatchError("zone validation failed: worker \"worker1\" has no zones configured, but zone \"custom-zone\" was provided"))
					Expect(effectiveZone).To(BeEmpty())
				})

				It("should allow empty zone when worker has no zones", func() {
					options.Zone = ""

					effectiveZone, err := ValidateZone(ctx, options, b)
					Expect(err).NotTo(HaveOccurred())
					Expect(effectiveZone).To(BeEmpty())
				})
			})

			Context("worker with single zone configured", func() {
				BeforeEach(func() {
					shoot.Spec.Provider.Workers = []gardencorev1beta1.Worker{
						{
							Name:    "worker1",
							Minimum: 1,
							Maximum: 1,
							Zones:   []string{"zone-1"},
						},
					}
					createCluster()
				})

				It("should auto-apply the single zone when not provided", func() {
					options.Zone = ""

					effectiveZone, err := ValidateZone(ctx, options, b)
					Expect(err).NotTo(HaveOccurred())
					Expect(effectiveZone).To(Equal("zone-1"))
				})

				It("should accept matching zone when provided", func() {
					options.Zone = "zone-1"

					effectiveZone, err := ValidateZone(ctx, options, b)
					Expect(err).NotTo(HaveOccurred())
					Expect(effectiveZone).To(Equal("zone-1"))
				})

				It("should reject non-matching zone when provided", func() {
					options.Zone = "zone-2"

					effectiveZone, err := ValidateZone(ctx, options, b)
					Expect(err).To(MatchError("zone validation failed: provided zone \"zone-2\" does not match the configured zones [zone-1] for worker \"worker1\""))
					Expect(effectiveZone).To(BeEmpty())
				})
			})

			Context("worker with multiple zones configured", func() {
				BeforeEach(func() {
					shoot.Spec.Provider.Workers = []gardencorev1beta1.Worker{
						{
							Name:    "worker1",
							Minimum: 1,
							Maximum: 1,
							Zones:   []string{"zone-1", "zone-2", "zone-3"},
						},
					}
					createCluster()
				})

				It("should require zone flag when not provided", func() {
					options.Zone = ""

					effectiveZone, err := ValidateZone(ctx, options, b)
					Expect(err).To(MatchError("zone validation failed: worker \"worker1\" has multiple zones configured [zone-1 zone-2 zone-3], --zone flag is required"))
					Expect(effectiveZone).To(BeEmpty())
				})

				It("should accept valid zone when provided", func() {
					options.Zone = "zone-2"

					effectiveZone, err := ValidateZone(ctx, options, b)
					Expect(err).NotTo(HaveOccurred())
					Expect(effectiveZone).To(Equal("zone-2"))
				})

				It("should reject invalid zone when provided", func() {
					options.Zone = "zone-4"

					effectiveZone, err := ValidateZone(ctx, options, b)
					Expect(err).To(MatchError("zone validation failed: provided zone \"zone-4\" does not match the configured zones [zone-1 zone-2 zone-3] for worker \"worker1\""))
					Expect(effectiveZone).To(BeEmpty())
				})
			})

			Context("specific worker pool selection", func() {
				BeforeEach(func() {
					shoot.Spec.Provider.Workers = []gardencorev1beta1.Worker{
						{
							Name:    "worker1",
							Minimum: 1,
							Maximum: 3,
							Zones:   []string{"zone-a"},
						},
						{
							Name:    "worker2",
							Minimum: 1,
							Maximum: 3,
							Zones:   []string{"zone-b", "zone-c"},
						},
					}
					createCluster()
				})

				It("should validate against specific worker pool when name is provided", func() {
					options.WorkerPoolName = "worker2"
					options.Zone = "zone-b"

					effectiveZone, err := ValidateZone(ctx, options, b)
					Expect(err).NotTo(HaveOccurred())
					Expect(effectiveZone).To(Equal("zone-b"))
				})

				It("should reject zone not in specific worker pool", func() {
					options.WorkerPoolName = "worker2"
					options.Zone = "zone-a"

					effectiveZone, err := ValidateZone(ctx, options, b)
					Expect(err).To(MatchError("zone validation failed: provided zone \"zone-a\" does not match the configured zones [zone-b zone-c] for worker \"worker2\""))
					Expect(effectiveZone).To(BeEmpty())
				})
			})
		})
	})
})
