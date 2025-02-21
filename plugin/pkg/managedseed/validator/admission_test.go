// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorehelper "github.com/gardener/gardener/pkg/apis/core/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/seedmanagement"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	corefake "github.com/gardener/gardener/pkg/client/core/clientset/versioned/fake"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	securityinformers "github.com/gardener/gardener/pkg/client/security/informers/externalversions"
	fakeseedmanagement "github.com/gardener/gardener/pkg/client/seedmanagement/clientset/versioned/fake"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/gardener/gardener/plugin/pkg/managedseed/validator"
)

const (
	name          = "foo"
	namespace     = "garden"
	domain        = "foo.example.com"
	provider      = "foo-provider"
	region        = "foo-region"
	zone1         = "foo-region-a"
	zone2         = "foo-region-b"
	dnsProvider   = "dns-provider"
	dnsSecretName = "bar"
)

var _ = Describe("ManagedSeed", func() {
	Describe("#Admit", func() {
		var (
			managedSeed             *seedmanagement.ManagedSeed
			shoot                   *gardencorev1beta1.Shoot
			secret                  *corev1.Secret
			dnsSecret               *corev1.Secret
			seed                    *core.Seed
			credentialsBinding      *securityv1alpha1.CredentialsBinding
			secretBinding           *gardencorev1beta1.SecretBinding
			coreInformerFactory     gardencoreinformers.SharedInformerFactory
			coreClient              *corefake.Clientset
			seedManagementClient    *fakeseedmanagement.Clientset
			kubeInformerFactory     kubeinformers.SharedInformerFactory
			securityInformerFactory securityinformers.SharedInformerFactory
			admissionHandler        *ManagedSeed
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

			shoot = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: gardencorev1beta1.ShootSpec{
					DNS: &gardencorev1beta1.DNS{
						Domain: ptr.To(domain),
					},
					Kubernetes: gardencorev1beta1.Kubernetes{
						EnableStaticTokenKubeconfig: ptr.To(true),
						Version:                     "1.27.5",
						VerticalPodAutoscaler: &gardencorev1beta1.VerticalPodAutoscaler{
							Enabled: true,
						},
					},
					Networking: &gardencorev1beta1.Networking{
						Pods:     ptr.To("100.96.0.0/11"),
						Nodes:    ptr.To("10.250.0.0/16"),
						Services: ptr.To("100.64.0.0/13"),
					},
					Provider: gardencorev1beta1.Provider{
						Type: provider,
						Workers: []gardencorev1beta1.Worker{
							{Zones: []string{zone1, zone2}},
						},
					},
					Region:   region,
					SeedName: ptr.To("parent-seed"),
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

			dnsSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      dnsSecretName,
					Namespace: namespace,
				},
			}
			credentialsBinding = &securityv1alpha1.CredentialsBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cb",
					Namespace: namespace,
				},
				CredentialsRef: corev1.ObjectReference{
					Name:      dnsSecretName,
					Namespace: namespace,
				},
			}
			secretBinding = &gardencorev1beta1.SecretBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "sb",
					Namespace: namespace,
				},
				SecretRef: corev1.SecretReference{
					Name:      dnsSecretName,
					Namespace: namespace,
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
						Nodes:    ptr.To("10.250.0.0/16"),
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
			admissionHandler.SetCoreInformerFactory(coreInformerFactory)

			coreClient = &corefake.Clientset{}
			admissionHandler.SetCoreClientSet(coreClient)

			seedManagementClient = &fakeseedmanagement.Clientset{}
			admissionHandler.SetSeedManagementClientSet(seedManagementClient)

			kubeInformerFactory = kubeinformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetKubeInformerFactory(kubeInformerFactory)

			securityInformerFactory = securityinformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetSecurityInformerFactory(securityInformerFactory)

			Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(shoot)).To(Succeed())
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
			Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Delete(shoot)).To(Succeed())

			err := admissionHandler.Admit(context.TODO(), getManagedSeedAttributes(managedSeed), nil)
			Expect(err).To(BeInvalidError())
			Expect(getErrorList(err)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.shoot.name"),
				})),
			))
		})

		It("should forbid the ManagedSeed if the Shoot does not have any worker", func() {
			shoot.Spec.Provider.Workers = []gardencorev1beta1.Worker{}

			err := admissionHandler.Admit(context.TODO(), getManagedSeedAttributes(managedSeed), nil)
			Expect(err).To(BeInvalidError())
			Expect(getErrorList(err)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.shoot.name"),
					"Detail": ContainSubstring("workerless shoot cannot be used to create managed seed"),
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
			shoot.Spec.Addons = &gardencorev1beta1.Addons{
				NginxIngress: &gardencorev1beta1.NginxIngress{
					Addon: gardencorev1beta1.Addon{
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

			seedManagementClient.AddReactor("list", "managedseeds", func(_ testing.Action) (bool, runtime.Object, error) {
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
				managedSeed.Spec.Gardenlet = seedmanagement.GardenletConfig{
					Config: &gardenletconfigv1alpha1.GardenletConfiguration{
						TypeMeta: metav1.TypeMeta{
							APIVersion: gardenletconfigv1alpha1.SchemeGroupVersion.String(),
							Kind:       "GardenletConfiguration",
						},
						SeedConfig: &gardenletconfigv1alpha1.SeedConfig{
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

			Context("seed label", func() {
				BeforeEach(func() {
					Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(secret)).To(Succeed())
				})

				It("should add the label for the parent seed name", func() {
					Expect(admissionHandler.Admit(context.TODO(), getManagedSeedAttributes(managedSeed), nil)).To(Succeed())

					Expect(managedSeed.Labels).To(And(
						HaveKeyWithValue("name.seed.gardener.cloud/parent-seed", "true"),
					))
				})

				It("should remove unneeded labels", func() {
					metav1.SetMetaDataLabel(&seed.ObjectMeta, "name.seed.gardener.cloud/foo", "true")

					Expect(admissionHandler.Admit(context.TODO(), getManagedSeedAttributes(managedSeed), nil)).To(Succeed())

					Expect(managedSeed.Labels).To(And(
						HaveKeyWithValue("name.seed.gardener.cloud/parent-seed", "true"),
						Not(HaveKey("name.seed.gardener.cloud/foo")),
					))
				})
			})

			It("should allow the ManagedSeed creation if the Shoot exists and can be registered as Seed", func() {
				Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(secret)).To(Succeed())

				err := admissionHandler.Admit(context.TODO(), getManagedSeedAttributes(managedSeed), nil)
				Expect(err).NotTo(HaveOccurred())

				seedx.Spec.Ingress.Controller.Kind = v1beta1constants.IngressKindNginx
				Expect(managedSeed.Spec.Gardenlet).To(Equal(seedmanagement.GardenletConfig{
					Config: &gardenletconfigv1alpha1.GardenletConfiguration{
						TypeMeta: metav1.TypeMeta{
							APIVersion: gardenletconfigv1alpha1.SchemeGroupVersion.String(),
							Kind:       "GardenletConfiguration",
						},
						SeedConfig: &gardenletconfigv1alpha1.SeedConfig{
							SeedTemplate: gardencorev1beta1.SeedTemplate{
								ObjectMeta: seedx.ObjectMeta,
								Spec:       seedx.Spec,
							},
						},
					},
				}))
			})

			It("should create the ManagedSeed and reuse the primary DNS provider from Shoot", func() {
				Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(dnsSecret)).To(Succeed())

				seedx.Spec.DNS.Provider = nil
				shoot.Spec.DNS.Providers = []gardencorev1beta1.DNSProvider{
					{
						Primary:    ptr.To(true),
						Type:       ptr.To("type"),
						SecretName: ptr.To(dnsSecretName),
					},
				}

				managedSeed.Spec.Gardenlet = seedmanagement.GardenletConfig{
					Config: &gardenletconfigv1alpha1.GardenletConfiguration{
						TypeMeta: metav1.TypeMeta{
							APIVersion: gardenletconfigv1alpha1.SchemeGroupVersion.String(),
							Kind:       "GardenletConfiguration",
						},
						SeedConfig: &gardenletconfigv1alpha1.SeedConfig{
							SeedTemplate: gardencorev1beta1.SeedTemplate{
								Spec: seedx.Spec,
							},
						},
					},
				}

				err := admissionHandler.Admit(context.TODO(), getManagedSeedAttributes(managedSeed), nil)
				Expect(err).NotTo(HaveOccurred())

				seedx.Spec.DNS.Provider = &gardencorev1beta1.SeedDNSProvider{
					Type:      "type",
					SecretRef: corev1.SecretReference{Name: "bar", Namespace: "garden"},
				}
				Expect(managedSeed.Spec.Gardenlet).To(Equal(seedmanagement.GardenletConfig{
					Config: &gardenletconfigv1alpha1.GardenletConfiguration{
						TypeMeta: metav1.TypeMeta{
							APIVersion: gardenletconfigv1alpha1.SchemeGroupVersion.String(),
							Kind:       "GardenletConfiguration",
						},
						SeedConfig: &gardenletconfigv1alpha1.SeedConfig{
							SeedTemplate: gardencorev1beta1.SeedTemplate{
								ObjectMeta: seedx.ObjectMeta,
								Spec:       seedx.Spec,
							},
						},
					},
				}))
			})

			It("should create the ManagedSeed and reuse the DNS secret referenced by the SecretBindingName of Shoot", func() {
				Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(dnsSecret)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(secretBinding)).To(Succeed())

				seedx.Spec.DNS.Provider = nil
				shoot.Spec.DNS.Providers = []gardencorev1beta1.DNSProvider{
					{
						Primary: ptr.To(true),
						Type:    ptr.To("type"),
					},
				}
				shoot.Spec.SecretBindingName = ptr.To(secretBinding.Name)
				managedSeed.Spec.Gardenlet = seedmanagement.GardenletConfig{
					Config: &gardenletconfigv1alpha1.GardenletConfiguration{
						TypeMeta: metav1.TypeMeta{
							APIVersion: gardenletconfigv1alpha1.SchemeGroupVersion.String(),
							Kind:       "GardenletConfiguration",
						},
						SeedConfig: &gardenletconfigv1alpha1.SeedConfig{
							SeedTemplate: gardencorev1beta1.SeedTemplate{
								Spec: seedx.Spec,
							},
						},
					},
				}

				err := admissionHandler.Admit(context.TODO(), getManagedSeedAttributes(managedSeed), nil)
				Expect(err).NotTo(HaveOccurred())

				seedx.Spec.DNS.Provider = &gardencorev1beta1.SeedDNSProvider{
					Type:      "type",
					SecretRef: corev1.SecretReference{Name: "bar", Namespace: "garden"},
				}
				Expect(managedSeed.Spec.Gardenlet).To(Equal(seedmanagement.GardenletConfig{
					Config: &gardenletconfigv1alpha1.GardenletConfiguration{
						TypeMeta: metav1.TypeMeta{
							APIVersion: gardenletconfigv1alpha1.SchemeGroupVersion.String(),
							Kind:       "GardenletConfiguration",
						},
						SeedConfig: &gardenletconfigv1alpha1.SeedConfig{
							SeedTemplate: gardencorev1beta1.SeedTemplate{
								ObjectMeta: seedx.ObjectMeta,
								Spec:       seedx.Spec,
							},
						},
					},
				}))
			})

			It("should create the ManagedSeed and reuse the DNS secret referenced by the CredentialsBindingName of Shoot", func() {
				Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(dnsSecret)).To(Succeed())
				Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(credentialsBinding)).To(Succeed())
				seedx.Spec.DNS.Provider = nil
				shoot.Spec.DNS.Providers = []gardencorev1beta1.DNSProvider{
					{
						Primary: ptr.To(true),
						Type:    ptr.To("type"),
					},
				}
				shoot.Spec.CredentialsBindingName = ptr.To(credentialsBinding.Name)

				managedSeed.Spec.Gardenlet = seedmanagement.GardenletConfig{
					Config: &gardenletconfigv1alpha1.GardenletConfiguration{
						TypeMeta: metav1.TypeMeta{
							APIVersion: gardenletconfigv1alpha1.SchemeGroupVersion.String(),
							Kind:       "GardenletConfiguration",
						},
						SeedConfig: &gardenletconfigv1alpha1.SeedConfig{
							SeedTemplate: gardencorev1beta1.SeedTemplate{
								Spec: seedx.Spec,
							},
						},
					},
				}

				err := admissionHandler.Admit(context.TODO(), getManagedSeedAttributes(managedSeed), nil)
				Expect(err).NotTo(HaveOccurred())

				seedx.Spec.DNS.Provider = &gardencorev1beta1.SeedDNSProvider{
					Type:      "type",
					SecretRef: corev1.SecretReference{Name: "bar", Namespace: "garden"},
				}
				Expect(managedSeed.Spec.Gardenlet).To(Equal(seedmanagement.GardenletConfig{
					Config: &gardenletconfigv1alpha1.GardenletConfiguration{
						TypeMeta: metav1.TypeMeta{
							APIVersion: gardenletconfigv1alpha1.SchemeGroupVersion.String(),
							Kind:       "GardenletConfiguration",
						},
						SeedConfig: &gardenletconfigv1alpha1.SeedConfig{
							SeedTemplate: gardencorev1beta1.SeedTemplate{
								ObjectMeta: seedx.ObjectMeta,
								Spec:       seedx.Spec,
							},
						},
					},
				}))
			})

			It("should fail if config could not be converted to GardenletConfiguration", func() {
				managedSeed.Spec.Gardenlet = seedmanagement.GardenletConfig{
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
						Nodes:    ptr.To("10.251.0.0/16"),
						Pods:     "100.97.0.0/11",
						Services: "100.65.0.0/13",
					},
					Provider: gardencorev1beta1.SeedProvider{
						Type:   "bar-provider",
						Region: "bar-region",
						Zones:  []string{"foo", "bar"},
					},
					Settings: &gardencorev1beta1.SeedSettings{
						VerticalPodAutoscaler: &gardencorev1beta1.SeedSettingVerticalPodAutoscaler{
							Enabled: true,
						},
					},
				}

				managedSeed.Spec.Gardenlet.Config = &gardenletconfigv1alpha1.GardenletConfiguration{
					TypeMeta: metav1.TypeMeta{
						APIVersion: gardenletconfigv1alpha1.SchemeGroupVersion.String(),
						Kind:       "GardenletConfiguration",
					},
					SeedConfig: &gardenletconfigv1alpha1.SeedConfig{
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
						"Field":  Equal("spec.gardenlet.config.seedConfig.spec.provider.zones"),
						"Detail": ContainSubstring("[]string{\"foo\", \"bar\"}: cannot use zone in seed provider that is not available in referenced shoot"),
					})),
				))
			})

			Context("when topology-aware routing Seed setting is enabled", func() {
				It("it should forbid when the TopologyAwareHints feature gate is disabled", func() {
					shoot.Spec.Kubernetes.KubeAPIServer = &gardencorev1beta1.KubeAPIServerConfig{
						KubernetesConfig: gardencorev1beta1.KubernetesConfig{
							FeatureGates: map[string]bool{"TopologyAwareHints": false},
						},
					}
					shoot.Spec.Kubernetes.KubeControllerManager = &gardencorev1beta1.KubeControllerManagerConfig{
						KubernetesConfig: gardencorev1beta1.KubernetesConfig{
							FeatureGates: map[string]bool{"TopologyAwareHints": false},
						},
					}
					shoot.Spec.Kubernetes.KubeProxy = &gardencorev1beta1.KubeProxyConfig{
						KubernetesConfig: gardencorev1beta1.KubernetesConfig{
							FeatureGates: map[string]bool{"TopologyAwareHints": false},
						},
					}

					coreClient.AddReactor("get", "shoots", func(_ testing.Action) (bool, runtime.Object, error) {
						return true, shoot, nil
					})

					seedx.Spec.Settings.TopologyAwareRouting = &gardencorev1beta1.SeedSettingTopologyAwareRouting{
						Enabled: true,
					}

					managedSeed.Spec.Gardenlet = seedmanagement.GardenletConfig{
						Config: &gardenletconfigv1alpha1.GardenletConfiguration{
							TypeMeta: metav1.TypeMeta{
								APIVersion: gardenletconfigv1alpha1.SchemeGroupVersion.String(),
								Kind:       "GardenletConfiguration",
							},
							SeedConfig: &gardenletconfigv1alpha1.SeedConfig{
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
							"Detail": ContainSubstring("the topology-aware routing seed setting cannot be enabled when the TopologyAwareHints feature gate is disabled for kube-apiserver"),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeInvalid),
							"Field":  Equal("spec.gardenlet.config.seedConfig.spec.settings.topologyAwareRouting.enabled"),
							"Detail": ContainSubstring("the topology-aware routing seed setting cannot be enabled when the TopologyAwareHints feature gate is disabled for kube-controller-manager"),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeInvalid),
							"Field":  Equal("spec.gardenlet.config.seedConfig.spec.settings.topologyAwareRouting.enabled"),
							"Detail": ContainSubstring("the topology-aware routing seed setting cannot be enabled when the TopologyAwareHints feature gate is disabled for kube-proxy"),
						})),
					))
				})

				It("should allow the ManagedSeed creation when the TopologyAwareHints feature gate is not disabled", func() {
					shoot.Spec.Kubernetes.KubeAPIServer = &gardencorev1beta1.KubeAPIServerConfig{}
					shoot.Spec.Kubernetes.KubeControllerManager = &gardencorev1beta1.KubeControllerManagerConfig{
						KubernetesConfig: gardencorev1beta1.KubernetesConfig{
							FeatureGates: map[string]bool{
								"TopologyAwareHints": true,
							},
						},
					}
					shoot.Spec.Kubernetes.KubeProxy = &gardencorev1beta1.KubeProxyConfig{
						KubernetesConfig: gardencorev1beta1.KubernetesConfig{
							FeatureGates: map[string]bool{},
						},
					}

					coreClient.AddReactor("get", "shoots", func(_ testing.Action) (bool, runtime.Object, error) {
						return true, shoot, nil
					})

					seedx.Spec.Settings.TopologyAwareRouting = &gardencorev1beta1.SeedSettingTopologyAwareRouting{
						Enabled: true,
					}

					managedSeed.Spec.Gardenlet = seedmanagement.GardenletConfig{
						Config: &gardenletconfigv1alpha1.GardenletConfiguration{
							TypeMeta: metav1.TypeMeta{
								APIVersion: gardenletconfigv1alpha1.SchemeGroupVersion.String(),
								Kind:       "GardenletConfiguration",
							},
							SeedConfig: &gardenletconfigv1alpha1.SeedConfig{
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

			Context("ManagedSeed Update", func() {
				var (
					ctx            = context.Background()
					newManagedSeed *seedmanagement.ManagedSeed
				)

				BeforeEach(func() {
					gardenletConfig := &gardenletconfigv1alpha1.GardenletConfiguration{
						TypeMeta: metav1.TypeMeta{
							APIVersion: gardenletconfigv1alpha1.SchemeGroupVersion.String(),
							Kind:       "GardenletConfiguration",
						},
						SeedConfig: &gardenletconfigv1alpha1.SeedConfig{
							SeedTemplate: gardencorev1beta1.SeedTemplate{
								Spec: gardencorev1beta1.SeedSpec{
									Ingress: seedx.Spec.Ingress,
									Provider: gardencorev1beta1.SeedProvider{
										Zones: []string{zone1, zone2},
									},
								},
							},
						},
					}
					managedSeed.Spec.Gardenlet = seedmanagement.GardenletConfig{Config: gardenletConfig}
					newManagedSeed = managedSeed.DeepCopy()

					Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(secret)).To(Succeed())
				})

				It("should allow zone removal when there are no shoots running on seed", func() {
					newGardenletConfig := newManagedSeed.Spec.Gardenlet.Config.(*gardenletconfigv1alpha1.GardenletConfiguration)
					shoot.Spec.Provider.Workers[0].Zones = []string{zone2}
					newGardenletConfig.SeedConfig.Spec.Provider.Zones = shoot.Spec.Provider.Workers[0].Zones

					coreClient.AddReactor("get", "shoots", func(_ testing.Action) (bool, runtime.Object, error) {
						return true, shoot, nil
					})
					Expect(admissionHandler.Admit(ctx, getManagedSeedUpdateAttributes(managedSeed, newManagedSeed), nil)).To(Succeed())
				})

				It("should forbid zone removal when at least one shoot is scheduled to seed", func() {
					newGardenletConfig := newManagedSeed.Spec.Gardenlet.Config.(*gardenletconfigv1alpha1.GardenletConfiguration)
					shoot.Spec.Provider.Workers[0].Zones = []string{zone2}
					newGardenletConfig.SeedConfig.Spec.Provider.Zones = shoot.Spec.Provider.Workers[0].Zones

					coreClient.AddReactor("get", "shoots", func(_ testing.Action) (bool, runtime.Object, error) {
						return true, shoot, nil
					})

					shoot := &gardencorev1beta1.Shoot{Spec: gardencorev1beta1.ShootSpec{SeedName: &newManagedSeed.Name}}
					Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(shoot)).To(Succeed())

					err := admissionHandler.Admit(ctx, getManagedSeedUpdateAttributes(managedSeed, newManagedSeed), nil)
					Expect(err).To(BeInvalidError())
					Expect(getErrorList(err)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeForbidden),
							"Field":  Equal("spec.gardenlet.config.seedConfig.spec.provider.zones"),
							"Detail": ContainSubstring("zones must not be removed while shoots are still scheduled onto seed"),
						})),
					))
				})

				It("should forbid adding a new zone that is not part of shoot workers", func() {
					// add a new zone to shoot workers
					shoot.Spec.Provider.Workers[0].Zones = append(shoot.Spec.Provider.Workers[0].Zones, "zone-bar")

					// add a different zone name to ManagedSeed config
					gardenletConfig := newManagedSeed.Spec.Gardenlet.Config.(*gardenletconfigv1alpha1.GardenletConfiguration)
					gardenletConfig.SeedConfig.Spec.Provider.Zones = append(gardenletConfig.SeedConfig.Spec.Provider.Zones, "zone-foo")

					coreClient.AddReactor("get", "shoots", func(_ testing.Action) (bool, runtime.Object, error) {
						return true, shoot, nil
					})

					shoot := &core.Shoot{Spec: core.ShootSpec{SeedName: &newManagedSeed.Name}}
					Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(shoot)).To(Succeed())

					err := admissionHandler.Admit(ctx, getManagedSeedUpdateAttributes(managedSeed, newManagedSeed), nil)
					Expect(err).To(BeInvalidError())
					Expect(getErrorList(err)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeInvalid),
							"Field":  Equal("spec.gardenlet.config.seedConfig.spec.provider.zones"),
							"Detail": ContainSubstring("added zones must match zone names configured for workers in the referenced shoot cluster"),
						})),
					))
				})

				It("should only forbid the newly added zone because of a naming mismatch", func() {
					// add a third zone so that we don't get a mismatch w.r.t. number of zones
					shoot.Spec.Provider.Workers[0].Zones = append(shoot.Spec.Provider.Workers[0].Zones, "zone-3")

					// create an artificial mismatch in zone names between the seed config and the shoot
					gardenletConfig := managedSeed.Spec.Gardenlet.Config.(*gardenletconfigv1alpha1.GardenletConfiguration)
					gardenletConfig.SeedConfig.Spec.Provider.Zones = []string{"zone-foo", "zone-bar"}

					// zones should still be configured in new ManagedSeed, plus an additional non-existing one
					newGardenletConfig := newManagedSeed.Spec.Gardenlet.Config.(*gardenletconfigv1alpha1.GardenletConfiguration)
					newGardenletConfig.SeedConfig.Spec.Provider.Zones = []string{"zone-foo", "zone-bar", "zone-foobar"}

					coreClient.AddReactor("get", "shoots", func(_ testing.Action) (bool, runtime.Object, error) {
						return true, shoot, nil
					})

					err := admissionHandler.Admit(ctx, getManagedSeedUpdateAttributes(managedSeed, newManagedSeed), nil)
					Expect(err).To(BeInvalidError())
					Expect(getErrorList(err)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeInvalid),
							"Field":  Equal("spec.gardenlet.config.seedConfig.spec.provider.zones"),
							"Detail": ContainSubstring("[]string{\"zone-foobar\"}: added zones must match zone names configured for workers in the referenced shoot cluster"),
						})),
					))
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
			Expect(registered).To(ContainElement("ManagedSeed"))
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
			admissionHandler.SetCoreInformerFactory(gardencoreinformers.NewSharedInformerFactory(nil, 0))
			admissionHandler.SetCoreClientSet(&corefake.Clientset{})
			admissionHandler.SetSeedManagementClientSet(&fakeseedmanagement.Clientset{})
			admissionHandler.SetKubeInformerFactory(kubeinformers.NewSharedInformerFactory(nil, 0))
			admissionHandler.SetSecurityInformerFactory(securityinformers.NewSharedInformerFactory(nil, 0))

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
