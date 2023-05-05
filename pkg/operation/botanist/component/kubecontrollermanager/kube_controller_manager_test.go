// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubecontrollermanager_test

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/Masterminds/semver"
	hvpav1alpha1 "github.com/gardener/hvpa-controller/api/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	autoscalingv2beta1 "k8s.io/api/autoscaling/v2beta1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	kubernetesfake "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/kubecontrollermanager"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

var _ = Describe("KubeControllerManager", func() {
	var (
		ctx                   = context.TODO()
		testLogger            = logr.Discard()
		ctrl                  *gomock.Controller
		c                     *mockclient.MockClient
		fakeClient            client.Client
		fakeInterface         kubernetes.Interface
		sm                    secretsmanager.Interface
		kubeControllerManager Interface
		values                Values

		_, podCIDR, _                 = net.ParseCIDR("100.96.0.0/11")
		_, serviceCIDR, _             = net.ParseCIDR("100.64.0.0/13")
		fakeErr                       = fmt.Errorf("fake error")
		namespace                     = "shoot--foo--bar"
		version                       = "1.22.2"
		semverVersion, _              = semver.NewVersion(version)
		runtimeKubernetesVersion      = semver.MustParse("1.25.0")
		image                         = "registry.k8s.io/kube-controller-manager:v1.22.2"
		hvpaConfigDisabled            = &HVPAConfig{Enabled: false}
		hvpaConfigEnabled             = &HVPAConfig{Enabled: true}
		hvpaConfigEnabledScaleDownOff = &HVPAConfig{Enabled: true, ScaleDownUpdateMode: pointer.String(hvpav1alpha1.UpdateModeOff)}
		isWorkerless                  = false

		hpaConfig = gardencorev1beta1.HorizontalPodAutoscalerConfig{
			CPUInitializationPeriod: &metav1.Duration{Duration: 5 * time.Minute},
			DownscaleStabilization:  &metav1.Duration{Duration: 5 * time.Minute},
			InitialReadinessDelay:   &metav1.Duration{Duration: 30 * time.Second},
			SyncPeriod:              &metav1.Duration{Duration: 30 * time.Second},
			Tolerance:               pointer.Float64(0.1),
		}

		nodeCIDRMask           int32 = 24
		podEvictionTimeout           = metav1.Duration{Duration: 3 * time.Minute}
		nodeMonitorGracePeriod       = metav1.Duration{Duration: 3 * time.Minute}
		kcmConfig                    = gardencorev1beta1.KubeControllerManagerConfig{
			KubernetesConfig:              gardencorev1beta1.KubernetesConfig{},
			HorizontalPodAutoscalerConfig: &hpaConfig,
			NodeCIDRMaskSize:              &nodeCIDRMask,
			PodEvictionTimeout:            &podEvictionTimeout,
			NodeMonitorGracePeriod:        &nodeMonitorGracePeriod,
		}
		clusterSigningDuration = pointer.Duration(time.Hour)
		controllerWorkers      = ControllerWorkers{
			StatefulSet:         pointer.Int(1),
			Deployment:          pointer.Int(2),
			ReplicaSet:          pointer.Int(3),
			Endpoint:            pointer.Int(4),
			GarbageCollector:    pointer.Int(5),
			Namespace:           pointer.Int(6),
			ResourceQuota:       pointer.Int(7),
			ServiceEndpoint:     pointer.Int(8),
			ServiceAccountToken: pointer.Int(9),
		}
		controllerWorkersWithDisabledControllers = ControllerWorkers{
			StatefulSet:         pointer.Int(1),
			Deployment:          pointer.Int(2),
			ReplicaSet:          pointer.Int(3),
			Endpoint:            pointer.Int(4),
			GarbageCollector:    pointer.Int(5),
			Namespace:           pointer.Int(0),
			ResourceQuota:       pointer.Int(0),
			ServiceEndpoint:     pointer.Int(8),
			ServiceAccountToken: pointer.Int(0),
		}
		controllerSyncPeriods = ControllerSyncPeriods{
			ResourceQuota: pointer.Duration(time.Minute),
		}

		genericTokenKubeconfigSecretName = "generic-token-kubeconfig"
		vpaName                          = "kube-controller-manager-vpa"
		hvpaName                         = "kube-controller-manager"
		pdbName                          = "kube-controller-manager"
		secretName                       = "shoot-access-kube-controller-manager"
		serviceName                      = "kube-controller-manager"
		managedResourceName              = "shoot-core-kube-controller-manager"
		managedResourceSecretName        = "managedresource-shoot-core-kube-controller-manager"
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
		fakeInterface = kubernetesfake.NewClientSetBuilder().WithAPIReader(c).WithClient(c).Build()
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).Build()
		sm = fakesecretsmanager.New(fakeClient, namespace)

		By("Create secrets managed outside of this package for whose secretsmanager.Get() will be called")
		Expect(fakeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca", Namespace: namespace}})).To(Succeed())
		Expect(fakeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "generic-token-kubeconfig", Namespace: namespace}})).To(Succeed())
		Expect(fakeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca-client-current", Namespace: namespace}})).To(Succeed())
		Expect(fakeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca-kubelet-current", Namespace: namespace}})).To(Succeed())
		Expect(fakeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "service-account-key-current", Namespace: namespace}})).To(Succeed())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Deploy", func() {
		Context("Tests expecting a failure", func() {
			BeforeEach(func() {
				values = Values{
					RuntimeVersion: runtimeKubernetesVersion,
					TargetVersion:  semverVersion,
					Image:          image,
					Config:         &kcmConfig,
					HVPAConfig:     hvpaConfigDisabled,
					IsWorkerless:   isWorkerless,
					PodNetwork:     podCIDR,
					ServiceNetwork: serviceCIDR,
				}
				kubeControllerManager = New(
					testLogger,
					fakeInterface,
					namespace,
					sm,
					values,
				)
			})

			It("should fail when the service cannot be created", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, serviceName), gomock.AssignableToTypeOf(&corev1.Service{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Service{}), gomock.Any()).Return(fakeErr),
				)

				Expect(kubeControllerManager.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail when the secret cannot be created", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, serviceName), gomock.AssignableToTypeOf(&corev1.Service{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Service{}), gomock.Any()),
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, secretName), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()).Return(fakeErr),
				)

				Expect(kubeControllerManager.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the deployment cannot be created", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, serviceName), gomock.AssignableToTypeOf(&corev1.Service{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Service{}), gomock.Any()),
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, secretName), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()),
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, v1beta1constants.DeploymentNameKubeControllerManager), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{}), gomock.Any()).Return(fakeErr),
				)

				Expect(kubeControllerManager.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the pod disruption budget cannot be created", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, serviceName), gomock.AssignableToTypeOf(&corev1.Service{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Service{}), gomock.Any()),
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, secretName), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()),
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, v1beta1constants.DeploymentNameKubeControllerManager), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{}), gomock.Any()),
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, pdbName), gomock.AssignableToTypeOf(&policyv1.PodDisruptionBudget{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&policyv1.PodDisruptionBudget{}), gomock.Any()).Return(fakeErr),
				)

				Expect(kubeControllerManager.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the hvpa cannot be deleted", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, serviceName), gomock.AssignableToTypeOf(&corev1.Service{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Service{}), gomock.Any()),
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, secretName), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()),
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, v1beta1constants.DeploymentNameKubeControllerManager), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{}), gomock.Any()),
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, pdbName), gomock.AssignableToTypeOf(&policyv1.PodDisruptionBudget{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&policyv1.PodDisruptionBudget{}), gomock.Any()),
					c.EXPECT().Delete(ctx, &hvpav1alpha1.Hvpa{ObjectMeta: metav1.ObjectMeta{Name: hvpaName, Namespace: namespace}}).Return(fakeErr),
				)

				Expect(kubeControllerManager.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the vpa cannot be created", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, serviceName), gomock.AssignableToTypeOf(&corev1.Service{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Service{}), gomock.Any()),
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, secretName), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()),
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, v1beta1constants.DeploymentNameKubeControllerManager), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{}), gomock.Any()),
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, pdbName), gomock.AssignableToTypeOf(&policyv1.PodDisruptionBudget{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&policyv1.PodDisruptionBudget{}), gomock.Any()),
					c.EXPECT().Delete(ctx, &hvpav1alpha1.Hvpa{ObjectMeta: metav1.ObjectMeta{Name: hvpaName, Namespace: namespace}}),
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, vpaName), gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{}), gomock.Any()).Return(fakeErr),
				)

				Expect(kubeControllerManager.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the managed resource secret cannot be created", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, serviceName), gomock.AssignableToTypeOf(&corev1.Service{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Service{}), gomock.Any()),
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, secretName), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()),
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, v1beta1constants.DeploymentNameKubeControllerManager), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{}), gomock.Any()),
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, pdbName), gomock.AssignableToTypeOf(&policyv1.PodDisruptionBudget{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&policyv1.PodDisruptionBudget{}), gomock.Any()),
					c.EXPECT().Delete(ctx, &hvpav1alpha1.Hvpa{ObjectMeta: metav1.ObjectMeta{Name: hvpaName, Namespace: namespace}}),
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, vpaName), gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{}), gomock.Any()),
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, managedResourceSecretName), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()).Return(fakeErr),
				)

				Expect(kubeControllerManager.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the managed resource cannot be created", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, serviceName), gomock.AssignableToTypeOf(&corev1.Service{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Service{}), gomock.Any()),
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, secretName), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()),
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, v1beta1constants.DeploymentNameKubeControllerManager), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{}), gomock.Any()),
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, pdbName), gomock.AssignableToTypeOf(&policyv1.PodDisruptionBudget{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&policyv1.PodDisruptionBudget{}), gomock.Any()),
					c.EXPECT().Delete(ctx, &hvpav1alpha1.Hvpa{ObjectMeta: metav1.ObjectMeta{Name: hvpaName, Namespace: namespace}}),
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, vpaName), gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{}), gomock.Any()),
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, managedResourceSecretName), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()),
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, managedResourceName), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{}), gomock.Any()).Return(fakeErr),
				)

				Expect(kubeControllerManager.Deploy(ctx)).To(MatchError(fakeErr))
			})
		})

		Context("Tests expecting success", func() {
			var (
				vpaUpdateMode    = vpaautoscalingv1.UpdateModeAuto
				controlledValues = vpaautoscalingv1.ContainerControlledValuesRequestsOnly
				vpa              = &vpaautoscalingv1.VerticalPodAutoscaler{
					ObjectMeta: metav1.ObjectMeta{Name: vpaName, Namespace: namespace},
					Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
						TargetRef: &autoscalingv1.CrossVersionObjectReference{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
							Name:       v1beta1constants.DeploymentNameKubeControllerManager,
						},
						UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
							UpdateMode: &vpaUpdateMode,
						},
						ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
							ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{{
								ContainerName: "kube-controller-manager",
								MinAllowed: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("100Mi"),
								},
								MaxAllowed: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("4"),
									corev1.ResourceMemory: resource.MustParse("10G"),
								},
								ControlledValues: &controlledValues,
							}},
						},
					},
				}

				secret = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      secretName,
						Namespace: namespace,
						Annotations: map[string]string{
							"serviceaccount.resources.gardener.cloud/name":      "kube-controller-manager",
							"serviceaccount.resources.gardener.cloud/namespace": "kube-system",
						},
						Labels: map[string]string{
							"resources.gardener.cloud/purpose": "token-requestor",
						},
					},
					Type: corev1.SecretTypeOpaque,
				}

				pdbMaxUnavailable = intstr.FromInt(1)
				pdb               = &policyv1.PodDisruptionBudget{
					ObjectMeta: metav1.ObjectMeta{
						Name:      pdbName,
						Namespace: namespace,
						Labels: map[string]string{
							"app":  "kubernetes",
							"role": "controller-manager",
						},
					},
					Spec: policyv1.PodDisruptionBudgetSpec{
						MaxUnavailable: &pdbMaxUnavailable,
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app":  "kubernetes",
								"role": "controller-manager",
							},
						},
					},
				}

				hvpaUpdateModeAuto = hvpav1alpha1.UpdateModeAuto
				hvpaFor            = func(config *HVPAConfig) *hvpav1alpha1.Hvpa {
					scaleDownUpdateMode := config.ScaleDownUpdateMode
					if scaleDownUpdateMode == nil {
						scaleDownUpdateMode = pointer.String(hvpav1alpha1.UpdateModeAuto)
					}

					return &hvpav1alpha1.Hvpa{
						ObjectMeta: metav1.ObjectMeta{
							Name:      hvpaName,
							Namespace: namespace,
							Labels: map[string]string{
								"app":  "kubernetes",
								"role": "controller-manager",
								"high-availability-config.resources.gardener.cloud/type": "controller",
							},
						},
						Spec: hvpav1alpha1.HvpaSpec{
							Replicas: pointer.Int32(1),
							Hpa: hvpav1alpha1.HpaSpec{
								Deploy: false,
								Selector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"app":  "kubernetes",
										"role": "controller-manager",
									},
								},
								Template: hvpav1alpha1.HpaTemplate{
									ObjectMeta: metav1.ObjectMeta{
										Labels: map[string]string{
											"app":  "kubernetes",
											"role": "controller-manager",
										},
									},
									Spec: hvpav1alpha1.HpaTemplateSpec{
										MinReplicas: pointer.Int32(int32(1)),
										MaxReplicas: int32(1),
									},
								},
							},
							Vpa: hvpav1alpha1.VpaSpec{
								Selector: &metav1.LabelSelector{MatchLabels: map[string]string{
									v1beta1constants.LabelRole: "kube-controller-manager-vpa",
								}},
								Deploy: true,
								ScaleUp: hvpav1alpha1.ScaleType{
									UpdatePolicy: hvpav1alpha1.UpdatePolicy{
										UpdateMode: &hvpaUpdateModeAuto,
									},
								},
								ScaleDown: hvpav1alpha1.ScaleType{
									UpdatePolicy: hvpav1alpha1.UpdatePolicy{
										UpdateMode: scaleDownUpdateMode,
									},
								},
								Template: hvpav1alpha1.VpaTemplate{
									ObjectMeta: metav1.ObjectMeta{
										Labels: map[string]string{
											v1beta1constants.LabelRole: "kube-controller-manager-vpa",
										},
									},
									Spec: hvpav1alpha1.VpaTemplateSpec{
										ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
											ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{{
												ContainerName: "kube-controller-manager",
												MinAllowed: corev1.ResourceList{
													corev1.ResourceMemory: resource.MustParse("100Mi"),
												},
												MaxAllowed: corev1.ResourceList{
													corev1.ResourceCPU:    resource.MustParse("4"),
													corev1.ResourceMemory: resource.MustParse("10G"),
												},
												ControlledValues: &controlledValues,
											}},
										},
									},
								},
							},
							WeightBasedScalingIntervals: []hvpav1alpha1.WeightBasedScalingInterval{
								{
									VpaWeight:         hvpav1alpha1.VpaOnly,
									StartReplicaCount: 1,
									LastReplicaCount:  1,
								},
							},
							TargetRef: &autoscalingv2beta1.CrossVersionObjectReference{
								APIVersion: appsv1.SchemeGroupVersion.String(),
								Kind:       "Deployment",
								Name:       "kube-controller-manager",
							},
						},
					}
				}

				serviceFor = func(version string) *corev1.Service {
					return &corev1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Name:      serviceName,
							Namespace: namespace,
							Labels: map[string]string{
								"app":  "kubernetes",
								"role": "controller-manager",
							},
							Annotations: map[string]string{
								"networking.resources.gardener.cloud/from-policy-pod-label-selector": "all-scrape-targets",
								"networking.resources.gardener.cloud/from-policy-allowed-ports":      `[{"protocol":"TCP","port":10257}]`,
							},
						},
						Spec: corev1.ServiceSpec{
							Selector: map[string]string{
								"app":  "kubernetes",
								"role": "controller-manager",
							},
							Type:      corev1.ServiceTypeClusterIP,
							ClusterIP: corev1.ClusterIPNone,
							Ports: []corev1.ServicePort{
								{
									Name:     "metrics",
									Protocol: corev1.ProtocolTCP,
									Port:     10257,
								},
							},
						},
					}
				}

				replicas      int32 = 1
				deploymentFor       = func(version string, config *gardencorev1beta1.KubeControllerManagerConfig, isWorkerless bool, controllerWorkers ControllerWorkers) *appsv1.Deployment {
					deploy := &appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{
							Name:      v1beta1constants.DeploymentNameKubeControllerManager,
							Namespace: namespace,
							Labels: map[string]string{
								"app":                 "kubernetes",
								"role":                "controller-manager",
								"gardener.cloud/role": "controlplane",
								"high-availability-config.resources.gardener.cloud/type": "controller",
							},
						},
						Spec: appsv1.DeploymentSpec{
							RevisionHistoryLimit: pointer.Int32(1),
							Replicas:             &replicas,
							Selector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"app":  "kubernetes",
									"role": "controller-manager",
								},
							},
							Template: corev1.PodTemplateSpec{
								ObjectMeta: metav1.ObjectMeta{
									Labels: map[string]string{
										"app":                                "kubernetes",
										"role":                               "controller-manager",
										"gardener.cloud/role":                "controlplane",
										"maintenance.gardener.cloud/restart": "true",
										"networking.gardener.cloud/to-dns":   "allowed",
										"networking.resources.gardener.cloud/to-kube-apiserver-tcp-443": "allowed",
									},
								},
								Spec: corev1.PodSpec{
									AutomountServiceAccountToken: pointer.Bool(false),
									PriorityClassName:            v1beta1constants.PriorityClassNameShootControlPlane300,
									Containers: []corev1.Container{
										{
											Name:            "kube-controller-manager",
											Image:           image,
											ImagePullPolicy: corev1.PullIfNotPresent,
											Command:         commandForKubernetesVersion(version, 10257, config.NodeCIDRMaskSize, config.PodEvictionTimeout, config.NodeMonitorGracePeriod, namespace, isWorkerless, serviceCIDR, podCIDR, getHorizontalPodAutoscalerConfig(config.HorizontalPodAutoscalerConfig), kubernetesutils.FeatureGatesToCommandLineParameter(config.FeatureGates), clusterSigningDuration, controllerWorkers, controllerSyncPeriods),
											LivenessProbe: &corev1.Probe{
												ProbeHandler: corev1.ProbeHandler{
													HTTPGet: &corev1.HTTPGetAction{
														Path:   "/healthz",
														Scheme: corev1.URISchemeHTTPS,
														Port:   intstr.FromInt(10257),
													},
												},
												SuccessThreshold:    1,
												FailureThreshold:    2,
												InitialDelaySeconds: 15,
												PeriodSeconds:       10,
												TimeoutSeconds:      15,
											},
											Ports: []corev1.ContainerPort{
												{
													Name:          "metrics",
													ContainerPort: 10257,
													Protocol:      corev1.ProtocolTCP,
												},
											},
											Resources: corev1.ResourceRequirements{
												Requests: corev1.ResourceList{
													corev1.ResourceCPU:    resource.MustParse("100m"),
													corev1.ResourceMemory: resource.MustParse("128Mi"),
												},
											},
											VolumeMounts: []corev1.VolumeMount{
												{
													Name:      "ca",
													MountPath: "/srv/kubernetes/ca",
												},
												{
													Name:      "ca-client",
													MountPath: "/srv/kubernetes/ca-client",
												},
												{
													Name:      "service-account-key",
													MountPath: "/srv/kubernetes/service-account-key",
												},
												{
													Name:      "server",
													MountPath: "/var/lib/kube-controller-manager-server",
												},
											},
										},
									},
									Volumes: []corev1.Volume{
										{
											Name: "ca",
											VolumeSource: corev1.VolumeSource{
												Secret: &corev1.SecretVolumeSource{
													SecretName: "ca",
												},
											},
										},
										{
											Name: "ca-client",
											VolumeSource: corev1.VolumeSource{
												Secret: &corev1.SecretVolumeSource{
													SecretName: "ca-client-current",
												},
											},
										},
										{
											Name: "service-account-key",
											VolumeSource: corev1.VolumeSource{
												Secret: &corev1.SecretVolumeSource{
													SecretName: "service-account-key-current",
												},
											},
										},
										{
											Name: "server",
											VolumeSource: corev1.VolumeSource{
												Secret: &corev1.SecretVolumeSource{
													SecretName: "kube-controller-manager-server",
												},
											},
										},
									},
								},
							},
						},
					}

					if !isWorkerless {
						deploy.Spec.Template.Spec.Containers[0].VolumeMounts = append(deploy.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
							Name:      "ca-kubelet",
							MountPath: "/srv/kubernetes/ca-kubelet",
						})

						deploy.Spec.Template.Spec.Volumes = append(deploy.Spec.Template.Spec.Volumes, corev1.Volume{
							Name: "ca-kubelet",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "ca-kubelet-current",
								},
							},
						})
					}

					Expect(gardenerutils.InjectGenericKubeconfig(deploy, genericTokenKubeconfigSecretName, secret.Name)).To(Succeed())
					return deploy
				}

				clusterRoleBindingYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  creationTimestamp: null
  name: gardener.cloud:target:kube-controller-manager
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:kube-controller-manager
subjects:
- kind: ServiceAccount
  name: kube-controller-manager
  namespace: kube-system
`
				managedResourceSecret = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      managedResourceSecretName,
						Namespace: namespace,
					},
					Type: corev1.SecretTypeOpaque,
					Data: map[string][]byte{
						"clusterrolebinding____gardener.cloud_target_kube-controller-manager.yaml": []byte(clusterRoleBindingYAML),
					},
				}
				managedResource = &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      managedResourceName,
						Namespace: namespace,
						Labels:    map[string]string{"origin": "gardener"},
					},
					Spec: resourcesv1alpha1.ManagedResourceSpec{
						SecretRefs: []corev1.LocalObjectReference{
							{Name: managedResourceSecretName},
						},
						InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
						KeepObjects:  pointer.Bool(true),
					},
				}

				emptyConfig                = &gardencorev1beta1.KubeControllerManagerConfig{}
				configWithAutoscalerConfig = &gardencorev1beta1.KubeControllerManagerConfig{
					// non default configuration
					HorizontalPodAutoscalerConfig: &gardencorev1beta1.HorizontalPodAutoscalerConfig{
						CPUInitializationPeriod: &metav1.Duration{Duration: 10 * time.Minute},
						DownscaleStabilization:  &metav1.Duration{Duration: 10 * time.Minute},
						InitialReadinessDelay:   &metav1.Duration{Duration: 20 * time.Second},
						SyncPeriod:              &metav1.Duration{Duration: 20 * time.Second},
						Tolerance:               pointer.Float64(0.3),
					},
					NodeCIDRMaskSize: nil,
				}
				configWithFeatureFlags           = &gardencorev1beta1.KubeControllerManagerConfig{KubernetesConfig: gardencorev1beta1.KubernetesConfig{FeatureGates: map[string]bool{"Foo": true, "Bar": false, "Baz": false}}}
				configWithNodeCIDRMaskSize       = &gardencorev1beta1.KubeControllerManagerConfig{NodeCIDRMaskSize: pointer.Int32(26)}
				configWithPodEvictionTimeout     = &gardencorev1beta1.KubeControllerManagerConfig{PodEvictionTimeout: &podEvictionTimeout}
				configWithNodeMonitorGracePeriod = &gardencorev1beta1.KubeControllerManagerConfig{NodeMonitorGracePeriod: &nodeMonitorGracePeriod}
			)

			DescribeTable("success tests for various kubernetes versions (shoots with workers)",
				func(version string, config *gardencorev1beta1.KubeControllerManagerConfig, hvpaConfig *HVPAConfig) {
					isWorkerless = false
					semverVersion, err := semver.NewVersion(version)
					Expect(err).NotTo(HaveOccurred())

					values = Values{
						RuntimeVersion:         runtimeKubernetesVersion,
						TargetVersion:          semverVersion,
						Image:                  image,
						Config:                 config,
						HVPAConfig:             hvpaConfig,
						IsWorkerless:           isWorkerless,
						PodNetwork:             podCIDR,
						ServiceNetwork:         serviceCIDR,
						ClusterSigningDuration: clusterSigningDuration,
						ControllerWorkers:      controllerWorkers,
						ControllerSyncPeriods:  controllerSyncPeriods,
					}
					kubeControllerManager = New(testLogger, fakeInterface, namespace, sm, values)
					kubeControllerManager.SetReplicaCount(replicas)

					if hvpaConfig.Enabled {
						c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, v1beta1constants.DeploymentNameKubeControllerManager), gomock.AssignableToTypeOf(&appsv1.Deployment{})).Return(apierrors.NewNotFound(schema.GroupResource{}, ""))
					}

					gomock.InOrder(
						c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, serviceName), gomock.AssignableToTypeOf(&corev1.Service{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Service{}), gomock.Any()).
							Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
								Expect(obj).To(DeepEqual(serviceFor(version)))
							}),
						c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, secretName), gomock.AssignableToTypeOf(&corev1.Secret{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()).
							Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
								Expect(obj).To(DeepEqual(secret))
							}),
						c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, v1beta1constants.DeploymentNameKubeControllerManager), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{}), gomock.Any()).
							Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
								Expect(obj).To(DeepEqual(deploymentFor(version, config, isWorkerless, controllerWorkers)))
							}),
						c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, pdbName), gomock.AssignableToTypeOf(&policyv1.PodDisruptionBudget{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&policyv1.PodDisruptionBudget{}), gomock.Any()).
							Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
								Expect(obj).To(DeepEqual(pdb))
							}),
					)

					if hvpaConfig.Enabled {
						gomock.InOrder(
							c.EXPECT().Delete(ctx, &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: vpaName, Namespace: namespace}}),
							c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, hvpaName), gomock.AssignableToTypeOf(&hvpav1alpha1.Hvpa{})),
							c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&hvpav1alpha1.Hvpa{}), gomock.Any()).
								Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
									Expect(obj).To(DeepEqual(hvpaFor(hvpaConfig)))
								}),
						)
					} else {
						gomock.InOrder(
							c.EXPECT().Delete(ctx, &hvpav1alpha1.Hvpa{ObjectMeta: metav1.ObjectMeta{Name: hvpaName, Namespace: namespace}}),
							c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, vpaName), gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{})),
							c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{}), gomock.Any()).
								Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
									Expect(obj).To(DeepEqual(vpa))
								}),
						)
					}

					gomock.InOrder(
						c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, managedResourceSecretName), gomock.AssignableToTypeOf(&corev1.Secret{})),
						c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).
							Do(func(ctx context.Context, obj client.Object, _ ...client.UpdateOption) {
								Expect(obj).To(DeepEqual(managedResourceSecret))
							}),
						c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, managedResourceName), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
						c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).
							Do(func(ctx context.Context, obj client.Object, _ ...client.UpdateOption) {
								Expect(obj).To(DeepEqual(managedResource))
							}),
					)

					Expect(kubeControllerManager.Deploy(ctx)).To(Succeed())
				},

				Entry("kubernetes 1.22 w/o config", "1.22.0", emptyConfig, hvpaConfigDisabled),
				Entry("kubernetes 1.22 with HVPA", "1.22.0", emptyConfig, hvpaConfigEnabled),
				Entry("kubernetes 1.22 with HVPA and custom scale-down update mode", "1.22.0", emptyConfig, hvpaConfigEnabledScaleDownOff),
				Entry("kubernetes 1.22 with non-default autoscaler config", "1.22.0", configWithAutoscalerConfig, hvpaConfigDisabled),
				Entry("kubernetes 1.22 with feature flags", "1.22.0", configWithFeatureFlags, hvpaConfigDisabled),
				Entry("kubernetes 1.22 with NodeCIDRMaskSize", "1.22.0", configWithNodeCIDRMaskSize, hvpaConfigDisabled),
				Entry("kubernetes 1.22 with PodEvictionTimeout", "1.22.0", configWithPodEvictionTimeout, hvpaConfigDisabled),
				Entry("kubernetes 1.22 with NodeMonitorGradePeriod", "1.22.0", configWithNodeMonitorGracePeriod, hvpaConfigDisabled),

				Entry("kubernetes 1.21 w/o config", "1.21.0", emptyConfig, hvpaConfigDisabled),
				Entry("kubernetes 1.21 with HVPA", "1.21.0", emptyConfig, hvpaConfigEnabled),
				Entry("kubernetes 1.21 with HVPA and custom scale-down update mode", "1.21.0", emptyConfig, hvpaConfigEnabledScaleDownOff),
				Entry("kubernetes 1.21 with non-default autoscaler config", "1.21.0", configWithAutoscalerConfig, hvpaConfigDisabled),
				Entry("kubernetes 1.21 with feature flags", "1.21.0", configWithFeatureFlags, hvpaConfigDisabled),
				Entry("kubernetes 1.21 with NodeCIDRMaskSize", "1.21.0", configWithNodeCIDRMaskSize, hvpaConfigDisabled),
				Entry("kubernetes 1.21 with PodEvictionTimeout", "1.21.0", configWithPodEvictionTimeout, hvpaConfigDisabled),
				Entry("kubernetes 1.21 with NodeMonitorGradePeriod", "1.21.0", configWithNodeMonitorGracePeriod, hvpaConfigDisabled),

				Entry("kubernetes 1.20 w/o config", "1.20.0", emptyConfig, hvpaConfigDisabled),
				Entry("kubernetes 1.20 with HVPA", "1.20.0", emptyConfig, hvpaConfigEnabled),
				Entry("kubernetes 1.20 with HVPA and custom scale-down update mode", "1.20.0", emptyConfig, hvpaConfigEnabledScaleDownOff),
				Entry("kubernetes 1.20 with non-default autoscaler config", "1.20.0", configWithAutoscalerConfig, hvpaConfigDisabled),
				Entry("kubernetes 1.20 with feature flags", "1.20.0", configWithFeatureFlags, hvpaConfigDisabled),
				Entry("kubernetes 1.20 with NodeCIDRMaskSize", "1.20.0", configWithNodeCIDRMaskSize, hvpaConfigDisabled),
				Entry("kubernetes 1.20 with PodEvictionTimeout", "1.20.0", configWithPodEvictionTimeout, hvpaConfigDisabled),
				Entry("kubernetes 1.20 with NodeMonitorGradePeriod", "1.20.0", configWithNodeMonitorGracePeriod, hvpaConfigDisabled),
			)

			DescribeTable("success tests for various kubernetes versions (workerless shoot)",
				func(version string, config *gardencorev1beta1.KubeControllerManagerConfig, hvpaConfig *HVPAConfig, controllerWorkers ControllerWorkers) {
					isWorkerless = true
					semverVersion, err := semver.NewVersion(version)
					Expect(err).NotTo(HaveOccurred())

					values = Values{
						RuntimeVersion:         runtimeKubernetesVersion,
						TargetVersion:          semverVersion,
						Image:                  image,
						Config:                 config,
						HVPAConfig:             hvpaConfig,
						IsWorkerless:           isWorkerless,
						PodNetwork:             podCIDR,
						ServiceNetwork:         serviceCIDR,
						ClusterSigningDuration: clusterSigningDuration,
						ControllerWorkers:      controllerWorkers,
						ControllerSyncPeriods:  controllerSyncPeriods,
					}
					kubeControllerManager = New(testLogger, fakeInterface, namespace, sm, values)
					kubeControllerManager.SetReplicaCount(replicas)

					if hvpaConfig.Enabled {
						c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, v1beta1constants.DeploymentNameKubeControllerManager), gomock.AssignableToTypeOf(&appsv1.Deployment{})).Return(apierrors.NewNotFound(schema.GroupResource{}, ""))
					}

					gomock.InOrder(
						c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, serviceName), gomock.AssignableToTypeOf(&corev1.Service{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Service{}), gomock.Any()).
							Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
								Expect(obj).To(DeepEqual(serviceFor(version)))
							}),
						c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, secretName), gomock.AssignableToTypeOf(&corev1.Secret{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()).
							Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
								Expect(obj).To(DeepEqual(secret))
							}),
						c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, v1beta1constants.DeploymentNameKubeControllerManager), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{}), gomock.Any()).
							Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
								Expect(obj).To(DeepEqual(deploymentFor(version, config, isWorkerless, controllerWorkers)))
							}),
						c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, pdbName), gomock.AssignableToTypeOf(&policyv1.PodDisruptionBudget{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&policyv1.PodDisruptionBudget{}), gomock.Any()).
							Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
								Expect(obj).To(DeepEqual(pdb))
							}),
					)

					if hvpaConfig.Enabled {
						gomock.InOrder(
							c.EXPECT().Delete(ctx, &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: vpaName, Namespace: namespace}}),
							c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, hvpaName), gomock.AssignableToTypeOf(&hvpav1alpha1.Hvpa{})),
							c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&hvpav1alpha1.Hvpa{}), gomock.Any()).
								Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
									Expect(obj).To(DeepEqual(hvpaFor(hvpaConfig)))
								}),
						)
					} else {
						gomock.InOrder(
							c.EXPECT().Delete(ctx, &hvpav1alpha1.Hvpa{ObjectMeta: metav1.ObjectMeta{Name: hvpaName, Namespace: namespace}}),
							c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, vpaName), gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{})),
							c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{}), gomock.Any()).
								Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
									Expect(obj).To(DeepEqual(vpa))
								}),
						)
					}

					gomock.InOrder(
						c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, managedResourceSecretName), gomock.AssignableToTypeOf(&corev1.Secret{})),
						c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).
							Do(func(ctx context.Context, obj client.Object, _ ...client.UpdateOption) {
								Expect(obj).To(DeepEqual(managedResourceSecret))
							}),
						c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, managedResourceName), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
						c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).
							Do(func(ctx context.Context, obj client.Object, _ ...client.UpdateOption) {
								Expect(obj).To(DeepEqual(managedResource))
							}),
					)

					Expect(kubeControllerManager.Deploy(ctx)).To(Succeed())
				},

				Entry("kubernetes 1.22 w/o config", "1.22.0", emptyConfig, hvpaConfigDisabled, controllerWorkers),
				Entry("kubernetes 1.22 with HVPA", "1.22.0", emptyConfig, hvpaConfigEnabled, controllerWorkers),
				Entry("kubernetes 1.22 with HVPA and custom scale-down update mode", "1.22.0", emptyConfig, hvpaConfigEnabledScaleDownOff, controllerWorkers),
				Entry("kubernetes 1.22 with non-default autoscaler config", "1.22.0", configWithAutoscalerConfig, hvpaConfigDisabled, controllerWorkers),
				Entry("kubernetes 1.22 with feature flags", "1.22.0", configWithFeatureFlags, hvpaConfigDisabled, controllerWorkers),
				Entry("kubernetes 1.22 with NodeCIDRMaskSize", "1.22.0", configWithNodeCIDRMaskSize, hvpaConfigDisabled, controllerWorkers),
				Entry("kubernetes 1.22 with PodEvictionTimeout", "1.22.0", configWithPodEvictionTimeout, hvpaConfigDisabled, controllerWorkers),
				Entry("kubernetes 1.22 with NodeMonitorGradePeriod", "1.22.0", configWithNodeMonitorGracePeriod, hvpaConfigDisabled, controllerWorkers),
				Entry("kubernetes 1.22 with disabled controllers", "1.22.0", configWithNodeMonitorGracePeriod, hvpaConfigDisabled, controllerWorkersWithDisabledControllers),

				Entry("kubernetes 1.21 w/o config", "1.21.0", emptyConfig, hvpaConfigDisabled, controllerWorkers),
				Entry("kubernetes 1.21 with HVPA", "1.21.0", emptyConfig, hvpaConfigEnabled, controllerWorkers),
				Entry("kubernetes 1.21 with HVPA and custom scale-down update mode", "1.21.0", emptyConfig, hvpaConfigEnabledScaleDownOff, controllerWorkers),
				Entry("kubernetes 1.21 with non-default autoscaler config", "1.21.0", configWithAutoscalerConfig, hvpaConfigDisabled, controllerWorkers),
				Entry("kubernetes 1.21 with feature flags", "1.21.0", configWithFeatureFlags, hvpaConfigDisabled, controllerWorkers),
				Entry("kubernetes 1.21 with NodeCIDRMaskSize", "1.21.0", configWithNodeCIDRMaskSize, hvpaConfigDisabled, controllerWorkers),
				Entry("kubernetes 1.21 with PodEvictionTimeout", "1.21.0", configWithPodEvictionTimeout, hvpaConfigDisabled, controllerWorkers),
				Entry("kubernetes 1.21 with NodeMonitorGradePeriod", "1.21.0", configWithNodeMonitorGracePeriod, hvpaConfigDisabled, controllerWorkers),
				Entry("kubernetes 1.21 with disabled controllers", "1.21.0", configWithNodeMonitorGracePeriod, hvpaConfigDisabled, controllerWorkersWithDisabledControllers),

				Entry("kubernetes 1.20 w/o config", "1.20.0", emptyConfig, hvpaConfigDisabled, controllerWorkers),
				Entry("kubernetes 1.20 with HVPA", "1.20.0", emptyConfig, hvpaConfigEnabled, controllerWorkers),
				Entry("kubernetes 1.20 with HVPA and custom scale-down update mode", "1.20.0", emptyConfig, hvpaConfigEnabledScaleDownOff, controllerWorkers),
				Entry("kubernetes 1.20 with non-default autoscaler config", "1.20.0", configWithAutoscalerConfig, hvpaConfigDisabled, controllerWorkers),
				Entry("kubernetes 1.20 with feature flags", "1.20.0", configWithFeatureFlags, hvpaConfigDisabled, controllerWorkers),
				Entry("kubernetes 1.20 with NodeCIDRMaskSize", "1.20.0", configWithNodeCIDRMaskSize, hvpaConfigDisabled, controllerWorkers),
				Entry("kubernetes 1.20 with PodEvictionTimeout", "1.20.0", configWithPodEvictionTimeout, hvpaConfigDisabled, controllerWorkers),
				Entry("kubernetes 1.20 with NodeMonitorGradePeriod", "1.20.0", configWithNodeMonitorGracePeriod, hvpaConfigDisabled, controllerWorkers),
				Entry("kubernetes 1.20 with disabled controllers", "1.20.0", configWithNodeMonitorGracePeriod, hvpaConfigDisabled, controllerWorkersWithDisabledControllers),
			)
		})
	})

	Describe("#Destroy", func() {
		It("should return nil as it's not implemented as of now", func() {
			Expect(kubeControllerManager.Destroy(ctx)).To(Succeed())
		})
	})

	Describe("#Wait", func() {
		var (
			deployment *appsv1.Deployment
			labels     = map[string]string{"role": "kcm"}
		)

		BeforeEach(func() {
			deployment = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kube-controller-manager",
					Namespace: namespace,
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: pointer.Int32(1),
					Selector: &metav1.LabelSelector{MatchLabels: labels},
				},
			}
		})

		It("should successfully wait for the deployment to be updated", func() {
			fakeClient := fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
			fakeKubernetesInterface := kubernetesfake.NewClientSetBuilder().WithAPIReader(fakeClient).WithClient(fakeClient).Build()

			values = Values{
				RuntimeVersion: semver.MustParse("1.25.0"),
				IsWorkerless:   isWorkerless,
			}
			kubeControllerManager = New(testLogger, fakeKubernetesInterface, namespace, nil, values)

			deploy := deployment.DeepCopy()

			defer test.WithVars(&IntervalWaitForDeployment, time.Millisecond)()
			defer test.WithVars(&TimeoutWaitForDeployment, 100*time.Millisecond)()

			Expect(fakeClient.Create(ctx, deploy)).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(deploy), deploy)).To(Succeed())

			Expect(fakeClient.Create(ctx, &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod",
					Namespace: deployment.Namespace,
					Labels:    labels,
				},
			})).To(Succeed())

			timer := time.AfterFunc(10*time.Millisecond, func() {
				deploy.Generation = 24
				deploy.Spec.Replicas = pointer.Int32(1)
				deploy.Status.Conditions = []appsv1.DeploymentCondition{
					{Type: appsv1.DeploymentProgressing, Status: "True", Reason: "NewReplicaSetAvailable"},
					{Type: appsv1.DeploymentAvailable, Status: "True"},
				}
				deploy.Status.ObservedGeneration = deploy.Generation
				deploy.Status.Replicas = *deploy.Spec.Replicas
				deploy.Status.UpdatedReplicas = *deploy.Spec.Replicas
				deploy.Status.AvailableReplicas = *deploy.Spec.Replicas
				Expect(fakeClient.Update(ctx, deploy)).To(Succeed())
			})
			defer timer.Stop()

			Expect(kubeControllerManager.Wait(ctx)).To(Succeed())
		})
	})

	Describe("#WaitCleanup", func() {
		It("should return nil as it's not implemented as of now", func() {
			Expect(kubeControllerManager.WaitCleanup(ctx)).To(Succeed())
		})
	})
})

// Utility functions

func commandForKubernetesVersion(
	version string,
	port int32,
	nodeCIDRMaskSize *int32,
	podEvictionTimeout *metav1.Duration,
	nodeMonitorGracePeriod *metav1.Duration,
	clusterName string,
	isWorkerless bool,
	serviceNetwork, podNetwork *net.IPNet,
	horizontalPodAutoscalerConfig *gardencorev1beta1.HorizontalPodAutoscalerConfig,
	featureGateFlags string,
	clusterSigningDuration *time.Duration,
	controllerWorkers ControllerWorkers,
	controllerSyncPeriods ControllerSyncPeriods,
) []string {
	var command []string

	podEvictionTimeoutSetting := "2m0s"
	if podEvictionTimeout != nil {
		podEvictionTimeoutSetting = podEvictionTimeout.Duration.String()
	}

	nodeMonitorGracePeriodSetting := "2m0s"
	if nodeMonitorGracePeriod != nil {
		nodeMonitorGracePeriodSetting = nodeMonitorGracePeriod.Duration.String()
	}

	command = append(command,
		"/usr/local/bin/kube-controller-manager",
		"--authentication-kubeconfig=/var/run/secrets/gardener.cloud/shoot/generic-kubeconfig/kubeconfig",
		"--authorization-kubeconfig=/var/run/secrets/gardener.cloud/shoot/generic-kubeconfig/kubeconfig",
		"--kubeconfig=/var/run/secrets/gardener.cloud/shoot/generic-kubeconfig/kubeconfig",
	)

	if !isWorkerless {
		if nodeCIDRMaskSize != nil {
			command = append(command, fmt.Sprintf("--node-cidr-mask-size=%d", *nodeCIDRMaskSize))
		}

		command = append(command,
			"--allocate-node-cidrs=true",
			"--attach-detach-reconcile-sync-period=1m0s",
			"--controllers=*,bootstrapsigner,tokencleaner",
			fmt.Sprintf("--cluster-cidr=%s", podNetwork.String()),
			"--cluster-signing-kubelet-client-cert-file=/srv/kubernetes/ca-client/ca.crt",
			"--cluster-signing-kubelet-client-key-file=/srv/kubernetes/ca-client/ca.key",
			"--cluster-signing-kubelet-serving-cert-file=/srv/kubernetes/ca-kubelet/ca.crt",
			"--cluster-signing-kubelet-serving-key-file=/srv/kubernetes/ca-kubelet/ca.key",
			fmt.Sprintf("--horizontal-pod-autoscaler-downscale-stabilization=%s", horizontalPodAutoscalerConfig.DownscaleStabilization.Duration.String()),
			fmt.Sprintf("--horizontal-pod-autoscaler-initial-readiness-delay=%s", horizontalPodAutoscalerConfig.InitialReadinessDelay.Duration.String()),
			fmt.Sprintf("--horizontal-pod-autoscaler-cpu-initialization-period=%s", horizontalPodAutoscalerConfig.CPUInitializationPeriod.Duration.String()),
			fmt.Sprintf("--horizontal-pod-autoscaler-sync-period=%s", horizontalPodAutoscalerConfig.SyncPeriod.Duration.String()),
			fmt.Sprintf("--horizontal-pod-autoscaler-tolerance=%v", *horizontalPodAutoscalerConfig.Tolerance),
			"--leader-elect=true",
			fmt.Sprintf("--node-monitor-grace-period=%s", nodeMonitorGracePeriodSetting),
			fmt.Sprintf("--pod-eviction-timeout=%s", podEvictionTimeoutSetting),
		)

		if v := controllerWorkers.Deployment; v == nil {
			command = append(command, "--concurrent-deployment-syncs=50")
		} else {
			command = append(command, fmt.Sprintf("--concurrent-deployment-syncs=%d", *v))
		}

		if v := controllerWorkers.ReplicaSet; v == nil {
			command = append(command, "--concurrent-replicaset-syncs=50")
		} else {
			command = append(command, fmt.Sprintf("--concurrent-replicaset-syncs=%d", *v))
		}

		if v := controllerWorkers.StatefulSet; v == nil {
			command = append(command, "--concurrent-statefulset-syncs=15")
		} else {
			command = append(command, fmt.Sprintf("--concurrent-statefulset-syncs=%d", *v))
		}
	} else {
		var controllers []string

		if controllerWorkers.Namespace == nil || *controllerWorkers.Namespace != 0 {
			controllers = append(controllers, "namespace")
		}

		controllers = append(controllers, "serviceaccount")

		if controllerWorkers.ServiceAccountToken == nil || *controllerWorkers.ServiceAccountToken != 0 {
			controllers = append(controllers, "serviceaccount-token")
		}

		controllers = append(controllers,
			"clusterrole-aggregation",
			"garbagecollector",
			"csrapproving",
			"csrcleaner",
			"csrsigning",
			"bootstrapsigner",
			"tokencleaner",
		)

		if controllerWorkers.ResourceQuota == nil || *controllerWorkers.ResourceQuota != 0 {
			controllers = append(controllers, "resourcequota")
		}

		command = append(command, "--controllers="+strings.Join(controllers, ","))
	}

	command = append(command,
		fmt.Sprintf("--cluster-name=%s", clusterName),
		"--cluster-signing-kube-apiserver-client-cert-file=/srv/kubernetes/ca-client/ca.crt",
		"--cluster-signing-kube-apiserver-client-key-file=/srv/kubernetes/ca-client/ca.key",
		"--cluster-signing-legacy-unknown-cert-file=/srv/kubernetes/ca-client/ca.crt",
		"--cluster-signing-legacy-unknown-key-file=/srv/kubernetes/ca-client/ca.key",
	)

	if clusterSigningDuration == nil {
		command = append(command, "--cluster-signing-duration=720h")
	} else {
		command = append(command, "--cluster-signing-duration="+clusterSigningDuration.String())
	}

	if v := controllerWorkers.Endpoint; v == nil {
		command = append(command, "--concurrent-endpoint-syncs=15")
	} else {
		command = append(command, fmt.Sprintf("--concurrent-endpoint-syncs=%d", *v))
	}

	if v := controllerWorkers.GarbageCollector; v == nil {
		command = append(command, "--concurrent-gc-syncs=30")
	} else {
		command = append(command, fmt.Sprintf("--concurrent-gc-syncs=%d", *v))
	}

	if v := controllerWorkers.ServiceEndpoint; v == nil {
		command = append(command, "--concurrent-service-endpoint-syncs=15")
	} else {
		command = append(command, fmt.Sprintf("--concurrent-service-endpoint-syncs=%d", *v))
	}

	if v := controllerWorkers.Namespace; v == nil {
		command = append(command, "--concurrent-namespace-syncs=50")
	} else if *v != 0 {
		command = append(command, fmt.Sprintf("--concurrent-namespace-syncs=%d", *v))
	}

	if v := controllerWorkers.ResourceQuota; v == nil {
		command = append(command, "--concurrent-resource-quota-syncs=15")
	} else if *v != 0 {
		command = append(command, fmt.Sprintf("--concurrent-resource-quota-syncs=%d", *v))

		if v := controllerSyncPeriods.ResourceQuota; v != nil {
			command = append(command, "--resource-quota-sync-period="+v.String())
		}
	}

	if v := controllerWorkers.ServiceAccountToken; v == nil {
		command = append(command, "--concurrent-serviceaccount-token-syncs=15")
	} else if *v != 0 {
		command = append(command, fmt.Sprintf("--concurrent-serviceaccount-token-syncs=%d", *v))
	}

	if len(featureGateFlags) > 0 {
		command = append(command, featureGateFlags)
	}

	if versionutils.ConstraintK8sLess124.Check(semver.MustParse(version)) {
		command = append(command, "--port=0")
	}

	command = append(command,
		"--root-ca-file=/srv/kubernetes/ca/bundle.crt",
		"--service-account-private-key-file=/srv/kubernetes/service-account-key/id_rsa",
		fmt.Sprintf("--secure-port=%d", port),
	)

	if serviceNetwork != nil {
		command = append(command,
			fmt.Sprintf("--service-cluster-ip-range=%s", serviceNetwork.String()),
		)
	}

	command = append(command,
		"--profiling=false",
		"--tls-cert-file=/var/lib/kube-controller-manager-server/tls.crt",
		"--tls-private-key-file=/var/lib/kube-controller-manager-server/tls.key",
	)

	if k8sVersionGreaterEqual122, _ := versionutils.CompareVersions(version, ">=", "1.22"); k8sVersionGreaterEqual122 {
		command = append(command, "--tls-cipher-suites=TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_AES_128_GCM_SHA256,TLS_AES_256_GCM_SHA384,TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384")
	} else {
		command = append(command, "--tls-cipher-suites=TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,TLS_RSA_WITH_AES_128_CBC_SHA,TLS_RSA_WITH_AES_256_CBC_SHA,TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA")
	}

	command = append(command,
		"--use-service-account-credentials=true",
		"--v=2",
	)

	return command
}

func getHorizontalPodAutoscalerConfig(config *gardencorev1beta1.HorizontalPodAutoscalerConfig) *gardencorev1beta1.HorizontalPodAutoscalerConfig {
	defaultHPATolerance := gardencorev1beta1.DefaultHPATolerance
	horizontalPodAutoscalerConfig := gardencorev1beta1.HorizontalPodAutoscalerConfig{
		CPUInitializationPeriod: &metav1.Duration{Duration: 5 * time.Minute},
		DownscaleStabilization:  &metav1.Duration{Duration: 5 * time.Minute},
		InitialReadinessDelay:   &metav1.Duration{Duration: 30 * time.Second},
		SyncPeriod:              &metav1.Duration{Duration: 30 * time.Second},
		Tolerance:               &defaultHPATolerance,
	}

	if config != nil {
		if config.CPUInitializationPeriod != nil {
			horizontalPodAutoscalerConfig.CPUInitializationPeriod = config.CPUInitializationPeriod
		}
		if config.DownscaleStabilization != nil {
			horizontalPodAutoscalerConfig.DownscaleStabilization = config.DownscaleStabilization
		}
		if config.InitialReadinessDelay != nil {
			horizontalPodAutoscalerConfig.InitialReadinessDelay = config.InitialReadinessDelay
		}
		if config.SyncPeriod != nil {
			horizontalPodAutoscalerConfig.SyncPeriod = config.SyncPeriod
		}
		if config.Tolerance != nil {
			horizontalPodAutoscalerConfig.Tolerance = config.Tolerance
		}
	}
	return &horizontalPodAutoscalerConfig
}
