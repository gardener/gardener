// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package validator_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/admission"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/testing"
	"k8s.io/utils/pointer"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorehelper "github.com/gardener/gardener/pkg/apis/core/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/seedmanagement"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	corefake "github.com/gardener/gardener/pkg/client/core/clientset/internalversion/fake"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/internalversion"
	fakeseedmanagement "github.com/gardener/gardener/pkg/client/seedmanagement/clientset/versioned/fake"
	gardenletv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/gardener/gardener/plugin/pkg/managedseed/validator"
)

const (
	name        = "foo"
	namespace   = "garden"
	domain      = "foo.example.com"
	provider    = "foo-provider"
	region      = "foo-region"
	zone1       = "foo-region-a"
	zone2       = "foo-region-b"
	dnsProvider = "dns-provider"
)

var _ = Describe("ManagedSeed", func() {
	Describe("#Admit", func() {
		var (
			managedSeed          *seedmanagement.ManagedSeed
			shoot                *core.Shoot
			secret               *corev1.Secret
			seed                 *core.Seed
			coreInformerFactory  gardencoreinformers.SharedInformerFactory
			coreClient           *corefake.Clientset
			seedManagementClient *fakeseedmanagement.Clientset
			kubeInformerFactory  kubeinformers.SharedInformerFactory
			admissionHandler     *ManagedSeed
		)

		BeforeEach(func() {
			managedSeed = &seedmanagement.ManagedSeed{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: seedmanagement.ManagedSeedSpec{
					Shoot: &seedmanagement.Shoot{
						Name: name,
					},
				},
			}

			shoot = &core.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: core.ShootSpec{
					DNS: &core.DNS{
						Domain: pointer.String(domain),
					},
					Kubernetes: core.Kubernetes{
						EnableStaticTokenKubeconfig: pointer.Bool(false),
						Version:                     "1.23.9",
						VerticalPodAutoscaler: &core.VerticalPodAutoscaler{
							Enabled: true,
						},
					},
					Networking: core.Networking{
						Pods:     pointer.String("100.96.0.0/11"),
						Nodes:    pointer.String("10.250.0.0/16"),
						Services: pointer.String("100.64.0.0/13"),
					},
					Provider: core.Provider{
						Type: provider,
						Workers: []core.Worker{
							{Zones: []string{zone1, zone2}},
						},
					},
					Region: region,
				},
			}

			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
					Labels: map[string]string{
						v1beta1constants.GardenRole: v1beta1constants.GardenRoleDefaultDomain,
					},
					Annotations: map[string]string{
						gardenerutils.DNSProvider: dnsProvider,
						gardenerutils.DNSDomain:   domain,
					},
				},
			}

			seed = &core.Seed{
				Spec: core.SeedSpec{
					Backup: &core.SeedBackup{
						Provider: provider,
					},
					DNS: core.SeedDNS{
						Provider: &core.SeedDNSProvider{
							Type: dnsProvider,
							SecretRef: corev1.SecretReference{
								Name:      name,
								Namespace: namespace,
							},
						},
					},
					Networks: core.SeedNetworks{
						Nodes:    pointer.String("10.250.0.0/16"),
						Pods:     "100.96.0.0/11",
						Services: "100.64.0.0/13",
					},
					Provider: core.SeedProvider{
						Type:   provider,
						Region: region,
						Zones:  []string{zone1, zone2},
					},
					Settings: &core.SeedSettings{
						VerticalPodAutoscaler: &core.SeedSettingVerticalPodAutoscaler{
							Enabled: false,
						},
					},
					Ingress: &core.Ingress{
						Domain: "ingress." + domain,
					},
				},
			}

			var err error
			admissionHandler, err = New()
			Expect(err).ToNot(HaveOccurred())
			admissionHandler.AssignReadyFunc(func() bool { return true })

			coreInformerFactory = gardencoreinformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetInternalCoreInformerFactory(coreInformerFactory)

			coreClient = &corefake.Clientset{}
			admissionHandler.SetInternalCoreClientset(coreClient)

			seedManagementClient = &fakeseedmanagement.Clientset{}
			admissionHandler.SetSeedManagementClientset(seedManagementClient)

			kubeInformerFactory = kubeinformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetKubeInformerFactory(kubeInformerFactory)

			Expect(coreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(shoot)).To(Succeed())
		})

		It("should do nothing if the resource is not a ManagedSeed", func() {
			attrs := admission.NewAttributesRecord(nil, nil, core.Kind(name).WithVersion("version"), managedSeed.Namespace, managedSeed.Name, core.Resource("foos").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

			err := admissionHandler.Admit(context.TODO(), attrs, nil)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should forbid the ManagedSeed creation with namespace different from garden", func() {
			managedSeed.Namespace = "foo"

			err := admissionHandler.Admit(context.TODO(), getManagedSeedAttributes(managedSeed), nil)
			Expect(err).To(BeInvalidError())
			Expect(getErrorList(err)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("metadata.namespace"),
				})),
			))
		})

		It("should forbid the ManagedSeed creation if a Shoot name is not specified", func() {
			managedSeed.Spec.Shoot.Name = ""

			err := admissionHandler.Admit(context.TODO(), getManagedSeedAttributes(managedSeed), nil)
			Expect(err).To(BeInvalidError())
			Expect(getErrorList(err)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.shoot.name"),
				})),
			))
		})

		It("should forbid the ManagedSeed creation if the Shoot does not exist", func() {
			Expect(coreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Delete(shoot)).To(Succeed())

			err := admissionHandler.Admit(context.TODO(), getManagedSeedAttributes(managedSeed), nil)
			Expect(err).To(BeInvalidError())
			Expect(getErrorList(err)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.shoot.name"),
				})),
			))
		})

		It("should forbid the ManagedSeed creation if the Shoot does not specify a domain", func() {
			shoot.Spec.DNS = nil

			err := admissionHandler.Admit(context.TODO(), getManagedSeedAttributes(managedSeed), nil)
			Expect(err).To(BeInvalidError())
			Expect(getErrorList(err)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.shoot.name"),
				})),
			))
		})

		It("should forbid the ManagedSeed creation if the Shoot enables the nginx-ingress addon", func() {
			shoot.Spec.Addons = &core.Addons{
				NginxIngress: &core.NginxIngress{
					Addon: core.Addon{
						Enabled: true,
					},
				},
			}

			err := admissionHandler.Admit(context.TODO(), getManagedSeedAttributes(managedSeed), nil)
			Expect(err).To(BeInvalidError())
			Expect(getErrorList(err)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.shoot.name"),
				})),
			))
		})

		It("should forbid the ManagedSeed creation if the Shoot does not enable VPA", func() {
			shoot.Spec.Kubernetes.VerticalPodAutoscaler.Enabled = false

			err := admissionHandler.Admit(context.TODO(), getManagedSeedAttributes(managedSeed), nil)
			Expect(err).To(BeInvalidError())
			Expect(getErrorList(err)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.shoot.name"),
				})),
			))
		})

		It("should forbid the ManagedSeed creation if the Shoot is already registered as Seed", func() {
			anotherManagedSeed := &seedmanagementv1alpha1.ManagedSeed{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "bar",
					Namespace: namespace,
				},
				Spec: seedmanagementv1alpha1.ManagedSeedSpec{
					Shoot: &seedmanagementv1alpha1.Shoot{
						Name: name,
					},
				},
			}

			seedManagementClient.AddReactor("list", "managedseeds", func(action testing.Action) (bool, runtime.Object, error) {
				return true, &seedmanagementv1alpha1.ManagedSeedList{Items: []seedmanagementv1alpha1.ManagedSeed{*anotherManagedSeed}}, nil
			})

			err := admissionHandler.Admit(context.TODO(), getManagedSeedAttributes(managedSeed), nil)
			Expect(err).To(BeInvalidError())
			Expect(getErrorList(err)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.shoot.name"),
				})),
			))
		})

		Context("gardenlet", func() {
			var (
				seedx *gardencorev1beta1.Seed
				err   error
			)

			BeforeEach(func() {
				managedSeed.Spec.Gardenlet = &seedmanagement.Gardenlet{
					Config: &gardenletv1alpha1.GardenletConfiguration{
						TypeMeta: metav1.TypeMeta{
							APIVersion: gardenletv1alpha1.SchemeGroupVersion.String(),
							Kind:       "GardenletConfiguration",
						},
						SeedConfig: &gardenletv1alpha1.SeedConfig{
							SeedTemplate: gardencorev1beta1.SeedTemplate{
								Spec: gardencorev1beta1.SeedSpec{
									Backup: &gardencorev1beta1.SeedBackup{},
								},
							},
						},
					},
				}

				seedx, err = gardencorehelper.ConvertSeedExternal(seed)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should allow the ManagedSeed creation if the Shoot exists and can be registered as Seed", func() {
				Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(secret)).To(Succeed())

				err := admissionHandler.Admit(context.TODO(), getManagedSeedAttributes(managedSeed), nil)
				Expect(err).NotTo(HaveOccurred())

				Expect(managedSeed.Spec.Gardenlet).To(Equal(&seedmanagement.Gardenlet{
					Config: &gardenletv1alpha1.GardenletConfiguration{
						TypeMeta: metav1.TypeMeta{
							APIVersion: gardenletv1alpha1.SchemeGroupVersion.String(),
							Kind:       "GardenletConfiguration",
						},
						SeedConfig: &gardenletv1alpha1.SeedConfig{
							SeedTemplate: gardencorev1beta1.SeedTemplate{
								ObjectMeta: seedx.ObjectMeta,
								Spec:       seedx.Spec,
							},
						},
					},
				}))
			})

			It("should fail if config could not be converted to GardenletConfiguration", func() {
				managedSeed.Spec.Gardenlet = &seedmanagement.Gardenlet{
					Config: &corev1.Pod{
						TypeMeta: metav1.TypeMeta{
							APIVersion: corev1.SchemeGroupVersion.String(),
							Kind:       "Pod",
						},
					},
				}

				err := admissionHandler.Admit(context.TODO(), getManagedSeedAttributes(managedSeed), nil)
				Expect(err).To(BeInternalServerError())
			})

			It("should forbid the ManagedSeed creation if the seed spec contains invalid values", func() {
				seedSpec := gardencorev1beta1.SeedSpec{
					Ingress: &gardencorev1beta1.Ingress{
						Domain: "bar.example.com",
						Controller: gardencorev1beta1.IngressController{
							Kind: "nginx",
						},
					},
					Networks: gardencorev1beta1.SeedNetworks{
						Nodes:    pointer.String("10.251.0.0/16"),
						Pods:     "100.97.0.0/11",
						Services: "100.65.0.0/13",
					},
					Provider: gardencorev1beta1.SeedProvider{
						Type:   "bar-provider",
						Region: "bar-region",
						Zones:  []string{"foo", "bar"},
					},
					SecretRef: &corev1.SecretReference{
						Name:      name,
						Namespace: namespace,
					},
					Settings: &gardencorev1beta1.SeedSettings{
						VerticalPodAutoscaler: &gardencorev1beta1.SeedSettingVerticalPodAutoscaler{
							Enabled: true,
						},
					},
				}

				managedSeed.Spec.Gardenlet.Config = &gardenletv1alpha1.GardenletConfiguration{
					TypeMeta: metav1.TypeMeta{
						APIVersion: gardenletv1alpha1.SchemeGroupVersion.String(),
						Kind:       "GardenletConfiguration",
					},
					SeedConfig: &gardenletv1alpha1.SeedConfig{
						SeedTemplate: gardencorev1beta1.SeedTemplate{
							Spec: seedSpec,
						},
					},
				}

				Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(secret)).NotTo(HaveOccurred())

				err := admissionHandler.Admit(context.TODO(), getManagedSeedAttributes(managedSeed), nil)
				Expect(err).To(BeInvalidError())
				Expect(getErrorList(err)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.gardenlet.config.seedConfig.spec.ingress.domain"),
						"Detail": ContainSubstring("seed ingress domain must be equal to shoot DNS domain"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.gardenlet.config.seedConfig.spec.networks.nodes"),
						"Detail": ContainSubstring("seed nodes CIDR must be equal to shoot nodes CIDR"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.gardenlet.config.seedConfig.spec.networks.pods"),
						"Detail": ContainSubstring("seed pods CIDR must be equal to shoot pods CIDR"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.gardenlet.config.seedConfig.spec.networks.services"),
						"Detail": ContainSubstring("seed services CIDR must be equal to shoot services CIDR"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.gardenlet.config.seedConfig.spec.provider.type"),
						"Detail": ContainSubstring("seed provider type must be equal to shoot provider type"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.gardenlet.config.seedConfig.spec.provider.region"),
						"Detail": ContainSubstring("seed provider region must be equal to shoot region"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.gardenlet.config.seedConfig.spec.settings.verticalPodAutoscaler.enabled"),
						"Detail": ContainSubstring("seed VPA is not supported for managed seeds - use the shoot VPA"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.gardenlet.config.seedConfig.spec.secretRef"),
						"Detail": ContainSubstring("seed secretRef cannot be specified when the shoot static token kubeconfig is disabled"),
					})),
				))
			})

			Context("when topology-aware routing Seed setting is enabled", func() {
				It("it should forbid when the TopologyAwareHints feature gate is not enabled", func() {
					coreClient.AddReactor("get", "shoots", func(action testing.Action) (bool, runtime.Object, error) {
						return true, shoot, nil
					})

					seedx.Spec.Settings.TopologyAwareRouting = &gardencorev1beta1.SeedSettingTopologyAwareRouting{
						Enabled: true,
					}

					managedSeed.Spec.Gardenlet = &seedmanagement.Gardenlet{
						Config: &gardenletv1alpha1.GardenletConfiguration{
							TypeMeta: metav1.TypeMeta{
								APIVersion: gardenletv1alpha1.SchemeGroupVersion.String(),
								Kind:       "GardenletConfiguration",
							},
							SeedConfig: &gardenletv1alpha1.SeedConfig{
								SeedTemplate: gardencorev1beta1.SeedTemplate{
									Spec: seedx.Spec,
								},
							},
						},
					}

					err := admissionHandler.Admit(context.TODO(), getManagedSeedAttributes(managedSeed), nil)
					Expect(err).To(BeInvalidError())
					Expect(getErrorList(err)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeInvalid),
							"Field":  Equal("spec.gardenlet.config.seedConfig.spec.settings.topologyAwareRouting.enabled"),
							"Detail": ContainSubstring("the topology-aware routing Seed setting cannot be enabled for K8s < 1.24 clusters when the TopologyAwareHints feature gate is not enabled for kube-apiserver"),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeInvalid),
							"Field":  Equal("spec.gardenlet.config.seedConfig.spec.settings.topologyAwareRouting.enabled"),
							"Detail": ContainSubstring("the topology-aware routing Seed setting cannot be enabled for K8s < 1.24 clusters when the TopologyAwareHints feature gate is not enabled for kube-controller-manager"),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeInvalid),
							"Field":  Equal("spec.gardenlet.config.seedConfig.spec.settings.topologyAwareRouting.enabled"),
							"Detail": ContainSubstring("the topology-aware routing Seed setting cannot be enabled for K8s < 1.24 clusters when the TopologyAwareHints feature gate is not enabled for kube-proxy"),
						})),
					))
				})

				It("should allow the ManagedSeed creation when the TopologyAwareHints feature gate is enabled and the K8s version is 1.23", func() {
					shoot.Spec.Kubernetes.KubeAPIServer = &core.KubeAPIServerConfig{
						KubernetesConfig: core.KubernetesConfig{
							FeatureGates: map[string]bool{
								"TopologyAwareHints": true,
							},
						},
					}
					shoot.Spec.Kubernetes.KubeControllerManager = &core.KubeControllerManagerConfig{
						KubernetesConfig: core.KubernetesConfig{
							FeatureGates: map[string]bool{
								"TopologyAwareHints": true,
							},
						},
					}
					shoot.Spec.Kubernetes.KubeProxy = &core.KubeProxyConfig{
						KubernetesConfig: core.KubernetesConfig{
							FeatureGates: map[string]bool{
								"TopologyAwareHints": true,
							},
						},
					}

					coreClient.AddReactor("get", "shoots", func(action testing.Action) (bool, runtime.Object, error) {
						return true, shoot, nil
					})

					seedx.Spec.Settings.TopologyAwareRouting = &gardencorev1beta1.SeedSettingTopologyAwareRouting{
						Enabled: true,
					}

					managedSeed.Spec.Gardenlet = &seedmanagement.Gardenlet{
						Config: &gardenletv1alpha1.GardenletConfiguration{
							TypeMeta: metav1.TypeMeta{
								APIVersion: gardenletv1alpha1.SchemeGroupVersion.String(),
								Kind:       "GardenletConfiguration",
							},
							SeedConfig: &gardenletv1alpha1.SeedConfig{
								SeedTemplate: gardencorev1beta1.SeedTemplate{
									Spec: seedx.Spec,
								},
							},
						},
					}

					err := admissionHandler.Admit(context.TODO(), getManagedSeedAttributes(managedSeed), nil)
					Expect(err).NotTo(HaveOccurred())
				})

				It("should allow the ManagedSeed creation when the K8s version is > 1.23", func() {
					shoot.Spec.Kubernetes.Version = "1.24.10"

					coreClient.AddReactor("get", "shoots", func(action testing.Action) (bool, runtime.Object, error) {
						return true, shoot, nil
					})

					seedx.Spec.Settings.TopologyAwareRouting = &gardencorev1beta1.SeedSettingTopologyAwareRouting{
						Enabled: true,
					}

					managedSeed.Spec.Gardenlet = &seedmanagement.Gardenlet{
						Config: &gardenletv1alpha1.GardenletConfiguration{
							TypeMeta: metav1.TypeMeta{
								APIVersion: gardenletv1alpha1.SchemeGroupVersion.String(),
								Kind:       "GardenletConfiguration",
							},
							SeedConfig: &gardenletv1alpha1.SeedConfig{
								SeedTemplate: gardencorev1beta1.SeedTemplate{
									Spec: seedx.Spec,
								},
							},
						},
					}

					err := admissionHandler.Admit(context.TODO(), getManagedSeedAttributes(managedSeed), nil)
					Expect(err).NotTo(HaveOccurred())
				})
			})
		})
	})

	Describe("#Register", func() {
		It("should register the plugin", func() {
			plugins := admission.NewPlugins()
			Register(plugins)

			registered := plugins.Registered()
			Expect(registered).To(HaveLen(1))
			Expect(registered).To(ContainElement(PluginName))
		})
	})

	Describe("#New", func() {
		It("should only handle CREATE and UPDATE operations", func() {
			admissionHandler, err := New()
			Expect(err).ToNot(HaveOccurred())
			Expect(admissionHandler.Handles(admission.Create)).To(BeTrue())
			Expect(admissionHandler.Handles(admission.Update)).To(BeTrue())
			Expect(admissionHandler.Handles(admission.Connect)).NotTo(BeTrue())
			Expect(admissionHandler.Handles(admission.Delete)).NotTo(BeTrue())
		})
	})

	Describe("#ValidateInitialization", func() {
		It("should fail if the required clients are not set", func() {
			admissionHandler, _ := New()

			err := admissionHandler.ValidateInitialization()
			Expect(err).To(HaveOccurred())
		})

		It("should not fail if the required clients are set", func() {
			admissionHandler, _ := New()
			admissionHandler.SetInternalCoreInformerFactory(gardencoreinformers.NewSharedInformerFactory(nil, 0))
			admissionHandler.SetInternalCoreClientset(&corefake.Clientset{})
			admissionHandler.SetSeedManagementClientset(&fakeseedmanagement.Clientset{})
			admissionHandler.SetKubeInformerFactory(kubeinformers.NewSharedInformerFactory(nil, 0))

			err := admissionHandler.ValidateInitialization()
			Expect(err).ToNot(HaveOccurred())
		})
	})
})

func getManagedSeedAttributes(managedSeed *seedmanagement.ManagedSeed) admission.Attributes {
	return admission.NewAttributesRecord(managedSeed, nil, seedmanagementv1alpha1.Kind("ManagedSeed").WithVersion("v1alpha1"), managedSeed.Namespace, managedSeed.Name, seedmanagementv1alpha1.Resource("managedseeds").WithVersion("v1alpha1"), "", admission.Create, &metav1.CreateOptions{}, false, nil)
}

func getManagedSeedUpdateAttributes(oldManagedSeed, newManagedSeed *seedmanagement.ManagedSeed) admission.Attributes {
	return admission.NewAttributesRecord(newManagedSeed,
		oldManagedSeed,
		seedmanagementv1alpha1.Kind("ManagedSeed").WithVersion("v1alpha1"),
		newManagedSeed.Namespace,
		newManagedSeed.Name,
		seedmanagementv1alpha1.Resource("managedseeds").WithVersion("v1alpha1"),
		"",
		admission.Update,
		&metav1.UpdateOptions{},
		false,
		nil)
}

func getErrorList(err error) field.ErrorList {
	statusError, ok := err.(*apierrors.StatusError)
	if !ok {
		return field.ErrorList{}
	}
	var errs field.ErrorList
	for _, cause := range statusError.ErrStatus.Details.Causes {
		errs = append(errs, &field.Error{
			Type:   field.ErrorType(cause.Type),
			Field:  cause.Field,
			Detail: cause.Message,
		})
	}
	return errs
}
