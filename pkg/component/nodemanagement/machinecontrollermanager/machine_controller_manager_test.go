// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package machinecontrollermanager_test

import (
	"context"
	"time"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/component/nodemanagement/machinecontrollermanager"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("MachineControllerManager", func() {
	var (
		ctx       = context.Background()
		namespace = "shoot--foo--bar"

		image        = "mcm-image:tag"
		namespaceUID = types.UID("uid")
		replicas     = int32(1)

		fakeClient client.Client
		sm         secretsmanager.Interface
		values     Values
		mcm        Interface

		clusterRoleYAML        string
		clusterRoleBindingYAML string
		roleYAML               string
		roleBindingYAML        string

		serviceAccount        *corev1.ServiceAccount
		clusterRoleBinding    *rbacv1.ClusterRoleBinding
		roleBinding           *rbacv1.RoleBinding
		role                  *rbacv1.Role
		service               *corev1.Service
		shootAccessSecret     *corev1.Secret
		deployment            *appsv1.Deployment
		podDisruptionBudget   *policyv1.PodDisruptionBudget
		vpa                   *vpaautoscalingv1.VerticalPodAutoscaler
		prometheusRule        *monitoringv1.PrometheusRule
		serviceMonitor        *monitoringv1.ServiceMonitor
		managedResourceSecret *corev1.Secret
		managedResource       *resourcesv1alpha1.ManagedResource
	)

	JustBeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		sm = fakesecretsmanager.New(fakeClient, namespace)
		values = Values{
			Image:    image,
			Replicas: replicas,
		}
		mcm = New(fakeClient, namespace, sm, values)
		mcm.SetNamespaceUID(namespaceUID)

		By("Create secrets managed outside of this package for whose secretsmanager.Get() will be called")
		Expect(fakeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "generic-token-kubeconfig", Namespace: namespace}})).To(Succeed())

		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "machine-controller-manager",
				Namespace: namespace,
			},
			AutomountServiceAccountToken: ptr.To(false),
		}

		clusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "machine-controller-manager-" + namespace,
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "v1",
					Kind:               "Namespace",
					Name:               namespace,
					UID:                namespaceUID,
					Controller:         ptr.To(true),
					BlockOwnerDeletion: ptr.To(true),
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

		roleBinding = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "machine-controller-manager",
				Namespace: namespace,
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Role",
				Name:     "machine-controller-manager",
			},
			Subjects: []rbacv1.Subject{{
				Kind:      "ServiceAccount",
				Name:      "machine-controller-manager",
				Namespace: namespace,
			}},
		}

		role = &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "machine-controller-manager",
				Namespace: namespace,
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{machinev1alpha1.GroupName},
					Resources: []string{
						"machineclasses",
						"machineclasses/status",
						"machinedeployments",
						"machinedeployments/status",
						"machines",
						"machines/status",
						"machinesets",
						"machinesets/status",
					},
					Verbs: []string{"create", "get", "list", "patch", "update", "watch", "delete", "deletecollection"},
				},
				{
					APIGroups: []string{corev1.GroupName},
					Resources: []string{"configmaps", "secrets", "endpoints", "events", "pods"},
					Verbs:     []string{"create", "get", "list", "patch", "update", "watch", "delete", "deletecollection"},
				},
				{
					APIGroups: []string{coordinationv1.GroupName},
					Resources: []string{"leases"},
					Verbs:     []string{"create"},
				},
				{
					APIGroups:     []string{coordinationv1.GroupName},
					Resources:     []string{"leases"},
					Verbs:         []string{"get", "watch", "update"},
					ResourceNames: []string{"machine-controller", "machine-controller-manager"},
				},
			},
		}

		service = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "machine-controller-manager",
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "kubernetes",
					"role": "machine-controller-manager",
				},
				Annotations: map[string]string{
					"networking.resources.gardener.cloud/from-all-scrape-targets-allowed-ports": `[{"protocol":"TCP","port":10258},{"protocol":"TCP","port":10259}]`,
				},
			},
			Spec: corev1.ServiceSpec{
				Type:      corev1.ServiceTypeClusterIP,
				ClusterIP: corev1.ClusterIPNone,
				Ports: []corev1.ServicePort{
					{
						Name:     "metrics",
						Port:     10258,
						Protocol: corev1.ProtocolTCP,
					},
					{
						Name:     "providermetrics",
						Port:     10259,
						Protocol: corev1.ProtocolTCP,
					},
				},
				Selector: map[string]string{
					"app":  "kubernetes",
					"role": "machine-controller-manager",
				},
			},
		}

		shootAccessSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shoot-access-machine-controller-manager",
				Namespace: namespace,
				Labels: map[string]string{
					"resources.gardener.cloud/purpose": "token-requestor",
					"resources.gardener.cloud/class":   "shoot",
				},
				Annotations: map[string]string{
					"serviceaccount.resources.gardener.cloud/name":      "machine-controller-manager",
					"serviceaccount.resources.gardener.cloud/namespace": "kube-system",
				},
			},
			Type: corev1.SecretTypeOpaque,
		}

		deployment = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "machine-controller-manager",
				Namespace: namespace,
				Labels: map[string]string{
					"app":                 "kubernetes",
					"role":                "machine-controller-manager",
					"gardener.cloud/role": "controlplane",
					"high-availability-config.resources.gardener.cloud/type":             "controller",
					"provider.extensions.gardener.cloud/mutated-by-controlplane-webhook": "true",
				},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas:             &replicas,
				RevisionHistoryLimit: ptr.To[int32](2),
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
								"--concurrent-syncs=30",
								"--kube-api-qps=150",
								"--kube-api-burst=200",
								"--v=3",
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/healthz",
										Port:   intstr.FromInt32(10258),
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
									corev1.ResourceCPU:    resource.MustParse("5m"),
									corev1.ResourceMemory: resource.MustParse("20M"),
								},
							},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: ptr.To(false),
							},
						}},
						PriorityClassName:             "gardener-system-300",
						ServiceAccountName:            "machine-controller-manager",
						TerminationGracePeriodSeconds: ptr.To[int64](5),
					},
				},
			},
		}
		Expect(gardenerutils.InjectGenericKubeconfig(deployment, "generic-token-kubeconfig", shootAccessSecret.Name)).To(Succeed())

		podDisruptionBudget = &policyv1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "machine-controller-manager",
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "kubernetes",
					"role": "machine-controller-manager",
				},
			},
			Spec: policyv1.PodDisruptionBudgetSpec{
				MaxUnavailable: ptr.To(intstr.FromInt32(1)),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app":  "kubernetes",
						"role": "machine-controller-manager",
					},
				},
				UnhealthyPodEvictionPolicy: ptr.To(policyv1.AlwaysAllow),
			},
		}

		vpa = &vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "machine-controller-manager-vpa",
				Namespace: namespace,
				Labels: map[string]string{
					"provider.extensions.gardener.cloud/mutated-by-controlplane-webhook": "true",
				},
			},
			Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
				TargetRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       "machine-controller-manager",
				},
				UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
					UpdateMode: ptr.To(vpaautoscalingv1.UpdateModeAuto),
				},
				ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
					ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{{
						ContainerName:    "machine-controller-manager",
						ControlledValues: ptr.To(vpaautoscalingv1.ContainerControlledValuesRequestsOnly),
					}},
				},
			},
		}
		prometheusRule = &monitoringv1.PrometheusRule{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shoot-machine-controller-manager",
				Namespace: namespace,
				Labels:    map[string]string{"prometheus": "shoot"},
			},
			Spec: monitoringv1.PrometheusRuleSpec{
				Groups: []monitoringv1.RuleGroup{{
					Name: "machine-controller-manager.rules",
					Rules: []monitoringv1.Rule{{
						Alert: "MachineControllerManagerDown",
						Expr:  intstr.FromString(`absent(up{job="machine-controller-manager"} == 1)`),
						For:   ptr.To(monitoringv1.Duration("15m")),
						Labels: map[string]string{
							"service":    "machine-controller-manager",
							"severity":   "critical",
							"type":       "seed",
							"visibility": "operator",
						},
						Annotations: map[string]string{
							"summary":     "Machine controller manager is down.",
							"description": "There are no running machine controller manager instances. No shoot nodes can be created/maintained.",
						},
					}},
				}},
			},
		}
		serviceMonitor = &monitoringv1.ServiceMonitor{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shoot-machine-controller-manager",
				Namespace: namespace,
				Labels:    map[string]string{"prometheus": "shoot"},
			},
			Spec: monitoringv1.ServiceMonitorSpec{
				Selector: metav1.LabelSelector{MatchLabels: map[string]string{
					"app":  "kubernetes",
					"role": "machine-controller-manager",
				}},
				Endpoints: []monitoringv1.Endpoint{
					{
						Port: "metrics",
						RelabelConfigs: []monitoringv1.RelabelConfig{{
							Action: "labelmap",
							Regex:  `__meta_kubernetes_service_label_(.+)`,
						}},
						MetricRelabelConfigs: []monitoringv1.RelabelConfig{{
							SourceLabels: []monitoringv1.LabelName{"__name__"},
							Action:       "keep",
							Regex:        `^(mcm_machine_deployment_items_total|mcm_machine_deployment_info|mcm_machine_deployment_info_spec_paused|mcm_machine_deployment_info_spec_replicas|mcm_machine_deployment_info_spec_min_ready_seconds|mcm_machine_deployment_info_spec_rolling_update_max_surge|mcm_machine_deployment_info_spec_rolling_update_max_unavailable|mcm_machine_deployment_info_spec_revision_history_limit|mcm_machine_deployment_info_spec_progress_deadline_seconds|mcm_machine_deployment_info_spec_rollback_to_revision|mcm_machine_deployment_status_condition|mcm_machine_deployment_status_available_replicas|mcm_machine_deployment_status_unavailable_replicas|mcm_machine_deployment_status_ready_replicas|mcm_machine_deployment_status_updated_replicas|mcm_machine_deployment_status_collision_count|mcm_machine_deployment_status_replicas|mcm_machine_deployment_failed_machines|mcm_machine_set_items_total|mcm_machine_set_info|mcm_machine_set_failed_machines|mcm_machine_set_info_spec_replicas|mcm_machine_set_info_spec_min_ready_seconds|mcm_machine_set_status_condition|mcm_machine_set_status_available_replicas|mcm_machine_set_status_fully_labelled_replicas|mcm_machine_set_status_replicas|mcm_machine_set_status_ready_replicas|mcm_machine_stale_machines_total|mcm_misc_scrape_failure_total|process_max_fds|process_open_fds|mcm_workqueue_adds_total|mcm_workqueue_depth|mcm_workqueue_queue_duration_seconds_bucket|mcm_workqueue_queue_duration_seconds_sum|mcm_workqueue_queue_duration_seconds_count|mcm_workqueue_work_duration_seconds_bucket|mcm_workqueue_work_duration_seconds_sum|mcm_workqueue_work_duration_seconds_count|mcm_workqueue_unfinished_work_seconds|mcm_workqueue_longest_running_processor_seconds|mcm_workqueue_retries_total)$`,
						}},
					},
					{
						Port: "providermetrics",
						RelabelConfigs: []monitoringv1.RelabelConfig{{
							Action: "labelmap",
							Regex:  `__meta_kubernetes_service_label_(.+)`,
						}},
						MetricRelabelConfigs: []monitoringv1.RelabelConfig{{
							SourceLabels: []monitoringv1.LabelName{"__name__"},
							Action:       "keep",
							Regex:        `^(mcm_machine_items_total|mcm_machine_current_status_phase|mcm_machine_info|mcm_machine_status_condition|mcm_cloud_api_requests_total|mcm_cloud_api_requests_failed_total|mcm_cloud_api_api_request_duration_seconds_bucket|mcm_cloud_api_api_request_duration_seconds_sum|mcm_cloud_api_api_request_duration_seconds_count|mcm_cloud_api_driver_request_duration_seconds_sum|mcm_cloud_api_driver_request_duration_seconds_count|mcm_cloud_api_driver_request_duration_seconds_bucket|mcm_cloud_api_driver_request_failed_total|mcm_machine_controller_frozen|process_max_fds|process_open_fds|mcm_workqueue_adds_total|mcm_workqueue_depth|mcm_workqueue_queue_duration_seconds_bucket|mcm_workqueue_queue_duration_seconds_sum|mcm_workqueue_queue_duration_seconds_count|mcm_workqueue_work_duration_seconds_bucket|mcm_workqueue_work_duration_seconds_sum|mcm_workqueue_work_duration_seconds_count|mcm_workqueue_unfinished_work_seconds|mcm_workqueue_longest_running_processor_seconds|mcm_workqueue_retries_total)$`,
						}},
					},
				},
			},
		}

		clusterRoleYAML = `apiVersion: rbac.authorization.k8s.io/v1
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
  - delete
  - get
  - list
  - watch
`

		clusterRoleBindingYAML = `apiVersion: rbac.authorization.k8s.io/v1
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

		roleYAML = `apiVersion: rbac.authorization.k8s.io/v1
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

		roleBindingYAML = `apiVersion: rbac.authorization.k8s.io/v1
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
			ObjectMeta: metav1.ObjectMeta{
				Name:      "managedresource-shoot-core-machine-controller-manager",
				Namespace: namespace,
			},
		}
		managedResource = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shoot-core-machine-controller-manager",
				Namespace: namespace,
				Labels:    map[string]string{"origin": "gardener"},
			},
			Spec: resourcesv1alpha1.ManagedResourceSpec{
				SecretRefs:   []corev1.LocalObjectReference{{Name: managedResourceSecret.Name}},
				InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
				KeepObjects:  ptr.To(false),
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

			//TODO: @aaronfern Remove this after g/g:v1.119 is released
			actualClusterRoleBinding := &rbacv1.ClusterRoleBinding{}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(clusterRoleBinding), actualClusterRoleBinding)).ToNot(Succeed())

			actualRoleBinding := &rbacv1.RoleBinding{}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(roleBinding), actualRoleBinding)).To(Succeed())
			roleBinding.ResourceVersion = "1"
			Expect(actualRoleBinding).To(Equal(roleBinding))

			actualRole := &rbacv1.Role{}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(role), actualRole)).To(Succeed())
			role.ResourceVersion = "1"
			Expect(actualRole).To(Equal(role))

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

			actualVPA := &vpaautoscalingv1.VerticalPodAutoscaler{}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(vpa), actualVPA)).To(Succeed())
			vpa.ResourceVersion = "1"
			Expect(actualVPA).To(Equal(vpa))

			actualPodDisruptionBudget := &policyv1.PodDisruptionBudget{}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(podDisruptionBudget), actualPodDisruptionBudget)).To(Succeed())
			podDisruptionBudget.ResourceVersion = "1"
			Expect(actualPodDisruptionBudget).To(Equal(podDisruptionBudget))

			actualPrometheusRule := &monitoringv1.PrometheusRule{}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(prometheusRule), actualPrometheusRule)).To(Succeed())
			prometheusRule.ResourceVersion = "1"
			Expect(actualPrometheusRule).To(DeepEqual(prometheusRule))

			actualServiceMonitor := &monitoringv1.ServiceMonitor{}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(serviceMonitor), actualServiceMonitor)).To(Succeed())
			serviceMonitor.ResourceVersion = "1"
			Expect(actualServiceMonitor).To(DeepEqual(serviceMonitor))

			actualManagedResource := &resourcesv1alpha1.ManagedResource{}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResource), actualManagedResource)).To(Succeed())
			managedResource.ResourceVersion = "1"
			managedResource.Spec.SecretRefs[0] = actualManagedResource.Spec.SecretRefs[0]
			utilruntime.Must(references.InjectAnnotations(managedResource))
			Expect(actualManagedResource).To(Equal(managedResource))

			actualManagedResourceSecret := &corev1.Secret{}
			managedResourceSecret.Name = actualManagedResource.Spec.SecretRefs[0].Name
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), actualManagedResourceSecret)).To(Succeed())
			Expect(actualManagedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(actualManagedResourceSecret.Immutable).To(Equal(ptr.To(true)))

			manifests, err := test.ExtractManifestsFromManagedResourceData(actualManagedResourceSecret.Data)
			Expect(err).NotTo(HaveOccurred())

			Expect(manifests).To(ConsistOf(
				clusterRoleYAML,
				clusterRoleBindingYAML,
				roleYAML,
				roleBindingYAML,
			))
		})

		//TODO: @aaronfern Remove this after g/g:v1.119 is released
		It("should successfully delete existing clusterRoleBinding when deploy is called", func() {
			Expect(fakeClient.Create(ctx, clusterRoleBinding)).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(clusterRoleBinding), &rbacv1.ClusterRoleBinding{})).To(Succeed())

			Expect(mcm.Deploy(ctx)).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(clusterRoleBinding), &rbacv1.ClusterRoleBinding{})).ToNot(Succeed())
		})
	})

	Describe("#Destroy", func() {
		It("should successfully destroy all resources", func() {
			Expect(fakeClient.Create(ctx, serviceAccount)).To(Succeed())
			Expect(fakeClient.Create(ctx, clusterRoleBinding)).To(Succeed())
			Expect(fakeClient.Create(ctx, role)).To(Succeed())
			Expect(fakeClient.Create(ctx, roleBinding)).To(Succeed())
			Expect(fakeClient.Create(ctx, service)).To(Succeed())
			Expect(fakeClient.Create(ctx, shootAccessSecret)).To(Succeed())
			Expect(fakeClient.Create(ctx, deployment)).To(Succeed())
			Expect(fakeClient.Create(ctx, podDisruptionBudget)).To(Succeed())
			Expect(fakeClient.Create(ctx, vpa)).To(Succeed())
			Expect(fakeClient.Create(ctx, prometheusRule)).To(Succeed())
			Expect(fakeClient.Create(ctx, serviceMonitor)).To(Succeed())
			Expect(fakeClient.Create(ctx, managedResource)).To(Succeed())

			Expect(mcm.Destroy(ctx)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResource), &resourcesv1alpha1.ManagedResource{})).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(serviceMonitor), &monitoringv1.ServiceMonitor{})).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(prometheusRule), &monitoringv1.PrometheusRule{})).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(vpa), &vpaautoscalingv1.VerticalPodAutoscaler{})).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(podDisruptionBudget), &policyv1.PodDisruptionBudget{})).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(deployment), &appsv1.Deployment{})).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(shootAccessSecret), &corev1.Secret{})).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(service), &corev1.Service{})).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(clusterRoleBinding), &rbacv1.ClusterRoleBinding{})).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(role), &rbacv1.ClusterRoleBinding{})).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(roleBinding), &rbacv1.ClusterRoleBinding{})).To(BeNotFoundError())
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
				deploy.Spec.Replicas = ptr.To[int32](1)
				deploy.Status.Conditions = []appsv1.DeploymentCondition{
					{Type: appsv1.DeploymentProgressing, Status: "True", Reason: "NewReplicaSetAvailable"},
					{Type: appsv1.DeploymentAvailable, Status: "True"},
				}
				deploy.Status.ObservedGeneration = deploy.Generation
				deploy.Status.Replicas = *deploy.Spec.Replicas
				deploy.Status.UpdatedReplicas = *deploy.Spec.Replicas
				deploy.Status.AvailableReplicas = *deploy.Spec.Replicas
				Expect(fakeClient.Status().Update(ctx, deploy)).To(Succeed())
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
