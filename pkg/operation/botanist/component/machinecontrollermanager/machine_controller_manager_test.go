// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package machinecontrollermanager_test

import (
	"context"
	"time"

	"github.com/Masterminds/semver"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/machinecontrollermanager"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("MachineControllerManager", func() {
	var (
		ctx       = context.TODO()
		namespace = "shoot--foo--bar"

		image                    = "mcm-image:tag"
		runtimeKubernetesVersion = semver.MustParse("1.26.1")
		namespaceUID             = types.UID("uid")
		replicas                 = int32(1)

		fakeClient client.Client
		sm         secretsmanager.Interface
		values     Values
		mcm        Interface

		serviceAccount        *corev1.ServiceAccount
		clusterRoleBinding    *rbacv1.ClusterRoleBinding
		service               *corev1.Service
		shootAccessSecret     *corev1.Secret
		deployment            *appsv1.Deployment
		podDisruptionBudget   *policyv1.PodDisruptionBudget
		vpa                   *vpaautoscalingv1.VerticalPodAutoscaler
		managedResourceSecret *corev1.Secret
		managedResource       *resourcesv1alpha1.ManagedResource
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		sm = fakesecretsmanager.New(fakeClient, namespace)
		values = Values{
			Image:                    image,
			Replicas:                 replicas,
			RuntimeKubernetesVersion: runtimeKubernetesVersion,
		}
		mcm = New(fakeClient, namespace, sm, values)
		mcm.SetNamespaceUID(namespaceUID)

		By("Create secrets managed outside of this package for whose secretsmanager.Get() will be called")
		Expect(fakeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "generic-token-kubeconfig", Namespace: namespace}})).To(Succeed())

		serviceAccount = &corev1.ServiceAccount{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "ServiceAccount",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "machine-controller-manager",
				Namespace: namespace,
			},
			AutomountServiceAccountToken: pointer.Bool(false),
		}

		clusterRoleBinding = &rbacv1.ClusterRoleBinding{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "rbac.authorization.k8s.io/v1",
				Kind:       "ClusterRoleBinding",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "machine-controller-manager-" + namespace,
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
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "system:machine-controller-manager-runtime",
			},
			Subjects: []rbacv1.Subject{{
				Kind:      "ServiceAccount",
				Name:      "machine-controller-manager",
				Namespace: namespace,
			}},
		}

		service = &corev1.Service{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Service",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "machine-controller-manager",
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "kubernetes",
					"role": "machine-controller-manager",
				},
				Annotations: map[string]string{
					"networking.resources.gardener.cloud/from-all-scrape-targets-allowed-ports": `[{"protocol":"TCP","port":10258}]`,
				},
			},
			Spec: corev1.ServiceSpec{
				Type:      corev1.ServiceTypeClusterIP,
				ClusterIP: corev1.ClusterIPNone,
				Ports: []corev1.ServicePort{{
					Name:     "metrics",
					Port:     10258,
					Protocol: corev1.ProtocolTCP,
				}},
				Selector: map[string]string{
					"app":  "kubernetes",
					"role": "machine-controller-manager",
				},
			},
		}

		shootAccessSecret = &corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Secret",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shoot-access-machine-controller-manager",
				Namespace: namespace,
				Labels: map[string]string{
					"resources.gardener.cloud/purpose": "token-requestor",
				},
				Annotations: map[string]string{
					"serviceaccount.resources.gardener.cloud/name":      "machine-controller-manager",
					"serviceaccount.resources.gardener.cloud/namespace": "kube-system",
				},
			},
			Type: corev1.SecretTypeOpaque,
		}

		deployment = &appsv1.Deployment{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "machine-controller-manager",
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "kubernetes",
					"role": "machine-controller-manager",
					"high-availability-config.resources.gardener.cloud/type": "controller",
				},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas:             &replicas,
				RevisionHistoryLimit: pointer.Int32(2),
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{
					"app":  "kubernetes",
					"role": "machine-controller-manager",
				}},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app":                                "kubernetes",
							"role":                               "machine-controller-manager",
							"gardener.cloud/role":                "controlplane",
							"maintenance.gardener.cloud/restart": "true",
							"networking.gardener.cloud/to-dns":   "allowed",
							"networking.gardener.cloud/to-public-networks":                  "allowed",
							"networking.gardener.cloud/to-private-networks":                 "allowed",
							"networking.gardener.cloud/to-runtime-apiserver":                "allowed",
							"networking.resources.gardener.cloud/to-kube-apiserver-tcp-443": "allowed",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:            "machine-controller-manager",
							Image:           image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command: []string{
								"./machine-controller-manager",
								"--control-kubeconfig=inClusterConfig",
								"--machine-safety-overshooting-period=1m",
								"--namespace=" + namespace,
								"--port=10258",
								"--safety-up=2",
								"--safety-down=1",
								"--target-kubeconfig=/var/run/secrets/gardener.cloud/shoot/generic-kubeconfig/kubeconfig",
								"--v=4",
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/healthz",
										Port:   intstr.FromInt(10258),
										Scheme: corev1.URISchemeHTTP,
									},
								},
								FailureThreshold:    3,
								InitialDelaySeconds: 30,
								PeriodSeconds:       10,
								SuccessThreshold:    1,
								TimeoutSeconds:      5,
							},
							Ports: []corev1.ContainerPort{{
								Name:          "metrics",
								ContainerPort: 10258,
								Protocol:      corev1.ProtocolTCP,
							}},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("31m"),
									corev1.ResourceMemory: resource.MustParse("70Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("1Gi"),
								},
							},
						}},
						PriorityClassName:             "gardener-system-300",
						ServiceAccountName:            "machine-controller-manager",
						TerminationGracePeriodSeconds: pointer.Int64(5),
					},
				},
			},
		}
		Expect(gardenerutils.InjectGenericKubeconfig(deployment, "generic-token-kubeconfig", shootAccessSecret.Name)).To(Succeed())

		podDisruptionBudget = &policyv1.PodDisruptionBudget{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "policy/v1",
				Kind:       "PodDisruptionBudget",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "machine-controller-manager",
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "kubernetes",
					"role": "machine-controller-manager",
				},
			},
			Spec: policyv1.PodDisruptionBudgetSpec{
				MaxUnavailable: utils.IntStrPtrFromInt(1),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app":  "kubernetes",
						"role": "machine-controller-manager",
					},
				},
			},
		}

		vpaUpdateMode := vpaautoscalingv1.UpdateModeAuto
		vpaControlledValues := vpaautoscalingv1.ContainerControlledValuesRequestsOnly
		vpa = &vpaautoscalingv1.VerticalPodAutoscaler{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "autoscaling.k8s.io/v1",
				Kind:       "VerticalPodAutoscaler",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "machine-controller-manager-vpa",
				Namespace: namespace,
			},
			Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
				TargetRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       "machine-controller-manager",
				},
				UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
					UpdateMode: &vpaUpdateMode,
				},
				ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
					ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{{
						ContainerName:    "machine-controller-manager",
						ControlledValues: &vpaControlledValues,
						MinAllowed: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("70Mi"),
						},
						MaxAllowed: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("2"),
							corev1.ResourceMemory: resource.MustParse("5G"),
						},
					}},
				},
			},
		}

		clusterRoleYAML := `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  creationTimestamp: null
  name: gardener.cloud:target:machine-controller-manager
rules:
- apiGroups:
  - ""
  resources:
  - nodes
  - nodes/status
  - endpoints
  - replicationcontrollers
  - pods
  - persistentvolumes
  - persistentvolumeclaims
  verbs:
  - create
  - delete
  - deletecollection
  - get
  - list
  - patch
  - update
  - watch
- apiGroups:
  - ""
  resources:
  - pods/eviction
  verbs:
  - create
- apiGroups:
  - apps
  resources:
  - replicasets
  - statefulsets
  - daemonsets
  - deployments
  verbs:
  - create
  - delete
  - deletecollection
  - get
  - list
  - patch
  - update
  - watch
- apiGroups:
  - batch
  resources:
  - jobs
  - cronjobs
  verbs:
  - create
  - delete
  - deletecollection
  - get
  - list
  - patch
  - update
  - watch
- apiGroups:
  - policy
  resources:
  - poddisruptionbudgets
  verbs:
  - list
  - watch
- apiGroups:
  - storage.k8s.io
  resources:
  - volumeattachments
  verbs:
  - get
  - list
  - watch
`

		clusterRoleBindingYAML := `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  creationTimestamp: null
  name: gardener.cloud:target:machine-controller-manager
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: gardener.cloud:target:machine-controller-manager
subjects:
- kind: ServiceAccount
  name: machine-controller-manager
  namespace: kube-system
`

		roleYAML := `apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  creationTimestamp: null
  name: gardener.cloud:target:machine-controller-manager
  namespace: kube-system
rules:
- apiGroups:
  - ""
  resources:
  - secrets
  verbs:
  - create
  - delete
  - get
  - list
`

		roleBindingYAML := `apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  creationTimestamp: null
  name: gardener.cloud:target:machine-controller-manager
  namespace: kube-system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: gardener.cloud:target:machine-controller-manager
subjects:
- kind: ServiceAccount
  name: machine-controller-manager
  namespace: kube-system
`

		managedResourceSecret = &corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Secret",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "managedresource-shoot-core-machine-controller-manager",
				Namespace: namespace,
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"clusterrole____gardener.cloud_target_machine-controller-manager.yaml":            []byte(clusterRoleYAML),
				"clusterrolebinding____gardener.cloud_target_machine-controller-manager.yaml":     []byte(clusterRoleBindingYAML),
				"role__kube-system__gardener.cloud_target_machine-controller-manager.yaml":        []byte(roleYAML),
				"rolebinding__kube-system__gardener.cloud_target_machine-controller-manager.yaml": []byte(roleBindingYAML),
			},
		}
		managedResource = &resourcesv1alpha1.ManagedResource{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "resources.gardener.cloud/v1alpha1",
				Kind:       "ManagedResource",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shoot-core-machine-controller-manager",
				Namespace: namespace,
				Labels:    map[string]string{"origin": "gardener"},
			},
			Spec: resourcesv1alpha1.ManagedResourceSpec{
				SecretRefs:   []corev1.LocalObjectReference{{Name: managedResourceSecret.Name}},
				InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
				KeepObjects:  pointer.Bool(false),
			},
		}
	})

	Describe("#Deploy", func() {
		It("should successfully deploy all resources", func() {
			Expect(mcm.Deploy(ctx)).To(Succeed())

			actualServiceAccount := &corev1.ServiceAccount{}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(serviceAccount), actualServiceAccount)).To(Succeed())
			serviceAccount.ResourceVersion = "1"
			Expect(actualServiceAccount).To(Equal(serviceAccount))

			actualClusterRoleBinding := &rbacv1.ClusterRoleBinding{}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(clusterRoleBinding), actualClusterRoleBinding)).To(Succeed())
			clusterRoleBinding.ResourceVersion = "1"
			Expect(actualClusterRoleBinding).To(Equal(clusterRoleBinding))

			actualService := &corev1.Service{}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(service), actualService)).To(Succeed())
			service.ResourceVersion = "1"
			Expect(actualService).To(Equal(service))

			actualShootAccessSecret := &corev1.Secret{}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(shootAccessSecret), actualShootAccessSecret)).To(Succeed())
			shootAccessSecret.ResourceVersion = "1"
			Expect(actualShootAccessSecret).To(Equal(shootAccessSecret))

			actualDeployment := &appsv1.Deployment{}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(deployment), actualDeployment)).To(Succeed())
			deployment.ResourceVersion = "1"
			Expect(actualDeployment).To(Equal(deployment))

			actualPodDisruptionBudget := &policyv1.PodDisruptionBudget{}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(podDisruptionBudget), actualPodDisruptionBudget)).To(Succeed())
			podDisruptionBudget.ResourceVersion = "1"
			Expect(actualPodDisruptionBudget).To(Equal(podDisruptionBudget))

			actualVPA := &vpaautoscalingv1.VerticalPodAutoscaler{}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(vpa), actualVPA)).To(Succeed())
			vpa.ResourceVersion = "1"
			Expect(actualVPA).To(Equal(vpa))

			actualManagedResourceSecret := &corev1.Secret{}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), actualManagedResourceSecret)).To(Succeed())
			managedResourceSecret.ResourceVersion = "1"
			Expect(actualManagedResourceSecret).To(Equal(managedResourceSecret))

			actualManagedResource := &resourcesv1alpha1.ManagedResource{}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResource), actualManagedResource)).To(Succeed())
			managedResource.ResourceVersion = "1"
			Expect(actualManagedResource).To(Equal(managedResource))
		})
	})

	Describe("#Destroy", func() {
		It("should successfully destroy all resources", func() {
			Expect(fakeClient.Create(ctx, serviceAccount)).To(Succeed())
			Expect(fakeClient.Create(ctx, clusterRoleBinding)).To(Succeed())
			Expect(fakeClient.Create(ctx, service)).To(Succeed())
			Expect(fakeClient.Create(ctx, shootAccessSecret)).To(Succeed())
			Expect(fakeClient.Create(ctx, deployment)).To(Succeed())
			Expect(fakeClient.Create(ctx, podDisruptionBudget)).To(Succeed())
			Expect(fakeClient.Create(ctx, vpa)).To(Succeed())
			Expect(fakeClient.Create(ctx, managedResourceSecret)).To(Succeed())
			Expect(fakeClient.Create(ctx, managedResource)).To(Succeed())

			Expect(mcm.Destroy(ctx)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResource), &resourcesv1alpha1.ManagedResource{})).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), &corev1.Secret{})).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(vpa), &vpaautoscalingv1.VerticalPodAutoscaler{})).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(podDisruptionBudget), &policyv1.PodDisruptionBudget{})).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(deployment), &appsv1.Deployment{})).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(shootAccessSecret), &corev1.Secret{})).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(service), &corev1.Service{})).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(clusterRoleBinding), &rbacv1.ClusterRoleBinding{})).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(serviceAccount), &corev1.ServiceAccount{})).To(BeNotFoundError())
		})
	})

	Describe("#Wait", func() {
		BeforeEach(func() {
			DeferCleanup(test.WithVars(
				&DefaultInterval, time.Millisecond,
				&DefaultTimeout, 100*time.Millisecond,
			))
		})

		It("should successfully wait for the deployment to be updated", func() {
			deploy := deployment.DeepCopy()

			Expect(fakeClient.Create(ctx, deploy)).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(deploy), deploy)).To(Succeed())

			Expect(fakeClient.Create(ctx, &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod",
					Namespace: deployment.Namespace,
					Labels:    deployment.Spec.Selector.MatchLabels,
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

			Expect(mcm.Wait(ctx)).To(Succeed())
		})
	})

	Describe("#WaitCleanup", func() {
		BeforeEach(func() {
			DeferCleanup(test.WithVars(
				&DefaultInterval, time.Millisecond,
				&DefaultTimeout, 100*time.Millisecond,
			))
		})

		It("should time out while waiting for the deployment to be deleted", func() {
			Expect(fakeClient.Create(ctx, deployment)).To(Succeed())
			Expect(mcm.WaitCleanup(ctx)).To(MatchError(ContainSubstring("context deadline exceeded")))
		})

		It("should successfully wait for the deployment to be deleted", func() {
			deploy := deployment.DeepCopy()

			Expect(fakeClient.Create(ctx, deploy)).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(deploy), deploy)).To(Succeed())

			timer := time.AfterFunc(10*time.Millisecond, func() {
				Expect(fakeClient.Delete(ctx, deploy)).To(Succeed())
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(deploy), deploy)).To(BeNotFoundError())
			})
			defer timer.Stop()

			Expect(mcm.WaitCleanup(ctx)).To(Succeed())
		})
	})
})
