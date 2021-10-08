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

package clusterautoscaler_test

import (
	"context"
	"fmt"
	"time"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	autoscalingv1beta2 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1beta2"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/clusterautoscaler"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("ClusterAutoscaler", func() {
	var (
		ctrl              *gomock.Controller
		c                 *mockclient.MockClient
		clusterAutoscaler Interface

		ctx                = context.TODO()
		fakeErr            = fmt.Errorf("fake error")
		namespace          = "shoot--foo--bar"
		namespaceUID       = types.UID("1234567890")
		image              = "k8s.gcr.io/cluster-autoscaler:v1.2.3"
		replicas     int32 = 1

		machineDeployment1Name       = "pool1"
		machineDeployment1Min  int32 = 2
		machineDeployment1Max  int32 = 4
		machineDeployment2Name       = "pool2"
		machineDeployment2Min  int32 = 3
		machineDeployment2Max  int32 = 5
		machineDeployments           = []extensionsv1alpha1.MachineDeployment{
			{Name: machineDeployment1Name, Minimum: machineDeployment1Min, Maximum: machineDeployment1Max},
			{Name: machineDeployment2Name, Minimum: machineDeployment2Min, Maximum: machineDeployment2Max},
		}

		configExpander                            = gardencorev1beta1.ClusterAutoscalerExpanderRandom
		configMaxGracefulTerminationSeconds int32 = 60 * 60 * 24
		configMaxNodeProvisionTime                = &metav1.Duration{Duration: time.Second}
		configScaleDownDelayAfterAdd              = &metav1.Duration{Duration: time.Second}
		configScaleDownDelayAfterDelete           = &metav1.Duration{Duration: time.Second}
		configScaleDownDelayAfterFailure          = &metav1.Duration{Duration: time.Second}
		configScaleDownUnneededTime               = &metav1.Duration{Duration: time.Second}
		configScaleDownUtilizationThreshold       = pointer.Float64(1.2345)
		configScanInterval                        = &metav1.Duration{Duration: time.Second}
		configFull                                = &gardencorev1beta1.ClusterAutoscaler{
			Expander:                      &configExpander,
			MaxGracefulTerminationSeconds: &configMaxGracefulTerminationSeconds,
			MaxNodeProvisionTime:          configMaxNodeProvisionTime,
			ScaleDownDelayAfterAdd:        configScaleDownDelayAfterAdd,
			ScaleDownDelayAfterDelete:     configScaleDownDelayAfterDelete,
			ScaleDownDelayAfterFailure:    configScaleDownDelayAfterFailure,
			ScaleDownUnneededTime:         configScaleDownUnneededTime,
			ScaleDownUtilizationThreshold: configScaleDownUtilizationThreshold,
			ScanInterval:                  configScanInterval,
		}

		secretNameKubeconfig     = "kubeconfig-secret"
		secretChecksumKubeconfig = "1234"
		secrets                  = Secrets{Kubeconfig: component.Secret{Name: secretNameKubeconfig, Checksum: secretChecksumKubeconfig}}

		serviceAccountName        = "cluster-autoscaler"
		clusterRoleBindingName    = "cluster-autoscaler-" + namespace
		vpaName                   = "cluster-autoscaler-vpa"
		serviceName               = "cluster-autoscaler"
		deploymentName            = "cluster-autoscaler"
		managedResourceName       = "shoot-core-cluster-autoscaler"
		managedResourceSecretName = "managedresource-shoot-core-cluster-autoscaler"

		vpaUpdateMode = autoscalingv1beta2.UpdateModeAuto
		vpa           = &autoscalingv1beta2.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{Name: vpaName, Namespace: namespace},
			Spec: autoscalingv1beta2.VerticalPodAutoscalerSpec{
				TargetRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       deploymentName,
				},
				UpdatePolicy: &autoscalingv1beta2.PodUpdatePolicy{
					UpdateMode: &vpaUpdateMode,
				},
				ResourcePolicy: &autoscalingv1beta2.PodResourcePolicy{
					ContainerPolicies: []autoscalingv1beta2.ContainerResourcePolicy{
						{
							ContainerName: autoscalingv1beta2.DefaultContainerResourcePolicy,
							MinAllowed: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("20m"),
								corev1.ResourceMemory: resource.MustParse("50Mi"),
							},
						},
					},
				},
			},
		}
		clusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterRoleBindingName,
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "v1",
					Kind:               "Namespace",
					Name:               namespace,
					UID:                namespaceUID,
					Controller:         pointer.Bool(true),
					BlockOwnerDeletion: pointer.Bool(true),
				}},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     "system:cluster-autoscaler-seed",
			},
			Subjects: []rbacv1.Subject{{
				Kind:      "ServiceAccount",
				Name:      serviceAccountName,
				Namespace: namespace,
			}},
		}
		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceAccountName,
				Namespace: namespace,
			},
		}
		service = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceName,
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "kubernetes",
					"role": "cluster-autoscaler",
				},
			},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{
					"app":  "kubernetes",
					"role": "cluster-autoscaler",
				},
				Type:      corev1.ServiceTypeClusterIP,
				ClusterIP: corev1.ClusterIPNone,
				Ports: []corev1.ServicePort{
					{
						Name:     "metrics",
						Protocol: corev1.ProtocolTCP,
						Port:     8085,
					},
				},
			},
		}
		deploymentFor = func(withConfig bool) *appsv1.Deployment {
			var commandConfigFlags []string
			if !withConfig {
				commandConfigFlags = append(commandConfigFlags,
					"--expander=least-waste",
					"--max-graceful-termination-sec=600",
					"--max-node-provision-time=20m0s",
					"--scale-down-utilization-threshold=0.500000",
					"--scale-down-unneeded-time=30m0s",
					"--scale-down-delay-after-add=1h0m0s",
					"--scale-down-delay-after-delete=0s",
					"--scale-down-delay-after-failure=3m0s",
					"--scan-interval=10s",
				)
			} else {
				commandConfigFlags = append(commandConfigFlags,
					fmt.Sprintf("--expander=%s", string(configExpander)),
					fmt.Sprintf("--max-graceful-termination-sec=%d", configMaxGracefulTerminationSeconds),
					fmt.Sprintf("--max-node-provision-time=%s", configMaxNodeProvisionTime.Duration),
					fmt.Sprintf("--scale-down-utilization-threshold=%f", *configScaleDownUtilizationThreshold),
					fmt.Sprintf("--scale-down-unneeded-time=%s", configScaleDownUnneededTime.Duration),
					fmt.Sprintf("--scale-down-delay-after-add=%s", configScaleDownDelayAfterAdd.Duration),
					fmt.Sprintf("--scale-down-delay-after-delete=%s", configScaleDownDelayAfterDelete.Duration),
					fmt.Sprintf("--scale-down-delay-after-failure=%s", configScaleDownDelayAfterFailure.Duration),
					fmt.Sprintf("--scan-interval=%s", configScanInterval.Duration),
				)
			}

			command := append([]string{
				"./cluster-autoscaler",
				"--address=:8085",
				"--kubeconfig=/var/lib/cluster-autoscaler/kubeconfig",
				"--cloud-provider=mcm",
				"--stderrthreshold=info",
				"--skip-nodes-with-system-pods=false",
				"--skip-nodes-with-local-storage=false",
				"--expendable-pods-priority-cutoff=-10",
				"--balance-similar-node-groups=true",
				"--v=2",
			}, commandConfigFlags...)
			command = append(command,
				fmt.Sprintf("--nodes=%d:%d:%s.%s", machineDeployment1Min, machineDeployment1Max, namespace, machineDeployment1Name),
				fmt.Sprintf("--nodes=%d:%d:%s.%s", machineDeployment2Min, machineDeployment2Max, namespace, machineDeployment2Name),
			)

			return &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      deploymentName,
					Namespace: namespace,
					Labels: map[string]string{
						"app":                 "kubernetes",
						"role":                "cluster-autoscaler",
						"gardener.cloud/role": "controlplane",
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas:             &replicas,
					RevisionHistoryLimit: pointer.Int32(1),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app":  "kubernetes",
							"role": "cluster-autoscaler",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								"checksum/secret-" + secretNameKubeconfig: secretChecksumKubeconfig,
							},
							Labels: map[string]string{
								"app":                              "kubernetes",
								"role":                             "cluster-autoscaler",
								"gardener.cloud/role":              "controlplane",
								"networking.gardener.cloud/to-dns": "allowed",
								"networking.gardener.cloud/to-seed-apiserver":  "allowed",
								"networking.gardener.cloud/to-shoot-apiserver": "allowed",
								"networking.gardener.cloud/from-prometheus":    "allowed",
							},
						},
						Spec: corev1.PodSpec{
							ServiceAccountName:            serviceAccountName,
							TerminationGracePeriodSeconds: pointer.Int64(5),
							Containers: []corev1.Container{
								{
									Name:            "cluster-autoscaler",
									Image:           image,
									ImagePullPolicy: corev1.PullIfNotPresent,
									Command:         command,
									Ports: []corev1.ContainerPort{
										{
											Name:          "metrics",
											ContainerPort: 8085,
											Protocol:      corev1.ProtocolTCP,
										},
									},
									Env: []corev1.EnvVar{
										{
											Name:  "CONTROL_NAMESPACE",
											Value: namespace,
										},
										{
											Name:  "TARGET_KUBECONFIG",
											Value: "/var/lib/cluster-autoscaler/kubeconfig",
										},
									},
									Resources: corev1.ResourceRequirements{
										Requests: corev1.ResourceList{
											corev1.ResourceCPU:    resource.MustParse("100m"),
											corev1.ResourceMemory: resource.MustParse("300Mi"),
										},
										Limits: corev1.ResourceList{
											corev1.ResourceCPU:    resource.MustParse("1"),
											corev1.ResourceMemory: resource.MustParse("3000Mi"),
										},
									},
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      secretNameKubeconfig,
											MountPath: "/var/lib/cluster-autoscaler",
											ReadOnly:  true,
										},
									},
								},
							},
							Volumes: []corev1.Volume{
								{
									Name: secretNameKubeconfig,
									VolumeSource: corev1.VolumeSource{
										Secret: &corev1.SecretVolumeSource{
											SecretName: secretNameKubeconfig,
										},
									},
								},
							},
						},
					},
				},
			}
		}

		clusterRoleYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  creationTimestamp: null
  name: system:cluster-autoscaler-shoot
rules:
- apiGroups:
  - ""
  resources:
  - events
  - endpoints
  verbs:
  - create
  - patch
- apiGroups:
  - ""
  resources:
  - pods/eviction
  - configmaps
  verbs:
  - create
- apiGroups:
  - ""
  resources:
  - pods/status
  verbs:
  - update
- apiGroups:
  - ""
  resourceNames:
  - cluster-autoscaler
  resources:
  - endpoints
  verbs:
  - get
  - update
- apiGroups:
  - ""
  resources:
  - nodes
  verbs:
  - watch
  - list
  - get
  - update
- apiGroups:
  - ""
  resources:
  - pods
  - services
  - replicationcontrollers
  - persistentvolumeclaims
  - persistentvolumes
  verbs:
  - watch
  - list
  - get
- apiGroups:
  - apps
  - extensions
  resources:
  - daemonsets
  - replicasets
  - statefulsets
  verbs:
  - watch
  - list
  - get
- apiGroups:
  - policy
  resources:
  - poddisruptionbudgets
  verbs:
  - watch
  - list
- apiGroups:
  - storage.k8s.io
  resources:
  - storageclasses
  - csinodes
  verbs:
  - watch
  - list
  - get
- apiGroups:
  - ""
  resourceNames:
  - cluster-autoscaler-status
  resources:
  - configmaps
  verbs:
  - delete
  - get
  - update
- apiGroups:
  - coordination.k8s.io
  resources:
  - leases
  verbs:
  - create
- apiGroups:
  - coordination.k8s.io
  resourceNames:
  - cluster-autoscaler
  resources:
  - leases
  verbs:
  - get
  - update
- apiGroups:
  - batch
  - extensions
  resources:
  - jobs
  verbs:
  - get
  - list
  - patch
  - watch
- apiGroups:
  - batch
  resources:
  - jobs
  - cronjobs
  verbs:
  - get
  - list
  - watch
`
		clusterRoleBindingYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  creationTimestamp: null
  name: system:cluster-autoscaler-shoot
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:cluster-autoscaler-shoot
subjects:
- kind: User
  name: system:cluster-autoscaler
`
		managedResourceSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceSecretName,
				Namespace: namespace,
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"clusterrole____system_cluster-autoscaler-shoot.yaml":        []byte(clusterRoleYAML),
				"clusterrolebinding____system_cluster-autoscaler-shoot.yaml": []byte(clusterRoleBindingYAML),
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
				KeepObjects:  pointer.Bool(false),
			},
		}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)

		clusterAutoscaler = New(c, namespace, image, replicas, nil)
		clusterAutoscaler.SetNamespaceUID(namespaceUID)
		clusterAutoscaler.SetMachineDeployments(machineDeployments)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Deploy", func() {
		Context("missing secret information", func() {
			It("should return an error because the kubeconfig secret information is not provided", func() {
				Expect(clusterAutoscaler.Deploy(ctx)).To(MatchError(ContainSubstring("missing kubeconfig secret information")))
			})
		})

		Context("secret information available", func() {
			BeforeEach(func() {
				clusterAutoscaler.SetSecrets(secrets)
			})

			It("should fail because the service account cannot be created", func() {
				gomock.InOrder(
					c.EXPECT().Create(ctx, serviceAccount).Return(fakeErr),
				)

				Expect(clusterAutoscaler.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the cluster role binding cannot be updated", func() {
				gomock.InOrder(
					c.EXPECT().Create(ctx, serviceAccount),
					c.EXPECT().Get(ctx, kutil.Key(clusterRoleBindingName), gomock.AssignableToTypeOf(&rbacv1.ClusterRoleBinding{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRoleBinding{}), gomock.Any()).Return(fakeErr),
				)

				Expect(clusterAutoscaler.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the service cannot be updated", func() {
				gomock.InOrder(
					c.EXPECT().Create(ctx, serviceAccount),
					c.EXPECT().Get(ctx, kutil.Key(clusterRoleBindingName), gomock.AssignableToTypeOf(&rbacv1.ClusterRoleBinding{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRoleBinding{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(namespace, serviceName), gomock.AssignableToTypeOf(&corev1.Service{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Service{}), gomock.Any()).Return(fakeErr),
				)

				Expect(clusterAutoscaler.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the deployment cannot be updated", func() {
				gomock.InOrder(
					c.EXPECT().Create(ctx, serviceAccount),
					c.EXPECT().Get(ctx, kutil.Key(clusterRoleBindingName), gomock.AssignableToTypeOf(&rbacv1.ClusterRoleBinding{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRoleBinding{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(namespace, serviceName), gomock.AssignableToTypeOf(&corev1.Service{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Service{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(namespace, deploymentName), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{}), gomock.Any()).Return(fakeErr),
				)

				Expect(clusterAutoscaler.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the vpa cannot be updated", func() {
				gomock.InOrder(
					c.EXPECT().Create(ctx, serviceAccount),
					c.EXPECT().Get(ctx, kutil.Key(clusterRoleBindingName), gomock.AssignableToTypeOf(&rbacv1.ClusterRoleBinding{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRoleBinding{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(namespace, serviceName), gomock.AssignableToTypeOf(&corev1.Service{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Service{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(namespace, deploymentName), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(namespace, vpaName), gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{}), gomock.Any()).Return(fakeErr),
				)

				Expect(clusterAutoscaler.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the managed resource secret cannot be updated", func() {
				gomock.InOrder(
					c.EXPECT().Create(ctx, serviceAccount),
					c.EXPECT().Get(ctx, kutil.Key(clusterRoleBindingName), gomock.AssignableToTypeOf(&rbacv1.ClusterRoleBinding{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRoleBinding{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(namespace, serviceName), gomock.AssignableToTypeOf(&corev1.Service{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Service{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(namespace, deploymentName), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(namespace, vpaName), gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(namespace, managedResourceSecretName), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Return(fakeErr),
				)

				Expect(clusterAutoscaler.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the managed resource cannot be updated", func() {
				gomock.InOrder(
					c.EXPECT().Create(ctx, serviceAccount),
					c.EXPECT().Get(ctx, kutil.Key(clusterRoleBindingName), gomock.AssignableToTypeOf(&rbacv1.ClusterRoleBinding{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRoleBinding{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(namespace, serviceName), gomock.AssignableToTypeOf(&corev1.Service{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Service{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(namespace, deploymentName), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(namespace, vpaName), gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(namespace, managedResourceSecretName), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, managedResourceName), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(fakeErr),
				)

				Expect(clusterAutoscaler.Deploy(ctx)).To(MatchError(fakeErr))
			})

			Context("should successfully deploy all the resources", func() {
				test := func(withConfig bool) {
					var config *gardencorev1beta1.ClusterAutoscaler
					if withConfig {
						config = configFull
					}

					clusterAutoscaler = New(c, namespace, image, replicas, config)
					clusterAutoscaler.SetNamespaceUID(namespaceUID)
					clusterAutoscaler.SetMachineDeployments(machineDeployments)
					clusterAutoscaler.SetSecrets(secrets)

					gomock.InOrder(
						c.EXPECT().Create(ctx, serviceAccount),
						c.EXPECT().Get(ctx, kutil.Key(clusterRoleBindingName), gomock.AssignableToTypeOf(&rbacv1.ClusterRoleBinding{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRoleBinding{}), gomock.Any()).
							Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
								Expect(obj).To(DeepEqual(clusterRoleBinding))
							}),
						c.EXPECT().Get(ctx, kutil.Key(namespace, serviceName), gomock.AssignableToTypeOf(&corev1.Service{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Service{}), gomock.Any()).
							Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
								Expect(obj).To(DeepEqual(service))
							}),
						c.EXPECT().Get(ctx, kutil.Key(namespace, deploymentName), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{}), gomock.Any()).
							Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
								Expect(obj).To(DeepEqual(deploymentFor(withConfig)))
							}),
						c.EXPECT().Get(ctx, kutil.Key(namespace, vpaName), gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{}), gomock.Any()).
							Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
								Expect(obj).To(DeepEqual(vpa))
							}),
						c.EXPECT().Get(ctx, kutil.Key(namespace, managedResourceSecretName), gomock.AssignableToTypeOf(&corev1.Secret{})),
						c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).
							Do(func(ctx context.Context, obj client.Object, _ ...client.UpdateOption) {
								Expect(obj).To(DeepEqual(managedResourceSecret))
							}),
						c.EXPECT().Get(ctx, kutil.Key(namespace, managedResourceName), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
						c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).
							Do(func(ctx context.Context, obj client.Object, _ ...client.UpdateOption) {
								Expect(obj).To(DeepEqual(managedResource))
							}),
					)

					Expect(clusterAutoscaler.Deploy(ctx)).To(Succeed())
				}

				It("w/o config", func() { test(false) })
				It("w/ config", func() { test(true) })
			})
		})
	})

	Describe("#Destroy", func() {
		It("should fail because the managed resource cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: managedResourceName}}).Return(fakeErr),
			)

			Expect(clusterAutoscaler.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the managed resource secret cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: managedResourceName}}),
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: managedResourceSecretName}}).Return(fakeErr),
			)

			Expect(clusterAutoscaler.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the vpa cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: managedResourceName}}),
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: managedResourceSecretName}}),
				c.EXPECT().Delete(ctx, &autoscalingv1beta2.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: vpaName}}).Return(fakeErr),
			)

			Expect(clusterAutoscaler.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the deployment cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: managedResourceName}}),
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: managedResourceSecretName}}),
				c.EXPECT().Delete(ctx, &autoscalingv1beta2.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: vpaName}}),
				c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: deploymentName}}).Return(fakeErr),
			)

			Expect(clusterAutoscaler.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the cluster role binding cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: managedResourceName}}),
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: managedResourceSecretName}}),
				c.EXPECT().Delete(ctx, &autoscalingv1beta2.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: vpaName}}),
				c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: deploymentName}}),
				c.EXPECT().Delete(ctx, &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: clusterRoleBindingName}}).Return(fakeErr),
			)

			Expect(clusterAutoscaler.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the service cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: managedResourceName}}),
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: managedResourceSecretName}}),
				c.EXPECT().Delete(ctx, &autoscalingv1beta2.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: vpaName}}),
				c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: deploymentName}}),
				c.EXPECT().Delete(ctx, &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: clusterRoleBindingName}}),
				c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: serviceName}}).Return(fakeErr),
			)

			Expect(clusterAutoscaler.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the service account cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: managedResourceName}}),
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: managedResourceSecretName}}),
				c.EXPECT().Delete(ctx, &autoscalingv1beta2.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: vpaName}}),
				c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: deploymentName}}),
				c.EXPECT().Delete(ctx, &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: clusterRoleBindingName}}),
				c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: serviceName}}),
				c.EXPECT().Delete(ctx, &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: serviceAccountName}}).Return(fakeErr),
			)

			Expect(clusterAutoscaler.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should successfully delete all the resources", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: managedResourceName}}),
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: managedResourceSecretName}}),
				c.EXPECT().Delete(ctx, &autoscalingv1beta2.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: vpaName}}),
				c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: deploymentName}}),
				c.EXPECT().Delete(ctx, &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: clusterRoleBindingName}}),
				c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: serviceName}}),
				c.EXPECT().Delete(ctx, &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: serviceAccountName}}),
			)

			Expect(clusterAutoscaler.Destroy(ctx)).To(Succeed())
		})
	})

	Describe("#Wait", func() {
		It("should return nil as it's not implemented as of now", func() {
			Expect(clusterAutoscaler.Wait(ctx)).To(Succeed())
		})
	})

	Describe("#WaitCleanup", func() {
		It("should return nil as it's not implemented as of now", func() {
			Expect(clusterAutoscaler.WaitCleanup(ctx)).To(Succeed())
		})
	})
})
