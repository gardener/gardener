// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package botanist_test

import (
	"context"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	"github.com/gardener/gardener/pkg/operation/garden"
	"github.com/gardener/gardener/pkg/operation/seed"
	"github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("Namespaces", func() {
	var (
		gardenClient  client.Client
		seedClient    client.Client
		seedClientSet kubernetes.Interface

		botanist *Botanist

		defaultShootInfo *gardencorev1beta1.Shoot

		ctx       = context.TODO()
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
						Kind:            extensionsv1alpha1.ExtensionResource,
						Type:            extensionType3,
						GloballyEnabled: pointer.Bool(true),
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
						Kind:            extensionsv1alpha1.ExtensionResource,
						Type:            extensionType4,
						GloballyEnabled: pointer.Bool(false),
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
			GardenClient:  gardenClient,
			SeedClientSet: seedClientSet,
			Seed:          &seed.Seed{},
			Shoot:         &shoot.Shoot{SeedNamespace: namespace},
			Garden:        &garden.Garden{},
		}}

		obj = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}
	})

	Describe("#DeploySeedNamespace", func() {
		var (
			seedProviderType       = "seed-provider"
			backupProviderType     = "backup-provider"
			shootProviderType      = "shoot-provider"
			networkingProviderType = "networking-provider"
			uid                    = types.UID("12345")
		)

		BeforeEach(func() {
			botanist.Seed.SetInfo(&gardencorev1beta1.Seed{
				Spec: gardencorev1beta1.SeedSpec{
					Provider: gardencorev1beta1.SeedProvider{
						Type: seedProviderType,
					},
					Settings: &gardencorev1beta1.SeedSettings{
						ShootDNS: &gardencorev1beta1.SeedSettingShootDNS{
							Enabled: true,
						},
					},
				},
			})

			defaultShootInfo = &gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Provider: gardencorev1beta1.Provider{
						Type: shootProviderType,
					},
					Networking: gardencorev1beta1.Networking{
						Type: networkingProviderType,
					},
				},
				Status: gardencorev1beta1.ShootStatus{
					UID: uid,
				},
			}
			botanist.Shoot.SetInfo(defaultShootInfo)
		})

		It("should successfully deploy the namespace w/o dedicated backup provider", func() {
			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: corev1.SchemeGroupVersion.Group, Resource: "namespaces"}, obj.Name)))

			Expect(botanist.SeedNamespaceObject).To(BeNil())
			Expect(botanist.DeploySeedNamespace(ctx)).To(Succeed())
			Expect(botanist.SeedNamespaceObject).To(Equal(&corev1.Namespace{
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
						"extensions.gardener.cloud/" + extensionType3: "true",
					},
					ResourceVersion: "1",
				},
			}))
		})

		It("should successfully deploy the namespace w/ dedicated backup provider", func() {
			botanist.Seed.GetInfo().Spec.Backup = &gardencorev1beta1.SeedBackup{Provider: backupProviderType}

			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: corev1.SchemeGroupVersion.Group, Resource: "namespaces"}, obj.Name)))

			Expect(botanist.SeedNamespaceObject).To(BeNil())
			Expect(botanist.DeploySeedNamespace(ctx)).To(Succeed())
			Expect(botanist.SeedNamespaceObject).To(Equal(&corev1.Namespace{
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
						"backup.gardener.cloud/provider":              backupProviderType,
						"extensions.gardener.cloud/" + extensionType3: "true",
					},
					ResourceVersion: "1",
				},
			}))
		})

		It("should successfully deploy the namespace with enabled extension labels", func() {
			defaultShootInfo.Spec.Extensions = []gardencorev1beta1.Extension{
				{
					Type: extensionType1,
				},
				{
					Type: extensionType2,
				},
			}
			botanist.Shoot.SetInfo(defaultShootInfo)

			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: corev1.SchemeGroupVersion.Group, Resource: "namespaces"}, obj.Name)))

			Expect(botanist.SeedNamespaceObject).To(BeNil())
			Expect(botanist.DeploySeedNamespace(ctx)).To(Succeed())
			Expect(botanist.SeedNamespaceObject).To(Equal(&corev1.Namespace{
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
					ResourceVersion: "1",
				},
			}))
		})

		It("should successfully remove extension labels from the namespace when extensions are deleted from shoot spec or marked as disabled", func() {
			defaultShootInfo.Spec.Extensions = []gardencorev1beta1.Extension{
				{Type: extensionType1},
				{Type: extensionType3, Disabled: pointer.Bool(true)},
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
			Expect(botanist.DeploySeedNamespace(ctx)).To(Succeed())
			Expect(botanist.SeedNamespaceObject).To(Equal(&corev1.Namespace{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Namespace",
				},
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
					},
					ResourceVersion: "2",
				},
			}))
		})

		It("should not overwrite other annotations or labels", func() {
			var (
				customAnnotations = map[string]string{"foo": "bar"}
				customLabels      = map[string]string{"bar": "foo"}
			)

			Expect(seedClient.Create(ctx, &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:        namespace,
					Annotations: customAnnotations,
					Labels:      customLabels,
				},
			})).To(Succeed())

			Expect(botanist.SeedNamespaceObject).To(BeNil())
			Expect(botanist.DeploySeedNamespace(ctx)).To(Succeed())
			Expect(botanist.SeedNamespaceObject).To(Equal(&corev1.Namespace{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Namespace",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: namespace,
					Annotations: utils.MergeStringMaps(customAnnotations, map[string]string{
						"shoot.gardener.cloud/uid": string(uid),
					}),
					Labels: utils.MergeStringMaps(customLabels, map[string]string{
						"gardener.cloud/role":                         "shoot",
						"seed.gardener.cloud/provider":                seedProviderType,
						"shoot.gardener.cloud/provider":               shootProviderType,
						"networking.shoot.gardener.cloud/provider":    networkingProviderType,
						"backup.gardener.cloud/provider":              seedProviderType,
						"extensions.gardener.cloud/" + extensionType3: "true",
					}),
					ResourceVersion: "2",
				},
			}))
		})

		Context("when HAControlPlanes are activated on a seed", func() {
			It("should add zone-pinning label for non-HA shoot in multi-zonal seed", func() {
				seed := botanist.Seed.GetInfo()
				metav1.SetMetaDataLabel(&seed.ObjectMeta, "seed.gardener.cloud/multi-zonal", "")
				botanist.Seed.SetInfo(seed)

				Expect(botanist.DeploySeedNamespace(ctx)).To(Succeed())
				Expect(botanist.SeedNamespaceObject.Labels).To(HaveKeyWithValue("control-plane.shoot.gardener.cloud/enforce-zone", ""))
			})

			It("should add zone-pinning label for single-zonal (identified by alpha HA annotation) shoot in multi-zonal seed", func() {
				shoot := botanist.Shoot.GetInfo()
				metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, v1beta1constants.ShootAlphaControlPlaneHighAvailability, v1beta1constants.ShootAlphaControlPlaneHighAvailabilitySingleZone)
				botanist.Shoot.SetInfo(shoot)

				seed := botanist.Seed.GetInfo()
				metav1.SetMetaDataLabel(&seed.ObjectMeta, "seed.gardener.cloud/multi-zonal", "true")
				botanist.Seed.SetInfo(seed)

				Expect(botanist.DeploySeedNamespace(ctx)).To(Succeed())
				Expect(botanist.SeedNamespaceObject.Labels).To(HaveKeyWithValue("control-plane.shoot.gardener.cloud/enforce-zone", ""))
			})

			It("should add zone-pinning label for single-zonal (identified by Shoot Spec ControlPlane.HighAvailability) shoot in multi-zonal seed", func() {
				shoot := botanist.Shoot.GetInfo()
				shoot.Spec.ControlPlane = &gardencorev1beta1.ControlPlane{
					HighAvailability: &gardencorev1beta1.HighAvailability{FailureTolerance: gardencorev1beta1.FailureTolerance{Type: gardencorev1beta1.FailureToleranceTypeNode}},
				}
				botanist.Shoot.SetInfo(shoot)

				seed := botanist.Seed.GetInfo()
				metav1.SetMetaDataLabel(&seed.ObjectMeta, "seed.gardener.cloud/multi-zonal", "true")
				botanist.Seed.SetInfo(seed)

				Expect(botanist.DeploySeedNamespace(ctx)).To(Succeed())
				Expect(botanist.SeedNamespaceObject.Labels).To(HaveKeyWithValue("control-plane.shoot.gardener.cloud/enforce-zone", ""))
			})
		})

	})

	Describe("#DeleteSeedNamespace", func() {
		It("should successfully delete the namespace despite 'not found' error", func() {
			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: corev1.SchemeGroupVersion.Group, Resource: "namespaces"}, obj.Name)))

			Expect(botanist.DeleteSeedNamespace(ctx)).To(Succeed())
		})

		It("should successfully delete the namespace (no error)", func() {
			Expect(seedClient.Create(ctx, obj)).To(Succeed())

			Expect(botanist.DeleteSeedNamespace(ctx)).To(Succeed())
		})
	})

	Describe("#AddZoneInformationToSeedNamespace", func() {
		var (
			seedLabels       map[string]string
			shootAnnotations map[string]string
		)

		AfterEach(func() {
			seedLabels = nil
			shootAnnotations = nil
		})

		JustBeforeEach(func() {
			botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: shootAnnotations,
				},
			})
			botanist.Seed.SetInfo(&gardencorev1beta1.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Labels: seedLabels,
				},
			})
		})

		Context("when seed is multi-zonal", func() {
			BeforeEach(func() {
				seedLabels = map[string]string{
					"seed.gardener.cloud/multi-zonal": "",
				}
			})

			Context("when shoot is multi-zonal", func() {
				BeforeEach(func() {
					shootAnnotations = map[string]string{
						"alpha.control-plane.shoot.gardener.cloud/high-availability": "multi-zone",
					}
				})

				It("should not update the namespace as zone pinning is not required", func() {
					Expect(botanist.AddZoneInformationToSeedNamespace(ctx)).To(Succeed())

					Expect(botanist.SeedNamespaceObject).To(BeNil())
				})
			})

			Context("when shoot needs zone pinning (no HA) but no pods exist", func() {
				BeforeEach(func() {
					shootAnnotations = map[string]string{
						"alpha.control-plane.shoot.gardener.cloud/high-availability": "single-zone",
					}
				})

				It("should not update the namespace but return an error", func() {
					Expect(botanist.AddZoneInformationToSeedNamespace(ctx)).To(MatchError("zone information cannot be extracted because no running pods found in control-plane"))
				})
			})

			Context("when shoot needs zone pinning (no HA) but no pods are scheduled", func() {
				BeforeEach(func() {
					shootAnnotations = map[string]string{
						"alpha.control-plane.shoot.gardener.cloud/high-availability": "single-zone",
					}

					for _, pod := range []corev1.Pod{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "pod-0",
								Namespace: namespace,
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "pod-1",
								Namespace: namespace,
							},
						},
					} {
						Expect(seedClient.Create(ctx, &pod)).To(Succeed())
					}
				})

				It("should not update the namespace but return an error", func() {
					Expect(botanist.AddZoneInformationToSeedNamespace(ctx)).To(MatchError("zone information cannot be extracted because no pods have been scheduled yet"))
				})
			})

			Context("when shoot needs zone pinning", func() {
				BeforeEach(func() {
					shootAnnotations = map[string]string{
						"alpha.control-plane.shoot.gardener.cloud/high-availability": "single-zone",
					}

					for _, node := range []corev1.Node{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "node-0",
								Labels: map[string]string{
									"topology.kubernetes.io/zone": "zone-a",
								},
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "node-1",
								Labels: map[string]string{
									"topology.kubernetes.io/zone": "zone-b",
								},
							},
						},
					} {
						Expect(seedClient.Create(ctx, &node)).To(Succeed())
					}

					for _, pod := range []corev1.Pod{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "pod-0",
								Namespace: "default",
							},
							Spec: corev1.PodSpec{
								NodeName: "node-1",
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "pod-0",
								Namespace: namespace,
							},
							Spec: corev1.PodSpec{
								NodeName: "node-0",
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "pod-1",
								Namespace: namespace,
							},
							Spec: corev1.PodSpec{
								NodeName: "node-0",
							},
						},
					} {
						Expect(seedClient.Create(ctx, &pod)).To(Succeed())
					}
				})

				It("should update the namespace with the zone", func() {
					botanist.SeedNamespaceObject = &corev1.Namespace{
						ObjectMeta: metav1.ObjectMeta{
							Name: namespace,
						},
					}
					Expect(seedClient.Create(ctx, botanist.SeedNamespaceObject)).To(Succeed())

					Expect(botanist.AddZoneInformationToSeedNamespace(ctx)).To(Succeed())
					Expect(botanist.SeedNamespaceObject).NotTo(BeNil())
					Expect(botanist.SeedNamespaceObject.Labels).To(HaveKeyWithValue("control-plane.shoot.gardener.cloud/enforce-zone", "zone-a"))
				})

				Context("when single-zone is configured", func() {
					BeforeEach(func() {
						shootAnnotations = map[string]string{
							"alpha.control-plane.shoot.gardener.cloud/high-availability": "single-zone",
						}
					})

					It("should update the namespace with the zone", func() {
						botanist.SeedNamespaceObject = &corev1.Namespace{
							ObjectMeta: metav1.ObjectMeta{
								Name: namespace,
							},
						}
						Expect(seedClient.Create(ctx, botanist.SeedNamespaceObject)).To(Succeed())

						Expect(botanist.AddZoneInformationToSeedNamespace(ctx)).To(Succeed())
						Expect(botanist.SeedNamespaceObject).NotTo(BeNil())
						Expect(botanist.SeedNamespaceObject.Labels).To(HaveKeyWithValue("control-plane.shoot.gardener.cloud/enforce-zone", "zone-a"))
					})
				})
			})
		})

		Context("when seed is not multi-zonal", func() {
			Context("when shoot doesn't have HA configured", func() {
				It("should not update the namespace as zone pinning is not required", func() {
					Expect(botanist.AddZoneInformationToSeedNamespace(ctx)).To(Succeed())

					Expect(botanist.SeedNamespaceObject).To(BeNil())
				})
			})

			Context("when shoot is single-zonal", func() {
				BeforeEach(func() {
					shootAnnotations = map[string]string{
						"alpha.control-plane.shoot.gardener.cloud/high-availability": "single-zone",
					}
				})

				It("should not update the namespace as zone pinning is not required", func() {
					Expect(botanist.AddZoneInformationToSeedNamespace(ctx)).To(Succeed())

					Expect(botanist.SeedNamespaceObject).To(BeNil())
				})
			})
		})

	})
})
