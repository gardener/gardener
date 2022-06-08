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

package charttest

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/version"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/seedmanagement"
	cr "github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/mock"
	"github.com/gardener/gardener/pkg/features"
	gardenletconfig "github.com/gardener/gardener/pkg/gardenlet/apis/config"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/utils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
)

var (
	expectedLabels = map[string]string{
		"app":      "gardener",
		"role":     "gardenlet",
		"chart":    "runtime-0.1.0",
		"release":  "gardenlet",
		"heritage": "Tiller",
	}
	expectedLabelsWithCollectableReference = utils.MergeStringMaps(expectedLabels, map[string]string{
		"resources.gardener.cloud/garbage-collectable-reference": "true",
	})
)

var _ = Describe("#Gardenlet Chart Test", func() {
	var (
		ctx              context.Context
		c                client.Client
		deployer         component.Deployer
		chartApplier     kubernetes.ChartApplier
		universalDecoder runtime.Decoder
	)

	BeforeEach(func() {
		ctx = context.TODO()

		s := runtime.NewScheme()
		// for gardenletconfig map
		Expect(corev1.AddToScheme(s)).NotTo(HaveOccurred())
		// for deployment
		Expect(appsv1.AddToScheme(s)).NotTo(HaveOccurred())
		// for unmarshal of GardenletConfiguration
		Expect(gardenletconfig.AddToScheme(s)).NotTo(HaveOccurred())
		Expect(gardenletconfigv1alpha1.AddToScheme(s)).NotTo(HaveOccurred())
		// for priority class
		Expect(schedulingv1.AddToScheme(s)).NotTo(HaveOccurred())
		// for ClusterRole and ClusterRoleBinding
		Expect(rbacv1.AddToScheme(s)).NotTo(HaveOccurred())
		// for deletion of PDB
		Expect(policyv1beta1.AddToScheme(s)).NotTo(HaveOccurred())
		// for vpa
		Expect(vpaautoscalingv1.AddToScheme(s)).NotTo(HaveOccurred())

		// create decoder for unmarshalling the GardenletConfiguration from the component gardenletconfig Config Map
		codecs := serializer.NewCodecFactory(s)
		universalDecoder = codecs.UniversalDecoder()

		// fake client to use for the chart applier
		c = fake.NewClientBuilder().WithScheme(s).Build()

		mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion, appsv1.SchemeGroupVersion})

		mapper.Add(appsv1.SchemeGroupVersion.WithKind("Deployment"), meta.RESTScopeNamespace)
		mapper.Add(corev1.SchemeGroupVersion.WithKind("ConfigMap"), meta.RESTScopeNamespace)
		mapper.Add(vpaautoscalingv1.SchemeGroupVersion.WithKind("VerticalPodAutoscaler"), meta.RESTScopeNamespace)
		mapper.Add(schedulingv1.SchemeGroupVersion.WithKind("PriorityClass"), meta.RESTScopeRoot)
		mapper.Add(rbacv1.SchemeGroupVersion.WithKind("ClusterRole"), meta.RESTScopeRoot)
		mapper.Add(rbacv1.SchemeGroupVersion.WithKind("ClusterRoleBinding"), meta.RESTScopeRoot)

		// set the git version required for rendering of the Gardenlet chart -  chart helpers determine resource API versions based on that
		renderer := cr.NewWithServerVersion(&version.Info{
			GitVersion: "1.14.0",
		})

		chartApplier = kubernetes.NewChartApplier(renderer, kubernetes.NewApplier(c, mapper))
		Expect(chartApplier).NotTo(BeNil(), "should return chart applier")
	})

	Describe("Destroy Gardenlet Resources", func() {
		It("should delete all resources", func() {
			ctrl := gomock.NewController(GinkgoT())
			defer ctrl.Finish()

			mockChartApplier := mock.NewMockChartApplier(ctrl)

			mockChartApplier.EXPECT().Delete(ctx, filepath.Join(chartsRootPath, "gardener", "gardenlet"), "garden", "gardenlet", kubernetes.Values(map[string]interface{}{}))

			deployer = NewGardenletChartApplier(mockChartApplier, map[string]interface{}{}, chartsRootPath)
			Expect(deployer.Destroy(ctx)).ToNot(HaveOccurred(), "Destroy Gardenlet resources succeeds")
		})
	})

	DescribeTable("#DeployGardenletChart",
		func(
			componentConfigTlsServerContentCert *string,
			componentConfigTlsServerContentKey *string,
			gardenClientConnectionKubeconfig *string,
			seedClientConnectionKubeconfig *string,
			bootstrapKubeconfig *corev1.SecretReference,
			bootstrapKubeconfigSecret *corev1.SecretReference,
			bootstrapKubeconfigContent *string,
			seedConfig *gardenletconfigv1alpha1.SeedConfig,
			deploymentConfiguration *seedmanagement.GardenletDeployment,
			imageVectorOverwrite *string,
			componentImageVectorOverwrites *string,
			featureGates map[string]bool,
			cmAndSecretNameToUniqueName map[string]string,
		) {
			gardenletValues := map[string]interface{}{
				"enabled": true,
			}

			componentConfigValues := map[string]interface{}{}

			componentConfigUsesTlsServerConfig := componentConfigTlsServerContentCert != nil && componentConfigTlsServerContentKey != nil
			if componentConfigUsesTlsServerConfig {
				componentConfigValues["server"] = map[string]interface{}{
					"https": map[string]interface{}{
						"tls": map[string]interface{}{
							"crt": *componentConfigTlsServerContentCert,
							"key": *componentConfigTlsServerContentKey,
						},
					},
				}
			}

			if gardenClientConnectionKubeconfig != nil {
				componentConfigValues["gardenClientConnection"] = map[string]interface{}{
					"kubeconfig": *gardenClientConnectionKubeconfig,
				}
			}

			if seedClientConnectionKubeconfig != nil {
				componentConfigValues["seedClientConnection"] = map[string]interface{}{
					"kubeconfig": *seedClientConnectionKubeconfig,
				}
			}

			// bootstrap configurations are tested in one test-case
			usesTLSBootstrapping := bootstrapKubeconfigContent != nil && bootstrapKubeconfig != nil && bootstrapKubeconfigSecret != nil
			if usesTLSBootstrapping {
				componentConfigValues["gardenClientConnection"] = map[string]interface{}{
					"bootstrapKubeconfig": map[string]interface{}{
						"name":       bootstrapKubeconfig.Name,
						"namespace":  bootstrapKubeconfig.Namespace,
						"kubeconfig": *bootstrapKubeconfigContent,
					},
					"kubeconfigSecret": map[string]interface{}{
						"name":      bootstrapKubeconfigSecret.Name,
						"namespace": bootstrapKubeconfigSecret.Namespace,
					},
				}
			}

			if seedConfig != nil {
				componentConfigValues["seedConfig"] = *seedConfig
			}

			if featureGates != nil {
				componentConfigValues["featureGates"] = featureGates
			}

			if len(componentConfigValues) > 0 {
				gardenletValues["config"] = componentConfigValues
			}

			if deploymentConfiguration == nil {
				deploymentConfiguration = &seedmanagement.GardenletDeployment{}
			}

			image := seedmanagement.Image{
				Repository: pointer.String("eu.gcr.io/gardener-project/gardener/gardenlet"),
				Tag:        pointer.String("latest"),
			}

			if deploymentConfiguration.ReplicaCount != nil {
				gardenletValues["replicaCount"] = *deploymentConfiguration.ReplicaCount
			}

			if deploymentConfiguration.ServiceAccountName != nil {
				gardenletValues["serviceAccountName"] = *deploymentConfiguration.ServiceAccountName
			}

			if deploymentConfiguration.RevisionHistoryLimit != nil {
				gardenletValues["revisionHistoryLimit"] = *deploymentConfiguration.RevisionHistoryLimit
			}

			if imageVectorOverwrite != nil {
				gardenletValues["imageVectorOverwrite"] = *imageVectorOverwrite
			}

			if componentImageVectorOverwrites != nil {
				gardenletValues["componentImageVectorOverwrites"] = *componentImageVectorOverwrites
			}

			if deploymentConfiguration.Resources != nil {
				gardenletValues["resources"] = *deploymentConfiguration.Resources
			}

			if deploymentConfiguration.PodLabels != nil {
				gardenletValues["podLabels"] = deploymentConfiguration.PodLabels
			}

			if deploymentConfiguration.PodAnnotations != nil {
				gardenletValues["podAnnotations"] = deploymentConfiguration.PodAnnotations
			}

			if deploymentConfiguration.AdditionalVolumeMounts != nil {
				gardenletValues["additionalVolumeMounts"] = deploymentConfiguration.AdditionalVolumeMounts
			}

			if deploymentConfiguration.AdditionalVolumes != nil {
				gardenletValues["additionalVolumes"] = deploymentConfiguration.AdditionalVolumes
			}

			if deploymentConfiguration.Env != nil {
				gardenletValues["env"] = deploymentConfiguration.Env
			}

			if deploymentConfiguration.VPA != nil {
				gardenletValues["vpa"] = *deploymentConfiguration.VPA
			}

			deployer = NewGardenletChartApplier(chartApplier, map[string]interface{}{
				"global": map[string]interface{}{
					"gardenlet": gardenletValues,
				},
			}, chartsRootPath)

			Expect(deployer.Deploy(ctx)).ToNot(HaveOccurred(), "Gardenlet chart deployment succeeds")

			ValidateGardenletChartPriorityClass(ctx, c)

			serviceAccountName := "gardenlet"
			if deploymentConfiguration.ServiceAccountName != nil {
				serviceAccountName = *deploymentConfiguration.ServiceAccountName
			}

			ValidateGardenletChartRBAC(ctx, c, expectedLabels, serviceAccountName, featureGates)

			ValidateGardenletChartServiceAccount(ctx, c, seedClientConnectionKubeconfig != nil, expectedLabels, serviceAccountName)

			expectedGardenletConfig := ComputeExpectedGardenletConfiguration(
				componentConfigUsesTlsServerConfig,
				gardenClientConnectionKubeconfig != nil,
				seedClientConnectionKubeconfig != nil,
				bootstrapKubeconfig,
				bootstrapKubeconfigSecret,
				seedConfig,
				featureGates)

			VerifyGardenletComponentConfigConfigMap(ctx,
				c,
				universalDecoder,
				expectedGardenletConfig,
				utils.MergeStringMaps(expectedLabels, map[string]string{
					"resources.gardener.cloud/garbage-collectable-reference": "true",
				}),
				cmAndSecretNameToUniqueName["gardenlet-configmap"],
			)

			expectedGardenletDeploymentSpec, err := ComputeExpectedGardenletDeploymentSpec(
				deploymentConfiguration,
				image,
				componentConfigUsesTlsServerConfig,
				gardenClientConnectionKubeconfig,
				seedClientConnectionKubeconfig,
				expectedLabels,
				imageVectorOverwrite,
				componentImageVectorOverwrites,
				cmAndSecretNameToUniqueName,
			)

			Expect(err).ToNot(HaveOccurred())

			VerifyGardenletDeployment(ctx,
				c,
				expectedGardenletDeploymentSpec,
				deploymentConfiguration,
				componentConfigUsesTlsServerConfig,
				gardenClientConnectionKubeconfig != nil,
				seedClientConnectionKubeconfig != nil,
				usesTLSBootstrapping,
				expectedLabels,
				imageVectorOverwrite,
				componentImageVectorOverwrites,
				cmAndSecretNameToUniqueName,
			)

			if imageVectorOverwrite != nil {
				cm := getEmptyImageVectorOverwriteConfigMap()
				validateImageVectorOverwriteConfigMap(ctx, c, cm, "images_overwrite.yaml", imageVectorOverwrite, cmAndSecretNameToUniqueName["gardenlet-imagevector-overwrite"])
			}

			if componentImageVectorOverwrites != nil {
				cm := getEmptyImageVectorOverwriteComponentsConfigMap()
				validateImageVectorOverwriteConfigMap(ctx, c, cm, "components.yaml", componentImageVectorOverwrites, cmAndSecretNameToUniqueName["gardenlet-imagevector-overwrite-components"])
			}

			if componentConfigUsesTlsServerConfig {
				validateCertSecret(ctx, c, componentConfigTlsServerContentCert, componentConfigTlsServerContentKey, cmAndSecretNameToUniqueName["gardenlet-cert"])
			}

			if gardenClientConnectionKubeconfig != nil {
				secret := getEmptyKubeconfigGardenSecret()
				validateKubeconfigSecret(ctx, c, secret, gardenClientConnectionKubeconfig, expectedLabelsWithCollectableReference, cmAndSecretNameToUniqueName["gardenlet-kubeconfig-garden"])
			}

			if seedClientConnectionKubeconfig != nil {
				secret := getEmptyKubeconfigSeedSecret()
				validateKubeconfigSecret(ctx, c, secret, seedClientConnectionKubeconfig, expectedLabelsWithCollectableReference, cmAndSecretNameToUniqueName["gardenlet-kubeconfig-seed"])
			}

			if bootstrapKubeconfigContent != nil {
				secret := getEmptyKubeconfigGardenBootstrapSecret()
				validateKubeconfigSecret(ctx, c, secret, bootstrapKubeconfigContent, expectedLabels, "gardenlet-kubeconfig-bootstrap")
			}

			if deploymentConfiguration != nil && deploymentConfiguration.VPA != nil && *deploymentConfiguration.VPA {
				ValidateGardenletChartVPA(ctx, c)
			}
		},
		Entry("verify the default values for the Gardenlet chart & the Gardenlet component config", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, map[string]string{"gardenlet-configmap": "gardenlet-configmap-11b5c474"}),
		Entry("verify Gardenlet with component config with TLS server configuration", pointer.String("dummy cert content"), pointer.String("dummy key content"), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, map[string]string{
			"gardenlet-configmap": "gardenlet-configmap-3a989e8d",
			"gardenlet-cert":      "gardenlet-cert-4a9a9f55",
		}),
		Entry("verify Gardenlet with component config having the Garden client connection kubeconfig set", nil, nil, pointer.String("dummy garden kubeconfig"), nil, nil, nil, nil, nil, nil, nil, nil, nil, map[string]string{
			"gardenlet-configmap":         "gardenlet-configmap-0701c430",
			"gardenlet-kubeconfig-garden": "gardenlet-kubeconfig-garden-8c9ae097",
		}),
		Entry("verify Gardenlet with component config having the Seed client connection kubeconfig set", nil, nil, nil, pointer.String("dummy seed kubeconfig"), nil, nil, nil, nil, nil, nil, nil, nil, map[string]string{
			"gardenlet-configmap":       "gardenlet-configmap-814ecbd5",
			"gardenlet-kubeconfig-seed": "gardenlet-kubeconfig-seed-662d92ae",
		}),
		Entry("verify Gardenlet with component config having a Bootstrap kubeconfig set", nil, nil, nil, nil, &corev1.SecretReference{
			Name:      "gardenlet-kubeconfig-bootstrap",
			Namespace: "garden",
		}, &corev1.SecretReference{
			Name:      "gardenlet-kubeconfig",
			Namespace: gardencorev1beta1constants.GardenNamespace,
		}, pointer.String("dummy bootstrap kubeconfig"), nil, nil, nil, nil, nil, map[string]string{
			"gardenlet-configmap": "gardenlet-configmap-71aa4ff5",
		}),
		Entry("verify that the SeedConfig is set in the component config Config Map", nil, nil, nil, nil, nil, nil, nil,
			&gardenletconfigv1alpha1.SeedConfig{
				SeedTemplate: gardencorev1beta1.SeedTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Name: "sweet-seed",
					},
				},
			}, nil, nil, nil, nil, map[string]string{"gardenlet-configmap": "gardenlet-configmap-c4a9a671"}),
		Entry("verify deployment with image vector override", nil, nil, nil, nil, nil, nil, nil, nil, nil, pointer.String("dummy-override-content"), nil, nil, map[string]string{
			"gardenlet-configmap":             "gardenlet-configmap-11b5c474",
			"gardenlet-imagevector-overwrite": "gardenlet-imagevector-overwrite-32ecb769",
		}),
		Entry("verify deployment with component image vector override", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, pointer.String("dummy-override-content"), nil, map[string]string{
			"gardenlet-configmap":                        "gardenlet-configmap-11b5c474",
			"gardenlet-imagevector-overwrite-components": "gardenlet-imagevector-overwrite-components-53f94952",
		}),

		Entry("verify deployment with replica count", nil, nil, nil, nil, nil, nil, nil, nil, &seedmanagement.GardenletDeployment{
			ReplicaCount: pointer.Int32(2),
		}, nil, nil, nil, map[string]string{"gardenlet-configmap": "gardenlet-configmap-11b5c474"}),

		Entry("verify deployment with service account", nil, nil, nil, nil, nil, nil, nil, nil, &seedmanagement.GardenletDeployment{
			ServiceAccountName: pointer.String("ax"),
		}, nil, nil, nil, map[string]string{"gardenlet-configmap": "gardenlet-configmap-11b5c474"}),

		Entry("verify deployment with resources", nil, nil, nil, nil, nil, nil, nil, nil, &seedmanagement.GardenletDeployment{
			Resources: &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("800m"),
					corev1.ResourceMemory: resource.MustParse("15Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("25Mi"),
				},
			},
		}, nil, nil, nil, map[string]string{"gardenlet-configmap": "gardenlet-configmap-11b5c474"}),

		Entry("verify deployment with pod labels", nil, nil, nil, nil, nil, nil, nil, nil, &seedmanagement.GardenletDeployment{
			PodLabels: map[string]string{
				"x": "y",
			},
		}, nil, nil, nil, map[string]string{"gardenlet-configmap": "gardenlet-configmap-11b5c474"}),

		Entry("verify deployment with pod annotations", nil, nil, nil, nil, nil, nil, nil, nil, &seedmanagement.GardenletDeployment{
			PodAnnotations: map[string]string{
				"x": "y",
			},
		}, nil, nil, nil, map[string]string{"gardenlet-configmap": "gardenlet-configmap-11b5c474"}),

		Entry("verify deployment with additional volumes", nil, nil, nil, nil, nil, nil, nil, nil, &seedmanagement.GardenletDeployment{
			AdditionalVolumes: []corev1.Volume{
				{
					Name:         "a",
					VolumeSource: corev1.VolumeSource{},
				},
			},
		}, nil, nil, nil, map[string]string{"gardenlet-configmap": "gardenlet-configmap-11b5c474"}),

		Entry("verify deployment with additional volume mounts", nil, nil, nil, nil, nil, nil, nil, nil, &seedmanagement.GardenletDeployment{
			AdditionalVolumeMounts: []corev1.VolumeMount{
				{
					Name: "a",
				},
			},
		}, nil, nil, nil, map[string]string{"gardenlet-configmap": "gardenlet-configmap-11b5c474"}),

		Entry("verify deployment with env variables", nil, nil, nil, nil, nil, nil, nil, nil, &seedmanagement.GardenletDeployment{
			Env: []corev1.EnvVar{
				{
					Name:  "KUBECONFIG",
					Value: "XY",
				},
			},
		}, nil, nil, nil, map[string]string{"gardenlet-configmap": "gardenlet-configmap-11b5c474"}),

		Entry("verify deployment with VPA enabled", nil, nil, nil, nil, nil, nil, nil, nil, &seedmanagement.GardenletDeployment{
			VPA: pointer.Bool(true),
		}, nil, nil, nil, map[string]string{"gardenlet-configmap": "gardenlet-configmap-11b5c474"}),
		Entry("verify Gardenlet RBACs when ManagedIstio is enabled", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, map[string]bool{string(features.ManagedIstio): true}, map[string]string{"gardenlet-configmap": "gardenlet-configmap-6d53f972"}),
		Entry("verify Gardenlet RBACs when APIServerSNI is enabled", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, map[string]bool{string(features.APIServerSNI): true}, map[string]string{"gardenlet-configmap": "gardenlet-configmap-fba13c38"}),
		Entry("verify Gardenlet RBACs when APIServerSNI is disabled", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, map[string]bool{string(features.APIServerSNI): false}, map[string]string{"gardenlet-configmap": "gardenlet-configmap-5dc34cd2"}),
	)
})

func validateKubeconfigSecret(ctx context.Context, c client.Client, secret *corev1.Secret, kubeconfig *string, expectedLabels map[string]string, name string) {
	expectedSecret := *secret
	expectedSecret.Labels = expectedLabels
	expectedSecret.Type = corev1.SecretTypeOpaque
	expectedSecret.Data = map[string][]byte{
		"kubeconfig": []byte(*kubeconfig),
	}

	Expect(c.Get(
		ctx,
		kutil.Key(secret.Namespace, name),
		secret,
	)).ToNot(HaveOccurred())
	Expect(secret.Labels).To(Equal(expectedSecret.Labels))
	Expect(secret.Data).To(Equal(expectedSecret.Data))
	Expect(secret.Type).To(Equal(expectedSecret.Type))
}

func validateCertSecret(ctx context.Context, c client.Client, cert *string, key *string, uniqueName string) {
	secret := getEmptyCertSecret()
	expectedSecret := getEmptyCertSecret()
	expectedSecret.Labels = expectedLabelsWithCollectableReference
	expectedSecret.Type = corev1.SecretTypeOpaque
	expectedSecret.Data = map[string][]byte{
		"gardenlet.crt": []byte(*cert),
		"gardenlet.key": []byte(*key),
	}

	Expect(c.Get(
		ctx,
		kutil.Key(secret.Namespace, uniqueName),
		secret,
	)).ToNot(HaveOccurred())
	Expect(secret.Labels).To(Equal(expectedSecret.Labels))
	Expect(secret.Data).To(Equal(expectedSecret.Data))
	Expect(secret.Type).To(Equal(expectedSecret.Type))
}

func validateImageVectorOverwriteConfigMap(ctx context.Context, c client.Client, cm *corev1.ConfigMap, cmKey string, content *string, uniqueName string) {
	expectedCm := *cm
	expectedCm.Labels = expectedLabelsWithCollectableReference
	expectedCm.Data = map[string]string{
		cmKey: fmt.Sprintf("%s\n", *content),
	}

	Expect(c.Get(
		ctx,
		kutil.Key(cm.Namespace, uniqueName),
		cm,
	)).ToNot(HaveOccurred())

	Expect(cm.Labels).To(Equal(expectedCm.Labels))
	Expect(cm.Data).To(Equal(expectedCm.Data))
}

func getEmptyImageVectorOverwriteConfigMap() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gardenlet-imagevector-overwrite",
			Namespace: gardencorev1beta1constants.GardenNamespace,
		},
	}
}

func getEmptyImageVectorOverwriteComponentsConfigMap() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gardenlet-imagevector-overwrite-components",
			Namespace: gardencorev1beta1constants.GardenNamespace,
		},
	}
}

func getEmptyCertSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gardenlet-cert",
			Namespace: gardencorev1beta1constants.GardenNamespace,
		},
	}
}

func getEmptyKubeconfigGardenSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gardenlet-kubeconfig-garden",
			Namespace: gardencorev1beta1constants.GardenNamespace,
		},
	}
}

func getEmptyKubeconfigSeedSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gardenlet-kubeconfig-seed",
			Namespace: gardencorev1beta1constants.GardenNamespace,
		},
	}
}

func getEmptyKubeconfigGardenBootstrapSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gardenlet-kubeconfig-bootstrap",
			Namespace: gardencorev1beta1constants.GardenNamespace,
		},
	}
}
