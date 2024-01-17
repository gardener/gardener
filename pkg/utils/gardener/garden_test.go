// Copyright 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package gardener_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"k8s.io/utils/pointer"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	settingsv1alpha1 "github.com/gardener/gardener/pkg/apis/settings/v1alpha1"
	. "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("Garden", func() {
	Describe("#GetDefaultDomains", func() {
		It("should return all default domain", func() {
			var (
				provider = "aws"
				domain   = "example.com"
				data     = map[string][]byte{
					"foo": []byte("bar"),
				}

				secret = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							DNSProvider: provider,
							DNSDomain:   domain,
						},
					},
					Data: data,
				}
				secrets = map[string]*corev1.Secret{
					fmt.Sprintf("%s-%s", constants.GardenRoleDefaultDomain, domain): secret,
				}
			)

			defaultDomains, err := GetDefaultDomains(secrets)

			Expect(err).NotTo(HaveOccurred())
			Expect(defaultDomains).To(Equal([]*Domain{
				{
					Domain:     domain,
					Provider:   provider,
					SecretData: data,
				},
			}))
		})

		It("should return an error", func() {
			secrets := map[string]*corev1.Secret{
				fmt.Sprintf("%s-%s", constants.GardenRoleDefaultDomain, "nip"): {
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							DNSProvider: "aws",
						},
					},
				},
			}

			_, err := GetDefaultDomains(secrets)

			Expect(err).To(HaveOccurred())
		})
	})

	Describe("#GetInternalDomain", func() {
		It("should return the internal domain", func() {
			var (
				provider = "aws"
				domain   = "example.com"
				data     = map[string][]byte{
					"foo": []byte("bar"),
				}

				secret = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							DNSProvider: provider,
							DNSDomain:   domain,
						},
					},
					Data: data,
				}
				secrets = map[string]*corev1.Secret{
					constants.GardenRoleInternalDomain: secret,
				}
			)

			internalDomain, err := GetInternalDomain(secrets)

			Expect(err).NotTo(HaveOccurred())
			Expect(internalDomain).To(Equal(&Domain{
				Domain:     domain,
				Provider:   provider,
				SecretData: data,
			}))
		})

		It("should return an error due to incomplete secrets map", func() {
			_, err := GetInternalDomain(map[string]*corev1.Secret{})

			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error", func() {
			secrets := map[string]*corev1.Secret{
				constants.GardenRoleInternalDomain: {
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							DNSProvider: "aws",
						},
					},
				},
			}

			_, err := GetInternalDomain(secrets)

			Expect(err).To(HaveOccurred())
		})
	})

	var (
		defaultDomainProvider   = "default-domain-provider"
		defaultDomainSecretData = map[string][]byte{"default": []byte("domain")}
		defaultDomain           = &Domain{
			Domain:     "bar.com",
			Provider:   defaultDomainProvider,
			SecretData: defaultDomainSecretData,
		}
	)

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
			Expect(InjectGenericGardenKubeconfig(&corev1.Service{}, genericTokenKubeconfigSecretName, tokenSecretName)).To(MatchError(ContainSubstring("unhandled object type")))
		})

		It("should do nothing because a container already has the GARDEN_KUBECONFIG env var", func() {
			container := podSpec.Containers[1]
			container.Env = []corev1.EnvVar{{Name: "GARDEN_KUBECONFIG"}}
			podSpec.Containers[1] = container

			Expect(InjectGenericGardenKubeconfig(deployment, genericTokenKubeconfigSecretName, tokenSecretName)).To(Succeed())

			Expect(podSpec.Volumes).To(BeEmpty())
			Expect(podSpec.Containers[0].VolumeMounts).To(BeEmpty())
			Expect(podSpec.Containers[1].VolumeMounts).To(BeEmpty())
		})

		It("should inject the generic kubeconfig into the specified container", func() {
			Expect(InjectGenericGardenKubeconfig(deployment, genericTokenKubeconfigSecretName, tokenSecretName, containerName1)).To(Succeed())

			Expect(podSpec.Volumes).To(ContainElement(corev1.Volume{
				Name: "garden-kubeconfig",
				VolumeSource: corev1.VolumeSource{
					Projected: &corev1.ProjectedVolumeSource{
						DefaultMode: pointer.Int32(420),
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
			Expect(config.TLSClientConfig.CAData).To(BeEquivalentTo("ca2"))

			// everything else should be equal
			config.Host = baseConfig.Host
			config.TLSClientConfig.CAData = baseConfig.TLSClientConfig.CAData
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

	DescribeTable("#IsServedByGardenerAPIServer",
		func(resource string, expected bool) {
			Expect(IsServedByGardenerAPIServer(resource)).To(Equal(expected))
		},
		Entry("gardener core resource", gardencorev1beta1.Resource("shoots").String(), true),
		Entry("operations resource", operationsv1alpha1.Resource("bastions").String(), true),
		Entry("settings resource", settingsv1alpha1.Resource("openidconnectpresets").String(), true),
		Entry("seedmanagement resource", seedmanagementv1alpha1.Resource("managedseeds").String(), true),
		Entry("any other resource", "foo", false),
	)

	DescribeTable("#IsServedByKubeAPIServer",
		func(resource string, expected bool) {
			Expect(IsServedByKubeAPIServer(resource)).To(Equal(expected))
		},
		Entry("kubernetes core resource", corev1.Resource("secrets").String(), true),
		Entry("gardener core resource", gardencorev1beta1.Resource("shoots").String(), false),
		Entry("operations resource", operationsv1alpha1.Resource("bastions").String(), false),
		Entry("settings resource", settingsv1alpha1.Resource("openidconnectpresets").String(), false),
		Entry("seedmanagement resource", seedmanagementv1alpha1.Resource("managedseeds").String(), false),
		Entry("any other resource", "foo", true),
	)
})
