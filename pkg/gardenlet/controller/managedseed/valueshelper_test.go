// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	componentbaseconfig "k8s.io/component-base/config"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"k8s.io/utils/pointer"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	gardenletv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
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

		parentConfig *config.GardenletConfiguration
		imageVector  imagevector.ImageVector

		vh ValuesHelper

		deployment      *seedmanagementv1alpha1.GardenletDeployment
		gardenletConfig *gardenletv1alpha1.GardenletConfiguration
		shoot           *gardencorev1beta1.Shoot

		mergedDeployment      *seedmanagementv1alpha1.GardenletDeployment
		mergedGardenletConfig func(bool) *gardenletv1alpha1.GardenletConfiguration

		gardenletChartValues func(bool, string, int32, map[string]interface{}) map[string]interface{}
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

		parentConfig = &config.GardenletConfiguration{
			GardenClientConnection: &config.GardenClientConnection{
				ClientConnectionConfiguration: componentbaseconfig.ClientConnectionConfiguration{
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
			SeedClientConnection: &config.SeedClientConnection{
				ClientConnectionConfiguration: componentbaseconfig.ClientConnectionConfiguration{
					Kubeconfig:         seedKubeconfigPath,
					AcceptContentTypes: "application/json",
					ContentType:        "application/json",
					QPS:                100,
					Burst:              130,
				},
			},
			Server: config.ServerConfiguration{
				HealthProbes: &config.Server{
					BindAddress: "0.0.0.0",
					Port:        2728,
				},
				Metrics: &config.Server{
					BindAddress: "0.0.0.0",
					Port:        2729,
				},
			},
			FeatureGates: map[string]bool{
				string("FooFeature"): true,
				string("BarFeature"): true,
			},
			Logging: &config.Logging{
				Enabled: pointer.Bool(true),
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
				Tag:        pointer.String("test-tag"),
			},
		}

		vh = NewValuesHelper(parentConfig, imageVector)

		deployment = &seedmanagementv1alpha1.GardenletDeployment{
			ReplicaCount:         pointer.Int32(1),
			RevisionHistoryLimit: pointer.Int32(1),
			Image: &seedmanagementv1alpha1.Image{
				PullPolicy: pullPolicyPtr(corev1.PullIfNotPresent),
			},
			PodAnnotations: map[string]string{
				"foo": "bar",
			},
			VPA: pointer.Bool(true),
		}
		gardenletConfig = &gardenletv1alpha1.GardenletConfiguration{
			TypeMeta: metav1.TypeMeta{
				APIVersion: gardenletv1alpha1.SchemeGroupVersion.String(),
				Kind:       "GardenletConfiguration",
			},
			FeatureGates: map[string]bool{
				"FooFeature": false,
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
			ReplicaCount:         pointer.Int32(1),
			RevisionHistoryLimit: pointer.Int32(1),
			Image: &seedmanagementv1alpha1.Image{
				Repository: pointer.String("test-repository"),
				Tag:        pointer.String("test-tag"),
				PullPolicy: pullPolicyPtr(corev1.PullIfNotPresent),
			},
			PodAnnotations: map[string]string{
				"foo": "bar",
			},
			VPA: pointer.Bool(true),
		}
		mergedGardenletConfig = func(withBootstrap bool) *gardenletv1alpha1.GardenletConfiguration {
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
			return &gardenletv1alpha1.GardenletConfiguration{
				TypeMeta: metav1.TypeMeta{
					APIVersion: gardenletv1alpha1.SchemeGroupVersion.String(),
					Kind:       "GardenletConfiguration",
				},
				GardenClientConnection: &gardenletv1alpha1.GardenClientConnection{
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
				SeedClientConnection: &gardenletv1alpha1.SeedClientConnection{
					ClientConnectionConfiguration: componentbaseconfigv1alpha1.ClientConnectionConfiguration{
						AcceptContentTypes: "application/json",
						ContentType:        "application/json",
						QPS:                100,
						Burst:              130,
					},
				},
				Server: gardenletv1alpha1.ServerConfiguration{
					HealthProbes: &gardenletv1alpha1.Server{
						BindAddress: "0.0.0.0",
						Port:        2728,
					},
					Metrics: &gardenletv1alpha1.Server{
						BindAddress: "0.0.0.0",
						Port:        2729,
					},
				},
				FeatureGates: map[string]bool{
					string("FooFeature"): false,
					string("BarFeature"): true,
				},
				Logging: &gardenletv1alpha1.Logging{
					Enabled: pointer.Bool(true),
				},
			}
		}

		gardenletChartValues = func(withBootstrap bool, bk string, replicaCount int32, additionalValues map[string]interface{}) map[string]interface{} {
			var kubeconfig string
			if !withBootstrap {
				kubeconfig = "garden kubeconfig"
			}

			result := map[string]interface{}{
				"replicaCount":         float64(replicaCount),
				"revisionHistoryLimit": float64(1),
				"image": map[string]interface{}{
					"repository": "test-repository",
					"tag":        "test-tag",
					"pullPolicy": "IfNotPresent",
				},
				"podAnnotations": map[string]interface{}{
					"foo": "bar",
				},
				"vpa":                            true,
				"imageVectorOverwrite":           "image vector overwrite",
				"componentImageVectorOverwrites": "component image vector overwrites",
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
						"healthProbes": map[string]interface{}{
							"bindAddress": "0.0.0.0",
							"port":        float64(2728),
						},
						"metrics": map[string]interface{}{
							"bindAddress": "0.0.0.0",
							"port":        float64(2729),
						},
					},
					"featureGates": map[string]interface{}{
						"FooFeature": false,
						"BarFeature": true,
					},
					"logging": map[string]interface{}{
						"enabled": true,
					},
					"logLevel":  "",
					"logFormat": "",
				},
			}

			if withBootstrap {
				bootstrapKubeconfig := map[string]interface{}{
					"name":       "gardenlet-kubeconfig-bootstrap",
					"namespace":  v1beta1constants.GardenNamespace,
					"kubeconfig": bk,
				}
				kubeconfigSecret := map[string]interface{}{
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
			Expect(result).To(Equal(gardenletChartValues(true, "bootstrap kubeconfig", 1, nil)))
		})

		It("should compute the correct gardenlet chart values without bootstrap", func() {
			result, err := vh.GetGardenletChartValues(mergedDeployment, mergedGardenletConfig(false), "")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(gardenletChartValues(false, "", 1, nil)))
		})
	})
})
