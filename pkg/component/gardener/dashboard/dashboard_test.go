// Copyright 2024 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package dashboard_test

import (
	"context"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/gardener/dashboard"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("GardenerDashboard", func() {
	var (
		ctx context.Context

		managedResourceNameRuntime = "gardener-dashboard-runtime"
		managedResourceNameVirtual = "gardener-dashboard-virtual"
		namespace                  = "some-namespace"

		image            = "gardener-dashboard-image:latest"
		apiServerURL     = "https://api.com"
		logLevel         = "debug"
		enableTokenLogin bool
		terminal         *TerminalValues

		fakeClient        client.Client
		fakeSecretManager secretsmanager.Interface
		deployer          component.DeployWaiter
		values            Values

		fakeOps   *retryfake.Ops
		consistOf func(...client.Object) types.GomegaMatcher

		managedResourceRuntime       *resourcesv1alpha1.ManagedResource
		managedResourceVirtual       *resourcesv1alpha1.ManagedResource
		managedResourceSecretRuntime *corev1.Secret
		managedResourceSecretVirtual *corev1.Secret

		virtualGardenAccessSecret *corev1.Secret
		sessionSecret             *corev1.Secret
		newConfigMap              func(enableTokenLogin bool, terminal *TerminalValues) *corev1.ConfigMap
		configMap                 *corev1.ConfigMap
		deployment                *appsv1.Deployment
		service                   *corev1.Service
		podDisruptionBudget       *policyv1.PodDisruptionBudget
		vpa                       *vpaautoscalingv1.VerticalPodAutoscaler

		clusterRole                      *rbacv1.ClusterRole
		clusterRoleBinding               *rbacv1.ClusterRoleBinding
		serviceAccountTerminal           *corev1.ServiceAccount
		clusterRoleTerminalProjectMember *rbacv1.ClusterRole
		clusterRoleBindingTerminal       *rbacv1.ClusterRoleBinding
	)

	BeforeEach(func() {
		enableTokenLogin = true
		terminal = nil

		ctx = context.Background()

		fakeClient = fakeclient.NewClientBuilder().WithScheme(operatorclient.RuntimeScheme).Build()
		fakeSecretManager = fakesecretsmanager.New(fakeClient, namespace)

		fakeOps = &retryfake.Ops{MaxAttempts: 2}
		DeferCleanup(test.WithVars(
			&retry.Until, fakeOps.Until,
			&retry.UntilTimeout, fakeOps.UntilTimeout,
		))

		consistOf = NewManagedResourceConsistOfObjectsMatcher(fakeClient)

		managedResourceRuntime = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceNameRuntime,
				Namespace: namespace,
			},
		}
		managedResourceVirtual = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceNameVirtual,
				Namespace: namespace,
			},
		}
		managedResourceSecretRuntime = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "managedresource-" + managedResourceRuntime.Name,
				Namespace: namespace,
			},
		}
		managedResourceSecretVirtual = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "managedresource-" + managedResourceVirtual.Name,
				Namespace: namespace,
			},
		}
	})

	JustBeforeEach(func() {
		virtualGardenAccessSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shoot-access-gardener-dashboard",
				Namespace: namespace,
				Labels: map[string]string{
					"resources.gardener.cloud/purpose": "token-requestor",
					"resources.gardener.cloud/class":   "shoot",
				},
				Annotations: map[string]string{
					"serviceaccount.resources.gardener.cloud/name":      "gardener-dashboard",
					"serviceaccount.resources.gardener.cloud/namespace": "kube-system",
				},
			},
			Type: corev1.SecretTypeOpaque,
		}
		sessionSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener-dashboard-session-secret-34ea1210",
				Namespace: namespace,
				Labels: map[string]string{
					"manager-identity":              "fake",
					"name":                          "gardener-dashboard-session-secret",
					"rotation-strategy":             "inplace",
					"checksum-of-config":            "5743303071195020433",
					"last-rotation-initiation-time": "",
					"managed-by":                    "secrets-manager",
				},
			},
			Type:      corev1.SecretTypeOpaque,
			Immutable: ptr.To(true),
			Data: map[string][]byte{
				"password": []byte("________________________________"),
				"username": []byte("admin"),
				"auth":     []byte("admin:{SHA}+GNV0g9PMmMEEXnq9rUTzZ3zAN4="),
			},
		}
		newConfigMap = func(enableTokenLogin bool, terminal *TerminalValues) *corev1.ConfigMap {
			obj := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "gardener-dashboard-config",
					Namespace: namespace,
					Labels: map[string]string{
						"app":  "gardener",
						"role": "dashboard",
					},
				},
				Data: make(map[string]string),
			}

			configRaw := `port: 8080
logFormat: text
logLevel: ` + logLevel + `
apiServerUrl: ` + apiServerURL + `
maxRequestBodySize: 500kb
experimentalUseWatchCacheForListShoots: "yes"
readinessProbe:
    periodSeconds: 10
unreachableSeeds:
    matchLabels:
        seed.gardener.cloud/network: private
`

			if terminal != nil {
				configRaw += `contentSecurityPolicy:
    connectSrc:
        - self`

				for _, host := range terminal.AllowedHostSourceList {
					configRaw += `
        - wss://` + host + `
        - https://` + host
				}

				configRaw += `
terminal:
    container:
        image: ` + terminal.Container.Image + `
    containerImageDescriptions:
        - image: /.*/
          description: ` + ptr.Deref(terminal.Container.Description, "") + `
    gardenTerminalHost:
        seedRef: ` + terminal.GardenTerminalSeedHost + `
    garden:
        operatorCredentials:
            serviceAccountRef:
                name: dashboard-terminal-admin
                namespace: kube-system
`
			}

			obj.Data["config.yaml"] = configRaw

			if enableTokenLogin {
				obj.Data["login-config.json"] = `{"loginTypes":["token"]}`
			} else {
				obj.Data["login-config.json"] = `{"loginTypes":null}`
			}

			utilruntime.Must(kubernetesutils.MakeUnique(obj))
			return obj
		}
		configMap = newConfigMap(enableTokenLogin, terminal)
		deployment = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener-dashboard",
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "gardener",
					"role": "dashboard",
					"high-availability-config.resources.gardener.cloud/type": "server",
				},
				Annotations: map[string]string{
					references.AnnotationKey("configmap", configMap.Name): configMap.Name,
					"reference.resources.gardener.cloud/secret-867d23cd":  "generic-token-kubeconfig",
					"reference.resources.gardener.cloud/secret-da1c7d68":  virtualGardenAccessSecret.Name,
					"reference.resources.gardener.cloud/secret-73330522":  sessionSecret.Name,
				},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas:             ptr.To[int32](1),
				RevisionHistoryLimit: ptr.To[int32](2),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app":  "gardener",
						"role": "dashboard",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app":                              "gardener",
							"role":                             "dashboard",
							"networking.gardener.cloud/to-dns": "allowed",
							"networking.gardener.cloud/to-public-networks":                                 "allowed",
							"networking.resources.gardener.cloud/to-virtual-garden-kube-apiserver-tcp-443": "allowed",
						},
						Annotations: map[string]string{
							references.AnnotationKey("configmap", configMap.Name): configMap.Name,
							"reference.resources.gardener.cloud/secret-867d23cd":  "generic-token-kubeconfig",
							"reference.resources.gardener.cloud/secret-da1c7d68":  virtualGardenAccessSecret.Name,
							"reference.resources.gardener.cloud/secret-73330522":  sessionSecret.Name,
						},
					},
					Spec: corev1.PodSpec{
						PriorityClassName:            "gardener-garden-system-200",
						AutomountServiceAccountToken: ptr.To(false),
						SecurityContext: &corev1.PodSecurityContext{
							RunAsNonRoot: ptr.To(true),
							RunAsUser:    ptr.To[int64](65532),
							RunAsGroup:   ptr.To[int64](65532),
							FSGroup:      ptr.To[int64](65532),
						},
						Containers: []corev1.Container{
							{
								Name:            "gardener-dashboard",
								Image:           image,
								ImagePullPolicy: corev1.PullIfNotPresent,
								Args: []string{
									"--optimize-for-size",
									"server.js",
								},
								Env: []corev1.EnvVar{
									{
										Name: "SESSION_SECRET",
										ValueFrom: &corev1.EnvVarSource{
											SecretKeyRef: &corev1.SecretKeySelector{
												LocalObjectReference: corev1.LocalObjectReference{Name: sessionSecret.Name},
												Key:                  "password",
											},
										},
									},
									{
										Name:  "GARDENER_CONFIG",
										Value: "/etc/gardener-dashboard/config/config.yaml",
									},
									{
										Name:  "KUBECONFIG",
										Value: "/var/run/secrets/gardener.cloud/shoot/generic-kubeconfig/kubeconfig",
									},
									{
										Name:  "METRICS_PORT",
										Value: "9050",
									},
									{
										Name: "POD_NAME",
										ValueFrom: &corev1.EnvVarSource{
											FieldRef: &corev1.ObjectFieldSelector{
												APIVersion: "v1",
												FieldPath:  "metadata.name",
											},
										},
									},
									{
										Name: "POD_NAMESPACE",
										ValueFrom: &corev1.EnvVarSource{
											FieldRef: &corev1.ObjectFieldSelector{
												APIVersion: "v1",
												FieldPath:  "metadata.namespace",
											},
										},
									},
								},
								Resources: corev1.ResourceRequirements{
									Requests: map[corev1.ResourceName]resource.Quantity{
										corev1.ResourceCPU:    resource.MustParse("50m"),
										corev1.ResourceMemory: resource.MustParse("128Mi"),
									},
								},
								Ports: []corev1.ContainerPort{
									{
										Name:          "http",
										ContainerPort: 8080,
										Protocol:      corev1.ProtocolTCP,
									},
									{
										Name:          "metrics",
										ContainerPort: 9050,
										Protocol:      corev1.ProtocolTCP,
									},
								},
								LivenessProbe: &corev1.Probe{
									ProbeHandler: corev1.ProbeHandler{
										TCPSocket: &corev1.TCPSocketAction{
											Port: intstr.FromString("http"),
										},
									},
									InitialDelaySeconds: 15,
									TimeoutSeconds:      5,
									FailureThreshold:    6,
									SuccessThreshold:    1,
									PeriodSeconds:       20,
								},
								ReadinessProbe: &corev1.Probe{
									ProbeHandler: corev1.ProbeHandler{
										HTTPGet: &corev1.HTTPGetAction{
											Path:   "/healthz",
											Port:   intstr.FromString("http"),
											Scheme: "HTTP",
										},
									},
									InitialDelaySeconds: 5,
									TimeoutSeconds:      5,
									FailureThreshold:    6,
									SuccessThreshold:    1,
									PeriodSeconds:       10,
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      "gardener-dashboard-config",
										MountPath: "/etc/gardener-dashboard/config",
									},
									{
										Name:      "gardener-dashboard-login-config",
										MountPath: "/app/public/login-config.json",
										SubPath:   "login-config.json",
									},
								},
							},
						},
						Volumes: []corev1.Volume{
							{
								Name: "gardener-dashboard-config",
								VolumeSource: corev1.VolumeSource{
									ConfigMap: &corev1.ConfigMapVolumeSource{
										LocalObjectReference: corev1.LocalObjectReference{Name: configMap.Name},
										Items: []corev1.KeyToPath{{
											Key:  "config.yaml",
											Path: "config.yaml",
										}},
									},
								},
							},
							{
								Name: "gardener-dashboard-login-config",
								VolumeSource: corev1.VolumeSource{
									ConfigMap: &corev1.ConfigMapVolumeSource{
										LocalObjectReference: corev1.LocalObjectReference{Name: configMap.Name},
										Items: []corev1.KeyToPath{{
											Key:  "login-config.json",
											Path: "login-config.json",
										}},
									},
								},
							},
						},
					},
				},
			},
		}
		utilruntime.Must(gardener.InjectGenericKubeconfig(deployment, "generic-token-kubeconfig", "shoot-access-gardener-dashboard"))
		service = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener-dashboard",
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "gardener",
					"role": "dashboard",
				},
			},
			Spec: corev1.ServiceSpec{
				Type:     corev1.ServiceTypeClusterIP,
				Selector: GetLabels(),
				Ports: []corev1.ServicePort{{
					Name:       "http",
					Port:       8080,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromInt32(8080),
				}},
				SessionAffinity: corev1.ServiceAffinityClientIP,
			},
		}
		podDisruptionBudget = &policyv1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener-dashboard",
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "gardener",
					"role": "dashboard",
				},
			},
			Spec: policyv1.PodDisruptionBudgetSpec{
				MaxUnavailable: utils.IntStrPtrFromInt32(1),
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{
					"app":  "gardener",
					"role": "dashboard",
				}},
				UnhealthyPodEvictionPolicy: ptr.To(policyv1.AlwaysAllow),
			},
		}
		vpa = &vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener-dashboard-vpa",
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "gardener",
					"role": "dashboard",
				},
			},
			Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
				TargetRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       "gardener-dashboard",
				},
				UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
					UpdateMode: ptr.To(vpaautoscalingv1.UpdateModeAuto),
				},
				ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
					ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
						{
							ContainerName: "*",
							MinAllowed: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("10m"),
								corev1.ResourceMemory: resource.MustParse("64Mi"),
							},
						},
					},
				},
			},
		}

		clusterRole = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:system:dashboard",
				Labels: map[string]string{
					"app":  "gardener",
					"role": "dashboard",
				},
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{"authentication.k8s.io"},
					Resources: []string{"tokenreviews"},
					Verbs:     []string{"create"},
				},
				{
					APIGroups: []string{"core.gardener.cloud"},
					Resources: []string{"quotas", "projects", "shoots", "controllerregistrations"},
					Verbs:     []string{"list", "watch"},
				},
				{
					APIGroups: []string{"apiregistration.k8s.io"},
					Resources: []string{"apiservices"},
					Verbs:     []string{"get"},
				},
				{
					APIGroups:     []string{""},
					Resources:     []string{"configmaps"},
					Verbs:         []string{"get"},
					ResourceNames: []string{"cluster-identity"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"resourcequotas"},
					Verbs:     []string{"list", "watch"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"secrets"},
					Verbs:     []string{"get"},
				},
			},
		}
		clusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:system:dashboard",
				Labels: map[string]string{
					"app":  "gardener",
					"role": "dashboard",
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "gardener.cloud:system:dashboard",
			},
			Subjects: []rbacv1.Subject{{
				Kind:      "ServiceAccount",
				Name:      "gardener-dashboard",
				Namespace: "kube-system",
			}},
		}
		serviceAccountTerminal = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "dashboard-terminal-admin",
				Namespace: "kube-system",
				Labels: map[string]string{
					"app":  "gardener",
					"role": "dashboard",
				},
			},
		}
		clusterRoleBindingTerminal = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:dashboard-terminal:admin",
				Labels: map[string]string{
					"app":  "gardener",
					"role": "dashboard",
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "gardener.cloud:system:administrators",
			},
			Subjects: []rbacv1.Subject{{
				Kind:      "ServiceAccount",
				Name:      "dashboard-terminal-admin",
				Namespace: "kube-system",
			}},
		}
		clusterRoleTerminalProjectMember = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "dashboard.gardener.cloud:system:project-member",
				Labels: map[string]string{
					"app":  "gardener",
					"role": "dashboard",
					"rbac.gardener.cloud/aggregate-to-project-member": "true",
				},
			},
			Rules: []rbacv1.PolicyRule{{
				APIGroups: []string{"dashboard.gardener.cloud"},
				Resources: []string{"terminals"},
				Verbs:     []string{"create", "delete", "deletecollection", "get", "list", "watch", "patch", "update"},
			}},
		}

		values = Values{
			LogLevel:         logLevel,
			RuntimeVersion:   semver.MustParse("1.26.4"),
			Image:            image,
			APIServerURL:     apiServerURL,
			EnableTokenLogin: enableTokenLogin,
			Terminal:         terminal,
		}
		deployer = New(fakeClient, namespace, fakeSecretManager, values)

		By("Create secrets managed outside of this package for whose secretsmanager.Get() will be called")
		Expect(fakeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "generic-token-kubeconfig", Namespace: namespace}})).To(Succeed())
	})

	Describe("#Deploy", func() {
		Context("resources generation", func() {
			var expectedRuntimeObject []client.Object

			BeforeEach(func() {
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceRuntime), managedResourceRuntime)).To(BeNotFoundError())
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceVirtual), managedResourceVirtual)).To(BeNotFoundError())
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretRuntime), managedResourceSecretRuntime)).To(BeNotFoundError())
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretVirtual), managedResourceSecretVirtual)).To(BeNotFoundError())

				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameRuntime,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: healthyManagedResourceStatus,
				})).To(Succeed())

				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameVirtual,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: healthyManagedResourceStatus,
				})).To(Succeed())
			})

			JustBeforeEach(func() {
				Expect(deployer.Deploy(ctx)).To(Succeed())

				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceRuntime), managedResourceRuntime)).To(Succeed())
				expectedRuntimeMr := &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:            managedResourceRuntime.Name,
						Namespace:       managedResourceRuntime.Namespace,
						ResourceVersion: "2",
						Generation:      1,
						Labels: map[string]string{
							"gardener.cloud/role":                "seed-system-component",
							"care.gardener.cloud/condition-type": "VirtualComponentsHealthy",
						},
					},
					Spec: resourcesv1alpha1.ManagedResourceSpec{
						Class:       ptr.To("seed"),
						SecretRefs:  []corev1.LocalObjectReference{{Name: managedResourceRuntime.Spec.SecretRefs[0].Name}},
						KeepObjects: ptr.To(false),
					},
					Status: healthyManagedResourceStatus,
				}
				utilruntime.Must(references.InjectAnnotations(expectedRuntimeMr))
				Expect(managedResourceRuntime).To(Equal(expectedRuntimeMr))

				managedResourceSecretRuntime.Name = managedResourceRuntime.Spec.SecretRefs[0].Name
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretRuntime), managedResourceSecretRuntime)).To(Succeed())

				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceVirtual), managedResourceVirtual)).To(Succeed())
				expectedVirtualMr := &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:            managedResourceVirtual.Name,
						Namespace:       managedResourceVirtual.Namespace,
						ResourceVersion: "2",
						Generation:      1,
						Labels: map[string]string{
							"origin":                             "gardener",
							"care.gardener.cloud/condition-type": "VirtualComponentsHealthy",
						},
					},
					Spec: resourcesv1alpha1.ManagedResourceSpec{
						InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
						SecretRefs:   []corev1.LocalObjectReference{{Name: managedResourceVirtual.Spec.SecretRefs[0].Name}},
						KeepObjects:  ptr.To(false),
					},
					Status: healthyManagedResourceStatus,
				}
				utilruntime.Must(references.InjectAnnotations(expectedVirtualMr))
				Expect(managedResourceVirtual).To(Equal(expectedVirtualMr))
				expectedRuntimeObject = []client.Object{
					configMap,
					deployment,
					service,
					podDisruptionBudget,
					vpa,
				}

				managedResourceSecretVirtual.Name = expectedVirtualMr.Spec.SecretRefs[0].Name
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretVirtual), managedResourceSecretVirtual)).To(Succeed())
				Expect(managedResourceSecretRuntime.Type).To(Equal(corev1.SecretTypeOpaque))
				Expect(managedResourceSecretRuntime.Immutable).To(Equal(ptr.To(true)))
				Expect(managedResourceSecretRuntime.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))

				Expect(managedResourceVirtual).To(consistOf(
					clusterRole,
					clusterRoleBinding,
					serviceAccountTerminal,
					clusterRoleBindingTerminal,
					clusterRoleTerminalProjectMember,
				))
				Expect(managedResourceSecretVirtual.Type).To(Equal(corev1.SecretTypeOpaque))
				Expect(managedResourceSecretVirtual.Immutable).To(Equal(ptr.To(true)))
				Expect(managedResourceSecretVirtual.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))
			})

			It("should successfully deploy all resources", func() {
				Expect(managedResourceRuntime).To(consistOf(expectedRuntimeObject...))
			})

			When("token login is disabled", func() {
				BeforeEach(func() {
					enableTokenLogin = false
				})

				It("should successfully deploy all resources", func() {
					Expect(managedResourceRuntime).To(consistOf(expectedRuntimeObject...))
				})
			})

			When("terminal is configured", func() {
				var (
					terminalContainerImage            = "some-image:latest"
					terminalContainerImageDescription = "cool image"
					allowedHostSourceList             = []string{"first", "second"}
					gardenTerminalSeedHost            = "terminal-host"
				)

				BeforeEach(func() {
					terminal = &TerminalValues{
						DashboardTerminal: operatorv1alpha1.DashboardTerminal{
							Container: operatorv1alpha1.DashboardTerminalContainer{
								Image:       terminalContainerImage,
								Description: &terminalContainerImageDescription,
							},
							AllowedHostSourceList: allowedHostSourceList,
						},
						GardenTerminalSeedHost: gardenTerminalSeedHost,
					}
				})

				It("should successfully deploy all resources", func() {
					Expect(managedResourceRuntime).To(consistOf(expectedRuntimeObject...))
				})
			})

			Context("secrets", func() {
				It("should successfully deploy the access secret for the virtual garden", func() {
					actualAccessSecret := &corev1.Secret{}
					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(virtualGardenAccessSecret), actualAccessSecret)).To(Succeed())
					virtualGardenAccessSecret.ResourceVersion = "1"
					Expect(actualAccessSecret).To(Equal(virtualGardenAccessSecret))
				})

				It("should successfully deploy the session secret", func() {
					actualSessionSecret := &corev1.Secret{}
					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(sessionSecret), actualSessionSecret)).To(Succeed())
					sessionSecret.ResourceVersion = "1"
					Expect(actualSessionSecret).To(Equal(sessionSecret))
				})
			})
		})
	})

	Describe("#Destroy", func() {
		It("should successfully destroy all resources", func() {
			Expect(fakeClient.Create(ctx, managedResourceRuntime)).To(Succeed())
			Expect(fakeClient.Create(ctx, managedResourceVirtual)).To(Succeed())
			Expect(fakeClient.Create(ctx, managedResourceSecretRuntime)).To(Succeed())
			Expect(fakeClient.Create(ctx, managedResourceSecretVirtual)).To(Succeed())
			Expect(fakeClient.Create(ctx, virtualGardenAccessSecret)).To(Succeed())

			Expect(deployer.Destroy(ctx)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceRuntime), managedResourceRuntime)).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceVirtual), managedResourceVirtual)).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretRuntime), managedResourceSecretRuntime)).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretVirtual), managedResourceSecretVirtual)).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(virtualGardenAccessSecret), virtualGardenAccessSecret)).To(BeNotFoundError())
		})
	})

	Context("waiting functions", func() {
		Describe("#Wait", func() {
			It("should fail because reading the runtime ManagedResource fails", func() {
				Expect(deployer.Wait(ctx)).To(MatchError(ContainSubstring("not found")))
			})

			It("should fail because the runtime and virtual ManagedResources are unhealthy", func() {
				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameRuntime,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: unhealthyManagedResourceStatus,
				})).To(Succeed())

				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameVirtual,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: unhealthyManagedResourceStatus,
				})).To(Succeed())

				Expect(deployer.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
			})

			It("should fail because the runtime ManagedResource is unhealthy", func() {
				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameRuntime,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: unhealthyManagedResourceStatus,
				})).To(Succeed())

				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameVirtual,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 1,
						Conditions: []gardencorev1beta1.Condition{
							{
								Type:   resourcesv1alpha1.ResourcesApplied,
								Status: gardencorev1beta1.ConditionTrue,
							},
							{
								Type:   resourcesv1alpha1.ResourcesHealthy,
								Status: gardencorev1beta1.ConditionTrue,
							},
							{
								Type:   resourcesv1alpha1.ResourcesProgressing,
								Status: gardencorev1beta1.ConditionFalse,
							},
						},
					},
				})).To(Succeed())

				Expect(deployer.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
			})

			It("should fail because the virtual ManagedResource is unhealthy", func() {
				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameRuntime,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 1,
						Conditions: []gardencorev1beta1.Condition{
							{
								Type:   resourcesv1alpha1.ResourcesApplied,
								Status: gardencorev1beta1.ConditionTrue,
							},
							{
								Type:   resourcesv1alpha1.ResourcesHealthy,
								Status: gardencorev1beta1.ConditionTrue,
							},
							{
								Type:   resourcesv1alpha1.ResourcesProgressing,
								Status: gardencorev1beta1.ConditionFalse,
							},
						},
					},
				})).To(Succeed())

				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameVirtual,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: unhealthyManagedResourceStatus,
				})).To(Succeed())

				Expect(deployer.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
			})

			It("should succeed because the runtime and virtual ManagedResource are healthy and progressing", func() {
				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameRuntime,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 1,
						Conditions: []gardencorev1beta1.Condition{
							{
								Type:   resourcesv1alpha1.ResourcesApplied,
								Status: gardencorev1beta1.ConditionTrue,
							},
							{
								Type:   resourcesv1alpha1.ResourcesHealthy,
								Status: gardencorev1beta1.ConditionTrue,
							},
							{
								Type:   resourcesv1alpha1.ResourcesProgressing,
								Status: gardencorev1beta1.ConditionTrue,
							},
						},
					},
				})).To(Succeed())

				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameVirtual,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 1,
						Conditions: []gardencorev1beta1.Condition{
							{
								Type:   resourcesv1alpha1.ResourcesApplied,
								Status: gardencorev1beta1.ConditionTrue,
							},
							{
								Type:   resourcesv1alpha1.ResourcesHealthy,
								Status: gardencorev1beta1.ConditionTrue,
							},
							{
								Type:   resourcesv1alpha1.ResourcesProgressing,
								Status: gardencorev1beta1.ConditionTrue,
							},
						},
					},
				})).To(Succeed())

				Expect(deployer.Wait(ctx)).To(Succeed())
			})

			It("should succeed because the both ManagedResource are healthy and progressed", func() {
				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameRuntime,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 1,
						Conditions: []gardencorev1beta1.Condition{
							{
								Type:   resourcesv1alpha1.ResourcesApplied,
								Status: gardencorev1beta1.ConditionTrue,
							},
							{
								Type:   resourcesv1alpha1.ResourcesHealthy,
								Status: gardencorev1beta1.ConditionTrue,
							},
							{
								Type:   resourcesv1alpha1.ResourcesProgressing,
								Status: gardencorev1beta1.ConditionFalse,
							},
						},
					},
				})).To(Succeed())

				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameVirtual,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 1,
						Conditions: []gardencorev1beta1.Condition{
							{
								Type:   resourcesv1alpha1.ResourcesApplied,
								Status: gardencorev1beta1.ConditionTrue,
							},
							{
								Type:   resourcesv1alpha1.ResourcesHealthy,
								Status: gardencorev1beta1.ConditionTrue,
							},
							{
								Type:   resourcesv1alpha1.ResourcesProgressing,
								Status: gardencorev1beta1.ConditionFalse,
							},
						},
					},
				})).To(Succeed())

				Expect(deployer.Wait(ctx)).To(Succeed())
			})
		})

		Describe("#WaitCleanup", func() {
			It("should fail when the wait for the runtime managed resource deletion times out", func() {
				Expect(fakeClient.Create(ctx, managedResourceRuntime)).To(Succeed())

				Expect(deployer.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
			})

			It("should fail when the wait for the virtual managed resource deletion times out", func() {
				Expect(fakeClient.Create(ctx, managedResourceVirtual)).To(Succeed())

				Expect(deployer.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
			})

			It("should not return an error when they are already removed", func() {
				Expect(deployer.WaitCleanup(ctx)).To(Succeed())
			})
		})
	})
})

var (
	healthyManagedResourceStatus = resourcesv1alpha1.ManagedResourceStatus{
		ObservedGeneration: 1,
		Conditions: []gardencorev1beta1.Condition{
			{
				Type:   resourcesv1alpha1.ResourcesApplied,
				Status: gardencorev1beta1.ConditionTrue,
			},
			{
				Type:   resourcesv1alpha1.ResourcesHealthy,
				Status: gardencorev1beta1.ConditionTrue,
			},
		},
	}
	unhealthyManagedResourceStatus = resourcesv1alpha1.ManagedResourceStatus{
		ObservedGeneration: 1,
		Conditions: []gardencorev1beta1.Condition{
			{
				Type:   resourcesv1alpha1.ResourcesApplied,
				Status: gardencorev1beta1.ConditionFalse,
			},
			{
				Type:   resourcesv1alpha1.ResourcesHealthy,
				Status: gardencorev1beta1.ConditionFalse,
			},
		},
	}
)
