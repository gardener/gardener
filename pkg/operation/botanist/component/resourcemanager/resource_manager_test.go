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

package resourcemanager_test

import (
	"context"
	"fmt"
	"time"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/resourcemanager"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	autoscalingv1beta2 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1beta2"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("ResourceManager", func() {
	var (
		ctrl            *gomock.Controller
		c               *mockclient.MockClient
		resourceManager Interface

		ctx                   = context.TODO()
		deployNamespace       = "fake-ns"
		fakeErr               = fmt.Errorf("fake error")
		image                 = "fake-image"
		replicas        int32 = 1
		healthPort      int32 = 8081
		metricsPort     int32 = 8080
		serverPort            = 9449

		// optional configuration
		clusterIdentity                     = "foo"
		secretNameKubeconfig                = "kubeconfig-secret"
		secretNameServer                    = "server-secret"
		secretMountPathKubeconfig           = "/etc/gardener-resource-manager"
		secretMountPathServer               = "/etc/gardener-resource-manager-tls"
		secretMountPathAPIAccess            = "/var/run/secrets/kubernetes.io/serviceaccount"
		secretChecksumKubeconfig            = "1234"
		secretChecksumServer                = "5678"
		secrets                             Secrets
		alwaysUpdate                              = true
		concurrentSyncs                     int32 = 20
		clusterRoleName                           = "gardener-resource-manager-seed"
		serviceAccountSecretName                  = "sa-secret"
		healthSyncPeriod                          = time.Minute
		leaseDuration                             = time.Second * 40
		maxConcurrentHealthWorkers          int32 = 20
		maxConcurrentTokenRequestorWorkers  int32 = 21
		maxConcurrentRootCAPublisherWorkers int32 = 22
		renewDeadline                             = time.Second * 10
		resourceClass                             = "fake-ResourceClass"
		retryPeriod                               = time.Second * 20
		syncPeriod                                = time.Second * 80
		watchedNamespace                          = "fake-ns"
		targetDisableCache                        = true
		maxUnavailable                            = intstr.FromInt(1)

		allowAll                   []rbacv1.PolicyRule
		allowManagedResources      []rbacv1.PolicyRule
		cfg                        Values
		clusterRole                *rbacv1.ClusterRole
		clusterRoleBinding         *rbacv1.ClusterRoleBinding
		cmd                        []string
		cmdWithoutWatchedNamespace []string
		deployment                 *appsv1.Deployment
		defaultLabels              map[string]string
		roleBinding                *rbacv1.RoleBinding
		role                       *rbacv1.Role
		service                    *corev1.Service
		serviceAccount             *corev1.ServiceAccount
		updateMode                 = autoscalingv1beta2.UpdateModeAuto
		pdb                        *policyv1beta1.PodDisruptionBudget
		vpa                        *autoscalingv1beta2.VerticalPodAutoscaler
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)

		secrets = Secrets{
			Kubeconfig: component.Secret{Name: secretNameKubeconfig, Checksum: secretChecksumKubeconfig},
			Server:     component.Secret{Name: secretNameServer, Checksum: secretChecksumServer},
		}
		allowAll = []rbacv1.PolicyRule{{
			APIGroups: []string{"*"},
			Resources: []string{"*"},
			Verbs:     []string{"*"},
		}}
		allowManagedResources = []rbacv1.PolicyRule{
			{
				APIGroups: []string{"resources.gardener.cloud"},
				Resources: []string{"managedresources", "managedresources/status"},
				Verbs:     []string{"get", "list", "watch", "update", "patch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"secrets"},
				Verbs:     []string{"get", "list", "watch", "update", "patch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"configmaps", "events"},
				Verbs:     []string{"create"},
			},
			{
				APIGroups:     []string{""},
				Resources:     []string{"configmaps"},
				ResourceNames: []string{"gardener-resource-manager"},
				Verbs:         []string{"get", "watch", "update", "patch"},
			},
			{
				APIGroups: []string{"coordination.k8s.io"},
				Resources: []string{"leases"},
				Verbs:     []string{"create"},
			},
			{
				APIGroups:     []string{"coordination.k8s.io"},
				Resources:     []string{"leases"},
				ResourceNames: []string{"gardener-resource-manager"},
				Verbs:         []string{"get", "watch", "update"},
			},
		}
		defaultLabels = map[string]string{
			v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
			v1beta1constants.LabelApp:   "gardener-resource-manager",
		}

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
				Type: corev1.ServiceTypeClusterIP,
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
					{
						Name:       "server",
						Port:       443,
						TargetPort: intstr.FromInt(serverPort),
						Protocol:   corev1.ProtocolTCP,
					},
				},
			},
		}
		cfg = Values{
			AlwaysUpdate:                        &alwaysUpdate,
			ClusterIdentity:                     &clusterIdentity,
			ConcurrentSyncs:                     &concurrentSyncs,
			HealthSyncPeriod:                    &healthSyncPeriod,
			LeaseDuration:                       &leaseDuration,
			MaxConcurrentHealthWorkers:          &maxConcurrentHealthWorkers,
			MaxConcurrentTokenRequestorWorkers:  &maxConcurrentTokenRequestorWorkers,
			MaxConcurrentRootCAPublisherWorkers: &maxConcurrentRootCAPublisherWorkers,
			RenewDeadline:                       &renewDeadline,
			ResourceClass:                       &resourceClass,
			RetryPeriod:                         &retryPeriod,
			SyncPeriod:                          &syncPeriod,
			TargetDisableCache:                  &targetDisableCache,
			WatchedNamespace:                    &watchedNamespace,
		}
		resourceManager = New(c, deployNamespace, image, replicas, cfg)
		resourceManager.SetSecrets(secrets)

		cmd = []string{"/gardener-resource-manager",
			"--always-update=true",
			"--cluster-id=" + clusterIdentity,
			"--garbage-collector-sync-period=12h",
			fmt.Sprintf("--health-bind-address=:%v", healthPort),
			fmt.Sprintf("--health-max-concurrent-workers=%v", maxConcurrentHealthWorkers),
			fmt.Sprintf("--token-requestor-max-concurrent-workers=%v", maxConcurrentTokenRequestorWorkers),
			fmt.Sprintf("--root-ca-publisher-max-concurrent-workers=%v", maxConcurrentRootCAPublisherWorkers),
			fmt.Sprintf("--root-ca-file=%s/ca.crt", secretMountPathAPIAccess),
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
			fmt.Sprintf("--port=%d", serverPort),
			fmt.Sprintf("--tls-cert-dir=%v", secretMountPathServer),
			fmt.Sprintf("--target-kubeconfig=%v/%v", secretMountPathKubeconfig, secretsutils.DataKeyKubeconfig),
		}
		cmdWithoutWatchedNamespace = []string{"/gardener-resource-manager",
			"--always-update=true",
			"--cluster-id=" + clusterIdentity,
			"--garbage-collector-sync-period=12h",
			fmt.Sprintf("--health-bind-address=:%v", healthPort),
			fmt.Sprintf("--health-max-concurrent-workers=%v", maxConcurrentHealthWorkers),
			fmt.Sprintf("--token-requestor-max-concurrent-workers=%v", maxConcurrentTokenRequestorWorkers),
			fmt.Sprintf("--root-ca-publisher-max-concurrent-workers=%v", maxConcurrentRootCAPublisherWorkers),
			fmt.Sprintf("--root-ca-file=%s/ca.crt", secretMountPathAPIAccess),
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
			fmt.Sprintf("--port=%d", serverPort),
			fmt.Sprintf("--tls-cert-dir=%v", secretMountPathServer),
			fmt.Sprintf("--target-kubeconfig=%v/%v", secretMountPathKubeconfig, secretsutils.DataKeyKubeconfig),
		}

		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{Name: "gardener-resource-manager",
				Namespace: deployNamespace,
				Labels:    defaultLabels,
			},
			AutomountServiceAccountToken: pointer.Bool(false),
		}
		deployment = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      v1beta1constants.DeploymentNameGardenerResourceManager,
				Namespace: deployNamespace,
				Labels:    defaultLabels,
			},
			Spec: appsv1.DeploymentSpec{
				Replicas:             pointer.Int32(1),
				RevisionHistoryLimit: pointer.Int32(1),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "gardener-resource-manager",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							"checksum/secret-" + secretNameKubeconfig: secretChecksumKubeconfig,
							"checksum/secret-" + secretNameServer:     secretChecksumServer,
						},
						Labels: map[string]string{
							"projected-token-mount.resources.gardener.cloud/skip": "true",
							"networking.gardener.cloud/to-dns":                    "allowed",
							"networking.gardener.cloud/to-seed-apiserver":         "allowed",
							"networking.gardener.cloud/to-shoot-apiserver":        "allowed",
							v1beta1constants.GardenRole:                           v1beta1constants.GardenRoleControlPlane,
							v1beta1constants.LabelApp:                             "gardener-resource-manager",
						},
					},
					Spec: corev1.PodSpec{
						Affinity: &corev1.Affinity{
							PodAntiAffinity: &corev1.PodAntiAffinity{
								PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
									{
										Weight: 100,
										PodAffinityTerm: corev1.PodAffinityTerm{
											TopologyKey: corev1.LabelHostname,
											LabelSelector: &metav1.LabelSelector{
												MatchLabels: map[string]string{
													v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
													v1beta1constants.LabelApp:   "gardener-resource-manager",
												},
											},
										},
									},
								},
							},
						},
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
										MountPath: secretMountPathAPIAccess,
										Name:      "kube-api-access-gardener",
										ReadOnly:  true,
									},
									{
										MountPath: secretMountPathServer,
										Name:      "tls",
										ReadOnly:  true,
									},
									{
										MountPath: secretMountPathKubeconfig,
										Name:      "gardener-resource-manager",
										ReadOnly:  true,
									},
								},
							},
						},
						Volumes: []corev1.Volume{
							{
								Name: "kube-api-access-gardener",
								VolumeSource: corev1.VolumeSource{
									Projected: &corev1.ProjectedVolumeSource{
										DefaultMode: pointer.Int32(420),
										Sources: []corev1.VolumeProjection{
											{
												ServiceAccountToken: &corev1.ServiceAccountTokenProjection{
													ExpirationSeconds: pointer.Int64(43200),
													Path:              "token",
												},
											},
											{
												Secret: &corev1.SecretProjection{
													LocalObjectReference: corev1.LocalObjectReference{
														Name: serviceAccountSecretName,
													},
													Items: []corev1.KeyToPath{{
														Key:  "ca.crt",
														Path: "ca.crt",
													}},
												},
											},
											{
												DownwardAPI: &corev1.DownwardAPIProjection{
													Items: []corev1.DownwardAPIVolumeFile{{
														FieldRef: &corev1.ObjectFieldSelector{
															APIVersion: "v1",
															FieldPath:  "metadata.namespace",
														},
														Path: "namespace",
													}},
												},
											},
										},
									},
								},
							},
							{
								Name: "tls",
								VolumeSource: corev1.VolumeSource{
									Secret: &corev1.SecretVolumeSource{
										SecretName:  secretNameServer,
										DefaultMode: pointer.Int32(420),
									},
								},
							},
							{
								Name: "gardener-resource-manager",
								VolumeSource: corev1.VolumeSource{
									Secret: &corev1.SecretVolumeSource{
										SecretName:  secretNameKubeconfig,
										DefaultMode: pointer.Int32(420),
									},
								},
							},
						},
					},
				},
			},
		}
		vpa = &autoscalingv1beta2.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener-resource-manager-vpa",
				Namespace: deployNamespace,
				Labels:    defaultLabels,
			},
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
		pdb = &policyv1beta1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener-resource-manager",
				Namespace: deployNamespace,
				Labels:    defaultLabels,
			},
			Spec: policyv1beta1.PodDisruptionBudgetSpec{
				MaxUnavailable: &maxUnavailable,
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
						v1beta1constants.LabelApp:   "gardener-resource-manager",
					},
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
				role.Namespace = watchedNamespace
				resourceManager = New(c, deployNamespace, image, replicas, cfg)
				resourceManager.SetSecrets(secrets)
			})

			It("should deploy a role in the watched namespace", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&corev1.ServiceAccount{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.ServiceAccount{}), gomock.Any()).
						Do(func(ctx context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(serviceAccount))
						}),
					c.EXPECT().Get(ctx, kutil.Key(watchedNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&rbacv1.Role{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&rbacv1.Role{}), gomock.Any()).
						Do(func(ctx context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(role))
						}),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&rbacv1.RoleBinding{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&rbacv1.RoleBinding{}), gomock.Any()).
						Do(func(ctx context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(roleBinding))
						}),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&corev1.Service{})).Times(2),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Service{}), gomock.Any()).
						Do(func(ctx context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(service))
						}),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, serviceAccount.Name), gomock.AssignableToTypeOf(&corev1.ServiceAccount{})).DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj *corev1.ServiceAccount) error {
						(&corev1.ServiceAccount{Secrets: []corev1.ObjectReference{{Name: serviceAccountSecretName}}}).DeepCopyInto(obj)
						return nil
					}),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{}), gomock.Any()).
						Do(func(ctx context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(deployment))
						}),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, pdb.Name), gomock.AssignableToTypeOf(&policyv1beta1.PodDisruptionBudget{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&policyv1beta1.PodDisruptionBudget{}), gomock.Any()).
						Do(func(ctx context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(pdb))
						}),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager-vpa"), gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{}), gomock.Any()).
						Do(func(ctx context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(vpa))
						}),
				)
				Expect(resourceManager.Deploy(ctx)).To(Succeed())
			})

			It("should fail because the service account cannot be updated", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&corev1.ServiceAccount{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.ServiceAccount{}), gomock.Any()).Return(fakeErr),
				)

				Expect(resourceManager.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the Role cannot be created", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&corev1.ServiceAccount{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.ServiceAccount{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(watchedNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&rbacv1.Role{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&rbacv1.Role{}), gomock.Any()).Return(fakeErr),
				)

				Expect(resourceManager.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the role binding cannot be created", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&corev1.ServiceAccount{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.ServiceAccount{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(watchedNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&rbacv1.Role{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&rbacv1.Role{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&rbacv1.RoleBinding{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&rbacv1.RoleBinding{}), gomock.Any()).Return(fakeErr),
				)

				Expect(resourceManager.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the service cannot be created", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&corev1.ServiceAccount{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.ServiceAccount{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(watchedNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&rbacv1.Role{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&rbacv1.Role{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&rbacv1.RoleBinding{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&rbacv1.RoleBinding{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&corev1.Service{})).Times(2),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Service{}), gomock.Any()).Return(fakeErr),
				)

				Expect(resourceManager.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the service account has no token yet", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&corev1.ServiceAccount{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.ServiceAccount{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(watchedNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&rbacv1.Role{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&rbacv1.Role{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&rbacv1.RoleBinding{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&rbacv1.RoleBinding{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&corev1.Service{})).Times(2),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Service{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, serviceAccount.Name), gomock.AssignableToTypeOf(&corev1.ServiceAccount{})),
				)

				Expect(resourceManager.Deploy(ctx)).To(MatchError(ContainSubstring("has no secrets yet")))
			})

			It("should fail because the deployment can not be created", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&corev1.ServiceAccount{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.ServiceAccount{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(watchedNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&rbacv1.Role{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&rbacv1.Role{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&rbacv1.RoleBinding{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&rbacv1.RoleBinding{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&corev1.Service{})).Times(2),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Service{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, serviceAccount.Name), gomock.AssignableToTypeOf(&corev1.ServiceAccount{})).DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj *corev1.ServiceAccount) error {
						(&corev1.ServiceAccount{Secrets: []corev1.ObjectReference{{Name: serviceAccountSecretName}}}).DeepCopyInto(obj)
						return nil
					}),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{}), gomock.Any()).Return(fakeErr),
				)

				Expect(resourceManager.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the PDB cannot be created", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&corev1.ServiceAccount{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.ServiceAccount{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(watchedNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&rbacv1.Role{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&rbacv1.Role{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&rbacv1.RoleBinding{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&rbacv1.RoleBinding{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&corev1.Service{})).Times(2),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Service{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, serviceAccount.Name), gomock.AssignableToTypeOf(&corev1.ServiceAccount{})).DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj *corev1.ServiceAccount) error {
						(&corev1.ServiceAccount{Secrets: []corev1.ObjectReference{{Name: serviceAccountSecretName}}}).DeepCopyInto(obj)
						return nil
					}),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, pdb.Name), gomock.AssignableToTypeOf(&policyv1beta1.PodDisruptionBudget{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&policyv1beta1.PodDisruptionBudget{}), gomock.Any()).Return(fakeErr),
				)

				Expect(resourceManager.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the VPA can not be created", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&corev1.ServiceAccount{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.ServiceAccount{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(watchedNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&rbacv1.Role{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&rbacv1.Role{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&rbacv1.RoleBinding{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&rbacv1.RoleBinding{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&corev1.Service{})).Times(2),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Service{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, serviceAccount.Name), gomock.AssignableToTypeOf(&corev1.ServiceAccount{})).DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj *corev1.ServiceAccount) error {
						(&corev1.ServiceAccount{Secrets: []corev1.ObjectReference{{Name: serviceAccountSecretName}}}).DeepCopyInto(obj)
						return nil
					}),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, pdb.Name), gomock.AssignableToTypeOf(&policyv1beta1.PodDisruptionBudget{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&policyv1beta1.PodDisruptionBudget{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager-vpa"), gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{}), gomock.Any()).Return(fakeErr),
				)

				Expect(resourceManager.Deploy(ctx)).To(MatchError(fakeErr))
			})
		})

		Context("kubeconfig is set; watched namespace is not set", func() {
			BeforeEach(func() {
				clusterRole.Rules = allowManagedResources
				cfg.WatchedNamespace = nil
				deployment.Spec.Template.Spec.Containers[0].Command = cmdWithoutWatchedNamespace

				resourceManager = New(c, deployNamespace, image, replicas, cfg)
				resourceManager.SetSecrets(secrets)
			})

			It("should deploy a ClusterRole allowing access to mr related resources", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&corev1.ServiceAccount{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.ServiceAccount{}), gomock.Any()).
						Do(func(ctx context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(serviceAccount))
						}),
					c.EXPECT().Get(ctx, kutil.Key(clusterRoleName), gomock.AssignableToTypeOf(&rbacv1.ClusterRole{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRole{}), gomock.Any()).
						Do(func(ctx context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(clusterRole))
						}),
					c.EXPECT().Get(ctx, kutil.Key(clusterRoleName), gomock.AssignableToTypeOf(&rbacv1.ClusterRoleBinding{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRoleBinding{}), gomock.Any()).
						Do(func(ctx context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(clusterRoleBinding))
						}),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&corev1.Service{})).Times(2),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Service{}), gomock.Any()).
						Do(func(ctx context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(service))
						}),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, serviceAccount.Name), gomock.AssignableToTypeOf(&corev1.ServiceAccount{})).DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj *corev1.ServiceAccount) error {
						(&corev1.ServiceAccount{Secrets: []corev1.ObjectReference{{Name: serviceAccountSecretName}}}).DeepCopyInto(obj)
						return nil
					}),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{}), gomock.Any()).
						Do(func(ctx context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(deployment))
						}),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, pdb.Name), gomock.AssignableToTypeOf(&policyv1beta1.PodDisruptionBudget{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&policyv1beta1.PodDisruptionBudget{}), gomock.Any()).
						Do(func(ctx context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(pdb))
						}),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager-vpa"), gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{}), gomock.Any()).
						Do(func(ctx context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(vpa))
						}),
				)
				Expect(resourceManager.Deploy(ctx)).To(Succeed())
			})

			It("should fail because the ClusterRole can not be created", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&corev1.ServiceAccount{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.ServiceAccount{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(clusterRoleName), gomock.AssignableToTypeOf(&rbacv1.ClusterRole{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRole{}), gomock.Any()).Return(fakeErr),
				)

				Expect(resourceManager.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the ClusterRoleBinding can not be created", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&corev1.ServiceAccount{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.ServiceAccount{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(clusterRoleName), gomock.AssignableToTypeOf(&rbacv1.ClusterRole{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRole{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(clusterRoleName), gomock.AssignableToTypeOf(&rbacv1.ClusterRoleBinding{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRoleBinding{}), gomock.Any()).Return(fakeErr),
				)

				Expect(resourceManager.Deploy(ctx)).To(MatchError(fakeErr))
			})
		})

		Context("kubeconfig is not set", func() {
			BeforeEach(func() {
				clusterRole.Rules = allowAll

				deployment.Spec.Template.Spec.Containers[0].Command = cmd[:len(cmd)-1]
				deployment.Spec.Template.Spec.Volumes = deployment.Spec.Template.Spec.Volumes[:len(deployment.Spec.Template.Spec.Volumes)-1]
				deployment.Spec.Template.Spec.Containers[0].VolumeMounts = deployment.Spec.Template.Spec.Containers[0].VolumeMounts[:len(deployment.Spec.Template.Spec.Containers[0].VolumeMounts)-1]
				delete(deployment.Spec.Template.ObjectMeta.Annotations, "checksum/secret-"+secretNameKubeconfig)
				deployment.Spec.Template.Labels["gardener.cloud/role"] = "seed"
				deployment.Spec.Template.Spec.Affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].PodAffinityTerm.LabelSelector.MatchLabels["gardener.cloud/role"] = "seed"
				pdb.Spec.Selector.MatchLabels["gardener.cloud/role"] = "seed"

				// Remove controlplane label from resources
				delete(serviceAccount.ObjectMeta.Labels, v1beta1constants.GardenRole)
				delete(clusterRole.ObjectMeta.Labels, v1beta1constants.GardenRole)
				delete(clusterRoleBinding.ObjectMeta.Labels, v1beta1constants.GardenRole)
				delete(service.ObjectMeta.Labels, v1beta1constants.GardenRole)
				delete(deployment.ObjectMeta.Labels, v1beta1constants.GardenRole)
				delete(vpa.ObjectMeta.Labels, v1beta1constants.GardenRole)
				delete(pdb.ObjectMeta.Labels, v1beta1constants.GardenRole)
				// Remove networking label from deployment template
				delete(deployment.Spec.Template.Labels, "networking.gardener.cloud/to-dns")
				delete(deployment.Spec.Template.Labels, "networking.gardener.cloud/to-seed-apiserver")
				delete(deployment.Spec.Template.Labels, "networking.gardener.cloud/to-shoot-apiserver")

				secrets.Kubeconfig.Name, secrets.Kubeconfig.Checksum = "", ""
				resourceManager = New(c, deployNamespace, image, replicas, cfg)
				resourceManager.SetSecrets(secrets)
			})

			It("should deploy a cluster role allowing all access", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&corev1.ServiceAccount{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.ServiceAccount{}), gomock.Any()).
						Do(func(ctx context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(serviceAccount))
						}),
					c.EXPECT().Get(ctx, kutil.Key(clusterRoleName), gomock.AssignableToTypeOf(&rbacv1.ClusterRole{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRole{}), gomock.Any()).
						Do(func(ctx context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(clusterRole))
						}),
					c.EXPECT().Get(ctx, kutil.Key(clusterRoleName), gomock.AssignableToTypeOf(&rbacv1.ClusterRoleBinding{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRoleBinding{}), gomock.Any()).
						Do(func(ctx context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(clusterRoleBinding))
						}),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&corev1.Service{})).Times(2),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Service{}), gomock.Any()).
						Do(func(ctx context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(service))
						}),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, serviceAccount.Name), gomock.AssignableToTypeOf(&corev1.ServiceAccount{})).DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj *corev1.ServiceAccount) error {
						(&corev1.ServiceAccount{Secrets: []corev1.ObjectReference{{Name: serviceAccountSecretName}}}).DeepCopyInto(obj)
						return nil
					}),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager"), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{}), gomock.Any()).
						Do(func(ctx context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(deployment))
						}),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, pdb.Name), gomock.AssignableToTypeOf(&policyv1beta1.PodDisruptionBudget{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&policyv1beta1.PodDisruptionBudget{}), gomock.Any()).
						Do(func(ctx context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(pdb))
						}),
					c.EXPECT().Get(ctx, kutil.Key(deployNamespace, "gardener-resource-manager-vpa"), gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{}), gomock.Any()).
						Do(func(ctx context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(vpa))
						}),
				)
				Expect(resourceManager.Deploy(ctx)).To(Succeed())
			})
		})
	})

	Describe("#Destroy", func() {
		Context("watched namespace is set", func() {
			BeforeEach(func() {
				resourceManager = New(c, deployNamespace, image, replicas, cfg)
			})

			It("should delete all created resources", func() {
				gomock.InOrder(
					c.EXPECT().Delete(ctx, &policyv1beta1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &autoscalingv1beta2.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager-vpa"}}),
					c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: clusterRoleName}}),
					c.EXPECT().Delete(ctx, &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: clusterRoleName}}),
					c.EXPECT().Delete(ctx, &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
				)

				Expect(resourceManager.Destroy(ctx)).To(Succeed())
			})

			It("should fail because the pdb cannot be deleted", func() {
				gomock.InOrder(
					c.EXPECT().Delete(ctx, &policyv1beta1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}).Return(fakeErr),
				)

				Expect(resourceManager.Destroy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the vpa cannot be deleted", func() {
				gomock.InOrder(
					c.EXPECT().Delete(ctx, &policyv1beta1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &autoscalingv1beta2.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager-vpa"}}).Return(fakeErr),
				)

				Expect(resourceManager.Destroy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the deployment cannot be deleted", func() {
				gomock.InOrder(
					c.EXPECT().Delete(ctx, &policyv1beta1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &autoscalingv1beta2.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager-vpa"}}),
					c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}).Return(fakeErr),
				)

				Expect(resourceManager.Destroy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the service cannot be deleted", func() {
				gomock.InOrder(
					c.EXPECT().Delete(ctx, &policyv1beta1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &autoscalingv1beta2.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager-vpa"}}),
					c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}).Return(fakeErr),
				)

				Expect(resourceManager.Destroy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the service account cannot be deleted", func() {
				gomock.InOrder(
					c.EXPECT().Delete(ctx, &policyv1beta1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &autoscalingv1beta2.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager-vpa"}}),
					c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}).Return(fakeErr),
				)

				Expect(resourceManager.Destroy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the cluster role cannot be deleted", func() {
				gomock.InOrder(
					c.EXPECT().Delete(ctx, &policyv1beta1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &autoscalingv1beta2.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager-vpa"}}),
					c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: clusterRoleName}}).Return(fakeErr),
				)

				Expect(resourceManager.Destroy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the cluster role binding cannot be deleted", func() {
				gomock.InOrder(
					c.EXPECT().Delete(ctx, &policyv1beta1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &autoscalingv1beta2.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager-vpa"}}),
					c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: clusterRoleName}}),
					c.EXPECT().Delete(ctx, &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: clusterRoleName}}).Return(fakeErr),
				)

				Expect(resourceManager.Destroy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the role cannot be deleted", func() {
				gomock.InOrder(
					c.EXPECT().Delete(ctx, &policyv1beta1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &autoscalingv1beta2.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager-vpa"}}),
					c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: clusterRoleName}}),
					c.EXPECT().Delete(ctx, &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: clusterRoleName}}),
					c.EXPECT().Delete(ctx, &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}).Return(fakeErr),
				)

				Expect(resourceManager.Destroy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the role binding cannot be deleted", func() {
				gomock.InOrder(
					c.EXPECT().Delete(ctx, &policyv1beta1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &autoscalingv1beta2.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager-vpa"}}),
					c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: clusterRoleName}}),
					c.EXPECT().Delete(ctx, &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: clusterRoleName}}),
					c.EXPECT().Delete(ctx, &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}).Return(fakeErr),
				)

				Expect(resourceManager.Destroy(ctx)).To(MatchError(fakeErr))
			})
		})

		Context("watched namespace is not set", func() {
			BeforeEach(func() {
				cfg.WatchedNamespace = nil
				deployment.Spec.Template.Spec.Containers[0].Command = cmdWithoutWatchedNamespace
				resourceManager = New(c, deployNamespace, image, replicas, cfg)
			})

			It("should delete all created resources", func() {
				gomock.InOrder(
					c.EXPECT().Delete(ctx, &policyv1beta1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &autoscalingv1beta2.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager-vpa"}}),
					c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: clusterRoleName}}),
					c.EXPECT().Delete(ctx, &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: clusterRoleName}}),
				)

				Expect(resourceManager.Destroy(ctx)).To(Succeed())
			})
		})
	})

	Describe("#Wait", func() {
		var fakeClient client.Client

		BeforeEach(func() {
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).Build()
			resourceManager = New(fakeClient, deployNamespace, image, replicas, cfg)
		})

		It("should successfully wait for the deployment to be ready", func() {
			defer test.WithVars(&IntervalWaitForDeployment, time.Millisecond)()
			defer test.WithVars(&TimeoutWaitForDeployment, 100*time.Millisecond)()

			Expect(fakeClient.Create(ctx, deployment)).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())

			timer := time.AfterFunc(10*time.Millisecond, func() {
				deployment.Status.Conditions = []appsv1.DeploymentCondition{
					{
						Type:   appsv1.DeploymentAvailable,
						Status: corev1.ConditionTrue,
					},
				}
				Expect(fakeClient.Status().Update(ctx, deployment)).To(Succeed())
			})
			defer timer.Stop()

			Expect(resourceManager.Wait(ctx)).To(Succeed())
		})

		It("should fail while waiting for the deployment to be ready", func() {
			defer test.WithVars(&IntervalWaitForDeployment, time.Millisecond)()
			defer test.WithVars(&TimeoutWaitForDeployment, 10*time.Millisecond)()

			deployment.Status.Conditions = []appsv1.DeploymentCondition{
				{
					Type:   appsv1.DeploymentAvailable,
					Status: corev1.ConditionFalse,
				},
			}

			Expect(fakeClient.Create(ctx, deployment)).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())

			Expect(resourceManager.Wait(ctx)).To(MatchError(ContainSubstring(`condition "Available" has invalid status False (expected True)`)))
		})
	})

	Describe("#WaitCleanup", func() {
		It("should return nil as it's not implemented as of now", func() {
			Expect(resourceManager.WaitCleanup(ctx)).To(Succeed())
		})
	})
})
