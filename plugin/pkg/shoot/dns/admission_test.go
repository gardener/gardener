// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package dns_test

import (
	"context"
	"fmt"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

var _ = Describe("dns", func() {

	Describe("#Admit", func() {
		var (
			admissionHandler    *DNS
			kubeInformerFactory kubeinformers.SharedInformerFactory
			coreInformerFactory gardencoreinformers.SharedInformerFactory

			seed  gardencorev1beta1.Seed
			shoot core.Shoot

			namespace   = "my-namespace"
			projectName = "my-project"
			seedName    = "my-seed"
			shootName   = "shoot"

			domain               = "example.com"
			domainHigherPriority = "higher.example.com"
			domainLowerPriority  = "lower.example.com"

			provider = core.DNSUnmanaged

			project = gardencorev1beta1.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: projectName,
				},
				Spec: gardencorev1beta1.ProjectSpec{
					Namespace: &namespace,
				},
			}

			defaultDomainProvider = "my-dns-provider"
			defaultDomainSecret   = corev1.Secret{
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

			defaultDomainSecretHigherPriority = corev1.Secret{
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

			defaultDomainSecretLowerPriority = corev1.Secret{
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

			seedBase = gardencorev1beta1.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Name: seedName,
				},
			}

			shootBase = core.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      shootName,
					Namespace: namespace,
				},
				Spec: core.ShootSpec{
					DNS:      &core.DNS{},
					SeedName: &seedName,
				},
			}
		)

		BeforeEach(func() {
			admissionHandler, _ = New()
			admissionHandler.AssignReadyFunc(func() bool { return true })
			kubeInformerFactory = kubeinformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetKubeInformerFactory(kubeInformerFactory)
			coreInformerFactory = gardencoreinformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetCoreInformerFactory(coreInformerFactory)

			shootBase.Spec.DNS.Domain = nil
			shootBase.Spec.DNS.Providers = []core.DNSProvider{
				{
					Type: &provider,
				},
			}
			shoot = shootBase
			seed = seedBase
		})

		It("should do nothing because the shoot status is updated", func() {
			shootCopy := shoot.DeepCopy()
			shootCopy.Spec.SeedName = nil
			shootBefore := shootCopy.DeepCopy()

			attrs := admission.NewAttributesRecord(shootCopy, nil, core.Kind("Shoot").WithVersion("version"), shootCopy.Namespace, shootCopy.Name, core.Resource("shoots").WithVersion("version"), "status", admission.Create, &metav1.CreateOptions{}, false, nil)

			err := admissionHandler.Admit(context.TODO(), attrs, nil)

			Expect(err).NotTo(HaveOccurred())
			Expect(*shootCopy).To(Equal(*shootBefore))
		})

		It("should do nothing because the shoot does not specify a seed (create)", func() {
			shootCopy := shoot.DeepCopy()
			shootCopy.Spec.SeedName = nil
			shootBefore := shootCopy.DeepCopy()

			attrs := admission.NewAttributesRecord(shootCopy, nil, core.Kind("Shoot").WithVersion("version"), shootCopy.Namespace, shootCopy.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

			err := admissionHandler.Admit(context.TODO(), attrs, nil)

			Expect(err).NotTo(HaveOccurred())
			Expect(*shootCopy).To(Equal(*shootBefore))
		})

		It("should do nothing because the shoot does not specify a seed (update)", func() {
			shootCopy := shoot.DeepCopy()
			shootCopy.Spec.SeedName = nil
			shootBefore := shootCopy.DeepCopy()

			attrs := admission.NewAttributesRecord(shootCopy, shootCopy, core.Kind("Shoot").WithVersion("version"), shootCopy.Namespace, shootCopy.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

			err := admissionHandler.Admit(context.TODO(), attrs, nil)

			Expect(err).NotTo(HaveOccurred())
			Expect(*shootCopy).To(Equal(*shootBefore))
		})

		It("should set the 'unmanaged' dns provider as the primary one", func() {
			shootBefore := shoot.DeepCopy()
			shootBefore.Spec.DNS.Providers[0].Primary = ptr.To(true)

			Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
			attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

			err := admissionHandler.Admit(context.TODO(), attrs, nil)

			Expect(err).NotTo(HaveOccurred())
			Expect(shoot).To(Equal(*shootBefore))
		})

		Context("provider is not 'unmanaged'", func() {
			var (
				providerType = "provider"
				secretName   = "secret"
			)

			BeforeEach(func() {
				shoot.Spec.DNS.Domain = nil
				shoot.Spec.DNS.Providers = nil
			})

			It("should pass because no default domain was generated for the shoot (with domain)", func() {
				var (
					shootDomain  = "my-shoot.my-private-domain.com"
					providerType = "provider"
				)
				shoot.Spec.DNS.Domain = &shootDomain
				shoot.Spec.DNS.Providers = []core.DNSProvider{
					{
						Type: &providerType,
					},
				}

				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(*shoot.Spec.DNS.Domain).To(Equal(shootDomain))
				Expect(shoot.Spec.DNS.Providers).To(ConsistOf(MatchFields(IgnoreExtras, Fields{
					"Type":    Equal(ptr.To(providerType)),
					"Primary": Equal(ptr.To(true)),
				})))
			})

			It("should set the correct primary DNS provider", func() {
				var (
					shootDomain = "my-shoot.my-private-domain.com"
				)
				shoot.Spec.DNS.Domain = &shootDomain
				shoot.Spec.DNS.Providers = []core.DNSProvider{
					{
						Type: &providerType,
					},
					{
						Type:       &providerType,
						SecretName: &secretName,
					},
				}

				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(*shoot.Spec.DNS.Domain).To(Equal(shootDomain))
				Expect(shoot.Spec.DNS.Providers).To(ConsistOf(
					MatchFields(IgnoreExtras, Fields{
						"Type":    Equal(ptr.To(providerType)),
						"Primary": Equal(ptr.To(true)),
					}),
					MatchFields(IgnoreExtras, Fields{
						"Type":       Equal(ptr.To(providerType)),
						"Primary":    BeNil(),
						"SecretName": Equal(ptr.To(secretName)),
					}),
				))
			})

			It("should re-assign the correct primary DNS provider on updates", func() {
				var (
					shootDomain = "my-shoot.my-private-domain.com"
					secretName2 = "secret2"
				)
				shoot.Spec.DNS.Domain = &shootDomain
				shoot.Spec.DNS.Providers = []core.DNSProvider{
					{
						Type:       &providerType,
						SecretName: &secretName2,
					},
					{
						Type:       &providerType,
						SecretName: &secretName,
					},
				}

				oldShoot := shoot.DeepCopy()
				oldShoot.Spec.DNS.Providers[1].Primary = ptr.To(true)

				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(*shoot.Spec.DNS.Domain).To(Equal(shootDomain))
				Expect(shoot.Spec.DNS.Providers).To(ConsistOf(
					MatchFields(IgnoreExtras, Fields{
						"Type": Equal(ptr.To(providerType)),
					}),
					MatchFields(IgnoreExtras, Fields{
						"Type":       Equal(ptr.To(providerType)),
						"Primary":    Equal(ptr.To(true)),
						"SecretName": Equal(ptr.To(secretName)),
					}),
				))
			})

			It("should not allow functionless DNS providers on create w/ seed assignment", func() {
				var (
					shootDomain = "my-shoot.my-private-domain.com"
				)
				shoot.Spec.DNS.Domain = &shootDomain
				shoot.Spec.DNS.Providers = []core.DNSProvider{
					{
						Type: &providerType,
					},
					{
						Type:       &providerType,
						SecretName: &secretName,
					},
					{
						SecretName: &secretName,
					},
				}

				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(PointTo(MatchFields(IgnoreExtras, Fields{
					"ErrStatus": MatchFields(IgnoreExtras, Fields{
						"Code": Equal(int32(http.StatusUnprocessableEntity)),
					})},
				)))
			})

			It("should not remove functionless DNS providers on create w/o seed assignment", func() {
				var (
					shootDomain = "my-shoot.my-private-domain.com"
				)
				shoot.Spec.SeedName = nil
				shoot.Spec.DNS.Domain = &shootDomain
				shoot.Spec.DNS.Providers = []core.DNSProvider{
					{
						Type: &providerType,
					},
					{
						Type:       &providerType,
						SecretName: &secretName,
					},
					{
						Type: &providerType,
					},
				}

				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(*shoot.Spec.DNS.Domain).To(Equal(shootDomain))
				Expect(shoot.Spec.DNS.Providers).To(ConsistOf(
					MatchFields(IgnoreExtras, Fields{
						"Type":    Equal(ptr.To(providerType)),
						"Primary": BeNil(),
					}),
					MatchFields(IgnoreExtras, Fields{
						"Type":       Equal(ptr.To(providerType)),
						"Primary":    BeNil(),
						"SecretName": Equal(ptr.To(secretName)),
					}),
					MatchFields(IgnoreExtras, Fields{
						"Type":    Equal(ptr.To(providerType)),
						"Primary": BeNil(),
					}),
				))
			})

			It("should forbid functionless DNS providers on updates w/ seed assignment", func() {
				var (
					shootDomain = "my-shoot.my-private-domain.com"
				)
				shoot.Spec.DNS.Domain = &shootDomain
				oldShoot := shoot.DeepCopy()

				providers := []core.DNSProvider{
					{
						Type: &providerType,
					},
					{
						Type:    &providerType,
						Primary: ptr.To(true),
					},
					{
						Type: &providerType,
					},
				}

				oldShoot.Spec.DNS.Providers = []core.DNSProvider{providers[1]}
				shoot.Spec.DNS.Providers = providers

				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(PointTo(MatchFields(IgnoreExtras, Fields{
					"ErrStatus": MatchFields(IgnoreExtras, Fields{
						"Code": Equal(int32(http.StatusUnprocessableEntity)),
					})},
				)))
			})

			It("should not remove functionless DNS providers on updates w/o seed assignment", func() {
				var (
					shootDomain = "my-shoot.my-private-domain.com"
				)
				shoot.Spec.SeedName = nil
				shoot.Spec.DNS.Domain = &shootDomain
				oldShoot := shoot.DeepCopy()

				providers := []core.DNSProvider{
					{
						Type: &providerType,
					},
					{
						Type:    &providerType,
						Primary: ptr.To(true),
					},
					{
						Type: &providerType,
					},
				}

				oldShoot.Spec.DNS.Providers = []core.DNSProvider{providers[1]}
				shoot.Spec.DNS.Providers = providers

				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(*shoot.Spec.DNS.Domain).To(Equal(shootDomain))
				Expect(shoot.Spec.DNS.Providers).To(ConsistOf(
					MatchFields(IgnoreExtras, Fields{
						"Type": Equal(ptr.To(providerType)),
					}),
					MatchFields(IgnoreExtras, Fields{
						"Type":    Equal(ptr.To(providerType)),
						"Primary": Equal(ptr.To(true)),
					}),
					MatchFields(IgnoreExtras, Fields{
						"Type": Equal(ptr.To(providerType)),
					}),
				))
			})

			It("should pass because a default domain was generated for the shoot (no domain)", func() {
				Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&defaultDomainSecret)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(shoot.Spec.DNS.Providers).To(BeNil())
				Expect(*shoot.Spec.DNS.Domain).To(Equal(fmt.Sprintf("%s.%s.%s", shootName, projectName, domain)))
			})

			It("should pass and generate a domain from the default domain with the highest priority (no domain)", func() {
				Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&defaultDomainSecret)).To(Succeed())
				Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&defaultDomainSecretLowerPriority)).To(Succeed())
				Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&defaultDomainSecretHigherPriority)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(shoot.Spec.DNS.Providers).To(BeNil())
				Expect(*shoot.Spec.DNS.Domain).To(Equal(fmt.Sprintf("%s.%s.%s", shootName, projectName, domainHigherPriority)))
			})

			It("should pass and generate a domain from the default domain without priority because default priority is 0 (no domain)", func() {
				Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&defaultDomainSecret)).To(Succeed())
				Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&defaultDomainSecretLowerPriority)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(shoot.Spec.DNS.Providers).To(BeNil())
				Expect(*shoot.Spec.DNS.Domain).To(Equal(fmt.Sprintf("%s.%s.%s", shootName, projectName, domain)))
			})

			It("should not set a primary provider because a default domain was generated for the shoot (no domain)", func() {
				shoot.Spec.DNS.Providers = []core.DNSProvider{
					{
						Type:       &providerType,
						SecretName: &secretName,
					},
				}

				Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&defaultDomainSecret)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(*shoot.Spec.DNS.Domain).To(Equal(fmt.Sprintf("%s.%s.%s", shootName, projectName, domain)))
				Expect(shoot.Spec.DNS.Providers).To(ConsistOf(MatchFields(IgnoreExtras, Fields{
					"Type":       Equal(ptr.To(providerType)),
					"Primary":    BeNil(),
					"SecretName": Equal(ptr.To(secretName)),
				})))
			})

			It("should forbid setting a primary provider because a default domain was generated for the shoot (no domain)", func() {
				shoot.Spec.DNS.Providers = []core.DNSProvider{
					{
						Type:       &providerType,
						SecretName: &secretName,
						Primary:    ptr.To(true),
					},
				}

				Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&defaultDomainSecret)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(PointTo(MatchFields(IgnoreExtras, Fields{
					"ErrStatus": MatchFields(IgnoreExtras, Fields{
						"Code": Equal(int32(http.StatusUnprocessableEntity)),
					}),
				})))
			})

			It("should forbid setting a primary provider because a default domain was manually configured for the shoot", func() {
				shootDomain := fmt.Sprintf("%s.%s.%s", shoot.Name, project.Name, domain)
				shoot.Spec.DNS.Domain = &shootDomain
				shoot.Spec.DNS.Providers = []core.DNSProvider{
					{
						Type:       &providerType,
						SecretName: &secretName,
						Primary:    ptr.To(true),
					},
				}

				Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&defaultDomainSecret)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(PointTo(MatchFields(IgnoreExtras, Fields{
					"ErrStatus": MatchFields(IgnoreExtras, Fields{
						"Code": Equal(int32(http.StatusUnprocessableEntity)),
					}),
				})))
			})

			It("should pass because the default domain was allowed for the shoot (with domain)", func() {
				shootDomain := fmt.Sprintf("%s.%s.%s", shoot.Name, project.Name, domain)
				shoot.Spec.DNS.Domain = &shootDomain

				Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&defaultDomainSecret)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(shoot.Spec.DNS.Providers).To(BeNil())
				Expect(*shoot.Spec.DNS.Domain).To(Equal(shootDomain))
			})

			It("should reject because a default domain was already used for the shoot but is invalid (with domain)", func() {
				shootDomain := fmt.Sprintf("%s.other-project.%s", shoot.Name, domain)
				shoot.Spec.DNS.Domain = &shootDomain

				Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&defaultDomainSecret)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
			})

			It("should reject because a default domain was already used for the shoot but is invalid (with domain) when seed is assigned", func() {
				shootDomain := fmt.Sprintf("%s.other-project.%s", shoot.Name, domain)
				shoot.Spec.DNS.Domain = &shootDomain
				oldShoot := shoot.DeepCopy()
				oldShoot.Spec.SeedName = nil

				Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&defaultDomainSecret)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
			})

			It("should reject shoots setting a non compliant default domain on updates if domain was previously not set", func() {
				oldShoot := shoot.DeepCopy()

				shootDomain := fmt.Sprintf("%s.other-project.%s", shoot.Name, domain)
				shoot.Spec.DNS.Domain = &shootDomain

				Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&defaultDomainSecret)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
			})

			It("should not reject shoots using a non complaint default domain on update if domain was previously set", func() {
				shootDomain := fmt.Sprintf("%s.other-project.%s", shoot.Name, domain)
				shoot.Spec.DNS.Domain = &shootDomain

				Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&defaultDomainSecret)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				attrs := admission.NewAttributesRecord(&shoot, &shoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should reject because no domain was configured for the shoot and project is missing", func() {
				Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&defaultDomainSecret)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(MatchError(error(apierrors.NewInternalError(fmt.Errorf("Project.core.gardener.cloud %q not found", "<unknown>")))))
			})

			It("should reject because no domain was configured for the shoot and default domain secret is missing", func() {
				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(BeInvalidError())
			})

			Context("#Shoot GenerateName used", func() {
				BeforeEach(func() {
					shoot.Name = ""
					shoot.GenerateName = "demo-"
				})

				It("should set different default domain for multiple shoots with same generate name", func() {
					shootCopy := shoot.DeepCopy()

					Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&defaultDomainSecret)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

					err := admissionHandler.Admit(context.TODO(), attrs, nil)
					Expect(err).To(Not(HaveOccurred()))

					Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&defaultDomainSecret)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					attrs = admission.NewAttributesRecord(shootCopy, nil, core.Kind("Shoot").WithVersion("version"), shootCopy.Namespace, shootCopy.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

					err = admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).To(Not(HaveOccurred()))

					Expect(*shoot.Spec.DNS.Domain).NotTo(Equal(*shootCopy.Spec.DNS.Domain))
				})

				It("should generate a default domain with shoot name for the shoot (no domain)", func() {
					shoot.Name = "foo"
					Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&defaultDomainSecret)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).NotTo(HaveOccurred())
					Expect(shoot.Spec.DNS.Providers).To(BeNil())
					Expect(*shoot.Spec.DNS.Domain).To(Equal(fmt.Sprintf("%s.%s.%s", shoot.Name, projectName, domain)))
				})

				It("should pass because a default domain was generated for the shoot (no domain)", func() {
					Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&defaultDomainSecret)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).NotTo(HaveOccurred())
					Expect(shoot.Spec.DNS.Providers).To(BeNil())
					Expect(*shoot.Spec.DNS.Domain).To(HaveSuffix(fmt.Sprintf(".%s.%s", projectName, domain)))
				})

				It("should pass because a default domain was re-assigned for the shoot (no domain)", func() {
					oldShoot := shoot.DeepCopy()
					shoot.Spec.DNS = nil

					Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&defaultDomainSecret)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).NotTo(HaveOccurred())
					Expect(shoot.Spec.DNS.Providers).To(BeNil())
					Expect(*shoot.Spec.DNS.Domain).To(HaveSuffix(fmt.Sprintf(".%s.%s", projectName, domain)))
				})

				It("should reject because a default domain was already used for the shoot but is invalid (with domain)", func() {
					shootDomain := fmt.Sprintf("%s.other-project.%s", shoot.Name, domain)
					shoot.Spec.DNS.Domain = &shootDomain

					Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&defaultDomainSecret)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).To(HaveOccurred())
				})

				It("should not reject shoots using a non compliant default domain on updates if domain was previously set", func() {
					shootDomain := fmt.Sprintf("%s.other-project.%s", shoot.Name, domain)
					shoot.Spec.DNS.Domain = &shootDomain

					Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&defaultDomainSecret)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					attrs := admission.NewAttributesRecord(&shoot, &shoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).NotTo(HaveOccurred())
				})

				It("should reject shoots setting a non compliant default domain on updates if domain was previously not set", func() {
					oldShoot := shoot.DeepCopy()

					shootDomain := fmt.Sprintf("%s.other-project.%s", shoot.Name, domain)
					shoot.Spec.DNS.Domain = &shootDomain

					Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&defaultDomainSecret)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).To(HaveOccurred())
				})
			})
		})

		Context("Shoot Control Plane Migration", func() {
			var (
				destinationSeedName = "my-seed-2"
				destinationSeed     core.Seed
			)

			BeforeEach(func() {
				destinationSeed = core.Seed{
					ObjectMeta: metav1.ObjectMeta{
						Name: destinationSeedName,
					},
				}

				shoot.Spec.DNS.Providers = nil
			})

			It("should accept shoot migration update", func() {
				shootDomain := fmt.Sprintf("%s.%s.%s", shoot.Name, project.Name, domain)
				shoot.Spec.DNS.Domain = &shootDomain

				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&destinationSeed)).To(Succeed())

				shoot.Spec.SeedName = &destinationSeedName
				attrs := admission.NewAttributesRecord(&shoot, &shoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})
