// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package dashboard_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
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
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/kubernetes/dashboard"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Kubernetes Dashboard", func() {
	var (
		ctx = context.Background()

		managedResourceName = "shoot-addon-kubernetes-dashboard"
		namespace           = "some-namespace"
		image               = "some-image:some-tag"
		scraperImage        = "scraper-image:scraper-tag"

		c         client.Client
		values    Values
		component component.DeployWaiter

		consistOf             func(...client.Object) types.GomegaMatcher
		managedResource       *resourcesv1alpha1.ManagedResource
		managedResourceSecret *corev1.Secret

		namespaces = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "kubernetes-dashboard",
				Labels: map[string]string{
					"gardener.cloud/purpose": "kubernetes-dashboard",
				},
			},
		}

		role = &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kubernetes-dashboard",
				Namespace: "kubernetes-dashboard",
				Labels: map[string]string{
					"k8s-app": "kubernetes-dashboard",
				},
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					ResourceNames: []string{
						"kubernetes-dashboard-key-holder",
						"kubernetes-dashboard-certs",
						"kubernetes-dashboard-csrf",
					},
					Resources: []string{"secrets"},
					Verbs: []string{
						"get",
						"update",
						"delete",
					},
				},
				{
					APIGroups: []string{""},
					ResourceNames: []string{
						"heapster",
						"dashboard-metrics-scraper",
					},
					Resources: []string{"services"},
					Verbs:     []string{"proxy"},
				},
				{
					APIGroups: []string{""},
					ResourceNames: []string{
						"kubernetes-dashboard-settings",
					},
					Resources: []string{"configmaps"},
					Verbs: []string{
						"get",
						"update",
					},
				},
				{
					APIGroups: []string{""},
					ResourceNames: []string{
						"heapster",
						"http:heapster:",
						"https:heapster:",
						"dashboard-metrics-scraper",
						"http:dashboard-metrics-scraper",
					},
					Resources: []string{"services/proxy"},
					Verbs:     []string{"get"},
				},
			},
		}

		roleBinding = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kubernetes-dashboard",
				Namespace: "kubernetes-dashboard",
				Labels: map[string]string{
					"k8s-app": "kubernetes-dashboard",
				},
				Annotations: map[string]string{
					"resources.gardener.cloud/delete-on-invalid-update": "true",
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Role",
				Name:     "kubernetes-dashboard",
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      "kubernetes-dashboard",
					Namespace: "kubernetes-dashboard",
				},
			},
		}

		clusterRole = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "kubernetes-dashboard",
				Labels: map[string]string{
					"k8s-app": "kubernetes-dashboard",
				},
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{"metrics.k8s.io"},
					Resources: []string{"pods", "nodes"},
					Verbs:     []string{"get", "list", "watch"},
				},
			},
		}

		clusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "kubernetes-dashboard",
				Annotations: map[string]string{
					"resources.gardener.cloud/delete-on-invalid-update": "true",
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "kubernetes-dashboard",
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      "kubernetes-dashboard",
					Namespace: "kubernetes-dashboard",
				},
			},
		}

		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kubernetes-dashboard",
				Namespace: "kubernetes-dashboard",
				Labels: map[string]string{
					"k8s-app": "kubernetes-dashboard",
				},
			},
			AutomountServiceAccountToken: ptr.To(false),
		}

		secretCerts = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kubernetes-dashboard-certs",
				Namespace: "kubernetes-dashboard",
				Labels: map[string]string{
					"k8s-app": "kubernetes-dashboard",
				},
			},
			Type: corev1.SecretTypeOpaque,
		}

		secretCSRF = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kubernetes-dashboard-csrf",
				Namespace: "kubernetes-dashboard",
				Labels: map[string]string{
					"k8s-app": "kubernetes-dashboard",
				},
			},
			Data: map[string][]byte{
				"csrf": []byte(""),
			},
			Type: corev1.SecretTypeOpaque,
		}

		secretKeyHolder = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kubernetes-dashboard-key-holder",
				Namespace: "kubernetes-dashboard",
				Labels: map[string]string{
					"k8s-app": "kubernetes-dashboard",
				},
			},
			Type: corev1.SecretTypeOpaque,
		}

		configMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kubernetes-dashboard-settings",
				Namespace: "kubernetes-dashboard",
				Labels: map[string]string{
					"k8s-app": "kubernetes-dashboard",
				},
			},
		}

		deploymentDashboardFor = func(apiserverHost *string, authenticationMode string) *appsv1.Deployment {
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kubernetes-dashboard",
					Namespace: "kubernetes-dashboard",
					Labels: map[string]string{
						"gardener.cloud/role": "optional-addon",
						"k8s-app":             "kubernetes-dashboard",
						"origin":              "gardener",
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas:             ptr.To[int32](1),
					RevisionHistoryLimit: ptr.To[int32](2),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"k8s-app": "kubernetes-dashboard",
						},
					},
					Strategy: appsv1.DeploymentStrategy{
						Type: appsv1.RollingUpdateDeploymentStrategyType,
						RollingUpdate: &appsv1.RollingUpdateDeployment{
							MaxSurge:       ptr.To(intstr.IntOrString{IntVal: 0}),
							MaxUnavailable: ptr.To(intstr.IntOrString{IntVal: 1}),
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"gardener.cloud/role": "optional-addon",
								"k8s-app":             "kubernetes-dashboard",
								"origin":              "gardener",
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "kubernetes-dashboard",
									Image: "some-image:some-tag",
									Args: []string{
										"--auto-generate-certificates",
										"--authentication-mode=" + authenticationMode,
										"--namespace=kubernetes-dashboard",
									},
									ImagePullPolicy: corev1.PullIfNotPresent,
									Ports: []corev1.ContainerPort{
										{
											ContainerPort: 8443,
											Protocol:      corev1.ProtocolTCP,
										},
									},
									Resources: corev1.ResourceRequirements{
										Limits: corev1.ResourceList{
											corev1.ResourceMemory: resource.MustParse("256Mi"),
										},
										Requests: corev1.ResourceList{
											corev1.ResourceCPU:    resource.MustParse("50m"),
											corev1.ResourceMemory: resource.MustParse("50Mi"),
										},
									},
									SecurityContext: &corev1.SecurityContext{
										AllowPrivilegeEscalation: ptr.To(false),
										ReadOnlyRootFilesystem:   ptr.To(true),
									},
									LivenessProbe: &corev1.Probe{
										ProbeHandler: corev1.ProbeHandler{
											HTTPGet: &corev1.HTTPGetAction{
												Path:   "/",
												Port:   intstr.IntOrString{IntVal: 8443},
												Scheme: corev1.URISchemeHTTPS,
											},
										},
										InitialDelaySeconds: 30,
										TimeoutSeconds:      30,
									},
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      "kubernetes-dashboard-certs",
											MountPath: "/certs",
										},
										{
											Name:      "tmp-volume",
											MountPath: "/tmp",
										},
									},
								},
							},
							SecurityContext: &corev1.PodSecurityContext{
								RunAsNonRoot:       ptr.To(true),
								RunAsUser:          ptr.To[int64](1001),
								RunAsGroup:         ptr.To[int64](2001),
								FSGroup:            ptr.To[int64](1),
								SupplementalGroups: []int64{1},
								SeccompProfile: &corev1.SeccompProfile{
									Type: corev1.SeccompProfileTypeRuntimeDefault,
								},
							},
							ServiceAccountName: "kubernetes-dashboard",
							Volumes: []corev1.Volume{
								{
									Name: "kubernetes-dashboard-certs",
									VolumeSource: corev1.VolumeSource{
										Secret: &corev1.SecretVolumeSource{
											SecretName: "kubernetes-dashboard-certs",
										},
									},
								},
								{
									Name:         "tmp-volume",
									VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
								},
							},
						},
					},
				},
			}

			if apiserverHost != nil {
				deployment.Spec.Template.Spec.Containers[0].Env = append(deployment.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{
					Name:  "KUBERNETES_SERVICE_HOST",
					Value: *apiserverHost,
				})
			}

			return deployment
		}

		deploymentMetricsScraper = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "dashboard-metrics-scraper",
				Namespace: "kubernetes-dashboard",
				Labels: map[string]string{
					"gardener.cloud/role": "optional-addon",
					"k8s-app":             "dashboard-metrics-scraper",
					"origin":              "gardener",
				},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas:             ptr.To[int32](1),
				RevisionHistoryLimit: ptr.To[int32](2),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"k8s-app": "dashboard-metrics-scraper",
					},
				},
				Strategy: appsv1.DeploymentStrategy{},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"gardener.cloud/role": "optional-addon",
							"k8s-app":             "dashboard-metrics-scraper",
							"origin":              "gardener",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "dashboard-metrics-scraper",
								Image: "scraper-image:scraper-tag",
								LivenessProbe: &corev1.Probe{
									ProbeHandler: corev1.ProbeHandler{
										HTTPGet: &corev1.HTTPGetAction{
											Path:   "/",
											Port:   intstr.IntOrString{IntVal: 8000},
											Scheme: corev1.URISchemeHTTP,
										},
									},
									InitialDelaySeconds: 30,
									TimeoutSeconds:      30,
								},
								Ports: []corev1.ContainerPort{
									{
										ContainerPort: 8000,
										Protocol:      corev1.ProtocolTCP,
									},
								},
								Resources: corev1.ResourceRequirements{},
								SecurityContext: &corev1.SecurityContext{
									AllowPrivilegeEscalation: ptr.To(false),
									ReadOnlyRootFilesystem:   ptr.To(true),
									RunAsNonRoot:             ptr.To(true),
									RunAsUser:                ptr.To[int64](1001),
									RunAsGroup:               ptr.To[int64](2001),
									SeccompProfile: &corev1.SeccompProfile{
										Type: corev1.SeccompProfileTypeRuntimeDefault,
									},
								},
								VolumeMounts: []corev1.VolumeMount{{MountPath: "/tmp", Name: "tmp-volume"}},
							},
						},
						SecurityContext: &corev1.PodSecurityContext{
							FSGroup:            ptr.To[int64](1),
							SupplementalGroups: []int64{1},
						},
						ServiceAccountName: "kubernetes-dashboard",
						Volumes: []corev1.Volume{
							{Name: "tmp-volume", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
						},
					},
				},
			},
		}

		serviceDashboard = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kubernetes-dashboard",
				Namespace: "kubernetes-dashboard",
				Labels: map[string]string{
					"k8s-app": "kubernetes-dashboard",
				},
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Port:       443,
						TargetPort: intstr.IntOrString{IntVal: 8443},
					},
				},
				Selector: map[string]string{
					"k8s-app": "kubernetes-dashboard",
				},
			},
		}

		serviceMetricsScraper = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "dashboard-metrics-scraper",
				Namespace: "kubernetes-dashboard",
				Labels: map[string]string{
					"k8s-app": "dashboard-metrics-scraper",
				},
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Port:       8000,
						TargetPort: intstr.IntOrString{IntVal: 8000},
					},
				},
				Selector: map[string]string{
					"k8s-app": "dashboard-metrics-scraper",
				},
			},
		}

		vpa = &vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kubernetes-dashboard",
				Namespace: "kubernetes-dashboard",
			},
			Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
				ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
					ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
						{
							ContainerName:    "*",
							ControlledValues: ptr.To(vpaautoscalingv1.ContainerControlledValuesRequestsOnly),
						},
					},
				},
				TargetRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       "kubernetes-dashboard",
				},
				UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
					UpdateMode: ptr.To(vpaautoscalingv1.UpdateModeAuto),
				},
			},
		}
	)

	BeforeEach(func() {
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		consistOf = NewManagedResourceConsistOfObjectsMatcher(c)
		values = Values{
			Image:               image,
			MetricsScraperImage: scraperImage,
		}
		component = New(c, namespace, values)

		managedResource = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceName,
				Namespace: namespace,
			},
		}
		managedResourceSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "managedresource-" + managedResource.Name,
				Namespace: namespace,
			},
		}
	})

	Describe("#Deploy", func() {
		var (
			vpaEnabled        bool
			expectedResources []client.Object
		)

		BeforeEach(func() {
			vpaEnabled = false
			expectedResources = make([]client.Object, 0)
		})

		JustBeforeEach(func() {
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())

			Expect(component.Deploy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
			expectedMr := &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:            managedResource.Name,
					Namespace:       managedResource.Namespace,
					ResourceVersion: "1",
					Labels:          map[string]string{"origin": "gardener"},
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
					SecretRefs: []corev1.LocalObjectReference{{
						Name: managedResource.Spec.SecretRefs[0].Name,
					}},
					KeepObjects: ptr.To(false),
				},
			}
			utilruntime.Must(references.InjectAnnotations(expectedMr))
			Expect(managedResource).To(DeepEqual(expectedMr))
			expectedResources = append(expectedResources,
				namespaces,
				role,
				roleBinding,
				clusterRole,
				clusterRoleBinding,
				serviceAccount,
				secretCerts,
				secretCSRF,
				secretKeyHolder,
				configMap,
				serviceDashboard,
				serviceMetricsScraper,
				deploymentMetricsScraper,
			)
			if vpaEnabled {
				expectedResources = append(expectedResources, vpa)
			}

			managedResourceSecret.Name = managedResource.Spec.SecretRefs[0].Name
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
			Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(managedResourceSecret.Immutable).To(Equal(ptr.To(true)))
		})

		Context("w/o apiserver host, w/o authentication mode, w/o vpa", func() {
			It("should successfully deploy all resources", func() {
				expectedResources = append(expectedResources, deploymentDashboardFor(nil, ""))
				Expect(managedResource).To(consistOf(expectedResources...))
			})
		})

		Context("w/ apiserver host, w/ authentication mode, w/ vpa", func() {
			var (
				apiserverHost      = "apiserver.host"
				authenticationMode = "token"
			)

			BeforeEach(func() {
				vpaEnabled = true
				values.VPAEnabled = true
				values.APIServerHost = &apiserverHost
				values.AuthenticationMode = authenticationMode
				component = New(c, namespace, values)
			})

			It("should successfully deploy all resources", func() {
				expectedResources = append(expectedResources, deploymentDashboardFor(&apiserverHost, authenticationMode))
				Expect(managedResource).To(consistOf(expectedResources...))
			})
		})
	})

	Describe("#Destroy", func() {
		It("should successfully destroy all resources", func() {
			Expect(c.Create(ctx, managedResource)).To(Succeed())
			Expect(c.Create(ctx, managedResourceSecret)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())

			Expect(component.Destroy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(BeNotFoundError())
		})
	})

	Context("waiting functions", func() {
		var (
			fakeOps   *retryfake.Ops
			resetVars func()
		)

		BeforeEach(func() {
			fakeOps = &retryfake.Ops{MaxAttempts: 1}
			resetVars = test.WithVars(
				&retry.Until, fakeOps.Until,
				&retry.UntilTimeout, fakeOps.UntilTimeout,
			)
		})

		AfterEach(func() {
			resetVars()
		})

		Describe("#Wait", func() {
			It("should fail because reading the ManagedResource fails", func() {
				Expect(component.Wait(ctx)).To(MatchError(ContainSubstring("not found")))
			})

			It("should fail because the ManagedResource doesn't become healthy", func() {
				fakeOps.MaxAttempts = 2

				Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceName,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
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
					},
				})).To(Succeed())

				Expect(component.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
			})

			It("should successfully wait for the managed resource to become healthy", func() {
				fakeOps.MaxAttempts = 2

				Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceName,
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
						},
					},
				})).To(Succeed())

				Expect(component.Wait(ctx)).To(Succeed())
			})
		})

		Describe("#WaitCleanup", func() {
			It("should fail when the wait for the managed resource deletion times out", func() {
				fakeOps.MaxAttempts = 2

				Expect(c.Create(ctx, managedResource)).To(Succeed())

				Expect(component.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
			})

			It("should not return an error when it's already removed", func() {
				Expect(component.WaitCleanup(ctx)).To(Succeed())
			})
		})
	})
})
