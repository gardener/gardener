// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package dns_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/admission"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/gardener/gardener/plugin/pkg/shoot/dns"
)

const (
	namespace   = "my-namespace"
	projectName = "my-project"
	seedName    = "my-seed"
	shootName   = "shoot"

	domain                = "example.com"
	domainHigherPriority  = "higher.example.com"
	domainLowerPriority   = "lower.example.com"
	defaultDomainProvider = "my-dns-provider"

	providerType = "provider"
	secretName   = "secret"
)

var _ = Describe("dns", func() {

	Describe("#Register", func() {
		It("should register the plugin", func() {
			plugins := admission.NewPlugins()
			Register(plugins)

			registered := plugins.Registered()
			Expect(registered).To(HaveLen(1))
			Expect(registered).To(ContainElement("ShootDNS"))
		})
	})

	Describe("#New", func() {
		It("should handle CREATE and UPDATE operations", func() {
			admissionHandler, err := New()
			Expect(err).NotTo(HaveOccurred())
			Expect(admissionHandler.Handles(admission.Create)).To(BeTrue())
			Expect(admissionHandler.Handles(admission.Update)).To(BeTrue())
			Expect(admissionHandler.Handles(admission.Connect)).To(BeFalse())
			Expect(admissionHandler.Handles(admission.Delete)).To(BeFalse())
		})
	})

	Describe("#ValidateInitialization", func() {
		It("should return error if a lister is missing", func() {
			admissionHandler, err := New()
			Expect(err).NotTo(HaveOccurred())

			err = admissionHandler.ValidateInitialization()
			Expect(err).To(MatchError("missing secret lister"))
		})

		It("should not return error if all listers are set", func() {
			admissionHandler, err := New()
			Expect(err).NotTo(HaveOccurred())
			kubeInformerFactory := kubeinformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetKubeInformerFactory(kubeInformerFactory)
			coreInformerFactory := gardencoreinformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetCoreInformerFactory(coreInformerFactory)

			Expect(admissionHandler.ValidateInitialization()).To(Succeed())
		})
	})

	Describe("#Admit", func() {
		var (
			ctx                               context.Context
			defaultDomainSecret               *corev1.Secret
			defaultDomainSecretHigherPriority *corev1.Secret
			defaultDomainSecretLowerPriority  *corev1.Secret
			project                           *gardencorev1beta1.Project
			seed                              *gardencorev1beta1.Seed
			shoot                             *core.Shoot
			admissionHandler                  *DNS
			kubeInformerFactory               kubeinformers.SharedInformerFactory
			coreInformerFactory               gardencoreinformers.SharedInformerFactory

			provider = core.DNSUnmanaged
		)

		BeforeEach(func() {
			ctx = context.Background()
			defaultDomainSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret-1",
					Namespace: v1beta1constants.GardenNamespace,
					Labels: map[string]string{
						v1beta1constants.GardenRole: v1beta1constants.GardenRoleDefaultDomain,
					},
					Annotations: map[string]string{
						gardenerutils.DNSDomain:   domain,
						gardenerutils.DNSProvider: defaultDomainProvider,
					},
				},
			}
			defaultDomainSecretHigherPriority = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret-2",
					Namespace: v1beta1constants.GardenNamespace,
					Labels: map[string]string{
						v1beta1constants.GardenRole: v1beta1constants.GardenRoleDefaultDomain,
					},
					Annotations: map[string]string{
						gardenerutils.DNSDomain:                domainHigherPriority,
						gardenerutils.DNSDefaultDomainPriority: "5",
						gardenerutils.DNSProvider:              defaultDomainProvider,
					},
				},
			}
			defaultDomainSecretLowerPriority = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret-2",
					Namespace: v1beta1constants.GardenNamespace,
					Labels: map[string]string{
						v1beta1constants.GardenRole: v1beta1constants.GardenRoleDefaultDomain,
					},
					Annotations: map[string]string{
						gardenerutils.DNSDomain:                domainLowerPriority,
						gardenerutils.DNSDefaultDomainPriority: "-5",
						gardenerutils.DNSProvider:              defaultDomainProvider,
					},
				},
			}
			project = &gardencorev1beta1.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: projectName,
				},
				Spec: gardencorev1beta1.ProjectSpec{
					Namespace: ptr.To(namespace),
				},
			}
			seed = &gardencorev1beta1.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Name: seedName,
				},
			}
			shoot = &core.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      shootName,
					Namespace: namespace,
				},
				Spec: core.ShootSpec{
					DNS: &core.DNS{
						Providers: []core.DNSProvider{
							{
								Type: &provider,
							},
						},
					},
					SeedName: ptr.To(seedName),
				},
			}

			admissionHandler, _ = New()
			admissionHandler.AssignReadyFunc(func() bool { return true })
			kubeInformerFactory = kubeinformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetKubeInformerFactory(kubeInformerFactory)
			coreInformerFactory = gardencoreinformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetCoreInformerFactory(coreInformerFactory)
		})

		It("should do nothing if the resource is not a Shoot", func() {
			attrs := admission.NewAttributesRecord(nil, nil, core.Kind("foo").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("foos").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

			Expect(admissionHandler.Admit(ctx, attrs, nil)).To(Succeed())
		})

		It("should do nothing because the shoot status is updated", func() {
			shootCopy := shoot.DeepCopy()
			shootCopy.Spec.SeedName = nil
			shootBefore := shootCopy.DeepCopy()

			attrs := admission.NewAttributesRecord(shootCopy, nil, core.Kind("Shoot").WithVersion("version"), shootCopy.Namespace, shootCopy.Name, core.Resource("shoots").WithVersion("version"), "status", admission.Update, &metav1.UpdateOptions{}, false, nil)

			Expect(admissionHandler.Admit(ctx, attrs, nil)).To(Succeed())
			Expect(*shootCopy).To(Equal(*shootBefore))
		})

		It("should do nothing because the shoot does not specify a seed (create)", func() {
			shootCopy := shoot.DeepCopy()
			shootCopy.Spec.SeedName = nil
			shootBefore := shootCopy.DeepCopy()

			attrs := admission.NewAttributesRecord(shootCopy, nil, core.Kind("Shoot").WithVersion("version"), shootCopy.Namespace, shootCopy.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

			Expect(admissionHandler.Admit(ctx, attrs, nil)).To(Succeed())
			Expect(*shootCopy).To(Equal(*shootBefore))
		})

		It("should do nothing because the shoot does not specify a seed (update)", func() {
			shootCopy := shoot.DeepCopy()
			shootCopy.Spec.SeedName = nil
			shootBefore := shootCopy.DeepCopy()

			attrs := admission.NewAttributesRecord(shootCopy, shootCopy, core.Kind("Shoot").WithVersion("version"), shootCopy.Namespace, shootCopy.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

			Expect(admissionHandler.Admit(ctx, attrs, nil)).To(Succeed())
			Expect(*shootCopy).To(Equal(*shootBefore))
		})

		It("should set the 'unmanaged' dns provider as the primary one", func() {
			shootBefore := shoot.DeepCopy()
			shootBefore.Spec.DNS.Providers[0].Primary = ptr.To(true)

			Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(seed)).To(Succeed())
			attrs := admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

			Expect(admissionHandler.Admit(ctx, attrs, nil)).To(Succeed())
			Expect(shoot).To(Equal(shootBefore))
		})

		Context("provider is not 'unmanaged'", func() {
			BeforeEach(func() {
				shoot.Spec.DNS.Domain = nil
				shoot.Spec.DNS.Providers = nil
			})

			It("should pass because no default domain was generated for the shoot (with domain)", func() {
				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(seed)).To(Succeed())

				var (
					shootDomain = "my-shoot.my-private-domain.com"
				)
				shoot.Spec.DNS.Domain = &shootDomain
				shoot.Spec.DNS.Providers = []core.DNSProvider{
					{
						Type: ptr.To(providerType),
					},
				}

				attrs := admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				Expect(admissionHandler.Admit(ctx, attrs, nil)).To(Succeed())
				Expect(*shoot.Spec.DNS.Domain).To(Equal(shootDomain))
				Expect(shoot.Spec.DNS.Providers).To(ConsistOf(MatchFields(IgnoreExtras, Fields{
					"Type":    Equal(ptr.To(providerType)),
					"Primary": Equal(ptr.To(true)),
				})))
			})

			It("should set the correct primary DNS provider", func() {
				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(seed)).To(Succeed())

				var (
					shootDomain = "my-shoot.my-private-domain.com"
				)
				shoot.Spec.DNS.Domain = &shootDomain
				shoot.Spec.DNS.Providers = []core.DNSProvider{
					{
						Type: ptr.To(providerType),
					},
					{
						Type: ptr.To(providerType),
						CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
							APIVersion: "v1",
							Kind:       "Secret",
							Name:       secretName,
						},
					},
				}

				attrs := admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				Expect(admissionHandler.Admit(ctx, attrs, nil)).To(Succeed())
				Expect(*shoot.Spec.DNS.Domain).To(Equal(shootDomain))
				Expect(shoot.Spec.DNS.Providers).To(ConsistOf(
					MatchFields(IgnoreExtras, Fields{
						"Type":    Equal(ptr.To(providerType)),
						"Primary": Equal(ptr.To(true)),
					}),
					MatchFields(IgnoreExtras, Fields{
						"Type":           Equal(ptr.To(providerType)),
						"Primary":        BeNil(),
						"CredentialsRef": Equal(&autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "Secret", Name: secretName}),
					}),
				))
			})

			It("should re-assign the correct primary DNS provider on updates", func() {
				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(seed)).To(Succeed())

				var (
					shootDomain = "my-shoot.my-private-domain.com"
					secretName2 = "secret2"
				)
				shoot.Spec.DNS.Domain = &shootDomain
				shoot.Spec.DNS.Providers = []core.DNSProvider{
					{
						Type: ptr.To(providerType),
						CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
							APIVersion: "v1",
							Kind:       "Secret",
							Name:       secretName2,
						},
					},
					{
						Type: ptr.To(providerType),
						CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
							APIVersion: "v1",
							Kind:       "Secret",
							Name:       secretName,
						},
					},
				}

				oldShoot := shoot.DeepCopy()
				oldShoot.Spec.DNS.Providers[1].Primary = ptr.To(true)

				attrs := admission.NewAttributesRecord(shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				Expect(admissionHandler.Admit(ctx, attrs, nil)).To(Succeed())
				Expect(*shoot.Spec.DNS.Domain).To(Equal(shootDomain))
				Expect(shoot.Spec.DNS.Providers).To(ConsistOf(
					MatchFields(IgnoreExtras, Fields{
						"Type": Equal(ptr.To(providerType)),
					}),
					MatchFields(IgnoreExtras, Fields{
						"Type":           Equal(ptr.To(providerType)),
						"Primary":        Equal(ptr.To(true)),
						"CredentialsRef": Equal(&autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "Secret", Name: secretName}),
					}),
				))
			})

			It("should generate a default domain for a shoot with no domain when the seed has explicit default dns configuration", func() {
				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(seed)).To(Succeed())

				overwriteSecret := corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret-overwrite",
						Namespace: v1beta1constants.GardenNamespace,
					},
				}
				Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&overwriteSecret)).To(Succeed())
				seed.Spec.DNS = gardencorev1beta1.SeedDNS{
					Defaults: []gardencorev1beta1.SeedDNSProviderConfig{
						{
							Type:   "foo",
							Domain: "overwrite.example.com",
							CredentialsRef: corev1.ObjectReference{
								APIVersion: "v1",
								Kind:       "Secret",
								Name:       overwriteSecret.Name,
								Namespace:  overwriteSecret.Namespace,
							},
						},
					},
				}

				attrs := admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				Expect(admissionHandler.Admit(ctx, attrs, nil)).To(Succeed())
				Expect(shoot.Spec.DNS.Providers).To(BeNil())
				Expect(*shoot.Spec.DNS.Domain).To(Equal(fmt.Sprintf("%s.%s.%s", shootName, projectName, "overwrite.example.com")))
			})

			It("should generate a default domain for a shoot with no domain", func() {
				Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(defaultDomainSecret)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(seed)).To(Succeed())

				attrs := admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				Expect(admissionHandler.Admit(ctx, attrs, nil)).To(Succeed())
				Expect(shoot.Spec.DNS.Providers).To(BeNil())
				Expect(*shoot.Spec.DNS.Domain).To(Equal(fmt.Sprintf("%s.%s.%s", shootName, projectName, domain)))
			})

			It("should generate a domain from the default domain with the highest priority with no domain", func() {
				Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(defaultDomainSecret)).To(Succeed())
				Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(defaultDomainSecretLowerPriority)).To(Succeed())
				Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(defaultDomainSecretHigherPriority)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(seed)).To(Succeed())

				attrs := admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				Expect(admissionHandler.Admit(ctx, attrs, nil)).To(Succeed())
				Expect(shoot.Spec.DNS.Providers).To(BeNil())
				Expect(*shoot.Spec.DNS.Domain).To(Equal(fmt.Sprintf("%s.%s.%s", shootName, projectName, domainHigherPriority)))
			})

			It("should generate a domain from the default domain without priority because default priority is 0 with no domain", func() {
				Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(defaultDomainSecret)).To(Succeed())
				Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(defaultDomainSecretLowerPriority)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(seed)).To(Succeed())

				attrs := admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				Expect(admissionHandler.Admit(ctx, attrs, nil)).To(Succeed())
				Expect(shoot.Spec.DNS.Providers).To(BeNil())
				Expect(*shoot.Spec.DNS.Domain).To(Equal(fmt.Sprintf("%s.%s.%s", shootName, projectName, domain)))
			})

			It("should not set a primary provider because a default domain was generated for the shoot with no domain", func() {
				Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(defaultDomainSecret)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(seed)).To(Succeed())

				shoot.Spec.DNS.Providers = []core.DNSProvider{
					{
						Type: ptr.To(providerType),
						CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
							APIVersion: "v1",
							Kind:       "Secret",
							Name:       secretName,
						},
					},
				}

				attrs := admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				Expect(admissionHandler.Admit(ctx, attrs, nil)).To(Succeed())
				Expect(*shoot.Spec.DNS.Domain).To(Equal(fmt.Sprintf("%s.%s.%s", shootName, projectName, domain)))
				Expect(shoot.Spec.DNS.Providers).To(ConsistOf(MatchFields(IgnoreExtras, Fields{
					"Type":           Equal(ptr.To(providerType)),
					"Primary":        BeNil(),
					"CredentialsRef": Equal(&autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "Secret", Name: secretName}),
				})))
			})

			It("should do nothing if the shoot domain is already set to the default domain", func() {
				Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(defaultDomainSecret)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(seed)).To(Succeed())

				shootDomain := fmt.Sprintf("%s.%s.%s", shoot.Name, project.Name, domain)
				shoot.Spec.DNS.Domain = &shootDomain

				attrs := admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				Expect(admissionHandler.Admit(ctx, attrs, nil)).To(Succeed())
				Expect(shoot.Spec.DNS.Providers).To(BeNil())
				Expect(*shoot.Spec.DNS.Domain).To(Equal(shootDomain))
			})

			It("should reject because no domain was configured for the shoot and project is missing", func() {
				Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(defaultDomainSecret)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(seed)).To(Succeed())

				attrs := admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(ctx, attrs, nil)
				Expect(err).To(MatchError(error(apierrors.NewInternalError(fmt.Errorf("Project.core.gardener.cloud %q not found", "<unknown>")))))
			})

			Context("#Shoot GenerateName used", func() {
				BeforeEach(func() {
					shoot.Name = ""
					shoot.GenerateName = "demo-"
				})

				It("should set different default domain for multiple shoots with same generate name", func() {
					Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(defaultDomainSecret)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(project)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(seed)).To(Succeed())

					shootCopy := shoot.DeepCopy()

					attrs := admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

					Expect(admissionHandler.Admit(ctx, attrs, nil)).To(Succeed())

					Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(defaultDomainSecret)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(project)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(seed)).To(Succeed())
					attrs = admission.NewAttributesRecord(shootCopy, nil, core.Kind("Shoot").WithVersion("version"), shootCopy.Namespace, shootCopy.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

					Expect(admissionHandler.Admit(ctx, attrs, nil)).To(Succeed())
					Expect(*shoot.Spec.DNS.Domain).NotTo(Equal(*shootCopy.Spec.DNS.Domain))
				})

				It("should generate a default domain with shoot name for the shoot with no domain", func() {
					Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(defaultDomainSecret)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(project)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(seed)).To(Succeed())

					shoot.Name = "foo"

					attrs := admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

					Expect(admissionHandler.Admit(ctx, attrs, nil)).To(Succeed())
					Expect(shoot.Spec.DNS.Providers).To(BeNil())
					Expect(*shoot.Spec.DNS.Domain).To(Equal(fmt.Sprintf("%s.%s.%s", shoot.Name, projectName, domain)))
				})

				It("should generate a default domain for the shoot with no domain", func() {
					Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(defaultDomainSecret)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(project)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(seed)).To(Succeed())

					attrs := admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

					Expect(admissionHandler.Admit(ctx, attrs, nil)).To(Succeed())
					Expect(shoot.Spec.DNS.Providers).To(BeNil())
					Expect(*shoot.Spec.DNS.Domain).To(HaveSuffix(fmt.Sprintf(".%s.%s", projectName, domain)))
				})

				It("should re-assign the default domain for the shoot when it is unset", func() {
					Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(defaultDomainSecret)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(project)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(seed)).To(Succeed())

					oldShoot := shoot.DeepCopy()
					shoot.Spec.DNS = nil

					attrs := admission.NewAttributesRecord(shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

					Expect(admissionHandler.Admit(ctx, attrs, nil)).To(Succeed())
					Expect(shoot.Spec.DNS.Providers).To(BeNil())
					Expect(*shoot.Spec.DNS.Domain).To(HaveSuffix(fmt.Sprintf(".%s.%s", projectName, domain)))
				})
			})
		})

		Context("Shoot Control Plane Migration", func() {
			var (
				destinationSeedName = "my-seed-2"
				destinationSeed     gardencorev1beta1.Seed
			)

			BeforeEach(func() {
				destinationSeed = gardencorev1beta1.Seed{
					ObjectMeta: metav1.ObjectMeta{
						Name: destinationSeedName,
					},
				}

				shoot.Spec.DNS.Providers = nil
			})

			It("should accept shoot migration update", func() {
				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(seed)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&destinationSeed)).To(Succeed())

				shootDomain := fmt.Sprintf("%s.%s.%s", shoot.Name, project.Name, domain)
				shoot.Spec.DNS.Domain = &shootDomain

				shoot.Spec.SeedName = &destinationSeedName
				attrs := admission.NewAttributesRecord(shoot, shoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				Expect(admissionHandler.Admit(ctx, attrs, nil)).To(Succeed())
			})
		})
	})

	Describe("#Validate", func() {
		var (
			ctx                 context.Context
			defaultDomainSecret *corev1.Secret
			project             *gardencorev1beta1.Project
			seed                *gardencorev1beta1.Seed
			shoot               *core.Shoot
			kubeInformerFactory kubeinformers.SharedInformerFactory
			coreInformerFactory gardencoreinformers.SharedInformerFactory
			admissionHandler    *DNS
		)

		BeforeEach(func() {
			ctx = context.Background()
			defaultDomainSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret-1",
					Namespace: v1beta1constants.GardenNamespace,
					Labels: map[string]string{
						v1beta1constants.GardenRole: v1beta1constants.GardenRoleDefaultDomain,
					},
					Annotations: map[string]string{
						gardenerutils.DNSDomain:   domain,
						gardenerutils.DNSProvider: defaultDomainProvider,
					},
				},
			}
			project = &gardencorev1beta1.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: projectName,
				},
				Spec: gardencorev1beta1.ProjectSpec{
					Namespace: ptr.To(namespace),
				},
			}
			seed = &gardencorev1beta1.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Name: seedName,
				},
			}
			shoot = &core.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      shootName,
					Namespace: namespace,
				},
				Spec: core.ShootSpec{
					DNS:      &core.DNS{},
					SeedName: ptr.To(seedName),
				},
			}

			var err error
			admissionHandler, err = New()
			Expect(err).ToNot(HaveOccurred())
			admissionHandler.AssignReadyFunc(func() bool { return true })
			kubeInformerFactory = kubeinformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetKubeInformerFactory(kubeInformerFactory)
			coreInformerFactory = gardencoreinformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetCoreInformerFactory(coreInformerFactory)
		})

		It("should do nothing if the resource is not a Shoot", func() {
			attrs := admission.NewAttributesRecord(nil, nil, core.Kind("foo").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("foos").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

			Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
		})

		It("should do nothing because the shoot status is updated", func() {
			attrs := admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "status", admission.Update, &metav1.UpdateOptions{}, false, nil)

			Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
		})

		It("should forbid when object is not Shoot", func() {
			project := core.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-project",
				},
			}
			attrs := admission.NewAttributesRecord(&project, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

			err := admissionHandler.Validate(ctx, attrs, nil)
			Expect(err).To(BeBadRequestError())
			Expect(err).To(MatchError("could not convert resource into Shoot object"))
		})

		Context("provider is not 'unmanaged'", func() {
			It("should forbid setting a primary provider because a default domain was manually configured for the shoot", func() {
				Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(defaultDomainSecret)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(seed)).To(Succeed())

				shootDomain := fmt.Sprintf("%s.%s.%s", shoot.Name, project.Name, domain)
				shoot.Spec.DNS.Domain = &shootDomain
				shoot.Spec.DNS.Providers = []core.DNSProvider{
					{
						Type: ptr.To(providerType),
						CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
							APIVersion: "v1",
							Kind:       "Secret",
							Name:       secretName,
						},
						Primary: ptr.To(true),
					},
				}

				attrs := admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Validate(ctx, attrs, nil)
				Expect(err).To(BeInvalidError())
				Expect(getErrorList(err)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.dns.providers[0].primary"),
						"Detail": ContainSubstring("primary dns provider must not be set when default domain is used"),
					})),
				))
			})

			It("should allow because the default domain was allowed for the shoot", func() {
				Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(defaultDomainSecret)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(seed)).To(Succeed())

				shootDomain := fmt.Sprintf("%s.%s.%s", shoot.Name, project.Name, domain)
				shoot.Spec.DNS.Domain = &shootDomain

				attrs := admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
			})

			It("should reject because a default domain was already used for the shoot but is invalid", func() {
				Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(defaultDomainSecret)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(seed)).To(Succeed())

				shootDomain := fmt.Sprintf("%s.other-project.%s", shoot.Name, domain)
				shoot.Spec.DNS.Domain = &shootDomain

				attrs := admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Validate(ctx, attrs, nil)
				Expect(err).To(BeInvalidError())
				Expect(getErrorList(err)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.dns.domain"),
						"Detail": ContainSubstring("shoot uses a default domain but does not match expected scheme: <shoot-name>.<project-name>.<default-domain> (expected 'shoot.my-project.example.com', but got 'shoot.other-project.example.com')"),
					})),
				))
			})

			It("should reject because a default domain was already used for the shoot but is invalid when seed is assigned", func() {
				Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(defaultDomainSecret)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(seed)).To(Succeed())

				shootDomain := fmt.Sprintf("%s.other-project.%s", shoot.Name, domain)
				shoot.Spec.DNS.Domain = &shootDomain
				oldShoot := shoot.DeepCopy()
				oldShoot.Spec.SeedName = nil

				attrs := admission.NewAttributesRecord(shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Validate(ctx, attrs, nil)
				Expect(err).To(BeInvalidError())
				Expect(getErrorList(err)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.dns.domain"),
						"Detail": ContainSubstring("shoot uses a default domain but does not match expected scheme: <shoot-name>.<project-name>.<default-domain> (expected 'shoot.my-project.example.com', but got 'shoot.other-project.example.com')"),
					})),
				))
			})

			It("should reject shoots setting a non compliant default domain on updates if domain was previously not set", func() {
				Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(defaultDomainSecret)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(seed)).To(Succeed())

				oldShoot := shoot.DeepCopy()

				shootDomain := fmt.Sprintf("%s.other-project.%s", shoot.Name, domain)
				shoot.Spec.DNS.Domain = &shootDomain

				attrs := admission.NewAttributesRecord(shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Validate(ctx, attrs, nil)
				Expect(err).To(BeInvalidError())
				Expect(getErrorList(err)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.dns.domain"),
						"Detail": ContainSubstring("shoot uses a default domain but does not match expected scheme: <shoot-name>.<project-name>.<default-domain> (expected 'shoot.my-project.example.com', but got 'shoot.other-project.example.com')"),
					})),
				))
			})

			It("should not reject shoots using a non compliant default domain on update if domain was previously set", func() {
				Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(defaultDomainSecret)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(seed)).To(Succeed())

				shootDomain := fmt.Sprintf("%s.other-project.%s", shoot.Name, domain)
				shoot.Spec.DNS.Domain = &shootDomain

				attrs := admission.NewAttributesRecord(shoot, shoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
			})

			Context("#Shoot GenerateName used", func() {
				BeforeEach(func() {
					shoot.Name = ""
					shoot.GenerateName = "demo-"
				})

				It("should reject because a default domain was already used for the shoot but is invalid", func() {
					Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(defaultDomainSecret)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(project)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(seed)).To(Succeed())

					shootDomain := fmt.Sprintf("%s.other-project.%s", shoot.Name, domain)
					shoot.Spec.DNS.Domain = &shootDomain

					attrs := admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

					err := admissionHandler.Validate(ctx, attrs, nil)
					Expect(err).To(BeInvalidError())
					Expect(getErrorList(err)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeInvalid),
							"Field":  Equal("spec.dns.domain"),
							"Detail": ContainSubstring("shoot with 'metadata.generateName' uses a default domain but does not match expected scheme: <random-subdomain>.<project-name>.<default-domain> (expected '.my-project.example.com' to be a suffix of '.other-project.example.com')"),
						})),
					))
				})

				It("should reject shoots setting a non compliant default domain on updates if domain was previously not set", func() {
					Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(defaultDomainSecret)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(project)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(seed)).To(Succeed())

					oldShoot := shoot.DeepCopy()

					shootDomain := fmt.Sprintf("%s.other-project.%s", shoot.Name, domain)
					shoot.Spec.DNS.Domain = &shootDomain

					attrs := admission.NewAttributesRecord(shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

					err := admissionHandler.Validate(ctx, attrs, nil)
					Expect(err).To(BeInvalidError())
					Expect(getErrorList(err)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeInvalid),
							"Field":  Equal("spec.dns.domain"),
							"Detail": ContainSubstring("shoot with 'metadata.generateName' uses a default domain but does not match expected scheme: <random-subdomain>.<project-name>.<default-domain> (expected '.my-project.example.com' to be a suffix of '.other-project.example.com')"),
						})),
					))
				})

				It("should not reject shoots using a non compliant default domain on update if domain was previously set", func() {
					Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(defaultDomainSecret)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(project)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(seed)).To(Succeed())

					shootDomain := fmt.Sprintf("%s.other-project.%s", shoot.Name, domain)
					shoot.Spec.DNS.Domain = &shootDomain

					attrs := admission.NewAttributesRecord(shoot, shoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

					Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
				})
			})
		})
	})
})

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
