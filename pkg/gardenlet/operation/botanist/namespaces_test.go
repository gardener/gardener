// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"
	"strings"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	"github.com/gardener/gardener/pkg/gardenlet/operation/garden"
	"github.com/gardener/gardener/pkg/gardenlet/operation/seed"
	"github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Namespaces", func() {
	var (
		gardenClient  client.Client
		seedClient    client.Client
		seedClientSet kubernetes.Interface

		botanist *Botanist

		defaultSeedInfo  *gardencorev1beta1.Seed
		defaultShootInfo *gardencorev1beta1.Shoot

		ctx       = context.Background()
		namespace = "shoot--foo--bar"

		obj *corev1.Namespace

		extensionType1          = "shoot-custom-service-1"
		extensionType2          = "shoot-custom-service-2"
		extensionType3          = "shoot-custom-service-3"
		extensionType4          = "shoot-custom-service-4"
		controllerRegistration1 = &gardencorev1beta1.ControllerRegistration{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ctrlreg1",
			},
			Spec: gardencorev1beta1.ControllerRegistrationSpec{
				Resources: []gardencorev1beta1.ControllerResource{
					{
						Kind:       extensionsv1alpha1.ExtensionResource,
						Type:       extensionType3,
						AutoEnable: []gardencorev1beta1.ClusterType{"shoot"},
					},
				},
			},
		}
		controllerRegistration2 = &gardencorev1beta1.ControllerRegistration{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ctrlreg2",
			},
			Spec: gardencorev1beta1.ControllerRegistrationSpec{
				Resources: []gardencorev1beta1.ControllerResource{
					{
						Kind: extensionsv1alpha1.ExtensionResource,
						Type: extensionType4,
					},
				},
			},
		}
	)

	BeforeEach(func() {
		gardenClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithObjects(controllerRegistration1, controllerRegistration2).Build()
		seedClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		seedClientSet = fakekubernetes.NewClientSetBuilder().WithClient(seedClient).Build()

		botanist = &Botanist{Operation: &operation.Operation{
			Logger:        logr.Discard(),
			GardenClient:  gardenClient,
			SeedClientSet: seedClientSet,
			Seed:          &seed.Seed{},
			Shoot:         &shoot.Shoot{ControlPlaneNamespace: namespace},
			Garden:        &garden.Garden{},
		}}

		obj = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}
	})

	Describe("#DeployControlPlaneNamespace", func() {
		var (
			seedProviderType       = "seed-provider"
			seedZones              = []string{"a", "b", "c", "d", "e"}
			backupProviderType     = "backup-provider"
			shootProviderType      = "shoot-provider"
			networkingProviderType = "networking-provider"
			uid                    = types.UID("12345")

			haveNumberOfZones = func(no int) gomegatypes.GomegaMatcher {
				return HaveLen(no + no - 1) // zones are comma-separated
			}
		)

		BeforeEach(func() {
			defaultSeedInfo = &gardencorev1beta1.Seed{
				Spec: gardencorev1beta1.SeedSpec{
					Provider: gardencorev1beta1.SeedProvider{
						Type:  seedProviderType,
						Zones: seedZones,
					},
				},
			}

			defaultShootInfo = &gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Provider: gardencorev1beta1.Provider{
						Type: shootProviderType,
					},
					Networking: &gardencorev1beta1.Networking{
						Type: ptr.To(networkingProviderType),
					},
				},
				Status: gardencorev1beta1.ShootStatus{
					UID: uid,
				},
			}
			botanist.Shoot.SetInfo(defaultShootInfo)
		})

		JustBeforeEach(func() {
			botanist.Seed.SetInfo(defaultSeedInfo)
		})

		defaultExpectations := func(failureToleranceType gardencorev1beta1.FailureToleranceType, numberOfZones int) {
			ExpectWithOffset(1, botanist.SeedNamespaceObject.Name).To(Equal(namespace))
			ExpectWithOffset(1, botanist.SeedNamespaceObject.Annotations).To(And(
				HaveKeyWithValue("shoot.gardener.cloud/uid", string(uid)),
				HaveKeyWithValue("high-availability-config.resources.gardener.cloud/failure-tolerance-type", string(failureToleranceType)),
			))

			if numberOfZones > 0 {
				ExpectWithOffset(1, botanist.SeedNamespaceObject.Annotations).To(HaveKeyWithValue("high-availability-config.resources.gardener.cloud/zones", haveNumberOfZones(numberOfZones)))
			} else {
				ExpectWithOffset(1, botanist.SeedNamespaceObject.Annotations).NotTo(HaveKey("high-availability-config.resources.gardener.cloud/zones"))
			}

			ExpectWithOffset(1, botanist.SeedNamespaceObject.Labels).To(And(
				HaveKeyWithValue("gardener.cloud/role", "shoot"),
				HaveKeyWithValue("seed.gardener.cloud/provider", seedProviderType),
				HaveKeyWithValue("shoot.gardener.cloud/provider", shootProviderType),
				HaveKeyWithValue("networking.shoot.gardener.cloud/provider", networkingProviderType),
				HaveKeyWithValue("high-availability-config.resources.gardener.cloud/consider", "true"),
				HaveKeyWithValue("pod-security.kubernetes.io/enforce", "privileged"),
			))
		}

		It("should successfully deploy the namespace", func() {
			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(BeNotFoundError())
			Expect(botanist.SeedNamespaceObject).To(BeNil())

			Expect(botanist.DeployControlPlaneNamespace(ctx)).To(Succeed())

			defaultExpectations("", 1)
		})

		It("should successfully deploy the namespace when seed has no zones", func() {
			defaultSeedInfo.Spec.Provider.Zones = nil
			botanist.Seed.SetInfo(defaultSeedInfo)

			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(BeNotFoundError())
			Expect(botanist.SeedNamespaceObject).To(BeNil())

			Expect(botanist.DeployControlPlaneNamespace(ctx)).To(Succeed())

			defaultExpectations("", 0)
		})

		It("should successfully deploy the namespace w/ dedicated backup provider", func() {
			defaultSeedInfo.Spec.Backup = &gardencorev1beta1.Backup{Provider: backupProviderType}
			botanist.Seed.SetInfo(defaultSeedInfo)

			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(BeNotFoundError())
			Expect(botanist.SeedNamespaceObject).To(BeNil())

			Expect(botanist.DeployControlPlaneNamespace(ctx)).To(Succeed())

			defaultExpectations("", 1)
			Expect(botanist.SeedNamespaceObject.Labels).To(And(
				HaveKeyWithValue("backup.gardener.cloud/provider", backupProviderType),
				HaveKeyWithValue("extensions.gardener.cloud/"+extensionType3, "true"),
			))
		})

		It("should successfully deploy the namespace with enabled extension labels", func() {
			defaultShootInfo.Spec.Extensions = []gardencorev1beta1.Extension{
				{Type: extensionType1},
				{Type: extensionType2},
			}
			botanist.Shoot.SetInfo(defaultShootInfo)

			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(BeNotFoundError())
			Expect(botanist.SeedNamespaceObject).To(BeNil())

			Expect(botanist.DeployControlPlaneNamespace(ctx)).To(Succeed())

			defaultExpectations("", 1)
			Expect(botanist.SeedNamespaceObject.Labels).To(And(
				HaveKeyWithValue("extensions.gardener.cloud/"+extensionType1, "true"),
				HaveKeyWithValue("extensions.gardener.cloud/"+extensionType2, "true"),
			))
		})

		It("should successfully deploy the namespace when failure tolerance type is zone", func() {
			defaultShootInfo.Spec.ControlPlane = &gardencorev1beta1.ControlPlane{
				HighAvailability: &gardencorev1beta1.HighAvailability{
					FailureTolerance: gardencorev1beta1.FailureTolerance{
						Type: gardencorev1beta1.FailureToleranceTypeZone,
					},
				},
			}
			botanist.Shoot.SetInfo(defaultShootInfo)

			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(BeNotFoundError())
			Expect(botanist.SeedNamespaceObject).To(BeNil())

			Expect(botanist.DeployControlPlaneNamespace(ctx)).To(Succeed())

			defaultExpectations(gardencorev1beta1.FailureToleranceTypeZone, 3)
		})

		It("should successfully deploy the namespace when failure tolerance type is zone and zones annotation already exists with too less zones", func() {
			Expect(botanist.DeployControlPlaneNamespace(ctx)).To(Succeed())

			defaultExpectations("", 1)
			zone := botanist.SeedNamespaceObject.Annotations["high-availability-config.resources.gardener.cloud/zones"]

			defaultShootInfo.Spec.ControlPlane = &gardencorev1beta1.ControlPlane{
				HighAvailability: &gardencorev1beta1.HighAvailability{
					FailureTolerance: gardencorev1beta1.FailureTolerance{
						Type: gardencorev1beta1.FailureToleranceTypeZone,
					},
				},
			}
			botanist.Shoot.SetInfo(defaultShootInfo)

			Expect(botanist.DeployControlPlaneNamespace(ctx)).To(Succeed())

			defaultExpectations(gardencorev1beta1.FailureToleranceTypeZone, 3)
			Expect(strings.Split(botanist.SeedNamespaceObject.Annotations["high-availability-config.resources.gardener.cloud/zones"], ",")).To(ContainElement(zone))
		})

		It("should successfully deploy the namespace when failure tolerance type is zone and zones annotation already exists with enough zones", func() {
			defaultShootInfo.Spec.ControlPlane = &gardencorev1beta1.ControlPlane{
				HighAvailability: &gardencorev1beta1.HighAvailability{
					FailureTolerance: gardencorev1beta1.FailureTolerance{
						Type: gardencorev1beta1.FailureToleranceTypeZone,
					},
				},
			}
			botanist.Shoot.SetInfo(defaultShootInfo)

			Expect(botanist.DeployControlPlaneNamespace(ctx)).To(Succeed())

			defaultExpectations(gardencorev1beta1.FailureToleranceTypeZone, 3)
			zones := botanist.SeedNamespaceObject.Annotations["high-availability-config.resources.gardener.cloud/zones"]

			Expect(botanist.DeployControlPlaneNamespace(ctx)).To(Succeed())

			defaultExpectations(gardencorev1beta1.FailureToleranceTypeZone, 3)
			Expect(botanist.SeedNamespaceObject.Annotations["high-availability-config.resources.gardener.cloud/zones"]).To(Equal(zones))
		})

		It("should fail deploying the namespace when seed specification does not contain enough zones", func() {
			defaultSeedInfo.Spec.Provider.Zones = []string{"a", "b"}
			botanist.Seed.SetInfo(defaultSeedInfo)

			defaultShootInfo.Spec.ControlPlane = &gardencorev1beta1.ControlPlane{
				HighAvailability: &gardencorev1beta1.HighAvailability{
					FailureTolerance: gardencorev1beta1.FailureTolerance{
						Type: gardencorev1beta1.FailureToleranceTypeZone,
					},
				},
			}
			botanist.Shoot.SetInfo(defaultShootInfo)

			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(BeNotFoundError())
			Expect(botanist.SeedNamespaceObject).To(BeNil())

			Expect(botanist.DeployControlPlaneNamespace(ctx)).To(MatchError(ContainSubstring("cannot select 3 zones for shoot because seed only specifies 2 zones in its specification")))
		})

		Context("zone pinning backwards compatibility", func() {
			Context("when volume creation is completed", func() {
				BeforeEach(func() {
					for key, existingZones := range map[string][]string{
						"failure-domain.beta.kubernetes.io/zone": {"a", "b"},
						"topology.foo.bar/zone":                  {"b", "a"},
					} {
						pv := &corev1.PersistentVolume{
							ObjectMeta: metav1.ObjectMeta{
								GenerateName: "pv-",
							},
							Spec: corev1.PersistentVolumeSpec{
								NodeAffinity: &corev1.VolumeNodeAffinity{
									Required: &corev1.NodeSelector{
										NodeSelectorTerms: []corev1.NodeSelectorTerm{
											{
												MatchExpressions: []corev1.NodeSelectorRequirement{
													{
														Key:      "foo",
														Operator: "In",
														Values:   []string{"11", "12", "13"},
													},
													{
														Key:      key,
														Operator: "In",
														Values:   existingZones,
													},
												},
											},
										},
									},
								},
							},
						}
						Expect(seedClient.Create(ctx, pv)).To(Succeed())

						pvc := &corev1.PersistentVolumeClaim{
							ObjectMeta: metav1.ObjectMeta{
								GenerateName: "pvc-",
								Namespace:    namespace,
							},
							Spec: corev1.PersistentVolumeClaimSpec{
								VolumeName: pv.Name,
							},
						}
						Expect(seedClient.Create(ctx, pvc)).To(Succeed())
					}
				})

				It("should use existing zones instead of picking a new zone", func() {
					Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(BeNotFoundError())
					Expect(botanist.SeedNamespaceObject).To(BeNil())

					Expect(botanist.DeployControlPlaneNamespace(ctx)).To(Succeed())

					defaultExpectations("", 2)
					Expect(botanist.SeedNamespaceObject.Annotations).To(HaveKeyWithValue("high-availability-config.resources.gardener.cloud/zones", "a,b"))
				})

				It("should use existing zones and pick new zones until the required number is reached", func() {
					defaultShootInfo.Spec.ControlPlane = &gardencorev1beta1.ControlPlane{
						HighAvailability: &gardencorev1beta1.HighAvailability{
							FailureTolerance: gardencorev1beta1.FailureTolerance{
								Type: gardencorev1beta1.FailureToleranceTypeZone,
							},
						},
					}
					botanist.Shoot.SetInfo(defaultShootInfo)

					Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(BeNotFoundError())
					Expect(botanist.SeedNamespaceObject).To(BeNil())

					Expect(botanist.DeployControlPlaneNamespace(ctx)).To(Succeed())

					defaultExpectations(gardencorev1beta1.FailureToleranceTypeZone, 3)
					Expect(strings.Split(botanist.SeedNamespaceObject.Annotations["high-availability-config.resources.gardener.cloud/zones"], ",")).To(ConsistOf(
						"a",
						"b",
						Or(Equal("c"), Equal("d"), Equal("e")),
					))
				})

				It("should use zone information from namespace and find existing ones via persistent volumes", func() {
					Expect(seedClient.Create(ctx, &corev1.Namespace{
						ObjectMeta: metav1.ObjectMeta{
							Name: namespace,
							Annotations: map[string]string{
								"high-availability-config.resources.gardener.cloud/zones": "a",
							},
						},
					})).To(Succeed())

					defaultShootInfo.Spec.ControlPlane = &gardencorev1beta1.ControlPlane{
						HighAvailability: &gardencorev1beta1.HighAvailability{
							FailureTolerance: gardencorev1beta1.FailureTolerance{
								Type: gardencorev1beta1.FailureToleranceTypeZone,
							},
						},
					}
					botanist.Shoot.SetInfo(defaultShootInfo)

					Expect(botanist.SeedNamespaceObject).To(BeNil())

					Expect(botanist.DeployControlPlaneNamespace(ctx)).To(Succeed())

					defaultExpectations(gardencorev1beta1.FailureToleranceTypeZone, 3)
					Expect(strings.Split(botanist.SeedNamespaceObject.Annotations["high-availability-config.resources.gardener.cloud/zones"], ",")).To(ConsistOf(
						"a",
						"b",
						Or(Equal("c"), Equal("d"), Equal("e")),
					))
				})

				It("should not use zone information from persistent volumes if zone is missing in seed spec", func() {
					defaultSeedInfo.Spec.Provider.Zones = []string{"1", "2", "3"}

					Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(BeNotFoundError())
					Expect(botanist.SeedNamespaceObject).To(BeNil())

					Expect(botanist.DeployControlPlaneNamespace(ctx)).To(Succeed())

					defaultExpectations("", 1)
					Expect(strings.Split(botanist.SeedNamespaceObject.Annotations["high-availability-config.resources.gardener.cloud/zones"], ",")).To(ConsistOf(
						Or(Equal("1"), Equal("2"), Equal("3")),
					))
				})

				It("should not amend zone information if failure tolerance is unchanged", func() {
					Expect(seedClient.Create(ctx, &corev1.Namespace{
						ObjectMeta: metav1.ObjectMeta{
							Name: namespace,
							Annotations: map[string]string{
								"high-availability-config.resources.gardener.cloud/zones": "1,2,a,b",
								"high-availability-config.resources.gardener.cloud/type":  "zone",
							},
						},
					})).To(Succeed())

					defaultShootInfo.Spec.ControlPlane = &gardencorev1beta1.ControlPlane{
						HighAvailability: &gardencorev1beta1.HighAvailability{
							FailureTolerance: gardencorev1beta1.FailureTolerance{
								Type: gardencorev1beta1.FailureToleranceTypeZone,
							},
						},
					}
					botanist.Shoot.SetInfo(defaultShootInfo)

					Expect(botanist.SeedNamespaceObject).To(BeNil())

					Expect(botanist.DeployControlPlaneNamespace(ctx)).To(Succeed())

					defaultExpectations(gardencorev1beta1.FailureToleranceTypeZone, 4)
					Expect(botanist.SeedNamespaceObject.Annotations).To(HaveKeyWithValue("high-availability-config.resources.gardener.cloud/zones", "1,2,a,b"))
				})
			})

			Context("when volume creation is in progress", func() {
				BeforeEach(func() {
					pvc := &corev1.PersistentVolumeClaim{
						ObjectMeta: metav1.ObjectMeta{
							GenerateName: "pvc-",
							Namespace:    namespace,
						},
						Spec: corev1.PersistentVolumeClaimSpec{},
					}
					Expect(seedClient.Create(ctx, pvc)).To(Succeed())
				})

				It("should pick a new zone", func() {
					Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(BeNotFoundError())
					Expect(botanist.SeedNamespaceObject).To(BeNil())

					Expect(botanist.DeployControlPlaneNamespace(ctx)).To(Succeed())

					defaultExpectations("", 1)
				})
			})
		})

		It("should successfully remove extension labels from the namespace when extensions are deleted from shoot spec or marked as disabled", func() {
			defaultShootInfo.Spec.Extensions = []gardencorev1beta1.Extension{
				{Type: extensionType1},
				{Type: extensionType3, Disabled: ptr.To(true)},
			}
			botanist.Shoot.SetInfo(defaultShootInfo)

			Expect(seedClient.Create(ctx, &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespace,
					Annotations: map[string]string{
						"shoot.gardener.cloud/uid": string(uid),
					},
					Labels: map[string]string{
						"gardener.cloud/role":                         "shoot",
						"seed.gardener.cloud/provider":                seedProviderType,
						"shoot.gardener.cloud/provider":               shootProviderType,
						"networking.shoot.gardener.cloud/provider":    networkingProviderType,
						"backup.gardener.cloud/provider":              seedProviderType,
						"extensions.gardener.cloud/" + extensionType1: "true",
						"extensions.gardener.cloud/" + extensionType2: "true",
						"extensions.gardener.cloud/" + extensionType3: "true",
					},
				},
			})).To(Succeed())

			Expect(botanist.SeedNamespaceObject).To(BeNil())
			Expect(botanist.DeployControlPlaneNamespace(ctx)).To(Succeed())

			defaultExpectations("", 1)
			Expect(botanist.SeedNamespaceObject.Labels).To(And(
				HaveKeyWithValue("extensions.gardener.cloud/"+extensionType1, "true"),
				Not(HaveKeyWithValue("extensions.gardener.cloud/"+extensionType2, "true")),
				Not(HaveKeyWithValue("extensions.gardener.cloud/"+extensionType3, "true")),
			))
		})

		It("should not overwrite other annotations or labels", func() {
			Expect(seedClient.Create(ctx, &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:        namespace,
					Annotations: map[string]string{"foo": "bar"},
					Labels:      map[string]string{"bar": "foo"},
				},
			})).To(Succeed())

			Expect(botanist.SeedNamespaceObject).To(BeNil())
			Expect(botanist.DeployControlPlaneNamespace(ctx)).To(Succeed())
			Expect(botanist.SeedNamespaceObject.Annotations).To(HaveKeyWithValue("foo", "bar"))
			Expect(botanist.SeedNamespaceObject.Labels).To(HaveKeyWithValue("bar", "foo"))
		})
	})

	Describe("#DeleteSeedNamespace", func() {
		It("should successfully delete the namespace despite 'not found' error", func() {
			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(BeNotFoundError())
			Expect(botanist.DeleteSeedNamespace(ctx)).To(Succeed())
			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(BeNotFoundError())
		})

		It("should successfully delete the namespace (no error)", func() {
			Expect(seedClient.Create(ctx, obj)).To(Succeed())
			Expect(botanist.DeleteSeedNamespace(ctx)).To(Succeed())
			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(BeNotFoundError())
		})
	})

	DescribeTable("#ExtractZonesFromNodeSelectorTerm",
		func(term corev1.NodeSelectorTerm, expectedZones []string) {
			actualZones := ExtractZonesFromNodeSelectorTerm(term)
			Expect(actualZones).To(ConsistOf(expectedZones))
		},

		Entry("without any matchExpressions",
			corev1.NodeSelectorTerm{
				MatchExpressions: []corev1.NodeSelectorRequirement{},
			},
			[]string{},
		),
		Entry("with operator != In",
			corev1.NodeSelectorTerm{
				MatchExpressions: []corev1.NodeSelectorRequirement{
					{
						Key:      "topology.foo.bar/zone",
						Operator: "NotIn",
						Values:   []string{"1", "2", "3"},
					},
				},
			},
			[]string{},
		),
		Entry("with GA topology label (topology.kubernetes.io/zone)",
			corev1.NodeSelectorTerm{
				MatchExpressions: []corev1.NodeSelectorRequirement{
					{
						Key:      corev1.LabelTopologyZone,
						Operator: "In",
						Values:   []string{"1", "2", "3"},
					},
				},
			},
			[]string{"1", "2", "3"},
		),
		Entry("with provider specific topology label (topology.foo.bar/zone)",
			corev1.NodeSelectorTerm{
				MatchExpressions: []corev1.NodeSelectorRequirement{
					{
						Key:      "topology.foo.bar/zone",
						Operator: "In",
						Values:   []string{"1", "2", "3"},
					},
				},
			},
			[]string{"1", "2", "3"},
		),
		Entry("with deprecated topology label (failure-domain.beta.kubernetes.io/zone)",
			corev1.NodeSelectorTerm{
				MatchExpressions: []corev1.NodeSelectorRequirement{
					{
						Key:      corev1.LabelFailureDomainBetaZone,
						Operator: "In",
						Values:   []string{"1", "2", "3"},
					},
				},
			},
			[]string{"1", "2", "3"},
		),
	)
})
