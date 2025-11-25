// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("Garden", func() {
	var (
		ctx        = context.Background()
		fakeClient client.Client

		defaultDomainProvider   = "default-domain-provider"
		defaultDomainSecretData = map[string][]byte{"default": []byte("domain")}
		defaultDomain           = &Domain{
			Domain:     "bar.com",
			Provider:   defaultDomainProvider,
			SecretData: defaultDomainSecretData,
		}
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).Build()
	})

	DescribeTable("#DomainIsDefaultDomain",
		func(domain string, defaultDomains []*Domain, expected gomegatypes.GomegaMatcher) {
			Expect(DomainIsDefaultDomain(domain, defaultDomains)).To(expected)
		},

		Entry("no default domain", "foo.bar.com", nil, BeNil()),
		Entry("default domain", "foo.bar.com", []*Domain{defaultDomain}, Equal(defaultDomain)),
		Entry("no default domain but with same suffix", "foo.foobar.com", []*Domain{defaultDomain}, BeNil()),
	)

	Describe("#NewGardenAccessSecret", func() {
		var (
			name      = "name"
			namespace = "namespace"
		)

		DescribeTable("default name/namespace",
			func(prefix string) {
				Expect(NewGardenAccessSecret(prefix+name, namespace)).To(Equal(&AccessSecret{
					Secret:             &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "garden-access-" + name, Namespace: namespace}},
					ServiceAccountName: name,
					Class:              "garden",
				}))
			},

			Entry("no prefix", ""),
			Entry("prefix", "garden-access-"),
		)

		It("should override the name and namespace", func() {
			Expect(NewGardenAccessSecret(name, namespace).
				WithNameOverride("other-name").
				WithNamespaceOverride("other-namespace"),
			).To(Equal(&AccessSecret{
				Secret:             &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "other-name", Namespace: "other-namespace"}},
				ServiceAccountName: name,
				Class:              "garden",
			}))
		})
	})

	Describe("#InjectGenericGardenKubeconfig", func() {
		var (
			genericTokenKubeconfigSecretName = "generic-token-kubeconfig-12345"
			tokenSecretName                  = "tokensecret"
			containerName1                   = "container1"
			containerName2                   = "container2"

			deployment *appsv1.Deployment
			podSpec    *corev1.PodSpec
		)

		BeforeEach(func() {
			deployment = &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{Name: containerName1},
								{Name: containerName2},
							},
						},
					},
				},
			}

			podSpec = &deployment.Spec.Template.Spec
		})

		It("should do nothing because object is not handled", func() {
			Expect(InjectGenericGardenKubeconfig(&corev1.Service{}, genericTokenKubeconfigSecretName, tokenSecretName, VolumeMountPathGenericGardenKubeconfig)).To(MatchError(ContainSubstring("unhandled object type")))
		})

		It("should do nothing because a container already has the GARDEN_KUBECONFIG env var", func() {
			container := podSpec.Containers[1]
			container.Env = []corev1.EnvVar{{Name: "GARDEN_KUBECONFIG"}}
			podSpec.Containers[1] = container

			Expect(InjectGenericGardenKubeconfig(deployment, genericTokenKubeconfigSecretName, tokenSecretName, VolumeMountPathGenericGardenKubeconfig)).To(Succeed())

			Expect(podSpec.Volumes).To(BeEmpty())
			Expect(podSpec.Containers[0].VolumeMounts).To(BeEmpty())
			Expect(podSpec.Containers[1].VolumeMounts).To(BeEmpty())
		})

		It("should inject the generic kubeconfig into the specified container", func() {
			Expect(InjectGenericGardenKubeconfig(deployment, genericTokenKubeconfigSecretName, tokenSecretName, VolumeMountPathGenericGardenKubeconfig, containerName1)).To(Succeed())

			Expect(deployment.GetAnnotations()).To(HaveKeyWithValue("reference.resources.gardener.cloud/secret-cd5ff419", "generic-token-kubeconfig-12345"))
			Expect(deployment.GetAnnotations()).To(HaveKeyWithValue("reference.resources.gardener.cloud/secret-d9db2144", "tokensecret"))

			Expect(podSpec.Volumes).To(ContainElement(corev1.Volume{
				Name: "garden-kubeconfig",
				VolumeSource: corev1.VolumeSource{
					Projected: &corev1.ProjectedVolumeSource{
						DefaultMode: ptr.To[int32](420),
						Sources: []corev1.VolumeProjection{
							{
								Secret: &corev1.SecretProjection{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: genericTokenKubeconfigSecretName,
									},
									Items: []corev1.KeyToPath{{
										Key:  "kubeconfig",
										Path: "kubeconfig",
									}},
									Optional: ptr.To(false),
								},
							},
							{
								Secret: &corev1.SecretProjection{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: tokenSecretName,
									},
									Items: []corev1.KeyToPath{{
										Key:  "token",
										Path: "token",
									}},
									Optional: ptr.To(false),
								},
							},
						},
					},
				},
			}))

			Expect(podSpec.Containers[0].VolumeMounts).To(ContainElement(corev1.VolumeMount{
				Name:      "garden-kubeconfig",
				MountPath: "/var/run/secrets/gardener.cloud/garden/generic-kubeconfig",
				ReadOnly:  true,
			}))
			Expect(podSpec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:  "GARDEN_KUBECONFIG",
				Value: "/var/run/secrets/gardener.cloud/garden/generic-kubeconfig/kubeconfig",
			}))

			Expect(podSpec.Containers[1].VolumeMounts).To(BeEmpty())
			Expect(podSpec.Containers[1].Env).To(BeEmpty())
		})
	})

	Describe("#PrepareGardenClientRestConfig", func() {
		var baseConfig *rest.Config

		BeforeEach(func() {
			baseConfig = &rest.Config{
				Username: "test",
				Host:     "https://garden.local.gardener.cloud",
				TLSClientConfig: rest.TLSClientConfig{
					CAData: []byte("ca"),
				},
			}
		})

		It("should copy the baseConfig if no overwrites are given", func() {
			config := PrepareGardenClientRestConfig(baseConfig, nil, nil)
			Expect(config).NotTo(BeIdenticalTo(baseConfig))
			Expect(config).To(Equal(baseConfig))
		})

		It("should use the overwrites but copy overthing else", func() {
			config := PrepareGardenClientRestConfig(baseConfig, ptr.To("other"), []byte("ca2"))
			Expect(config).NotTo(BeIdenticalTo(baseConfig))
			Expect(config.Host).To(Equal("other"))
			Expect(config.CAData).To(BeEquivalentTo("ca2"))

			// everything else should be equal
			config.Host = baseConfig.Host
			config.CAData = baseConfig.CAData
			Expect(config).To(Equal(baseConfig))
		})
	})

	Describe("#DefaultGardenerGVKsForEncryption", func() {
		It("should return all default GroupVersionKinds", func() {
			Expect(DefaultGardenerGVKsForEncryption()).To(ConsistOf(
				schema.GroupVersionKind{Group: "core.gardener.cloud", Version: "v1beta1", Kind: "ControllerDeployment"},
				schema.GroupVersionKind{Group: "core.gardener.cloud", Version: "v1beta1", Kind: "ControllerRegistration"},
				schema.GroupVersionKind{Group: "core.gardener.cloud", Version: "v1beta1", Kind: "InternalSecret"},
				schema.GroupVersionKind{Group: "core.gardener.cloud", Version: "v1beta1", Kind: "ShootState"},
			))
		})
	})

	Describe("#DefaultGardenerGroupResourcesForEncryption", func() {
		It("should return all default GroupVersionKinds", func() {
			Expect(DefaultGardenerGroupResourcesForEncryption()).To(ConsistOf(
				schema.GroupResource{Group: "core.gardener.cloud", Resource: "controllerdeployments"},
				schema.GroupResource{Group: "core.gardener.cloud", Resource: "controllerregistrations"},
				schema.GroupResource{Group: "core.gardener.cloud", Resource: "internalsecrets"},
				schema.GroupResource{Group: "core.gardener.cloud", Resource: "shootstates"},
			))
		})
	})

	Describe("#DefaultGardenerResourcesForEncryption", func() {
		It("should return all default resources", func() {
			Expect(DefaultGardenerResourcesForEncryption().UnsortedList()).To(ConsistOf(
				"controllerdeployments.core.gardener.cloud",
				"controllerregistrations.core.gardener.cloud",
				"internalsecrets.core.gardener.cloud",
				"shootstates.core.gardener.cloud",
			))
		})
	})

	Describe("#GetGardenWildcardCertificate", func() {
		var (
			namespace    string
			gardenSecret *corev1.Secret
		)

		BeforeEach(func() {
			namespace = "garden"
			gardenSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "secret-",
					Namespace:    namespace,
					Labels:       map[string]string{"gardener.cloud/role": "garden-cert"},
				},
			}
		})

		It("should return an error because there are more than one Garden wildcard certificates", func() {
			secret2 := gardenSecret.DeepCopy()
			Expect(fakeClient.Create(ctx, gardenSecret)).To(Succeed())
			Expect(fakeClient.Create(ctx, secret2)).To(Succeed())

			result, err := GetGardenWildcardCertificate(ctx, fakeClient, "garden")
			Expect(result).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("misconfigured cluster: not possible to provide more than one secret with label")))
		})

		It("should return no certificate", func() {
			result, err := GetGardenWildcardCertificate(ctx, fakeClient, "garden")
			Expect(result).To(BeNil())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the expected wildcard certificate", func() {
			Expect(fakeClient.Create(ctx, gardenSecret)).To(Succeed())

			result, err := GetGardenWildcardCertificate(ctx, fakeClient, "garden")
			Expect(result).To(Equal(gardenSecret))
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("#GetRequiredGardenWildcardCertificate", func() {
		var (
			namespace    string
			gardenSecret *corev1.Secret
		)

		BeforeEach(func() {
			namespace = "garden"
			gardenSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "secret-",
					Namespace:    namespace,
					Labels:       map[string]string{"gardener.cloud/role": "garden-cert"},
				},
			}
		})

		It("should return an error because there are more than one Garden wildcard certificates", func() {
			secret2 := gardenSecret.DeepCopy()
			Expect(fakeClient.Create(ctx, gardenSecret)).To(Succeed())
			Expect(fakeClient.Create(ctx, secret2)).To(Succeed())

			result, err := GetRequiredGardenWildcardCertificate(ctx, fakeClient, "garden")
			Expect(result).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("misconfigured cluster: not possible to provide more than one secret with label")))
		})

		It("should return an error because certificate is not found", func() {
			result, err := GetRequiredGardenWildcardCertificate(ctx, fakeClient, "garden")
			Expect(result).To(BeNil())
			Expect(err).To(MatchError("no garden wildcard certificate secret found"))
		})

		It("should return the expected wildcard certificate", func() {
			Expect(fakeClient.Create(ctx, gardenSecret)).To(Succeed())

			result, err := GetRequiredGardenWildcardCertificate(ctx, fakeClient, "garden")
			Expect(result).To(Equal(gardenSecret))
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("#ReadGardenInternalDomain", func() {
		var (
			seedDNSProvider *gardencorev1beta1.SeedDNSProviderConfig
			secret          *corev1.Secret
			namespace       = "garden"
			providerType    = "route-53"
			domain          = "internal.example.com"
			zone            = "zone-1"
		)

		BeforeEach(func() {
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
			seedDNSProvider = &gardencorev1beta1.SeedDNSProviderConfig{
				Type:   providerType,
				Domain: domain,
				Zone:   ptr.To(zone),
				CredentialsRef: corev1.ObjectReference{
					APIVersion: "v1",
					Kind:       "Secret",
					Name:       "internal-domain",
					Namespace:  namespace,
				},
			}
			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "internal-domain",
					Namespace: namespace,
				},
				Data: map[string][]byte{"foo": []byte("bar")},
			}
		})

		It("should return domain information from SeedDNSProviderConf", func() {
			Expect(fakeClient.Create(ctx, secret)).To(Succeed())

			result, err := ReadGardenInternalDomain(ctx, fakeClient, namespace, true, seedDNSProvider)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(&Domain{
				Domain:     domain,
				Provider:   providerType,
				Zone:       zone,
				SecretData: map[string][]byte{"foo": []byte("bar")},
			}))
		})

		It("should return domain information from a labeled secret", func() {
			secret.Labels = map[string]string{
				constants.GardenRole: constants.GardenRoleInternalDomain,
			}
			secret.Annotations = map[string]string{
				"dns.gardener.cloud/provider": providerType,
				"dns.gardener.cloud/domain":   domain,
				"dns.gardener.cloud/zone":     zone,
			}

			Expect(fakeClient.Create(ctx, secret)).To(Succeed())

			result, err := ReadGardenInternalDomain(ctx, fakeClient, namespace, true, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(&Domain{
				Domain:     domain,
				Provider:   providerType,
				Zone:       zone,
				SecretData: map[string][]byte{"foo": []byte("bar")},
			}))
		})

		It("should return nil if no secret and enforceSecret is false", func() {
			result, err := ReadGardenInternalDomain(ctx, fakeClient, namespace, false, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeNil())
		})

		It("should error if no secret and enforceSecret is true", func() {
			result, err := ReadGardenInternalDomain(ctx, fakeClient, namespace, true, nil)
			Expect(result).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("need an internal domain secret")))
		})

		It("should error if more than one secret is found", func() {
			secret.Labels = map[string]string{
				constants.GardenRole: constants.GardenRoleInternalDomain,
			}
			secret.Annotations = map[string]string{
				"dns.gardener.cloud/provider": providerType,
				"dns.gardener.cloud/domain":   domain,
				"dns.gardener.cloud/zone":     zone,
			}

			secret2 := secret.DeepCopy()
			secret2.Name = "internal-domain-2"

			Expect(fakeClient.Create(ctx, secret)).To(Succeed())
			Expect(fakeClient.Create(ctx, secret2)).To(Succeed())

			result, err := ReadGardenInternalDomain(ctx, fakeClient, namespace, true, nil)
			Expect(result).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("more than one internal domain secret")))
		})

		It("should error if secret is malformed", func() {
			secret.Labels = map[string]string{
				constants.GardenRole: constants.GardenRoleInternalDomain,
			}
			secret.Annotations = map[string]string{
				"dns.gardener.cloud/provider": providerType,
				// Missing domain annotation
			}

			Expect(fakeClient.Create(ctx, secret)).To(Succeed())

			result, err := ReadGardenInternalDomain(ctx, fakeClient, namespace, true, nil)
			Expect(result).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("error constructing internal domain from secret")))
		})
	})

	Describe("#ReadInternalDomainSecret", func() {
		var (
			namespace    = "garden"
			providerType = "route-53"
			domain       = "internal.example.com"
			zone         = "zone-1"
			secret       *corev1.Secret
		)

		BeforeEach(func() {
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "internal-domain",
					Namespace: namespace,
					Labels: map[string]string{
						constants.GardenRole: constants.GardenRoleInternalDomain,
					},
					Annotations: map[string]string{
						"dns.gardener.cloud/provider": providerType,
						"dns.gardener.cloud/domain":   domain,
						"dns.gardener.cloud/zone":     zone,
					},
				},
				Data: map[string][]byte{"foo": []byte("bar")},
			}
		})

		It("should return the internal domain secret", func() {
			Expect(fakeClient.Create(ctx, secret)).To(Succeed())

			result, err := ReadInternalDomainSecret(ctx, fakeClient, namespace, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())
			Expect(result.Name).To(Equal("internal-domain"))
		})

		It("should return nil if no secret and enforceSecret is false and no secret is found", func() {
			result, err := ReadInternalDomainSecret(ctx, fakeClient, namespace, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeNil())
		})

		It("should return the secret if enforceSecret is false but secret is found", func() {
			Expect(fakeClient.Create(ctx, secret)).To(Succeed())

			result, err := ReadInternalDomainSecret(ctx, fakeClient, namespace, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())
		})

		It("should error if no secret and enforceSecret is true", func() {
			result, err := ReadInternalDomainSecret(ctx, fakeClient, namespace, true)
			Expect(result).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("need an internal domain secret")))
		})

		It("should error if more than one secret is found", func() {
			secret2 := secret.DeepCopy()
			secret2.Name = "internal-domain-2"
			Expect(fakeClient.Create(ctx, secret)).To(Succeed())
			Expect(fakeClient.Create(ctx, secret2)).To(Succeed())

			result, err := ReadInternalDomainSecret(ctx, fakeClient, namespace, true)
			Expect(result).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("more than one internal domain secret")))
		})
	})

	Describe("#ReadGardenDefaultDomains", func() {
		var (
			namespace     = "garden"
			providerType1 = "route-53"
			domain1       = "default1.example.com"
			zone1         = "zone-1"
			providerType2 = "cloudflare"
			domain2       = "default2.example.com"
			zone2         = "zone-2"
		)

		BeforeEach(func() {
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
		})

		It("should return domain information from SeedDNSProviderConfig array", func() {
			secret1 := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default-domain-1",
					Namespace: namespace,
				},
				Data: map[string][]byte{"foo": []byte("bar")},
			}
			secret2 := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default-domain-2",
					Namespace: namespace,
				},
				Data: map[string][]byte{"baz": []byte("qux")},
			}

			Expect(fakeClient.Create(ctx, secret1)).To(Succeed())
			Expect(fakeClient.Create(ctx, secret2)).To(Succeed())

			seedDNSDefaults := []gardencorev1beta1.SeedDNSProviderConfig{
				{
					Type:   providerType1,
					Domain: domain1,
					Zone:   ptr.To(zone1),
					CredentialsRef: corev1.ObjectReference{
						APIVersion: "v1",
						Kind:       "Secret",
						Name:       "default-domain-1",
						Namespace:  namespace,
					},
				},
				{
					Type:   providerType2,
					Domain: domain2,
					Zone:   ptr.To(zone2),
					CredentialsRef: corev1.ObjectReference{
						APIVersion: "v1",
						Kind:       "Secret",
						Name:       "default-domain-2",
						Namespace:  namespace,
					},
				},
			}

			result, err := ReadGardenDefaultDomains(ctx, fakeClient, namespace, seedDNSDefaults)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal([]*Domain{
				{
					Domain:     domain1,
					Provider:   providerType1,
					Zone:       zone1,
					SecretData: map[string][]byte{"foo": []byte("bar")},
				},
				{
					Domain:     domain2,
					Provider:   providerType2,
					Zone:       zone2,
					SecretData: map[string][]byte{"baz": []byte("qux")},
				},
			}))
		})

		It("should return domain information from labeled secrets when no seedDNSDefaults provided", func() {
			secret1 := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default-domain-1",
					Namespace: namespace,
					Labels: map[string]string{
						constants.GardenRole: constants.GardenRoleDefaultDomain,
					},
					Annotations: map[string]string{
						"dns.gardener.cloud/provider": providerType1,
						"dns.gardener.cloud/domain":   domain1,
						"dns.gardener.cloud/zone":     zone1,
					},
				},
				Data: map[string][]byte{"foo": []byte("bar")},
			}
			secret2 := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default-domain-2",
					Namespace: namespace,
					Labels: map[string]string{
						constants.GardenRole: constants.GardenRoleDefaultDomain,
					},
					Annotations: map[string]string{
						"dns.gardener.cloud/provider": providerType2,
						"dns.gardener.cloud/domain":   domain2,
						"dns.gardener.cloud/zone":     zone2,
					},
				},
				Data: map[string][]byte{"baz": []byte("qux")},
			}

			Expect(fakeClient.Create(ctx, secret1)).To(Succeed())
			Expect(fakeClient.Create(ctx, secret2)).To(Succeed())

			result, err := ReadGardenDefaultDomains(ctx, fakeClient, namespace, nil)
			Expect(err).NotTo(HaveOccurred())

			Expect(result).To(ConsistOf(
				&Domain{
					Domain:     domain1,
					Provider:   providerType1,
					Zone:       zone1,
					SecretData: map[string][]byte{"foo": []byte("bar")},
				},
				&Domain{
					Domain:     domain2,
					Provider:   providerType2,
					Zone:       zone2,
					SecretData: map[string][]byte{"baz": []byte("qux")},
				},
			))
		})

		It("should return empty slice when no default domains found", func() {
			result, err := ReadGardenDefaultDomains(ctx, fakeClient, namespace, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeEmpty())
		})

		It("should return empty slice when empty seedDNSDefaults provided", func() {
			result, err := ReadGardenDefaultDomains(ctx, fakeClient, namespace, []gardencorev1beta1.SeedDNSProviderConfig{})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeEmpty())
		})

		It("should error if referenced secret in seedDNSDefaults is not found", func() {
			seedDNSDefaults := []gardencorev1beta1.SeedDNSProviderConfig{
				{
					Type:   providerType1,
					Domain: domain1,
					Zone:   ptr.To(zone1),
					CredentialsRef: corev1.ObjectReference{
						APIVersion: "v1",
						Kind:       "Secret",
						Name:       "non-existent-secret",
						Namespace:  namespace,
					},
				},
			}

			result, err := ReadGardenDefaultDomains(ctx, fakeClient, namespace, seedDNSDefaults)
			Expect(result).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("cannot fetch default domain secret")))
		})

		It("should error if default domain secret is malformed", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "malformed-secret",
					Namespace: namespace,
					Labels: map[string]string{
						constants.GardenRole: constants.GardenRoleDefaultDomain,
					},
					Annotations: map[string]string{
						"dns.gardener.cloud/provider": providerType1,
						// Missing domain annotation
					},
				},
				Data: map[string][]byte{"foo": []byte("bar")},
			}

			Expect(fakeClient.Create(ctx, secret)).To(Succeed())

			result, err := ReadGardenDefaultDomains(ctx, fakeClient, namespace, nil)
			Expect(result).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("error constructing default domain from secret")))
		})

		It("should sort default domains by priority", func() {
			// Create secrets with different priorities
			secretHighPriority := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "high-priority-domain",
					Namespace: namespace,
					Labels: map[string]string{
						constants.GardenRole: constants.GardenRoleDefaultDomain,
					},
					Annotations: map[string]string{
						"dns.gardener.cloud/provider":                providerType1,
						"dns.gardener.cloud/domain":                  "high.example.com",
						"dns.gardener.cloud/zone":                    zone1,
						"dns.gardener.cloud/domain-default-priority": "10",
					},
				},
				Data: map[string][]byte{"high": []byte("priority")},
			}
			secretMediumPriority := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "medium-priority-domain",
					Namespace: namespace,
					Labels: map[string]string{
						constants.GardenRole: constants.GardenRoleDefaultDomain,
					},
					Annotations: map[string]string{
						"dns.gardener.cloud/provider":                providerType2,
						"dns.gardener.cloud/domain":                  "medium.example.com",
						"dns.gardener.cloud/zone":                    zone2,
						"dns.gardener.cloud/domain-default-priority": "5",
					},
				},
				Data: map[string][]byte{"medium": []byte("priority")},
			}
			secretLowPriority := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "low-priority-domain",
					Namespace: namespace,
					Labels: map[string]string{
						constants.GardenRole: constants.GardenRoleDefaultDomain,
					},
					Annotations: map[string]string{
						"dns.gardener.cloud/provider": "dns-provider",
						"dns.gardener.cloud/domain":   "low.example.com",
						"dns.gardener.cloud/zone":     "zone-3",
					},
				},
				Data: map[string][]byte{"low": []byte("priority")},
			}

			Expect(fakeClient.Create(ctx, secretLowPriority)).To(Succeed())
			Expect(fakeClient.Create(ctx, secretHighPriority)).To(Succeed())
			Expect(fakeClient.Create(ctx, secretMediumPriority)).To(Succeed())

			result, err := ReadGardenDefaultDomains(ctx, fakeClient, namespace, nil)
			Expect(err).NotTo(HaveOccurred())

			Expect(result).To(Equal([]*Domain{
				{
					Domain:     "high.example.com",
					Provider:   providerType1,
					Zone:       zone1,
					SecretData: map[string][]byte{"high": []byte("priority")},
				},
				{
					Domain:     "medium.example.com",
					Provider:   providerType2,
					Zone:       zone2,
					SecretData: map[string][]byte{"medium": []byte("priority")},
				},
				{
					Domain:     "low.example.com",
					Provider:   "dns-provider",
					Zone:       "zone-3",
					SecretData: map[string][]byte{"low": []byte("priority")},
				},
			}))
		})
	})

	Describe("#ReadGardenDefaultDomainsSecrets", func() {
		var namespace = "garden"

		BeforeEach(func() {
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
		})

		It("should return empty slice when no default domain secrets exist", func() {
			result, err := ReadGardenDefaultDomainsSecrets(ctx, fakeClient, namespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeEmpty())
		})

		It("should return secrets sorted by priority descending", func() {
			high := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "high",
					Namespace:   namespace,
					Labels:      map[string]string{constants.GardenRole: constants.GardenRoleDefaultDomain},
					Annotations: map[string]string{DNSProvider: "p", DNSDomain: "high.example.com", DNSDefaultDomainPriority: "10"},
				},
			}
			medium := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "medium",
					Namespace:   namespace,
					Labels:      map[string]string{constants.GardenRole: constants.GardenRoleDefaultDomain},
					Annotations: map[string]string{DNSProvider: "p", DNSDomain: "medium.example.com", DNSDefaultDomainPriority: "5"},
				},
			}
			invalid := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "invalid",
					Namespace:   namespace,
					Labels:      map[string]string{constants.GardenRole: constants.GardenRoleDefaultDomain},
					Annotations: map[string]string{DNSProvider: "p", DNSDomain: "invalid.example.com", DNSDefaultDomainPriority: "abc"}, // invalid/missing priorities treated as 0
				},
			}
			missing := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "missing",
					Namespace:   namespace,
					Labels:      map[string]string{constants.GardenRole: constants.GardenRoleDefaultDomain},
					Annotations: map[string]string{DNSProvider: "p", DNSDomain: "missing.example.com"}, // invalid/missing priorities treated as 0
				},
			}

			Expect(fakeClient.Create(ctx, medium)).To(Succeed())
			Expect(fakeClient.Create(ctx, invalid)).To(Succeed())
			Expect(fakeClient.Create(ctx, high)).To(Succeed())
			Expect(fakeClient.Create(ctx, missing)).To(Succeed())

			result, err := ReadGardenDefaultDomainsSecrets(ctx, fakeClient, namespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveLen(4))
			Expect([]string{result[0].Name, result[1].Name, result[2].Name, result[3].Name}).To(Equal([]string{"high", "medium", "invalid", "missing"}))
		})

		It("should keep stable ordering for secrets with identical priorities", func() {
			first := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "first",
					Namespace:   namespace,
					Labels:      map[string]string{constants.GardenRole: constants.GardenRoleDefaultDomain},
					Annotations: map[string]string{DNSProvider: "p", DNSDomain: "first.example.com", DNSDefaultDomainPriority: "7"},
				},
			}
			second := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "second",
					Namespace:   namespace,
					Labels:      map[string]string{constants.GardenRole: constants.GardenRoleDefaultDomain},
					Annotations: map[string]string{DNSProvider: "p", DNSDomain: "second.example.com", DNSDefaultDomainPriority: "7"},
				},
			}

			Expect(fakeClient.Create(ctx, second)).To(Succeed())
			Expect(fakeClient.Create(ctx, first)).To(Succeed())

			result, err := ReadGardenDefaultDomainsSecrets(ctx, fakeClient, namespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveLen(2))
			Expect([]string{result[0].Name, result[1].Name}).To(Equal([]string{"first", "second"}))
		})
	})

	Describe("#ReconcileGardenNamespace", func() {
		var (
			namespaceName = "garden"
			zones         = []string{"1,2"}
		)

		When("metadata should not be managed", func() {
			It("should create the garden namespace with the expected metadata", func() {
				Expect(ReconcileGardenNamespace(ctx, fakeClient, namespaceName, zones, false, nil)).To(Succeed())

				namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "garden"}}
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(namespace), namespace)).To(Succeed())
				Expect(namespace.Annotations).To(BeEmpty())
				Expect(namespace.Labels).To(BeEmpty())
			})

			It("should reconcile the garden namespace with the expected metadata", func() {
				namespace := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "garden",
					},
				}
				Expect(fakeClient.Create(ctx, namespace)).To(Succeed())

				Expect(ReconcileGardenNamespace(ctx, fakeClient, namespaceName, zones, false, nil)).To(Succeed())

				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(namespace), namespace)).To(Succeed())
				Expect(namespace.Annotations).To(BeEmpty())
				Expect(namespace.Labels).To(BeEmpty())
			})
		})

		When("metadata should be managed", func() {
			It("should create the garden namespace with the expected metadata", func() {
				Expect(ReconcileGardenNamespace(ctx, fakeClient, namespaceName, zones, true, nil)).To(Succeed())

				namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "garden"}}
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(namespace), namespace)).To(Succeed())
				Expect(namespace.Annotations).To(HaveKeyWithValue("high-availability-config.resources.gardener.cloud/zones", "1,2"))
				Expect(namespace.Labels).To(And(
					HaveKeyWithValue("pod-security.kubernetes.io/enforce", "privileged"),
					HaveKeyWithValue("high-availability-config.resources.gardener.cloud/consider", "true"),
					HaveKeyWithValue("gardener.cloud/role", "garden"),
				))
			})

			It("should reconcile the garden namespace with the expected metadata", func() {
				namespace := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "garden",
						Annotations: map[string]string{
							"high-availability-config.resources.gardener.cloud/zones": "none",
						},
						Labels: map[string]string{
							"pod-security.kubernetes.io/enforce":                         "unprivileged",
							"high-availability-config.resources.gardener.cloud/consider": "",
						},
					},
				}
				Expect(fakeClient.Create(ctx, namespace)).To(Succeed())

				Expect(ReconcileGardenNamespace(ctx, fakeClient, namespaceName, zones, true, func(namespace *corev1.Namespace) {
					metav1.SetMetaDataLabel(&namespace.ObjectMeta, "foo", "bar")
				})).To(Succeed())

				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(namespace), namespace)).To(Succeed())
				Expect(namespace.Annotations).To(HaveKeyWithValue("high-availability-config.resources.gardener.cloud/zones", "1,2"))
				Expect(namespace.Labels).To(And(
					HaveKeyWithValue("pod-security.kubernetes.io/enforce", "privileged"),
					HaveKeyWithValue("high-availability-config.resources.gardener.cloud/consider", "true"),
					HaveKeyWithValue("foo", "bar"),
					HaveKeyWithValue("gardener.cloud/role", "garden"),
				))
			})
		})
	})
})
