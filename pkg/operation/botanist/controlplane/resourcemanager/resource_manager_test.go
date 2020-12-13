// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package resourcemanager_test

import (
	"context"
	"fmt"

	"github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/controlplane/resourcemanager"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/secrets"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	autoscalingv1beta2 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1beta2"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("ResourceManager", func() {
	var (
		ctrl            *gomock.Controller
		c               *mockclient.MockClient
		resourceManager *resourcemanager.ResourceManager

		ctx                   = context.TODO()
		deployNamespace       = "fake-ns"
		fakeErr               = fmt.Errorf("fake error")
		healthPort      int32 = 8081
		image                 = "fake-image"
		metricsPort     int32 = 8080
		replicas        int32 = 1

		// optional configuration
		secretNameKubeconfig           = "kubeconfig-secret"
		secretMountPath                = "/etc/gardener-resource-manager"
		secretChecksumKubeconfig       = "1234"
		kubeCfg                        = component.Secret{Name: secretNameKubeconfig, Checksum: secretChecksumKubeconfig}
		alwaysUpdate                   = true
		concurrentSyncs          int32 = 20
		clusterRoleName                = "fake-cr-name"
		defaultLabels                  = map[string]string{
			"fake-label-name": "fake-value",
			"app":             "gardener-resource-manager",
		}
		healthSyncPeriod                 = "1m0s"
		leaseDuration                    = "40s"
		maxConcurrentHealthWorkers int32 = 20
		renewDeadline                    = "10s"
		resourceClass                    = "fake-ResourceClass"
		retryPeriod                      = "20s"
		syncPeriod                       = "1m20s"
		targetDisableCache               = true
		watchedNamespace                 = "fake-ns"

		allowAll                   []rbacv1.PolicyRule
		allowManagedResources      []rbacv1.PolicyRule
		cfg                        resourcemanager.Config
		clusterRole                *rbacv1.ClusterRole
		clusterRoleBinding         *rbacv1.ClusterRoleBinding
		cmd                        []string
		cmdWithoutKubeconfig       []string
		cmdWithoutWatchedNamespace []string
		deployment                 *appsv1.Deployment
		roleBinding                *rbacv1.RoleBinding
		role                       *rbacv1.Role
		service                    *corev1.Service
		serviceAccount             *corev1.ServiceAccount
		updateMode                 = autoscalingv1beta2.UpdateModeAuto
		vpa                        *autoscalingv1beta2.VerticalPodAutoscaler
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)

		allowAll = []rbacv1.PolicyRule{{
			APIGroups: []string{"*"},
			Resources: []string{"*"},
			Verbs:     []string{"*"},
		}}
		allowManagedResources = []rbacv1.PolicyRule{{
			APIGroups: []string{"resources.gardener.cloud"},
			Resources: []string{"managedresources", "managedresources/status"},
			Verbs:     []string{"get", "list", "watch", "update", "patch"},
		}, {
			APIGroups: []string{""},
			Resources: []string{"secrets"},
			Verbs:     []string{"get", "list", "watch", "update", "patch"},
		}, {
			APIGroups: []string{""},
			Resources: []string{"configmaps", "events"},
			Verbs:     []string{"create"},
		}, {
			APIGroups:     []string{""},
			Resources:     []string{"configmaps"},
			ResourceNames: []string{"gardener-resource-manager"},
			Verbs:         []string{"get", "watch", "update", "patch"},
		}}

		clusterRole = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:   clusterRoleName,
				Labels: defaultLabels,
			},
			Rules: allowAll}
		clusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:   clusterRoleName,
				Labels: defaultLabels,
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     clusterRoleName,
			},
			Subjects: []rbacv1.Subject{{
				Kind:      "ServiceAccount",
				Name:      "gardener-resource-manager",
				Namespace: deployNamespace,
			}}}
		role = &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: deployNamespace,
				Name:      "gardener-resource-manager",
				Labels:    defaultLabels,
			},
			Rules: allowManagedResources}
		roleBinding = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: deployNamespace,
				Name:      "gardener-resource-manager",
				Labels:    defaultLabels,
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "Role",
				Name:     "gardener-resource-manager",
			},
			Subjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      "gardener-resource-manager",
				Namespace: deployNamespace,
			}}}

		service = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener-resource-manager",
				Namespace: deployNamespace,
				Labels:    defaultLabels},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{
					"app": "gardener-resource-manager"},
				Type:      corev1.ServiceTypeClusterIP,
				ClusterIP: corev1.ClusterIPNone,
				Ports: []corev1.ServicePort{
					{
						Name:     "metrics",
						Port:     metricsPort,
						Protocol: corev1.ProtocolTCP,
					},
					{
						Name:     "health",
						Port:     healthPort,
						Protocol: corev1.ProtocolTCP,
					},
				},
			},
		}
		cfg = resourcemanager.Config{
			AlwaysUpdate:               &alwaysUpdate,
			ConcurrentSyncs:            &concurrentSyncs,
			ClusterRoleName:            &clusterRoleName,
			DefaultLabels:              &defaultLabels,
			HealthSyncPeriod:           &healthSyncPeriod,
			KubeConfig:                 &kubeCfg,
			LeaseDuration:              &leaseDuration,
			MaxConcurrentHealthWorkers: &maxConcurrentHealthWorkers,
			RenewDeadline:              &renewDeadline,
			ResourceClass:              &resourceClass,
			RetryPeriod:                &retryPeriod,
			SyncPeriod:                 &syncPeriod,
			TargetDisableCache:         &targetDisableCache,
			WatchedNamespace:           &watchedNamespace,
		}
		resourceManager = resourcemanager.New(c, deployNamespace, image, replicas, cfg)

		cmd = []string{"/gardener-resource-manager",
			"--always-update=true",
			fmt.Sprintf("--health-bind-address=:%v", healthPort),
			fmt.Sprintf("--health-max-concurrent-workers=%v", maxConcurrentHealthWorkers),
			fmt.Sprintf("--health-sync-period=%v", healthSyncPeriod),
			"--leader-election=true",
			fmt.Sprintf("--leader-election-lease-duration=%v", leaseDuration),
			fmt.Sprintf("--leader-election-namespace=%v", deployNamespace),
			fmt.Sprintf("--leader-election-renew-deadline=%v", renewDeadline),
			fmt.Sprintf("--leader-election-retry-period=%v", retryPeriod),
			fmt.Sprintf("--max-concurrent-workers=%v", concurrentSyncs),
			fmt.Sprintf("--metrics-bind-address=:%v", metricsPort),
			fmt.Sprintf("--namespace=%v", watchedNamespace),
			fmt.Sprintf("--resource-class=%v", resourceClass),
			fmt.Sprintf("--sync-period=%v", syncPeriod),
			"--target-disable-cache",
			fmt.Sprintf("--target-kubeconfig=%v/%v", secretMountPath, secrets.DataKeyKubeconfig),
		}
		cmdWithoutKubeconfig = cmd[:len(cmd)-1]
		cmdWithoutWatchedNamespace = []string{"/gardener-resource-manager",
			"--always-update=true",
			fmt.Sprintf("--health-bind-address=:%v", healthPort),
			fmt.Sprintf("--health-max-concurrent-workers=%v", maxConcurrentHealthWorkers),
			fmt.Sprintf("--health-sync-period=%v", healthSyncPeriod),
			"--leader-election=true",
			fmt.Sprintf("--leader-election-lease-duration=%v", leaseDuration),
			fmt.Sprintf("--leader-election-namespace=%v", deployNamespace),
			fmt.Sprintf("--leader-election-renew-deadline=%v", renewDeadline),
			fmt.Sprintf("--leader-election-retry-period=%v", retryPeriod),
			fmt.Sprintf("--max-concurrent-workers=%v", concurrentSyncs),
			fmt.Sprintf("--metrics-bind-address=:%v", metricsPort),
			fmt.Sprintf("--resource-class=%v", resourceClass),
			fmt.Sprintf("--sync-period=%v", syncPeriod),
			"--target-disable-cache",
			fmt.Sprintf("--target-kubeconfig=%v/%v", secretMountPath, secrets.DataKeyKubeconfig),
		}

		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{Name: "gardener-resource-manager",
				Namespace: deployNamespace,
				Labels:    defaultLabels,
			},
		}
		deployment = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      constants.DeploymentNameGardenerResourceManager,
				Namespace: deployNamespace,
				Labels:    defaultLabels,
			},
			Spec: appsv1.DeploymentSpec{
				Replicas:             pointer.Int32Ptr(1),
				RevisionHistoryLimit: pointer.Int32Ptr(0),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "gardener-resource-manager",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							"checksum/secret-" + secretNameKubeconfig: secretChecksumKubeconfig,
						},
						Labels: map[string]string{
							"networking.gardener.cloud/to-dns":             "allowed",
							"networking.gardener.cloud/to-seed-apiserver":  "allowed",
							"networking.gardener.cloud/to-shoot-apiserver": "allowed",
							"fake-label-name":                              "fake-value",
							"app":                                          "gardener-resource-manager",
						},
					},
					Spec: corev1.PodSpec{
						ServiceAccountName: "gardener-resource-manager",
						Containers: []corev1.Container{
							{
								Command:         cmd,
								Image:           image,
								ImagePullPolicy: corev1.PullIfNotPresent,
								LivenessProbe: &corev1.Probe{
									Handler: corev1.Handler{
										HTTPGet: &corev1.HTTPGetAction{
											Path:   "/healthz",
											Scheme: "HTTP",
											Port:   intstr.FromInt(int(healthPort)),
										},
									},
									InitialDelaySeconds: 30,
									FailureThreshold:    5,
									PeriodSeconds:       10,
									SuccessThreshold:    1,
									TimeoutSeconds:      5,
								},
								Name: "gardener-resource-manager",
								Ports: []corev1.ContainerPort{
									{
										Name:          "metrics",
										ContainerPort: metricsPort,
										Protocol:      corev1.ProtocolTCP,
									},
									{
										Name:          "health",
										ContainerPort: healthPort,
										Protocol:      corev1.ProtocolTCP,
									},
								},
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("23m"),
										corev1.ResourceMemory: resource.MustParse("47Mi"),
									},
									Limits: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("400m"),
										corev1.ResourceMemory: resource.MustParse("512Mi"),
									},
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										MountPath: secretMountPath,
										Name:      "gardener-resource-manager",
										ReadOnly:  true,
									},
								},
							},
						},
						Volumes: []corev1.Volume{
							{
								Name: "gardener-resource-manager",
								VolumeSource: corev1.VolumeSource{
									Secret: &corev1.SecretVolumeSource{
										SecretName:  secretNameKubeconfig,
										DefaultMode: pointer.Int32Ptr(420),
									},
								},
							},
						},
					},
				},
			},
		}
		vpa = &autoscalingv1beta2.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{Name: "gardener-resource-manager",
				Namespace: deployNamespace,
				Labels:    defaultLabels},
			Spec: autoscalingv1beta2.VerticalPodAutoscalerSpec{
				TargetRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       "gardener-resource-manager",
				},
				UpdatePolicy: &autoscalingv1beta2.PodUpdatePolicy{
					UpdateMode: &updateMode,
				},
			},
		}

	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Deploy", func() {

		Context("kubeconfig is set; watched namespace is set", func() {
			BeforeEach(func() {
				cfg.ClusterRoleName = nil
				role.Namespace = watchedNamespace
				resourceManager = resourcemanager.New(c, deployNamespace, image, replicas, cfg)
			})
			It("should deploy a role in the watched namespace", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&corev1.ServiceAccount{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.ServiceAccount{})).Do(func(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) {
						Expect(obj).To(DeepEqual(serviceAccount))
					}),
					c.EXPECT().Get(ctx, kutil.Key(watchedNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&rbacv1.Role{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&rbacv1.Role{})).Do(func(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) {
						Expect(obj).To(DeepEqual(role))
					}),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&rbacv1.RoleBinding{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&rbacv1.RoleBinding{})).Do(func(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) {
						Expect(obj).To(DeepEqual(roleBinding))
					}),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&corev1.Service{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Service{})).Do(func(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) {
						Expect(obj).To(DeepEqual(service))
					}),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{})).Do(func(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) {
						Expect(obj).To(DeepEqual(deployment))
					}),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{})).Do(func(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) {
						Expect(obj).To(DeepEqual(vpa))
					}),
				)
				Expect(resourceManager.Deploy(ctx)).To(Succeed())
			})
			It("is also possible to set the kubeconfig via SetKubeConfig", func() {
				cfg.KubeConfig = nil
				rm := resourcemanager.New(c, deployNamespace, image, replicas, cfg)
				rm.SetKubeConfig(&kubeCfg)

				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&corev1.ServiceAccount{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.ServiceAccount{})).Do(func(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) {
						Expect(obj).To(DeepEqual(serviceAccount))
					}),
					c.EXPECT().Get(ctx, kutil.Key(watchedNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&rbacv1.Role{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&rbacv1.Role{})).Do(func(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) {
						Expect(obj).To(DeepEqual(role))
					}),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&rbacv1.RoleBinding{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&rbacv1.RoleBinding{})).Do(func(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) {
						Expect(obj).To(DeepEqual(roleBinding))
					}),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&corev1.Service{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Service{})).Do(func(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) {
						Expect(obj).To(DeepEqual(service))
					}),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{})).Do(func(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) {
						Expect(obj).To(DeepEqual(deployment))
					}),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{})).Do(func(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) {
						Expect(obj).To(DeepEqual(vpa))
					}),
				)
				Expect(rm.Deploy(ctx)).To(Succeed())
			})
			It("should fail because the service account cannot be updated", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&corev1.ServiceAccount{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.ServiceAccount{})).Return(fakeErr),
				)

				Expect(resourceManager.Deploy(ctx)).To(MatchError(fakeErr))
			})
			It("should fail because the Role cannot be created", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&corev1.ServiceAccount{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.ServiceAccount{})),
					c.EXPECT().Get(ctx, kutil.Key(watchedNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&rbacv1.Role{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&rbacv1.Role{})).Return(fakeErr),
				)

				Expect(resourceManager.Deploy(ctx)).To(MatchError(fakeErr))
			})
			It("should fail because the role binding cannot be created", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&corev1.ServiceAccount{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.ServiceAccount{})),
					c.EXPECT().Get(ctx, kutil.Key(watchedNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&rbacv1.Role{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&rbacv1.Role{})),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&rbacv1.RoleBinding{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&rbacv1.RoleBinding{})).Return(fakeErr),
				)

				Expect(resourceManager.Deploy(ctx)).To(MatchError(fakeErr))
			})
			It("should fail because the service cannot be created", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&corev1.ServiceAccount{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.ServiceAccount{})),
					c.EXPECT().Get(ctx, kutil.Key(watchedNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&rbacv1.Role{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&rbacv1.Role{})),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&rbacv1.RoleBinding{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&rbacv1.RoleBinding{})),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&corev1.Service{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Service{})).Return(fakeErr),
				)

				Expect(resourceManager.Deploy(ctx)).To(MatchError(fakeErr))
			})
			It("should fail because the deployment can not be created", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&corev1.ServiceAccount{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.ServiceAccount{})),
					c.EXPECT().Get(ctx, kutil.Key(watchedNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&rbacv1.Role{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&rbacv1.Role{})),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&rbacv1.RoleBinding{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&rbacv1.RoleBinding{})),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&corev1.Service{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Service{})),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{})).Return(fakeErr),
				)

				Expect(resourceManager.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the deployment can not be created", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&corev1.ServiceAccount{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.ServiceAccount{})),
					c.EXPECT().Get(ctx, kutil.Key(watchedNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&rbacv1.Role{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&rbacv1.Role{})),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&rbacv1.RoleBinding{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&rbacv1.RoleBinding{})),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&corev1.Service{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Service{})),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{})).Return(fakeErr),
				)

				Expect(resourceManager.Deploy(ctx)).To(MatchError(fakeErr))
			})
		})

		Context("kubeconfig is set; watched namespace is not set", func() {
			BeforeEach(func() {
				clusterRole.Rules = allowManagedResources
				cfg.WatchedNamespace = nil
				deployment.Spec.Template.Spec.Containers[0].Command = cmdWithoutWatchedNamespace

				resourceManager = resourcemanager.New(c, deployNamespace, image, replicas, cfg)
			})
			It("should deploy a Clusterrole allowing access to mr related resources", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&corev1.ServiceAccount{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.ServiceAccount{})).Do(func(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) {
						Expect(obj).To(DeepEqual(serviceAccount))
					}),
					c.EXPECT().Get(ctx, kutil.Key(clusterRoleName), gomock.AssignableToTypeOf(&rbacv1.ClusterRole{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRole{})).Do(func(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) {
						Expect(obj).To(DeepEqual(clusterRole))
					}),
					c.EXPECT().Get(ctx, kutil.Key(clusterRoleName), gomock.AssignableToTypeOf(&rbacv1.ClusterRoleBinding{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRoleBinding{})).Do(func(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) {
						Expect(obj).To(DeepEqual(clusterRoleBinding))
					}),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&corev1.Service{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Service{})).Do(func(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) {
						Expect(obj).To(DeepEqual(service))
					}),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{})).Do(func(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) {
						Expect(obj).To(DeepEqual(deployment))
					}),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{})).Do(func(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) {
						Expect(obj).To(DeepEqual(vpa))
					}),
				)
				Expect(resourceManager.Deploy(ctx)).To(Succeed())
			})
			It("should fail because the ClusterRole can not be created", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&corev1.ServiceAccount{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.ServiceAccount{})),
					c.EXPECT().Get(ctx, kutil.Key(clusterRoleName), gomock.AssignableToTypeOf(&rbacv1.ClusterRole{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRole{})).Return(fakeErr),
				)

				Expect(resourceManager.Deploy(ctx)).To(MatchError(fakeErr))
			})
			It("should fail because the ClusterRoleBinding can not be created", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&corev1.ServiceAccount{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.ServiceAccount{})),
					c.EXPECT().Get(ctx, kutil.Key(clusterRoleName), gomock.AssignableToTypeOf(&rbacv1.ClusterRole{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRole{})),
					c.EXPECT().Get(ctx, kutil.Key(clusterRoleName), gomock.AssignableToTypeOf(&rbacv1.ClusterRoleBinding{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRoleBinding{})).Return(fakeErr),
				)

				Expect(resourceManager.Deploy(ctx)).To(MatchError(fakeErr))
			})
		})
		Context("kubeconfig is not set", func() {
			BeforeEach(func() {
				clusterRole.Rules = allowAll
				deployment.Spec.Template.Spec.Containers[0].Command = cmdWithoutKubeconfig
				deployment.Spec.Template.Spec.Volumes = nil
				deployment.Spec.Template.Spec.Containers[0].VolumeMounts = nil
				delete(deployment.Spec.Template.ObjectMeta.Annotations, "checksum/secret-"+secretNameKubeconfig)
				cfg.KubeConfig = nil

				resourceManager = resourcemanager.New(c, deployNamespace, image, replicas, cfg)
			})
			It("should deploy a cluster role allowing all access", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&corev1.ServiceAccount{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.ServiceAccount{})).Do(func(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) {
						Expect(obj).To(DeepEqual(serviceAccount))
					}),
					c.EXPECT().Get(ctx, kutil.Key(clusterRoleName), gomock.AssignableToTypeOf(&rbacv1.ClusterRole{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRole{})).Do(func(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) {
						Expect(obj).To(DeepEqual(clusterRole))
					}),
					c.EXPECT().Get(ctx, kutil.Key(clusterRoleName), gomock.AssignableToTypeOf(&rbacv1.ClusterRoleBinding{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRoleBinding{})).Do(func(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) {
						Expect(obj).To(DeepEqual(clusterRoleBinding))
					}),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&corev1.Service{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Service{})).Do(func(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) {
						Expect(obj).To(DeepEqual(service))
					}),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{})).Do(func(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) {
						Expect(obj).To(DeepEqual(deployment))
					}),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{})).Do(func(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) {
						Expect(obj).To(DeepEqual(vpa))
					}),
				)
				Expect(resourceManager.Deploy(ctx)).To(Succeed())
			})
		})
	})

	Describe("#Destroy", func() {
		Context("kubeconfig is set; watched namespace is set", func() {
			BeforeEach(func() {
				cfg.ClusterRoleName = nil
				resourceManager = resourcemanager.New(c, deployNamespace, image, replicas, cfg)
			})
			It("should delete all created resources", func() {
				gomock.InOrder(
					c.EXPECT().Delete(ctx, &autoscalingv1beta2.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
				)

				Expect(resourceManager.Destroy(ctx)).To(Succeed())
			})
			It("should fail because the vpa cannot be deleted", func() {
				gomock.InOrder(
					c.EXPECT().Delete(ctx, &autoscalingv1beta2.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}).Return(fakeErr),
				)

				Expect(resourceManager.Destroy(ctx)).To(MatchError(fakeErr))
			})
			It("should fail because the deployment cannot be deleted", func() {
				gomock.InOrder(
					c.EXPECT().Delete(ctx, &autoscalingv1beta2.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}).Return(fakeErr),
				)

				Expect(resourceManager.Destroy(ctx)).To(MatchError(fakeErr))
			})
			It("should fail because the service cannot be deleted", func() {
				gomock.InOrder(
					c.EXPECT().Delete(ctx, &autoscalingv1beta2.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}).Return(fakeErr),
				)

				Expect(resourceManager.Destroy(ctx)).To(MatchError(fakeErr))
			})
			It("should fail because the service account cannot be deleted", func() {
				gomock.InOrder(
					c.EXPECT().Delete(ctx, &autoscalingv1beta2.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}).Return(fakeErr),
				)

				Expect(resourceManager.Destroy(ctx)).To(MatchError(fakeErr))
			})
			It("should fail because the role cannot be deleted", func() {
				gomock.InOrder(
					c.EXPECT().Delete(ctx, &autoscalingv1beta2.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}).Return(fakeErr),
				)

				Expect(resourceManager.Destroy(ctx)).To(MatchError(fakeErr))
			})
			It("should fail because the role binding account cannot be deleted", func() {
				gomock.InOrder(
					c.EXPECT().Delete(ctx, &autoscalingv1beta2.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}).Return(fakeErr),
				)

				Expect(resourceManager.Destroy(ctx)).To(MatchError(fakeErr))
			})

		})

		Context("kubeconfig is set; watched namespace is not set", func() {
			BeforeEach(func() {
				cfg.WatchedNamespace = nil
				deployment.Spec.Template.Spec.Containers[0].Command = cmdWithoutWatchedNamespace
				resourceManager = resourcemanager.New(c, deployNamespace, image, replicas, cfg)
			})
			It("should delete all created resources", func() {
				gomock.InOrder(
					c.EXPECT().Delete(ctx, &autoscalingv1beta2.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: clusterRoleName}}),
					c.EXPECT().Delete(ctx, &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: clusterRoleName}}),
				)

				Expect(resourceManager.Destroy(ctx)).To(Succeed())
			})
			It("should fail because the cluster role cannot be deleted", func() {
				gomock.InOrder(
					c.EXPECT().Delete(ctx, &autoscalingv1beta2.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: clusterRoleName}}).Return(fakeErr),
				)

				Expect(resourceManager.Destroy(ctx)).To(MatchError(fakeErr))
			})
			It("should fail because the cluster role binding cannot be deleted", func() {
				gomock.InOrder(
					c.EXPECT().Delete(ctx, &autoscalingv1beta2.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: clusterRoleName}}),
					c.EXPECT().Delete(ctx, &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: clusterRoleName}}).Return(fakeErr),
				)

				Expect(resourceManager.Destroy(ctx)).To(MatchError(fakeErr))
			})
		})

		Context("kubeconfig is not set", func() {
			BeforeEach(func() {
				cfg.KubeConfig = nil
				deployment.Spec.Template.Spec.Containers[0].Command = cmdWithoutKubeconfig
				deployment.Spec.Template.Spec.Volumes = nil
				deployment.Spec.Template.Spec.Containers[0].VolumeMounts = nil

				resourceManager = resourcemanager.New(c, deployNamespace, image, replicas, cfg)
			})
			It("should delete all created resources", func() {
				gomock.InOrder(
					c.EXPECT().Delete(ctx, &autoscalingv1beta2.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: clusterRoleName}}),
					c.EXPECT().Delete(ctx, &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: clusterRoleName}}),
				)

				Expect(resourceManager.Destroy(ctx)).To(Succeed())
			})
		})
	})

	Describe("#Wait", func() {
		It("should return nil as it's not implemented as of now", func() {
			Expect(resourceManager.Wait(ctx)).To(Succeed())
		})
	})

	Describe("#WaitCleanup", func() {
		It("should return nil as it's not implemented as of now", func() {
			Expect(resourceManager.WaitCleanup(ctx)).To(Succeed())
		})
	})
})
