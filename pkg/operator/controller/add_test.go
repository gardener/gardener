// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package controller_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/gardenlet/v1alpha1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/seedmanagement/encoding"
	gardenletdeployer "github.com/gardener/gardener/pkg/controller/gardenletdeployer"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	. "github.com/gardener/gardener/pkg/operator/controller"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("#TargetNamespaceForTokenRequestorController", func() {
	var (
		ctx        context.Context
		fakeClient *fakeclient.ClientBuilder
	)

	BeforeEach(func() {
		ctx = context.Background()
		fakeClient = fakeclient.NewClientBuilder().WithScheme(operatorclient.RuntimeScheme)
	})

	clustersCRD := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "clusters.extensions.gardener.cloud",
		},
	}

	buildGardenletDeployment := func(configMapName string) *appsv1.Deployment {
		return &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      v1beta1constants.DeploymentNameGardenlet,
				Namespace: v1beta1constants.GardenNamespace,
			},
			Spec: appsv1.DeploymentSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Volumes: []corev1.Volume{
							{
								Name: gardenletdeployer.GardenletConfigVolumeName,
								VolumeSource: corev1.VolumeSource{
									ConfigMap: &corev1.ConfigMapVolumeSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: configMapName,
										},
									},
								},
							},
						},
					},
				},
			},
		}
	}

	buildGardenletConfigMap := func(configMapName, seedName string) *corev1.ConfigMap {
		cfg := &gardenletconfigv1alpha1.GardenletConfiguration{
			SeedConfig: &gardenletconfigv1alpha1.SeedConfig{},
		}
		cfg.SeedConfig.Name = seedName

		configBytes, err := encoding.EncodeGardenletConfigurationToBytes(cfg)
		Expect(err).NotTo(HaveOccurred())

		return &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      configMapName,
				Namespace: v1beta1constants.GardenNamespace,
			},
			Data: map[string]string{
				"config.yaml": string(configBytes),
			},
		}
	}

	It("should return the garden namespace when the cluster is not a seed cluster", func() {
		c := fakeClient.Build()

		fn := TargetNamespaceForTokenRequestorController()
		namespace, err := fn(ctx, c)

		Expect(err).NotTo(HaveOccurred())
		Expect(namespace).To(Equal(v1beta1constants.GardenNamespace))
	})

	When("the cluster is a seed cluster", func() {
		const (
			seedName      = "my-seed"
			configMapName = "gardenlet-configmap-abcd1234"
		)

		It("should return the seed namespace derived from the gardenlet config", func() {
			c := fakeClient.WithObjects(
				clustersCRD,
				buildGardenletDeployment(configMapName),
				buildGardenletConfigMap(configMapName, seedName),
			).Build()

			fn := TargetNamespaceForTokenRequestorController()
			namespace, err := fn(ctx, c)

			Expect(err).NotTo(HaveOccurred())
			Expect(namespace).To(Equal(gardenerutils.ComputeGardenNamespace(seedName)))
		})

		It("should cache the seed name and not re-read the deployment on subsequent calls", func() {
			c := fakeClient.WithObjects(
				clustersCRD,
				buildGardenletDeployment(configMapName),
				buildGardenletConfigMap(configMapName, seedName),
			).Build()

			fn := TargetNamespaceForTokenRequestorController()

			namespace, err := fn(ctx, c)
			Expect(err).NotTo(HaveOccurred())
			Expect(namespace).To(Equal(gardenerutils.ComputeGardenNamespace(seedName)))

			// Delete the deployment and configmap to prove the result is served from cache.
			Expect(c.Delete(ctx, buildGardenletDeployment(configMapName))).To(Succeed())

			namespace, err = fn(ctx, c)
			Expect(err).NotTo(HaveOccurred())
			Expect(namespace).To(Equal(gardenerutils.ComputeGardenNamespace(seedName)))
		})

		It("should return an error when the gardenlet deployment is not found", func() {
			c := fakeClient.WithObjects(clustersCRD).Build()

			fn := TargetNamespaceForTokenRequestorController()
			_, err := fn(ctx, c)

			Expect(err).To(MatchError(ContainSubstring("failed reading gardenlet deployment")))
		})

		It("should return an error when the gardenlet-config volume is missing from the deployment", func() {
			deploymentWithoutConfigVolume := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      v1beta1constants.DeploymentNameGardenlet,
					Namespace: v1beta1constants.GardenNamespace,
				},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Volumes: []corev1.Volume{},
						},
					},
				},
			}
			c := fakeClient.WithObjects(clustersCRD, deploymentWithoutConfigVolume).Build()

			fn := TargetNamespaceForTokenRequestorController()
			_, err := fn(ctx, c)

			Expect(err).To(MatchError(ContainSubstring("gardenlet deployment has no volume named")))
		})

		It("should return an error when the gardenlet configmap is not found", func() {
			c := fakeClient.WithObjects(clustersCRD, buildGardenletDeployment(configMapName)).Build()

			fn := TargetNamespaceForTokenRequestorController()
			_, err := fn(ctx, c)

			Expect(err).To(MatchError(ContainSubstring("failed reading gardenlet ConfigMap")))
		})

		It("should return an error when the gardenlet config has no seed name", func() {
			cfgWithoutSeed := &gardenletconfigv1alpha1.GardenletConfiguration{}
			configBytes, err := encoding.EncodeGardenletConfigurationToBytes(cfgWithoutSeed)
			Expect(err).NotTo(HaveOccurred())

			configMapWithoutSeed := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      configMapName,
					Namespace: v1beta1constants.GardenNamespace,
				},
				Data: map[string]string{
					"config.yaml": string(configBytes),
				},
			}
			c := fakeClient.WithObjects(clustersCRD, buildGardenletDeployment(configMapName), configMapWithoutSeed).Build()

			fn := TargetNamespaceForTokenRequestorController()
			_, err = fn(ctx, c)

			Expect(err).To(MatchError(ContainSubstring("has no seed name")))
		})
	})
})
