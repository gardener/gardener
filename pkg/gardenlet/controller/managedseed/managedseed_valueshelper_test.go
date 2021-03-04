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

package managedseed

import (
	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	configv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	"github.com/gardener/gardener/pkg/utils/test"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	componentbaseconfig "k8s.io/component-base/config"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"k8s.io/utils/pointer"
)

var _ = Describe("ValuesHelper", func() {
	var (
		imageVectorOverwrite, componentImageVectorOverwrites string

		imageVectorOverwritePath, componentImageVectorOverwritesPath string
		gardenKubeconfigPath, seedKubeconfigPath                     string
		serverCertPath, serverKeyPath                                string

		cleanupFuncs []func()

		parentConfig *config.GardenletConfiguration
		imageVector  imagevector.ImageVector

		vh ValuesHelper

		deployment      *seedmanagementv1alpha1.GardenletDeployment
		gardenletConfig *configv1alpha1.GardenletConfiguration
		shoot           *gardencorev1beta1.Shoot

		mergedDeployment      *seedmanagementv1alpha1.GardenletDeployment
		mergedGardenletConfig func(bool) *configv1alpha1.GardenletConfiguration

		gardenletChartValues func(bool, string) map[string]interface{}
	)

	BeforeEach(func() {
		gardenletfeatures.RegisterFeatureGates()

		imageVectorOverwrite = `images:
- name: test-image1
  sourceRepository: test-source-repo 
  repository: test-repo
  tag: test-tag
`
		componentImageVectorOverwrites = `components:
- name: test-component1
  imageVectorOverwrite: |
    images:
    - name: test-component-image1
      sourceRepository: test-source-repo
      repository: test-repo
      tag: test-tag
`
		cleanupFuncs = []func(){
			test.WithFeatureGate(gardenletfeatures.FeatureGate, features.APIServerSNI, true),
			test.WithTempFile("", "image-vector-overwrite", []byte(imageVectorOverwrite), &imageVectorOverwritePath),
			test.WithTempFile("", "component-image-vector-overwrites", []byte(componentImageVectorOverwrites), &componentImageVectorOverwritesPath),
			test.WithTempFile("", "garden-kubeconfig", []byte("garden kubeconfig"), &gardenKubeconfigPath),
			test.WithTempFile("", "seed-kubeconfig", []byte("seed kubeconfig"), &seedKubeconfigPath),
			test.WithTempFile("", "server-cert", []byte("server cert"), &serverCertPath),
			test.WithTempFile("", "server-key", []byte("server key"), &serverKeyPath),
			test.WithEnvVar(imagevector.OverrideEnv, imageVectorOverwritePath),
			test.WithEnvVar(imagevector.ComponentOverrideEnv, componentImageVectorOverwritesPath),
		}

		parentConfig = &config.GardenletConfiguration{
			GardenClientConnection: &config.GardenClientConnection{
				ClientConnectionConfiguration: componentbaseconfig.ClientConnectionConfiguration{
					Kubeconfig:         gardenKubeconfigPath,
					AcceptContentTypes: "application/json",
					ContentType:        "application/json",
					QPS:                100,
					Burst:              130,
				},
			},
			SeedClientConnection: &config.SeedClientConnection{
				ClientConnectionConfiguration: componentbaseconfig.ClientConnectionConfiguration{
					Kubeconfig:         seedKubeconfigPath,
					AcceptContentTypes: "application/json",
					ContentType:        "application/json",
					QPS:                100,
					Burst:              130,
				},
			},
			Server: &config.ServerConfiguration{
				HTTPS: config.HTTPSServer{
					Server: config.Server{
						BindAddress: "0.0.0.0",
						Port:        2720,
					},
					TLS: &config.TLSServer{
						ServerCertPath: serverCertPath,
						ServerKeyPath:  serverKeyPath,
					},
				},
			},
			FeatureGates: map[string]bool{
				string(features.Logging): true,
				string(features.HVPA):    true,
			},
			SeedConfig: &config.SeedConfig{
				SeedTemplate: gardencore.SeedTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Name: "bar",
					},
				},
			},
		}
		imageVector = []*imagevector.ImageSource{
			{
				Name:       "gardenlet",
				Repository: "test-repository",
				Tag:        pointer.StringPtr("test-tag"),
			},
		}

		vh = newValuesHelper(parentConfig, imageVector)

		deployment = &seedmanagementv1alpha1.GardenletDeployment{
			ReplicaCount:         pointer.Int32Ptr(1),
			RevisionHistoryLimit: pointer.Int32Ptr(1),
			Image: &seedmanagementv1alpha1.Image{
				PullPolicy: pullPolicyPtr(corev1.PullIfNotPresent),
			},
			PodAnnotations: map[string]string{
				"foo": "bar",
			},
			VPA: pointer.BoolPtr(true),
			ImageVectorOverwrite: gardencorev1beta1.ImageVector{
				{
					Name:       "test-image1",
					Repository: "my-repo",
					Tag:        pointer.StringPtr("my-tag"),
				},
			},
			ComponentImageVectorOverwrites: gardencorev1beta1.ComponentImageVectors{
				{
					Name: "test-component1",
					ImageVector: gardencorev1beta1.ImageVector{
						{
							Name:       "test-component-image1",
							Repository: "my-repo",
							Tag:        pointer.StringPtr("my-tag"),
						},
					},
				},
			},
		}
		gardenletConfig = &configv1alpha1.GardenletConfiguration{
			TypeMeta: metav1.TypeMeta{
				APIVersion: configv1alpha1.SchemeGroupVersion.String(),
				Kind:       "GardenletConfiguration",
			},
			FeatureGates: map[string]bool{
				string(features.Logging):              false,
				string(features.CachedRuntimeClients): true,
			},
		}
		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Status: gardencorev1beta1.ShootStatus{
				Gardener: gardencorev1beta1.Gardener{
					Version: "1.19.0",
				},
			},
		}

		mergedDeployment = &seedmanagementv1alpha1.GardenletDeployment{
			ReplicaCount:         pointer.Int32Ptr(1),
			RevisionHistoryLimit: pointer.Int32Ptr(1),
			Image: &seedmanagementv1alpha1.Image{
				Repository: pointer.StringPtr("test-repository"),
				Tag:        pointer.StringPtr("test-tag"),
				PullPolicy: pullPolicyPtr(corev1.PullIfNotPresent),
			},
			PodAnnotations: map[string]string{
				"foo": "bar",
				"networking.gardener.cloud/seed-sni-enabled": "true",
			},
			VPA: pointer.BoolPtr(true),
			ImageVectorOverwrite: gardencorev1beta1.ImageVector{
				{
					Name:       "test-image1",
					Repository: "my-repo",
					Tag:        pointer.StringPtr("my-tag"),
				},
			},
			ComponentImageVectorOverwrites: gardencorev1beta1.ComponentImageVectors{
				{
					Name: "test-component1",
					ImageVector: gardencorev1beta1.ImageVector{
						{
							Name:       "test-component-image1",
							Repository: "my-repo",
							Tag:        pointer.StringPtr("my-tag"),
						},
					},
				},
			},
		}
		mergedGardenletConfig = func(withBootstrap bool) *configv1alpha1.GardenletConfiguration {
			var kubeconfigPath string
			var bootstrapKubeconfig, kubeconfigSecret *corev1.SecretReference
			if withBootstrap {
				bootstrapKubeconfig = &corev1.SecretReference{
					Name:      gardenletKubeconfigBootstrapSecretName,
					Namespace: v1beta1constants.GardenNamespace,
				}
				kubeconfigSecret = &corev1.SecretReference{
					Name:      gardenletKubeconfigSecretName,
					Namespace: v1beta1constants.GardenNamespace,
				}
			} else {
				kubeconfigPath = gardenKubeconfigPath
			}
			return &configv1alpha1.GardenletConfiguration{
				TypeMeta: metav1.TypeMeta{
					APIVersion: configv1alpha1.SchemeGroupVersion.String(),
					Kind:       "GardenletConfiguration",
				},
				GardenClientConnection: &configv1alpha1.GardenClientConnection{
					ClientConnectionConfiguration: componentbaseconfigv1alpha1.ClientConnectionConfiguration{
						Kubeconfig:         kubeconfigPath,
						AcceptContentTypes: "application/json",
						ContentType:        "application/json",
						QPS:                100,
						Burst:              130,
					},
					BootstrapKubeconfig: bootstrapKubeconfig,
					KubeconfigSecret:    kubeconfigSecret,
				},
				SeedClientConnection: &configv1alpha1.SeedClientConnection{
					ClientConnectionConfiguration: componentbaseconfigv1alpha1.ClientConnectionConfiguration{
						AcceptContentTypes: "application/json",
						ContentType:        "application/json",
						QPS:                100,
						Burst:              130,
					},
				},
				Server: &configv1alpha1.ServerConfiguration{
					HTTPS: configv1alpha1.HTTPSServer{
						Server: configv1alpha1.Server{
							BindAddress: "0.0.0.0",
							Port:        2720,
						},
						TLS: &configv1alpha1.TLSServer{
							ServerCertPath: serverCertPath,
							ServerKeyPath:  serverKeyPath,
						},
					},
				},
				FeatureGates: map[string]bool{
					string(features.Logging):              false,
					string(features.HVPA):                 true,
					string(features.CachedRuntimeClients): true,
				},
			}
		}

		gardenletChartValues = func(withBootstrap bool, bk string) map[string]interface{} {
			var kubeconfig string
			if !withBootstrap {
				kubeconfig = "garden kubeconfig"
			}

			result := map[string]interface{}{
				"global": map[string]interface{}{
					"gardenlet": map[string]interface{}{
						"replicaCount":         float64(1),
						"revisionHistoryLimit": float64(1),
						"image": map[string]interface{}{
							"repository": "test-repository",
							"tag":        "test-tag",
							"pullPolicy": "IfNotPresent",
						},
						"podAnnotations": map[string]interface{}{
							"foo": "bar",
							"networking.gardener.cloud/seed-sni-enabled": "true",
						},
						"vpa": true,
						"imageVectorOverwrite": `images:
- name: test-image1
  repository: my-repo
  tag: my-tag
`,
						"componentImageVectorOverwrites": `components:
- name: test-component1
  imageVectorOverwrite: |
    images:
    - name: test-component-image1
      repository: my-repo
      tag: my-tag
`,
						"config": map[string]interface{}{
							"apiVersion": "gardenlet.config.gardener.cloud/v1alpha1",
							"kind":       "GardenletConfiguration",
							"gardenClientConnection": map[string]interface{}{
								"kubeconfig":         kubeconfig,
								"acceptContentTypes": "application/json",
								"contentType":        "application/json",
								"qps":                float64(100),
								"burst":              float64(130),
							},
							"seedClientConnection": map[string]interface{}{
								"kubeconfig":         "",
								"acceptContentTypes": "application/json",
								"contentType":        "application/json",
								"qps":                float64(100),
								"burst":              float64(130),
							},
							"server": map[string]interface{}{
								"https": map[string]interface{}{
									"tls": map[string]interface{}{
										"crt": "server cert",
										"key": "server key",
									},
									"bindAddress": "0.0.0.0",
									"port":        float64(2720),
								},
							},
							"featureGates": map[string]interface{}{
								"Logging":              false,
								"HVPA":                 true,
								"CachedRuntimeClients": true,
							},
						},
					},
				},
			}

			if withBootstrap {
				bootstrapKubeconfig := map[string]interface{}{
					"name":       gardenletKubeconfigBootstrapSecretName,
					"namespace":  v1beta1constants.GardenNamespace,
					"kubeconfig": bk,
				}
				kubeconfigSecret := map[string]interface{}{
					"name":      gardenletKubeconfigSecretName,
					"namespace": v1beta1constants.GardenNamespace,
				}

				var err error
				result, err = utils.SetToValuesMap(result, bootstrapKubeconfig, "global", "gardenlet", "config", "gardenClientConnection", "bootstrapKubeconfig")
				Expect(err).ToNot(HaveOccurred())
				result, err = utils.SetToValuesMap(result, kubeconfigSecret, "global", "gardenlet", "config", "gardenClientConnection", "kubeconfigSecret")
				Expect(err).ToNot(HaveOccurred())
			}
			return result
		}
	})

	AfterEach(func() {
		for _, f := range cleanupFuncs {
			f()
		}
	})

	Describe("#MergeGardenletDeployment", func() {
		It("should merge the deployment with the values from the parent gardenlet", func() {
			result, err := vh.MergeGardenletDeployment(deployment, shoot)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(mergedDeployment))
		})
	})

	Describe("#MergeGardenletConfiguration", func() {
		It("should merge the gardenlet config with the parent gardenlet config", func() {
			result, err := vh.MergeGardenletConfiguration(gardenletConfig)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(mergedGardenletConfig(false)))
		})
	})

	Describe("#GetGardenletChartValues", func() {
		It("should compute the correct gardenlet chart values with bootstrap", func() {
			result, err := vh.GetGardenletChartValues(mergedDeployment, mergedGardenletConfig(true), "bootstrap kubeconfig")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(gardenletChartValues(true, "bootstrap kubeconfig")))
		})

		It("should compute the correct gardenlet chart values without bootstrap", func() {
			result, err := vh.GetGardenletChartValues(mergedDeployment, mergedGardenletConfig(false), "")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(gardenletChartValues(false, "")))
		})
	})

	Describe("#getParentPodAnnotations", func() {
		DescribeTable("seed-sni-enabled annotation", func(enabled bool, version string, added bool) {
			Expect(gardenletfeatures.FeatureGate.SetFromMap(map[string]bool{"APIServerSNI": enabled})).To(Succeed())

			shoot := &gardencorev1beta1.Shoot{
				Status: gardencorev1beta1.ShootStatus{
					Gardener: gardencorev1beta1.Gardener{
						Version: version,
					},
				},
			}

			actualAnnotations := getParentPodAnnotations(shoot)

			if added {
				Expect(actualAnnotations).To(HaveKeyWithValue("networking.gardener.cloud/seed-sni-enabled", "true"))
				Expect(actualAnnotations).To(HaveLen(1))
			} else {
				Expect(actualAnnotations).To(BeEmpty())
			}
		},
			Entry("should be added for SNI enabled and release 1.14.1", true, "1.14.1", true),
			Entry("should be added for SNI enabled and pre-release 1.14", true, "1.14-dev", true),
			Entry("should be added for SNI enabled and pre-release 1.14.0", true, "1.14.0-dev", true),
			Entry("should be added for SNI enabled and release 1.13.3", true, "1.13.3", true),
			Entry("should be added for SNI enabled and pre-release 1.13", true, "1.13-dev", true),
			Entry("should be added for SNI enabled and pre-release 1.13.0", true, "1.13.0-dev", true),
			Entry("should not be added for SNI enabled and release 1.12.8", true, "1.12.8", false),
			Entry("should not be added for SNI enabled and unparsable version", true, "not a semver", false),

			Entry("should not be added for SNI disabled and release 1.14.1", false, "1.14.1", false),
			Entry("should not be added for SNI disabled and pre-release 1.14", false, "1.14-dev", false),
			Entry("should not be added for SNI disabled and pre-release 1.14.0", false, "1.14.0-dev", false),
			Entry("should not be added for SNI disabled and release 1.13.3", false, "1.13.3", false),
			Entry("should not be added for SNI disabled and pre-release 1.13", false, "1.13-dev", false),
			Entry("should not be added for SNI disabled and pre-release 1.13.0", false, "1.13.0-dev", false),
			Entry("should not be added for SNI disabled and release 1.12.8", false, "1.12.8", false),
			Entry("should not be added for SNI disabled and unparsable version", false, "not a semver", false),
		)
	})
})
