// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package test_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
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
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/charts"
	. "github.com/gardener/gardener/charts/gardener/gardenlet/test"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/seedmanagement"
	"github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/mock"
	"github.com/gardener/gardener/pkg/component"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
)

var (
	expectedLabels = map[string]string{
		"app":      "gardener",
		"role":     "gardenlet",
		"chart":    "gardenlet-0.1.0",
		"release":  "gardenlet",
		"heritage": "Helm",
	}
	expectedLabelsWithCollectableReference = utils.MergeStringMaps(expectedLabels, map[string]string{
		"resources.gardener.cloud/garbage-collectable-reference": "true",
	})
	expectedLabelsWithSkippedWebhooks = utils.MergeStringMaps(expectedLabels, map[string]string{
		"projected-token-mount.resources.gardener.cloud/skip":                         "true",
		"seccompprofile.resources.gardener.cloud/skip":                                "true",
		"topology-spread-constraints.resources.gardener.cloud/skip":                   "true",
		"networking.resources.gardener.cloud/to-all-shoots-etcd-main-client-tcp-8080": "allowed",
		"networking.resources.gardener.cloud/to-all-shoots-kube-apiserver-tcp-443":    "allowed",
	})
)

var _ = Describe("#Gardenlet Chart Test", func() {
	var (
		ctx              context.Context
		c                client.Client
		deployer         component.Deployer
		chartApplier     kubernetes.ChartApplier
		universalDecoder runtime.Decoder
		mapper           *meta.DefaultRESTMapper
	)

	JustBeforeEach(func() {
		ctx = context.TODO()

		s := runtime.NewScheme()
		// for gardenletconfig map
		Expect(corev1.AddToScheme(s)).To(Succeed())
		// for deployment
		Expect(appsv1.AddToScheme(s)).To(Succeed())
		// for unmarshal of GardenletConfiguration
		Expect(gardenletconfigv1alpha1.AddToScheme(s)).To(Succeed())
		// for priority class
		Expect(schedulingv1.AddToScheme(s)).To(Succeed())
		// for ClusterRole and ClusterRoleBinding
		Expect(rbacv1.AddToScheme(s)).To(Succeed())
		// for PDB
		Expect(policyv1.AddToScheme(s)).To(Succeed())
		// for vpa
		Expect(vpaautoscalingv1.AddToScheme(s)).To(Succeed())

		// create decoder for unmarshalling the GardenletConfiguration from the component gardenletconfig Config Map
		codecs := serializer.NewCodecFactory(s)
		universalDecoder = codecs.UniversalDecoder()

		// fake client to use for the chart applier
		c = fake.NewClientBuilder().WithScheme(s).Build()

		mapper = meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion, appsv1.SchemeGroupVersion})

		mapper.Add(appsv1.SchemeGroupVersion.WithKind("Deployment"), meta.RESTScopeNamespace)
		mapper.Add(corev1.SchemeGroupVersion.WithKind("ConfigMap"), meta.RESTScopeNamespace)
		mapper.Add(vpaautoscalingv1.SchemeGroupVersion.WithKind("VerticalPodAutoscaler"), meta.RESTScopeNamespace)
		mapper.Add(schedulingv1.SchemeGroupVersion.WithKind("PriorityClass"), meta.RESTScopeRoot)
		mapper.Add(rbacv1.SchemeGroupVersion.WithKind("ClusterRole"), meta.RESTScopeRoot)
		mapper.Add(rbacv1.SchemeGroupVersion.WithKind("ClusterRoleBinding"), meta.RESTScopeRoot)

		// set the git version required for rendering of the Gardenlet chart -  chart helpers determine resource API versions based on that
		renderer := chartrenderer.NewWithServerVersion(&version.Info{GitVersion: "1.25.0"})

		chartApplier = kubernetes.NewChartApplier(renderer, kubernetes.NewApplier(c, mapper))
		Expect(chartApplier).NotTo(BeNil(), "should return chart applier")
	})

	Describe("Destroy Gardenlet Resources", func() {
		It("should delete all resources", func() {
			ctrl := gomock.NewController(GinkgoT())
			defer ctrl.Finish()

			mockChartApplier := mock.NewMockChartApplier(ctrl)

			mockChartApplier.EXPECT().DeleteFromEmbeddedFS(ctx, charts.ChartGardenlet, charts.ChartPathGardenlet, "garden", "gardenlet", kubernetes.Values(map[string]any{}))

			deployer = NewGardenletChartApplier(mockChartApplier, map[string]any{})
			Expect(deployer.Destroy(ctx)).ToNot(HaveOccurred(), "Destroy Gardenlet resources succeeds")
		})
	})

	DescribeTable("#DeployGardenletChart",
		func(
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
			gardenletValues := map[string]any{
				"enabled": true,
			}

			componentConfigValues := map[string]any{}

			if gardenClientConnectionKubeconfig != nil {
				componentConfigValues["gardenClientConnection"] = map[string]any{
					"kubeconfig": *gardenClientConnectionKubeconfig,
				}
			}

			if seedClientConnectionKubeconfig != nil {
				componentConfigValues["seedClientConnection"] = map[string]any{
					"kubeconfig": *seedClientConnectionKubeconfig,
				}
			}

			// bootstrap configurations are tested in one test-case
			usesTLSBootstrapping := bootstrapKubeconfigContent != nil && bootstrapKubeconfig != nil && bootstrapKubeconfigSecret != nil
			if usesTLSBootstrapping {
				componentConfigValues["gardenClientConnection"] = map[string]any{
					"bootstrapKubeconfig": map[string]any{
						"name":       bootstrapKubeconfig.Name,
						"namespace":  bootstrapKubeconfig.Namespace,
						"kubeconfig": *bootstrapKubeconfigContent,
					},
					"kubeconfigSecret": map[string]any{
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
				Repository: ptr.To("europe-docker.pkg.dev/gardener-project/releases/gardener/gardenlet"),
				Tag:        ptr.To("latest"),
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

			deployer = NewGardenletChartApplier(chartApplier, gardenletValues)

			Expect(deployer.Deploy(ctx)).ToNot(HaveOccurred(), "Gardenlet chart deployment succeeds")

			ValidateGardenletChartPriorityClass(ctx, c)

			serviceAccountName := "gardenlet"
			if deploymentConfiguration.ServiceAccountName != nil {
				serviceAccountName = *deploymentConfiguration.ServiceAccountName
			}

			ValidateGardenletChartRBAC(ctx, c, expectedLabels, serviceAccountName)

			ValidateGardenletChartServiceAccount(ctx, c, seedClientConnectionKubeconfig != nil, expectedLabels, serviceAccountName)

			var replicaCount *int32
			if deploymentConfiguration != nil {
				replicaCount = deploymentConfiguration.ReplicaCount
			}
			ValidateGardenletChartPodDisruptionBudget(ctx, c, expectedLabels, replicaCount)

			expectedGardenletConfig := ComputeExpectedGardenletConfiguration(
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
				expectedLabelsWithCollectableReference,
				cmAndSecretNameToUniqueName["gardenlet-configmap"],
			)

			expectedGardenletDeploymentSpec, err := ComputeExpectedGardenletDeploymentSpec(
				deploymentConfiguration,
				image,
				gardenClientConnectionKubeconfig,
				seedClientConnectionKubeconfig,
				expectedLabelsWithSkippedWebhooks,
				imageVectorOverwrite,
				componentImageVectorOverwrites,
				cmAndSecretNameToUniqueName,
				seedConfig,
			)

			Expect(err).ToNot(HaveOccurred())

			VerifyGardenletDeployment(ctx,
				c,
				expectedGardenletDeploymentSpec,
				deploymentConfiguration,
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
		},
		Entry("verify the default values for the Gardenlet chart & the Gardenlet component config", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, map[string]string{"gardenlet-configmap": "gardenlet-configmap-f44c8fea"}),
		Entry("verify Gardenlet with component config having the Garden client connection kubeconfig set", ptr.To("dummy garden kubeconfig"), nil, nil, nil, nil, nil, nil, nil, nil, nil, map[string]string{
			"gardenlet-configmap":         "gardenlet-configmap-2e316485",
			"gardenlet-kubeconfig-garden": "gardenlet-kubeconfig-garden-8c9ae097",
		}),
		Entry("verify Gardenlet with component config having the Seed client connection kubeconfig set", nil, ptr.To("dummy seed kubeconfig"), nil, nil, nil, nil, nil, nil, nil, nil, map[string]string{
			"gardenlet-configmap":       "gardenlet-configmap-08984926",
			"gardenlet-kubeconfig-seed": "gardenlet-kubeconfig-seed-662d92ae",
		}),
		Entry("verify Gardenlet with component config having a Bootstrap kubeconfig set", nil, nil, &corev1.SecretReference{
			Name:      "gardenlet-kubeconfig-bootstrap",
			Namespace: "garden",
		}, &corev1.SecretReference{
			Name:      "gardenlet-kubeconfig",
			Namespace: v1beta1constants.GardenNamespace,
		}, ptr.To("dummy bootstrap kubeconfig"), nil, nil, nil, nil, nil, map[string]string{
			"gardenlet-configmap": "gardenlet-configmap-800a10ff",
		}),
		Entry("verify that the SeedConfig is set in the component config Config Map", nil, nil, nil, nil, nil,
			&gardenletconfigv1alpha1.SeedConfig{
				SeedTemplate: gardencorev1beta1.SeedTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Name: "sweet-seed",
					},
					Spec: gardencorev1beta1.SeedSpec{
						Provider: gardencorev1beta1.SeedProvider{},
					},
				},
			}, nil, nil, nil, nil, map[string]string{"gardenlet-configmap": "gardenlet-configmap-7ca44afd"}),
		Entry("verify deployment with two replica and three zones", nil, nil, nil, nil, nil,
			&gardenletconfigv1alpha1.SeedConfig{
				SeedTemplate: gardencorev1beta1.SeedTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Name: "sweet-seed",
					},
					Spec: gardencorev1beta1.SeedSpec{
						Provider: gardencorev1beta1.SeedProvider{
							Zones: []string{"a", "b", "c"},
						},
					},
				},
			}, &seedmanagement.GardenletDeployment{
				ReplicaCount: ptr.To[int32](2),
			}, nil, nil, nil, map[string]string{"gardenlet-configmap": "gardenlet-configmap-387e51e0"}),
		Entry("verify deployment with only one replica", nil, nil, nil, nil, nil,
			&gardenletconfigv1alpha1.SeedConfig{
				SeedTemplate: gardencorev1beta1.SeedTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Name: "sweet-seed",
					},
					Spec: gardencorev1beta1.SeedSpec{
						Provider: gardencorev1beta1.SeedProvider{
							Zones: []string{"a", "b", "c"},
						},
					},
				},
			}, &seedmanagement.GardenletDeployment{
				ReplicaCount: ptr.To[int32](1),
			}, nil, nil, nil, map[string]string{"gardenlet-configmap": "gardenlet-configmap-387e51e0"}),
		Entry("verify deployment with only one zone", nil, nil, nil, nil, nil,
			&gardenletconfigv1alpha1.SeedConfig{
				SeedTemplate: gardencorev1beta1.SeedTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Name: "sweet-seed",
					},
					Spec: gardencorev1beta1.SeedSpec{
						Provider: gardencorev1beta1.SeedProvider{
							Zones: []string{"a"},
						},
					},
				},
			}, nil, nil, nil, nil, map[string]string{"gardenlet-configmap": "gardenlet-configmap-7d75ba46"}),
		Entry("verify deployment with image vector override", nil, nil, nil, nil, nil, nil, nil, ptr.To("dummy-override-content"), nil, nil, map[string]string{
			"gardenlet-configmap":             "gardenlet-configmap-f44c8fea",
			"gardenlet-imagevector-overwrite": "gardenlet-imagevector-overwrite-32ecb769",
		}),
		Entry("verify deployment with component image vector override", nil, nil, nil, nil, nil, nil, nil, nil, ptr.To("dummy-override-content"), nil, map[string]string{
			"gardenlet-configmap":                        "gardenlet-configmap-f44c8fea",
			"gardenlet-imagevector-overwrite-components": "gardenlet-imagevector-overwrite-components-53f94952",
		}),

		Entry("verify deployment with custom replica count", nil, nil, nil, nil, nil, nil, &seedmanagement.GardenletDeployment{
			ReplicaCount: ptr.To[int32](3),
		}, nil, nil, nil, map[string]string{"gardenlet-configmap": "gardenlet-configmap-f44c8fea"}),

		Entry("verify deployment with service account", nil, nil, nil, nil, nil, nil, &seedmanagement.GardenletDeployment{
			ServiceAccountName: ptr.To("ax"),
		}, nil, nil, nil, map[string]string{"gardenlet-configmap": "gardenlet-configmap-f44c8fea"}),

		Entry("verify deployment with resources", nil, nil, nil, nil, nil, nil, &seedmanagement.GardenletDeployment{
			Resources: &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("800m"),
					corev1.ResourceMemory: resource.MustParse("15Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("25Mi"),
				},
			},
		}, nil, nil, nil, map[string]string{"gardenlet-configmap": "gardenlet-configmap-f44c8fea"}),

		Entry("verify deployment with pod labels", nil, nil, nil, nil, nil, nil, &seedmanagement.GardenletDeployment{
			PodLabels: map[string]string{
				"x": "y",
			},
		}, nil, nil, nil, map[string]string{"gardenlet-configmap": "gardenlet-configmap-f44c8fea"}),

		Entry("verify deployment with pod annotations", nil, nil, nil, nil, nil, nil, &seedmanagement.GardenletDeployment{
			PodAnnotations: map[string]string{
				"x": "y",
			},
		}, nil, nil, nil, map[string]string{"gardenlet-configmap": "gardenlet-configmap-f44c8fea"}),

		Entry("verify deployment with additional volumes", nil, nil, nil, nil, nil, nil, &seedmanagement.GardenletDeployment{
			AdditionalVolumes: []corev1.Volume{
				{
					Name:         "a",
					VolumeSource: corev1.VolumeSource{},
				},
			},
		}, nil, nil, nil, map[string]string{"gardenlet-configmap": "gardenlet-configmap-f44c8fea"}),

		Entry("verify deployment with additional volume mounts", nil, nil, nil, nil, nil, nil, &seedmanagement.GardenletDeployment{
			AdditionalVolumeMounts: []corev1.VolumeMount{
				{
					Name: "a",
				},
			},
		}, nil, nil, nil, map[string]string{"gardenlet-configmap": "gardenlet-configmap-f44c8fea"}),

		Entry("verify deployment with env variables", nil, nil, nil, nil, nil, nil, &seedmanagement.GardenletDeployment{
			Env: []corev1.EnvVar{
				{
					Name:  "KUBECONFIG",
					Value: "XY",
				},
			},
		}, nil, nil, nil, map[string]string{"gardenlet-configmap": "gardenlet-configmap-f44c8fea"}),
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
		client.ObjectKey{Namespace: secret.Namespace, Name: name},
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
		client.ObjectKey{Namespace: cm.Namespace, Name: uniqueName},
		cm,
	)).ToNot(HaveOccurred())

	Expect(cm.Labels).To(Equal(expectedCm.Labels))
	Expect(cm.Data).To(Equal(expectedCm.Data))
}

func getEmptyImageVectorOverwriteConfigMap() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gardenlet-imagevector-overwrite",
			Namespace: v1beta1constants.GardenNamespace,
		},
	}
}

func getEmptyImageVectorOverwriteComponentsConfigMap() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gardenlet-imagevector-overwrite-components",
			Namespace: v1beta1constants.GardenNamespace,
		},
	}
}

func getEmptyKubeconfigGardenSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gardenlet-kubeconfig-garden",
			Namespace: v1beta1constants.GardenNamespace,
		},
	}
}

func getEmptyKubeconfigSeedSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gardenlet-kubeconfig-seed",
			Namespace: v1beta1constants.GardenNamespace,
		},
	}
}

func getEmptyKubeconfigGardenBootstrapSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gardenlet-kubeconfig-bootstrap",
			Namespace: v1beta1constants.GardenNamespace,
		},
	}
}
