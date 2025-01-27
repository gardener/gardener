// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package eventlogger_test

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
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/observability/logging/eventlogger"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("EventLogger", func() {
	const (
		namespace           = "shoot--foo--bar"
		managedResourceName = "shoot-event-logger"
		name                = "event-logger"
		vpaName             = "event-logger-vpa"
		image               = "europe-docker.pkg.dev/gardener-project/releases/gardener/event-logger:v0.41.0"
	)

	var (
		ctx = context.Background()
		c   client.Client

		managedResource            *resourcesv1alpha1.ManagedResource
		managedResourceSecret      *corev1.Secret
		eventLoggerServiceAccount  *corev1.ServiceAccount
		seedEventLoggerRole        *rbacv1.Role
		seedEventLoggerRoleBinding *rbacv1.RoleBinding
		eventLoggerDeployment      *appsv1.Deployment
		vpa                        *vpaautoscalingv1.VerticalPodAutoscaler

		eventLoggerDeployer component.Deployer
		fakeSecretManager   secretsmanager.Interface
		consistOf           func(...client.Object) types.GomegaMatcher

		clusterRoleForShoot = func() *rbacv1.ClusterRole {
			return &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
					Labels: map[string]string{
						v1beta1constants.LabelApp:   name,
						v1beta1constants.LabelRole:  v1beta1constants.LabelLogging,
						v1beta1constants.GardenRole: v1beta1constants.GardenRoleLogging,
					},
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups: []string{
							"",
						},
						Resources: []string{
							"events",
						},
						Verbs: []string{
							"get",
							"list",
							"watch",
						},
					},
				},
			}
		}

		clusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:logging:event-logger",
				Labels: map[string]string{
					v1beta1constants.LabelApp:   name,
					v1beta1constants.LabelRole:  v1beta1constants.LabelLogging,
					v1beta1constants.GardenRole: v1beta1constants.GardenRoleLogging,
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     name,
			},
			Subjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      name,
				Namespace: metav1.NamespaceSystem,
			}},
		}

		roleBindingFor = func(clusterType component.ClusterType, namespace string, removeResourceVersion bool) *rbacv1.RoleBinding {
			obj := &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "gardener.cloud:logging:event-logger",
					Namespace: namespace,
					Labels: map[string]string{
						v1beta1constants.LabelApp:   name,
						v1beta1constants.LabelRole:  v1beta1constants.LabelLogging,
						v1beta1constants.GardenRole: v1beta1constants.GardenRoleLogging,
					},
					ResourceVersion: "1",
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: rbacv1.GroupName,
					Kind:     "ClusterRole",
					Name:     name,
				},
				Subjects: []rbacv1.Subject{{
					Kind:      rbacv1.ServiceAccountKind,
					Name:      name,
					Namespace: metav1.NamespaceSystem,
				}},
			}

			if clusterType == component.ClusterTypeSeed {
				obj.Subjects[0].Namespace = namespace
				obj.Name = name
				obj.RoleRef.Kind = "Role"
			}

			if removeResourceVersion {
				obj.ResourceVersion = ""
			}

			return obj
		}

		vpaUpdateMode    = vpaautoscalingv1.UpdateModeAuto
		controlledValues = vpaautoscalingv1.ContainerControlledValuesRequestsOnly
	)

	type newEventLoggerArgs struct {
		client         client.Client
		namespace      string
		secretsManager secretsmanager.Interface
		image          string
	}

	DescribeTable("#New",
		func(args newEventLoggerArgs, matchError, matchDeployer types.GomegaMatcher) {
			deployer, err := New(
				args.client,
				args.namespace,
				args.secretsManager,
				Values{
					Image:    args.image,
					Replicas: 1,
				},
			)

			Expect(err).To(matchError)
			Expect(deployer).To(matchDeployer)
		},

		Entry("pass args with nil shoot client", newEventLoggerArgs{
			client:    nil,
			namespace: namespace,
		},
			MatchError(ContainSubstring("client cannot be nil")),
			BeNil()),
		Entry("pass args with empty namespace", newEventLoggerArgs{
			client:    client.NewDryRunClient(nil),
			namespace: "",
		},
			MatchError(ContainSubstring("namespace cannot be empty")),
			BeNil()),
		Entry("pass args with empty image", newEventLoggerArgs{
			client:    client.NewDryRunClient(nil),
			namespace: namespace,
			image:     "",
		},
			MatchError(ContainSubstring("image cannot be empty")),
			BeNil()),
		Entry("pass args with nil secret manager", newEventLoggerArgs{
			client:    client.NewDryRunClient(nil),
			namespace: namespace,
			image:     image,
		},
			MatchError(ContainSubstring("secret manager cannot be nil")),
			BeNil()),
		Entry("pass valid options", newEventLoggerArgs{
			client:         client.NewDryRunClient(nil),
			namespace:      namespace,
			secretsManager: fakesecretsmanager.New(client.NewDryRunClient(nil), namespace),
			image:          image,
		},
			BeNil(),
			Not(BeNil())),
	)

	BeforeEach(func() {
		var err error
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		fakeSecretManager = fakesecretsmanager.New(c, namespace)
		consistOf = NewManagedResourceConsistOfObjectsMatcher(c)

		eventLoggerDeployer, err = New(
			c,
			namespace,
			fakeSecretManager,
			Values{
				Image:    image,
				Replicas: 1,
			},
		)
		Expect(err).ToNot(HaveOccurred())

		By("Create secrets managed outside of this package for which secretsmanager.Get() will be called")
		Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "generic-token-kubeconfig", Namespace: namespace}})).To(Succeed())
	})

	JustBeforeEach(func() {
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

		eventLoggerServiceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
		}

		seedEventLoggerRole = &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
		}

		seedEventLoggerRoleBinding = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
		}

		eventLoggerDeployment = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
		}

		vpa = &vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      vpaName,
				Namespace: namespace,
			},
		}

	})

	Describe("#Deploy", func() {
		It("should successfully deploy all resources", func() {
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(eventLoggerServiceAccount), eventLoggerServiceAccount)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(seedEventLoggerRole), seedEventLoggerRole)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(seedEventLoggerRoleBinding), seedEventLoggerRoleBinding)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(eventLoggerDeployment), eventLoggerDeployment)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(vpa), vpa)).To(BeNotFoundError())

			Expect(eventLoggerDeployer.Deploy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
			expectedMr := &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:            managedResourceName,
					Namespace:       namespace,
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
			Expect(managedResource).To(consistOf(clusterRoleForShoot(), clusterRoleBinding))

			managedResourceSecret.Name = managedResource.Spec.SecretRefs[0].Name
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
			Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(managedResourceSecret.Immutable).To(Equal(ptr.To(true)))
			Expect(managedResourceSecret.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))

			Expect(c.Get(ctx, client.ObjectKeyFromObject(eventLoggerServiceAccount), eventLoggerServiceAccount)).To(Succeed())
			Expect(eventLoggerServiceAccount).To(DeepEqual(&corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
					Labels: map[string]string{
						v1beta1constants.LabelApp:   name,
						v1beta1constants.GardenRole: v1beta1constants.GardenRoleLogging,
						v1beta1constants.LabelRole:  v1beta1constants.LabelLogging,
					},
					ResourceVersion: "1",
				},
				AutomountServiceAccountToken: ptr.To(false),
			}))

			Expect(c.Get(ctx, client.ObjectKeyFromObject(seedEventLoggerRole), seedEventLoggerRole)).To(Succeed())
			Expect(seedEventLoggerRole).To(DeepEqual(&rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
					Labels: map[string]string{
						v1beta1constants.LabelApp:   name,
						v1beta1constants.LabelRole:  v1beta1constants.LabelLogging,
						v1beta1constants.GardenRole: v1beta1constants.GardenRoleLogging,
					},
					ResourceVersion: "1",
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups: []string{
							"",
						},
						Resources: []string{
							"events",
						},
						Verbs: []string{
							"get",
							"list",
							"watch",
						},
					},
				},
			}))

			Expect(c.Get(ctx, client.ObjectKeyFromObject(seedEventLoggerRoleBinding), seedEventLoggerRoleBinding)).To(Succeed())
			Expect(seedEventLoggerRoleBinding).To(DeepEqual(roleBindingFor(component.ClusterTypeSeed, namespace, false)))
			Expect(c.Get(ctx, client.ObjectKeyFromObject(eventLoggerDeployment), eventLoggerDeployment)).To(Succeed())
			Expect(eventLoggerDeployment).To(DeepEqual(&appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
					Labels: map[string]string{
						"app":                 name,
						"role":                "logging",
						"gardener.cloud/role": "logging",
					},
					ResourceVersion: "1",
				},
				Spec: appsv1.DeploymentSpec{
					RevisionHistoryLimit: ptr.To[int32](1),
					Replicas:             ptr.To[int32](1),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app":                 name,
							"role":                "logging",
							"gardener.cloud/role": "logging",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app":                              name,
								"role":                             "logging",
								"gardener.cloud/role":              "logging",
								"networking.gardener.cloud/to-dns": "allowed",
								"networking.gardener.cloud/to-runtime-apiserver":                "allowed",
								"networking.resources.gardener.cloud/to-kube-apiserver-tcp-443": "allowed",
							},
						},
						Spec: corev1.PodSpec{
							ServiceAccountName: name,
							PriorityClassName:  "gardener-system-100",
							Containers: []corev1.Container{
								{
									Name:            name,
									Image:           image,
									ImagePullPolicy: corev1.PullIfNotPresent,
									Command: []string{
										"./event-logger",
										"--seed-event-namespaces=" + namespace,
										"--shoot-kubeconfig=/var/run/secrets/gardener.cloud/shoot/generic-kubeconfig/kubeconfig",
										"--shoot-event-namespaces=kube-system,default",
									},
									SecurityContext: &corev1.SecurityContext{
										AllowPrivilegeEscalation: ptr.To(false),
									},
									Resources: corev1.ResourceRequirements{
										Requests: corev1.ResourceList{
											corev1.ResourceCPU:    resource.MustParse("12m"),
											corev1.ResourceMemory: resource.MustParse("50Mi"),
										},
									},
									VolumeMounts: []corev1.VolumeMount{
										{
											MountPath: "/var/run/secrets/gardener.cloud/shoot/generic-kubeconfig",
											Name:      "kubeconfig",
											ReadOnly:  true,
										},
									},
								},
							},
							Volumes: []corev1.Volume{
								{
									Name: "kubeconfig",
									VolumeSource: corev1.VolumeSource{
										Projected: &corev1.ProjectedVolumeSource{
											DefaultMode: ptr.To[int32](420),
											Sources: []corev1.VolumeProjection{
												{
													Secret: &corev1.SecretProjection{
														Items: []corev1.KeyToPath{
															{
																Key:  "kubeconfig",
																Path: "kubeconfig",
															},
														},
														LocalObjectReference: corev1.LocalObjectReference{
															Name: "generic-token-kubeconfig",
														},
														Optional: ptr.To(false),
													},
												},
												{
													Secret: &corev1.SecretProjection{
														Items: []corev1.KeyToPath{
															{
																Key:  "token",
																Path: "token",
															},
														},
														LocalObjectReference: corev1.LocalObjectReference{
															Name: "shoot-access-" + name,
														},
														Optional: ptr.To(false),
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			}))

			Expect(c.Get(ctx, client.ObjectKeyFromObject(vpa), vpa)).To(Succeed())
			Expect(vpa).To(DeepEqual(&vpaautoscalingv1.VerticalPodAutoscaler{
				ObjectMeta: metav1.ObjectMeta{
					Name:            vpaName,
					Namespace:       namespace,
					ResourceVersion: "1",
				},
				Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
					TargetRef: &autoscalingv1.CrossVersionObjectReference{
						APIVersion: appsv1.SchemeGroupVersion.String(),
						Kind:       "Deployment",
						Name:       name,
					},
					UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
						UpdateMode: &vpaUpdateMode,
					},
					ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
						ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
							{
								ContainerName: vpaautoscalingv1.DefaultContainerResourcePolicy,
								MinAllowed: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("20Mi"),
								},
								ControlledValues: &controlledValues,
							},
						},
					},
				},
			}))
		})
	})

	Describe("#Destroy", func() {
		It("should successfully destroy all resources", func() {
			Expect(c.Create(ctx, managedResource)).To(Succeed())
			Expect(c.Create(ctx, managedResourceSecret)).To(Succeed())
			Expect(c.Create(ctx, eventLoggerServiceAccount)).To(Succeed())
			Expect(c.Create(ctx, seedEventLoggerRole)).To(Succeed())
			Expect(c.Create(ctx, seedEventLoggerRoleBinding)).To(Succeed())
			Expect(c.Create(ctx, eventLoggerDeployment)).To(Succeed())
			Expect(c.Create(ctx, vpa)).To(Succeed())

			Expect(eventLoggerDeployer.Destroy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(eventLoggerServiceAccount), eventLoggerServiceAccount)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(seedEventLoggerRole), seedEventLoggerRole)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(seedEventLoggerRoleBinding), seedEventLoggerRoleBinding)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(eventLoggerDeployment), eventLoggerDeployment)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(vpa), vpa)).To(BeNotFoundError())
		})
	})
})
