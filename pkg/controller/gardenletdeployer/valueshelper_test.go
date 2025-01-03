// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardenletdeployer

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("ValuesHelper", func() {
	var (
		imageVectorOverwritePath, componentImageVectorOverwritesPath string
		gardenKubeconfigPath, seedKubeconfigPath                     string

		cleanupFuncs []func()

		parentConfig *gardenletconfigv1alpha1.GardenletConfiguration

		vh ValuesHelper

		deployment      *seedmanagementv1alpha1.GardenletDeployment
		gardenletConfig *gardenletconfigv1alpha1.GardenletConfiguration

		mergedDeployment      *seedmanagementv1alpha1.GardenletDeployment
		mergedGardenletConfig func(bool) *gardenletconfigv1alpha1.GardenletConfiguration

		gardenletChartValues func(bool, string, int32, map[string]any) map[string]any
	)

	BeforeEach(func() {
		gardenletfeatures.RegisterFeatureGates()

		cleanupFuncs = []func(){
			test.WithTempFile("", "image-vector-overwrite", []byte("image vector overwrite"), &imageVectorOverwritePath),
			test.WithTempFile("", "component-image-vector-overwrites", []byte("component image vector overwrites"), &componentImageVectorOverwritesPath),
			test.WithTempFile("", "garden-kubeconfig", []byte("garden kubeconfig"), &gardenKubeconfigPath),
			test.WithTempFile("", "seed-kubeconfig", []byte("seed kubeconfig"), &seedKubeconfigPath),
			test.WithEnvVar(imagevector.OverrideEnv, imageVectorOverwritePath),
			test.WithEnvVar(imagevector.ComponentOverrideEnv, componentImageVectorOverwritesPath),
		}

		parentConfig = &gardenletconfigv1alpha1.GardenletConfiguration{
			GardenClientConnection: &gardenletconfigv1alpha1.GardenClientConnection{
				ClientConnectionConfiguration: componentbaseconfigv1alpha1.ClientConnectionConfiguration{
					Kubeconfig:         gardenKubeconfigPath,
					AcceptContentTypes: "application/json",
					ContentType:        "application/json",
					QPS:                100,
					Burst:              130,
				},
				BootstrapKubeconfig: &corev1.SecretReference{
					Name:      "gardenlet-kubeconfig-bootstrap",
					Namespace: v1beta1constants.GardenNamespace,
				},
			},
			SeedClientConnection: &gardenletconfigv1alpha1.SeedClientConnection{
				ClientConnectionConfiguration: componentbaseconfigv1alpha1.ClientConnectionConfiguration{
					Kubeconfig:         seedKubeconfigPath,
					AcceptContentTypes: "application/json",
					ContentType:        "application/json",
					QPS:                100,
					Burst:              130,
				},
			},
			Server: gardenletconfigv1alpha1.ServerConfiguration{
				HealthProbes: &gardenletconfigv1alpha1.Server{
					BindAddress: "0.0.0.0",
					Port:        2728,
				},
				Metrics: &gardenletconfigv1alpha1.Server{
					BindAddress: "0.0.0.0",
					Port:        2729,
				},
			},
			FeatureGates: map[string]bool{
				string("FooFeature"): true,
				string("BarFeature"): true,
			},
			Logging: &gardenletconfigv1alpha1.Logging{
				Enabled: ptr.To(true),
			},
			SeedConfig: &gardenletconfigv1alpha1.SeedConfig{
				SeedTemplate: gardencorev1beta1.SeedTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Name: "bar",
					},
				},
			},
		}

		vh = NewValuesHelper(parentConfig)

		deployment = &seedmanagementv1alpha1.GardenletDeployment{
			ReplicaCount:         ptr.To[int32](1),
			RevisionHistoryLimit: ptr.To[int32](1),
			Image: &seedmanagementv1alpha1.Image{
				PullPolicy: ptr.To(corev1.PullIfNotPresent),
			},
			PodAnnotations: map[string]string{
				"foo": "bar",
			},
		}
		gardenletConfig = &gardenletconfigv1alpha1.GardenletConfiguration{
			TypeMeta: metav1.TypeMeta{
				APIVersion: gardenletconfigv1alpha1.SchemeGroupVersion.String(),
				Kind:       "GardenletConfiguration",
			},
			FeatureGates: map[string]bool{
				"FooFeature": false,
			},
		}

		mergedDeployment = &seedmanagementv1alpha1.GardenletDeployment{
			ReplicaCount:         ptr.To[int32](1),
			RevisionHistoryLimit: ptr.To[int32](1),
			Image: &seedmanagementv1alpha1.Image{
				Repository: ptr.To("europe-docker.pkg.dev/gardener-project/releases/gardener/gardenlet"),
				Tag:        ptr.To("v0.0.0-master+$Format:%H$"),
				PullPolicy: ptr.To(corev1.PullIfNotPresent),
			},
			PodAnnotations: map[string]string{
				"foo": "bar",
			},
		}
		mergedGardenletConfig = func(withBootstrap bool) *gardenletconfigv1alpha1.GardenletConfiguration {
			var kubeconfigPath string
			var bootstrapKubeconfig, kubeconfigSecret *corev1.SecretReference
			if withBootstrap {
				bootstrapKubeconfig = &corev1.SecretReference{
					Name:      "gardenlet-kubeconfig-bootstrap",
					Namespace: v1beta1constants.GardenNamespace,
				}
				kubeconfigSecret = &corev1.SecretReference{
					Name:      "gardenlet-kubeconfig",
					Namespace: v1beta1constants.GardenNamespace,
				}
			} else {
				kubeconfigPath = gardenKubeconfigPath
			}
			return &gardenletconfigv1alpha1.GardenletConfiguration{
				TypeMeta: metav1.TypeMeta{
					APIVersion: gardenletconfigv1alpha1.SchemeGroupVersion.String(),
					Kind:       "GardenletConfiguration",
				},
				GardenClientConnection: &gardenletconfigv1alpha1.GardenClientConnection{
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
				SeedClientConnection: &gardenletconfigv1alpha1.SeedClientConnection{
					ClientConnectionConfiguration: componentbaseconfigv1alpha1.ClientConnectionConfiguration{
						AcceptContentTypes: "application/json",
						ContentType:        "application/json",
						QPS:                100,
						Burst:              130,
					},
				},
				Server: gardenletconfigv1alpha1.ServerConfiguration{
					HealthProbes: &gardenletconfigv1alpha1.Server{
						BindAddress: "0.0.0.0",
						Port:        2728,
					},
					Metrics: &gardenletconfigv1alpha1.Server{
						BindAddress: "0.0.0.0",
						Port:        2729,
					},
				},
				FeatureGates: map[string]bool{
					string("FooFeature"): false,
					string("BarFeature"): true,
				},
				Logging: &gardenletconfigv1alpha1.Logging{
					Enabled: ptr.To(true),
				},
			}
		}

		gardenletChartValues = func(withBootstrap bool, bk string, replicaCount int32, additionalValues map[string]any) map[string]any {
			var kubeconfig string
			if !withBootstrap {
				kubeconfig = "garden kubeconfig"
			}

			result := map[string]any{
				"replicaCount":         float64(replicaCount),
				"revisionHistoryLimit": float64(1),
				"image": map[string]any{
					"repository": "europe-docker.pkg.dev/gardener-project/releases/gardener/gardenlet",
					"tag":        "v0.0.0-master+$Format:%H$",
					"pullPolicy": "IfNotPresent",
				},
				"podAnnotations": map[string]any{
					"foo": "bar",
				},
				"imageVectorOverwrite":           "image vector overwrite",
				"componentImageVectorOverwrites": "component image vector overwrites",
				"config": map[string]any{
					"apiVersion": "gardenlet.config.gardener.cloud/v1alpha1",
					"kind":       "GardenletConfiguration",
					"gardenClientConnection": map[string]any{
						"kubeconfig":         kubeconfig,
						"acceptContentTypes": "application/json",
						"contentType":        "application/json",
						"qps":                float64(100),
						"burst":              float64(130),
					},
					"seedClientConnection": map[string]any{
						"kubeconfig":         "",
						"acceptContentTypes": "application/json",
						"contentType":        "application/json",
						"qps":                float64(100),
						"burst":              float64(130),
					},
					"server": map[string]any{
						"healthProbes": map[string]any{
							"bindAddress": "0.0.0.0",
							"port":        float64(2728),
						},
						"metrics": map[string]any{
							"bindAddress": "0.0.0.0",
							"port":        float64(2729),
						},
					},
					"featureGates": map[string]any{
						"FooFeature": false,
						"BarFeature": true,
					},
					"logging": map[string]any{
						"enabled": true,
					},
					"logLevel":  "",
					"logFormat": "",
				},
			}

			if withBootstrap {
				bootstrapKubeconfig := map[string]any{
					"name":       "gardenlet-kubeconfig-bootstrap",
					"namespace":  v1beta1constants.GardenNamespace,
					"kubeconfig": bk,
				}
				kubeconfigSecret := map[string]any{
					"name":      "gardenlet-kubeconfig",
					"namespace": v1beta1constants.GardenNamespace,
				}

				var err error
				result, err = utils.SetToValuesMap(result, bootstrapKubeconfig, "config", "gardenClientConnection", "bootstrapKubeconfig")
				Expect(err).ToNot(HaveOccurred())
				result, err = utils.SetToValuesMap(result, kubeconfigSecret, "config", "gardenClientConnection", "kubeconfigSecret")
				Expect(err).ToNot(HaveOccurred())
			}

			if additionalValues != nil {
				result = utils.MergeMaps(result, additionalValues)
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
			result, err := vh.MergeGardenletDeployment(deployment)
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
			Expect(result).To(Equal(gardenletChartValues(true, "bootstrap kubeconfig", 1, nil)))
		})

		It("should compute the correct gardenlet chart values without bootstrap", func() {
			result, err := vh.GetGardenletChartValues(mergedDeployment, mergedGardenletConfig(false), "")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(gardenletChartValues(false, "", 1, nil)))
		})
	})
})
