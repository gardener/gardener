// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package vpa_test

import (
	"context"
	_ "embed"
	"fmt"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/autoscaling/vpa"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var (
	//go:embed templates/crd-autoscaling.k8s.io_verticalpodautoscalers.yaml
	verticalPodAutoscalerCRD []byte
	//go:embed templates/crd-autoscaling.k8s.io_verticalpodautoscalercheckpoints.yaml
	verticalPodAutoscalerCheckpointCRD []byte
)

var _ = Describe("VPA", func() {
	var (
		ctx = context.Background()

		namespace    = "some-namespace"
		secretNameCA = "ca"

		genericTokenKubeconfigSecretName = "generic-token-kubeconfig"
		pathGenericKubeconfig            = "/var/run/secrets/gardener.cloud/shoot/generic-kubeconfig/kubeconfig"

		runtimeKubernetesVersion = semver.MustParse("1.25.0")
		values                   = Values{
			SecretNameServerCA:       secretNameCA,
			RuntimeKubernetesVersion: runtimeKubernetesVersion,
		}

		c         client.Client
		sm        secretsmanager.Interface
		vpa       component.DeployWaiter
		consistOf func(...client.Object) types.GomegaMatcher
		contain   func(...client.Object) types.GomegaMatcher
		vpaFor    func(component.ClusterType, bool) component.DeployWaiter

		imageAdmissionController = "some-image:for-admission-controller"
		imageRecommender         = "some-image:for-recommender"
		imageUpdater             = "some-image:for-updater"

		livenessProbeVpa = &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   "/health-check",
					Port:   intstr.IntOrString{Type: intstr.String, StrVal: "metrics"},
					Scheme: corev1.URISchemeHTTP,
				},
			},
			InitialDelaySeconds: 120,
			PeriodSeconds:       60,
			TimeoutSeconds:      30,
			SuccessThreshold:    1,
			FailureThreshold:    3,
		}

		valuesAdmissionController ValuesAdmissionController
		valuesRecommender         ValuesRecommender
		valuesUpdater             ValuesUpdater

		vpaUpdateModeAuto   = vpaautoscalingv1.UpdateModeAuto
		vpaControlledValues = vpaautoscalingv1.ContainerControlledValuesRequestsOnly
		maxUnavailable      = intstr.FromInt32(1)

		webhookFailurePolicy      = admissionregistrationv1.Ignore
		webhookMatchPolicy        = admissionregistrationv1.Exact
		webhookReinvocationPolicy = admissionregistrationv1.IfNeededReinvocationPolicy
		webhookSideEffects        = admissionregistrationv1.SideEffectClassNone
		webhookScope              = admissionregistrationv1.AllScopes

		managedResourceName   string
		managedResource       *resourcesv1alpha1.ManagedResource
		managedResourceSecret *corev1.Secret

		serviceAccountUpdater           *corev1.ServiceAccount
		clusterRoleUpdater              *rbacv1.ClusterRole
		clusterRoleBindingUpdater       *rbacv1.ClusterRoleBinding
		roleLeaderLockingUpdater        *rbacv1.Role
		roleBindingLeaderLockingUpdater *rbacv1.RoleBinding
		shootAccessSecretUpdater        *corev1.Secret
		deploymentUpdaterFor            func(bool, *metav1.Duration, *metav1.Duration, *int32, *float64, *float64, string) *appsv1.Deployment
		podDisruptionBudgetUpdater      *policyv1.PodDisruptionBudget
		vpaUpdater                      *vpaautoscalingv1.VerticalPodAutoscaler

		serviceAccountRecommender                    *corev1.ServiceAccount
		clusterRoleRecommenderMetricsReader          *rbacv1.ClusterRole
		clusterRoleBindingRecommenderMetricsReader   *rbacv1.ClusterRoleBinding
		clusterRoleRecommenderCheckpointActor        *rbacv1.ClusterRole
		clusterRoleBindingRecommenderCheckpointActor *rbacv1.ClusterRoleBinding
		clusterRoleRecommenderStatusActor            *rbacv1.ClusterRole
		clusterRoleBindingRecommenderStatusActor     *rbacv1.ClusterRoleBinding
		roleLeaderLockingRecommender                 *rbacv1.Role
		roleBindingLeaderLockingRecommender          *rbacv1.RoleBinding
		serviceRecommenderFor                        func(component.ClusterType) *corev1.Service
		shootAccessSecretRecommender                 *corev1.Secret
		deploymentRecommenderFor                     func(bool, *metav1.Duration, *float64, component.ClusterType, *float64, *float64, *float64, *metav1.Duration, *float64, *float64, *float64, *metav1.Duration, *metav1.Duration, *int64, string) *appsv1.Deployment
		podDisruptionBudgetRecommender               *policyv1.PodDisruptionBudget
		vpaRecommender                               *vpaautoscalingv1.VerticalPodAutoscaler
		serviceMonitorRecommenderFor                 func(clusterType component.ClusterType) *monitoringv1.ServiceMonitor

		serviceAccountAdmissionController      *corev1.ServiceAccount
		clusterRoleAdmissionController         *rbacv1.ClusterRole
		clusterRoleBindingAdmissionController  *rbacv1.ClusterRoleBinding
		shootAccessSecretAdmissionController   *corev1.Secret
		serviceAdmissionControllerFor          func(component.ClusterType, bool) *corev1.Service
		deploymentAdmissionControllerFor       func(bool) *appsv1.Deployment
		podDisruptionBudgetAdmissionController *policyv1.PodDisruptionBudget
		vpaAdmissionController                 *vpaautoscalingv1.VerticalPodAutoscaler
		serviceMonitorAdmissionController      *monitoringv1.ServiceMonitor

		clusterRoleGeneralActor               *rbacv1.ClusterRole
		clusterRoleBindingGeneralActor        *rbacv1.ClusterRoleBinding
		clusterRoleGeneralTargetReader        *rbacv1.ClusterRole
		clusterRoleBindingGeneralTargetReader *rbacv1.ClusterRoleBinding
		mutatingWebhookConfiguration          *admissionregistrationv1.MutatingWebhookConfiguration
	)

	BeforeEach(func() {
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		sm = fakesecretsmanager.New(c, namespace)
		consistOf = NewManagedResourceConsistOfObjectsMatcher(c)
		contain = NewManagedResourceContainsObjectsMatcher(c)

		valuesAdmissionController = ValuesAdmissionController{
			Image:             imageAdmissionController,
			PriorityClassName: "priority-admission-controller",
		}
		valuesRecommender = ValuesRecommender{
			Image:             imageRecommender,
			PriorityClassName: "priority-recommender",
		}
		valuesUpdater = ValuesUpdater{
			Image:             imageUpdater,
			PriorityClassName: "priority-updater",
		}

		vpa = New(c, namespace, sm, values)
		managedResourceName = ""

		By("Create secrets managed outside of this package for whose secretsmanager.Get() will be called")
		Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca", Namespace: namespace}})).To(Succeed())
		Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "generic-token-kubeconfig", Namespace: namespace}})).To(Succeed())

		vpaFor = func(clusterType component.ClusterType, IsGardenCluster bool) component.DeployWaiter {
			vpa = New(c, namespace, sm, Values{
				ClusterType:              clusterType,
				IsGardenCluster:          IsGardenCluster,
				Enabled:                  true,
				SecretNameServerCA:       secretNameCA,
				RuntimeKubernetesVersion: runtimeKubernetesVersion,
				AdmissionController:      valuesAdmissionController,
				Recommender:              valuesRecommender,
				Updater:                  valuesUpdater,
			})
			return vpa
		}

		serviceAccountUpdater = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vpa-updater",
				Namespace: namespace,
				Labels: map[string]string{
					"gardener.cloud/role": "vpa",
				},
			},
			AutomountServiceAccountToken: ptr.To(false),
		}
		clusterRoleUpdater = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:vpa:target:evictioner",
				Labels: map[string]string{
					"gardener.cloud/role": "vpa",
				},
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{"apps", "extensions"},
					Resources: []string{"replicasets"},
					Verbs:     []string{"get"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"pods/eviction"},
					Verbs:     []string{"create"},
				},
			},
		}
		clusterRoleBindingUpdater = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:vpa:target:evictioner",
				Labels: map[string]string{
					"gardener.cloud/role": "vpa",
				},
				Annotations: map[string]string{
					"resources.gardener.cloud/delete-on-invalid-update": "true",
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "gardener.cloud:vpa:target:evictioner",
			},
			Subjects: []rbacv1.Subject{{
				Kind:      "ServiceAccount",
				Name:      "vpa-updater",
				Namespace: namespace,
			}},
		}
		roleLeaderLockingUpdater = &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener.cloud:vpa:target:leader-locking-vpa-updater",
				Namespace: namespace,
				Labels: map[string]string{
					"gardener.cloud/role": "vpa",
				},
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{"coordination.k8s.io"},
					Resources: []string{"leases"},
					Verbs:     []string{"create"},
				},
				{
					APIGroups:     []string{"coordination.k8s.io"},
					Resources:     []string{"leases"},
					ResourceNames: []string{"vpa-updater"},
					Verbs:         []string{"get", "watch", "update"},
				},
			},
		}
		roleBindingLeaderLockingUpdater = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener.cloud:vpa:target:leader-locking-vpa-updater",
				Namespace: namespace,
				Labels: map[string]string{
					"gardener.cloud/role": "vpa",
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Role",
				Name:     "gardener.cloud:vpa:target:leader-locking-vpa-updater",
			},
			Subjects: []rbacv1.Subject{{
				Kind:      "ServiceAccount",
				Name:      "vpa-updater",
				Namespace: namespace,
			}},
		}
		shootAccessSecretUpdater = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shoot-access-vpa-updater",
				Namespace: namespace,
				Labels: map[string]string{
					"resources.gardener.cloud/purpose": "token-requestor",
					"resources.gardener.cloud/class":   "shoot",
				},
				Annotations: map[string]string{
					"serviceaccount.resources.gardener.cloud/name":      "vpa-updater",
					"serviceaccount.resources.gardener.cloud/namespace": "kube-system",
				},
			},
			Type: corev1.SecretTypeOpaque,
		}
		deploymentUpdaterFor = func(
			withServiceAccount bool,
			interval *metav1.Duration,
			evictAfterOOMThreshold *metav1.Duration,
			evictionRateBurst *int32,
			evictionRateLimit *float64,
			evictionTolerance *float64,
			leaderElectionNamespace string,
		) *appsv1.Deployment {
			var (
				flagEvictionToleranceValue      = "0.500000"
				flagEvictionRateBurstValue      = "1"
				flagEvictionRateLimitValue      = "-1.000000"
				flagEvictAfterOomThresholdValue = "10m0s"
				flagUpdaterIntervalValue        = "1m0s"
			)

			if interval != nil {
				flagUpdaterIntervalValue = interval.Duration.String()
			}
			if evictAfterOOMThreshold != nil {
				flagEvictAfterOomThresholdValue = evictAfterOOMThreshold.Duration.String()
			}
			if evictionRateBurst != nil {
				flagEvictionRateBurstValue = fmt.Sprintf("%d", *evictionRateBurst)
			}
			if evictionRateLimit != nil {
				flagEvictionRateLimitValue = fmt.Sprintf("%f", *evictionRateLimit)
			}
			if evictionTolerance != nil {
				flagEvictionToleranceValue = fmt.Sprintf("%f", *evictionTolerance)
			}

			obj := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vpa-updater",
					Namespace: namespace,
					Labels: map[string]string{
						"app":                 "vpa-updater",
						"gardener.cloud/role": "vpa",
						"high-availability-config.resources.gardener.cloud/type": "controller",
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas:             ptr.To[int32](1),
					RevisionHistoryLimit: ptr.To[int32](2),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "vpa-updater",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app":                              "vpa-updater",
								"gardener.cloud/role":              "vpa",
								"networking.gardener.cloud/to-dns": "allowed",
								"networking.resources.gardener.cloud/to-kube-apiserver-tcp-443": "allowed",
							},
						},
						Spec: corev1.PodSpec{
							PriorityClassName: valuesUpdater.PriorityClassName,
							Containers: []corev1.Container{{
								Name:            "updater",
								Image:           imageUpdater,
								ImagePullPolicy: corev1.PullIfNotPresent,
								Args: []string{
									"--min-replicas=1",
									fmt.Sprintf("--eviction-tolerance=%s", flagEvictionToleranceValue),
									fmt.Sprintf("--eviction-rate-burst=%s", flagEvictionRateBurstValue),
									fmt.Sprintf("--eviction-rate-limit=%s", flagEvictionRateLimitValue),
									fmt.Sprintf("--evict-after-oom-threshold=%s", flagEvictAfterOomThresholdValue),
									fmt.Sprintf("--updater-interval=%s", flagUpdaterIntervalValue),
									"--stderrthreshold=info",
									"--v=2",
									"--kube-api-qps=100",
									"--kube-api-burst=120",
									"--leader-elect=true",
									fmt.Sprintf("--leader-elect-resource-namespace=%s", leaderElectionNamespace),
								},
								LivenessProbe: livenessProbeVpa,
								Ports: []corev1.ContainerPort{
									{
										Name:          "server",
										ContainerPort: 8080,
									},
									{
										Name:          "metrics",
										ContainerPort: 8943,
									},
								},
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("10m"),
										corev1.ResourceMemory: resource.MustParse("15Mi"),
									},
								},
								SecurityContext: &corev1.SecurityContext{
									AllowPrivilegeEscalation: ptr.To(false),
								},
							}},
						},
					},
				},
			}

			if withServiceAccount {
				obj.Spec.Template.Spec.ServiceAccountName = serviceAccountUpdater.Name
				obj.Spec.Template.Spec.Containers[0].Env = append(obj.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{
					Name: "NAMESPACE",
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: "metadata.namespace",
						},
					},
				})
			} else {
				obj.Labels["gardener.cloud/role"] = "controlplane"
				obj.Spec.Template.Spec.AutomountServiceAccountToken = ptr.To(false)
				obj.Spec.Template.Spec.Containers[0].Args = append(obj.Spec.Template.Spec.Containers[0].Args, "--kubeconfig="+pathGenericKubeconfig)

				Expect(gardenerutils.InjectGenericKubeconfig(obj, genericTokenKubeconfigSecretName, shootAccessSecretUpdater.Name)).To(Succeed())
			}

			return obj
		}
		podDisruptionBudgetUpdater = &policyv1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vpa-updater",
				Namespace: namespace,
				Labels:    map[string]string{"gardener.cloud/role": "vpa"},
			},
			Spec: policyv1.PodDisruptionBudgetSpec{
				MaxUnavailable: &maxUnavailable,
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "vpa-updater",
					},
				},
			},
		}
		vpaUpdater = &vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vpa-updater",
				Namespace: namespace,
			},
			Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
				TargetRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       "vpa-updater",
				},
				UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{UpdateMode: &vpaUpdateModeAuto},
				ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
					ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
						{
							ContainerName:    "*",
							ControlledValues: &vpaControlledValues,
						},
					},
				},
			},
		}

		serviceAccountRecommender = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vpa-recommender",
				Namespace: namespace,
				Labels: map[string]string{
					"gardener.cloud/role": "vpa",
				},
			},
			AutomountServiceAccountToken: ptr.To(false),
		}
		clusterRoleRecommenderMetricsReader = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:vpa:target:metrics-reader",
				Labels: map[string]string{
					"gardener.cloud/role": "vpa",
				},
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{"metrics.k8s.io"},
					Resources: []string{"pods"},
					Verbs:     []string{"get", "list"},
				},
			},
		}
		clusterRoleBindingRecommenderMetricsReader = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:vpa:target:metrics-reader",
				Labels: map[string]string{
					"gardener.cloud/role": "vpa",
				},
				Annotations: map[string]string{
					"resources.gardener.cloud/delete-on-invalid-update": "true",
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "gardener.cloud:vpa:target:metrics-reader",
			},
			Subjects: []rbacv1.Subject{{
				Kind:      "ServiceAccount",
				Name:      "vpa-recommender",
				Namespace: namespace,
			}},
		}
		clusterRoleRecommenderCheckpointActor = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:vpa:target:checkpoint-actor",
				Labels: map[string]string{
					"gardener.cloud/role": "vpa",
				},
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{"autoscaling.k8s.io"},
					Resources: []string{"verticalpodautoscalercheckpoints"},
					Verbs:     []string{"get", "list", "watch", "create", "patch", "delete"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"namespaces"},
					Verbs:     []string{"get", "list"},
				},
			},
		}
		clusterRoleBindingRecommenderCheckpointActor = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:vpa:target:checkpoint-actor",
				Labels: map[string]string{
					"gardener.cloud/role": "vpa",
				},
				Annotations: map[string]string{
					"resources.gardener.cloud/delete-on-invalid-update": "true",
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "gardener.cloud:vpa:target:checkpoint-actor",
			},
			Subjects: []rbacv1.Subject{{
				Kind:      "ServiceAccount",
				Name:      "vpa-recommender",
				Namespace: namespace,
			}},
		}
		clusterRoleRecommenderStatusActor = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:vpa:target:status-actor",
				Labels: map[string]string{
					"gardener.cloud/role": "vpa",
				},
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{"autoscaling.k8s.io"},
					Resources: []string{"verticalpodautoscalers/status"},
					Verbs:     []string{"get", "patch"},
				},
			},
		}
		clusterRoleBindingRecommenderStatusActor = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:vpa:target:status-actor",
				Labels: map[string]string{
					"gardener.cloud/role": "vpa",
				},
				Annotations: map[string]string{
					"resources.gardener.cloud/delete-on-invalid-update": "true",
				},
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      "vpa-recommender",
					Namespace: namespace,
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "gardener.cloud:vpa:target:status-actor",
			},
		}
		roleLeaderLockingRecommender = &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener.cloud:vpa:target:leader-locking-vpa-recommender",
				Namespace: namespace,
				Labels: map[string]string{
					"gardener.cloud/role": "vpa",
				},
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{"coordination.k8s.io"},
					Resources: []string{"leases"},
					Verbs:     []string{"create"},
				},
				{
					APIGroups:     []string{"coordination.k8s.io"},
					Resources:     []string{"leases"},
					ResourceNames: []string{"vpa-recommender"},
					Verbs:         []string{"get", "watch", "update"},
				},
			},
		}
		roleBindingLeaderLockingRecommender = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener.cloud:vpa:target:leader-locking-vpa-recommender",
				Namespace: namespace,
				Labels: map[string]string{
					"gardener.cloud/role": "vpa",
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Role",
				Name:     "gardener.cloud:vpa:target:leader-locking-vpa-recommender",
			},
			Subjects: []rbacv1.Subject{{
				Kind:      "ServiceAccount",
				Name:      "vpa-recommender",
				Namespace: namespace,
			}},
		}
		serviceRecommenderFor = func(clusterType component.ClusterType) *corev1.Service {
			obj := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vpa-recommender",
					Namespace: namespace,
					Labels:    map[string]string{"app": "vpa-recommender"},
				},
				Spec: corev1.ServiceSpec{
					Selector: map[string]string{"app": "vpa-recommender"},
					Ports: []corev1.ServicePort{{
						Port:       8942,
						TargetPort: intstr.FromInt32(8942),
						Name:       "metrics",
					}},
				},
			}

			if clusterType == "seed" {
				obj.Annotations = map[string]string{
					"networking.resources.gardener.cloud/from-all-seed-scrape-targets-allowed-ports": `[{"protocol":"TCP","port":8942}]`,
				}
			} else if clusterType == "shoot" {
				obj.Annotations = map[string]string{
					"networking.resources.gardener.cloud/from-all-scrape-targets-allowed-ports": `[{"protocol":"TCP","port":8942}]`,
				}
			}

			return obj
		}
		shootAccessSecretRecommender = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shoot-access-vpa-recommender",
				Namespace: namespace,
				Labels: map[string]string{
					"resources.gardener.cloud/purpose": "token-requestor",
					"resources.gardener.cloud/class":   "shoot",
				},
				Annotations: map[string]string{
					"serviceaccount.resources.gardener.cloud/name":      "vpa-recommender",
					"serviceaccount.resources.gardener.cloud/namespace": "kube-system",
				},
			},
			Type: corev1.SecretTypeOpaque,
		}
		deploymentRecommenderFor = func(
			withServiceAccount bool,
			interval *metav1.Duration,
			recommendationMarginFraction *float64,
			clusterType component.ClusterType,
			targetCPUPercentile *float64,
			recommendationLowerBoundCPUPercentile *float64,
			recommendationUpperBoundCPUPercentile *float64,
			cpuHistogramDecayHalfLife *metav1.Duration,
			targetMemoryPercentile *float64,
			recommendationLowerBoundMemoryPercentile *float64,
			recommendationUpperBoundMemoryPercentile *float64,
			memoryHistogramDecayHalfLife *metav1.Duration,
			memoryAggregationInterval *metav1.Duration,
			memoryAggregationIntervalCount *int64,
			leaderElectionNamespace string,
		) *appsv1.Deployment {
			var cpuRequest string
			var memoryRequest string
			if clusterType == component.ClusterTypeShoot {
				cpuRequest = "10m"
				memoryRequest = "15Mi"
			} else {
				cpuRequest = "200m"
				memoryRequest = "800M"
			}

			obj := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vpa-recommender",
					Namespace: namespace,
					Labels: map[string]string{
						"app":                 "vpa-recommender",
						"gardener.cloud/role": "vpa",
						"high-availability-config.resources.gardener.cloud/type": "controller",
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas:             ptr.To[int32](1),
					RevisionHistoryLimit: ptr.To[int32](2),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "vpa-recommender",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app":                              "vpa-recommender",
								"gardener.cloud/role":              "vpa",
								"networking.gardener.cloud/to-dns": "allowed",
								"networking.resources.gardener.cloud/to-kube-apiserver-tcp-443": "allowed",
							},
						},
						Spec: corev1.PodSpec{
							PriorityClassName: valuesRecommender.PriorityClassName,
							Containers: []corev1.Container{{
								Name:            "recommender",
								Image:           imageRecommender,
								ImagePullPolicy: corev1.PullIfNotPresent,
								Args: []string{
									"--v=3",
									"--stderrthreshold=info",
									"--pod-recommendation-min-cpu-millicores=5",
									"--pod-recommendation-min-memory-mb=10",
									fmt.Sprintf("--recommendation-margin-fraction=%f", ptr.Deref(recommendationMarginFraction, 0.15)),
									fmt.Sprintf("--recommender-interval=%s", ptr.Deref(interval, metav1.Duration{Duration: time.Minute}).Duration),
									"--kube-api-qps=100",
									"--kube-api-burst=120",
									"--memory-saver=true",
									fmt.Sprintf("--target-cpu-percentile=%f", ptr.Deref(targetCPUPercentile, 0.9)),
									fmt.Sprintf("--recommendation-lower-bound-cpu-percentile=%f", ptr.Deref(recommendationLowerBoundCPUPercentile, 0.5)),
									fmt.Sprintf("--recommendation-upper-bound-cpu-percentile=%f", ptr.Deref(recommendationUpperBoundCPUPercentile, 0.95)),
									fmt.Sprintf("--cpu-histogram-decay-half-life=%s", ptr.Deref(cpuHistogramDecayHalfLife, metav1.Duration{Duration: 24 * time.Hour}).Duration),
									fmt.Sprintf("--target-memory-percentile=%f", ptr.Deref(targetMemoryPercentile, 0.9)),
									fmt.Sprintf("--recommendation-lower-bound-memory-percentile=%f", ptr.Deref(recommendationLowerBoundMemoryPercentile, 0.5)),
									fmt.Sprintf("--recommendation-upper-bound-memory-percentile=%f", ptr.Deref(recommendationUpperBoundMemoryPercentile, 0.95)),
									fmt.Sprintf("--memory-histogram-decay-half-life=%s", ptr.Deref(memoryHistogramDecayHalfLife, metav1.Duration{Duration: 24 * time.Hour}).Duration),
									fmt.Sprintf("--memory-aggregation-interval=%s", ptr.Deref(memoryAggregationInterval, metav1.Duration{Duration: 24 * time.Hour}).Duration),
									fmt.Sprintf("--memory-aggregation-interval-count=%d", ptr.Deref(memoryAggregationIntervalCount, 8)),
									"--leader-elect=true",
									fmt.Sprintf("--leader-elect-resource-namespace=%s", leaderElectionNamespace),
								},
								LivenessProbe: livenessProbeVpa,
								Ports: []corev1.ContainerPort{
									{
										Name:          "server",
										ContainerPort: 8080,
									},
									{
										Name:          "metrics",
										ContainerPort: 8942,
									},
								},
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse(cpuRequest),
										corev1.ResourceMemory: resource.MustParse(memoryRequest),
									},
								},
								SecurityContext: &corev1.SecurityContext{
									AllowPrivilegeEscalation: ptr.To(false),
								},
							}},
						},
					},
				},
			}

			if withServiceAccount {
				obj.Spec.Template.Spec.ServiceAccountName = serviceAccountRecommender.Name
			} else {
				obj.Labels["gardener.cloud/role"] = "controlplane"
				obj.Spec.Template.Spec.AutomountServiceAccountToken = ptr.To(false)
				obj.Spec.Template.Spec.Containers[0].Args = append(obj.Spec.Template.Spec.Containers[0].Args, "--kubeconfig="+pathGenericKubeconfig)

				Expect(gardenerutils.InjectGenericKubeconfig(obj, genericTokenKubeconfigSecretName, shootAccessSecretRecommender.Name)).To(Succeed())
			}

			return obj
		}
		podDisruptionBudgetRecommender = &policyv1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vpa-recommender",
				Namespace: namespace,
				Labels:    map[string]string{"gardener.cloud/role": "vpa"},
			},
			Spec: policyv1.PodDisruptionBudgetSpec{
				MaxUnavailable: &maxUnavailable,
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "vpa-recommender",
					},
				},
			},
		}
		vpaRecommender = &vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vpa-recommender",
				Namespace: namespace,
			},
			Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
				TargetRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       "vpa-recommender",
				},
				UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{UpdateMode: &vpaUpdateModeAuto},
				ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
					ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
						{
							ContainerName:    "*",
							ControlledValues: &vpaControlledValues,
						},
					},
				},
			},
		}
		serviceMonitorRecommenderFor = func(clusterType component.ClusterType) *monitoringv1.ServiceMonitor {
			obj := &monitoringv1.ServiceMonitor{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
				},
				Spec: monitoringv1.ServiceMonitorSpec{
					Selector: metav1.LabelSelector{MatchLabels: map[string]string{"app": "vpa-recommender"}},
					Endpoints: []monitoringv1.Endpoint{{
						Port: "metrics",
						RelabelConfigs: []monitoringv1.RelabelConfig{
							{
								Action:      "replace",
								Replacement: ptr.To("vpa-recommender"),
								TargetLabel: "job",
							},
							{
								SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_pod_container_port_name"},
								Regex:        "metrics",
								Action:       "keep",
							},
							{
								Action: "labelmap",
								Regex:  `__meta_kubernetes_pod_label_(.+)`,
							},
						},
					}},
				},
			}

			if clusterType == component.ClusterTypeSeed {
				obj.Labels = map[string]string{"prometheus": "seed"}
				obj.Name = "seed-vpa-recommender"
			}

			if clusterType == component.ClusterTypeShoot {
				obj.Labels = map[string]string{"prometheus": "shoot"}
				obj.Name = "shoot-vpa-recommender"
			}

			return obj
		}

		serviceAccountAdmissionController = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vpa-admission-controller",
				Namespace: namespace,
				Labels: map[string]string{
					"gardener.cloud/role": "vpa",
				},
			},
			AutomountServiceAccountToken: ptr.To(false),
		}
		clusterRoleAdmissionController = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:vpa:target:admission-controller",
				Labels: map[string]string{
					"gardener.cloud/role": "vpa",
				},
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"pods", "configmaps", "nodes", "limitranges"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{"admissionregistration.k8s.io"},
					Resources: []string{"mutatingwebhookconfigurations"},
					Verbs:     []string{"create", "delete", "get", "list"},
				},
				{
					APIGroups: []string{"autoscaling.k8s.io"},
					Resources: []string{"verticalpodautoscalers"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{"coordination.k8s.io"},
					Resources: []string{"leases"},
					Verbs:     []string{"create", "update", "get", "list", "watch"},
				},
			},
		}
		clusterRoleBindingAdmissionController = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:vpa:target:admission-controller",
				Labels: map[string]string{
					"gardener.cloud/role": "vpa",
				},
				Annotations: map[string]string{
					"resources.gardener.cloud/delete-on-invalid-update": "true",
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "gardener.cloud:vpa:target:admission-controller",
			},
			Subjects: []rbacv1.Subject{{
				Kind:      "ServiceAccount",
				Name:      "vpa-admission-controller",
				Namespace: namespace,
			}},
		}
		shootAccessSecretAdmissionController = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shoot-access-vpa-admission-controller",
				Namespace: namespace,
				Labels: map[string]string{
					"resources.gardener.cloud/purpose": "token-requestor",
					"resources.gardener.cloud/class":   "shoot",
				},
				Annotations: map[string]string{
					"serviceaccount.resources.gardener.cloud/name":      "vpa-admission-controller",
					"serviceaccount.resources.gardener.cloud/namespace": "kube-system",
				},
			},
			Type: corev1.SecretTypeOpaque,
		}
		serviceAdmissionControllerFor = func(clusterType component.ClusterType, topologyAwareRoutingEnabled bool) *corev1.Service {
			obj := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vpa-webhook",
					Namespace: namespace,
					Labels: map[string]string{
						"app":                 "vpa-admission-controller",
						"gardener.cloud/role": "vpa",
					},
				},
				Spec: corev1.ServiceSpec{
					Selector: map[string]string{"app": "vpa-admission-controller"},
					Ports: []corev1.ServicePort{
						{
							Name:        "metrics",
							Protocol:    "TCP",
							AppProtocol: nil,
							Port:        8944,
							TargetPort:  intstr.FromInt32(8944),
							NodePort:    0,
						},
						{
							Name:        "server",
							Protocol:    "",
							AppProtocol: nil,
							Port:        443,
							TargetPort:  intstr.FromInt32(10250),
							NodePort:    0,
						},
					},
				},
			}

			if clusterType == "seed" {
				metav1.SetMetaDataAnnotation(&obj.ObjectMeta, "networking.resources.gardener.cloud/from-world-to-ports", `[{"protocol":"TCP","port":10250}]`)
				metav1.SetMetaDataAnnotation(&obj.ObjectMeta, "networking.resources.gardener.cloud/from-all-seed-scrape-targets-allowed-ports", `[{"protocol":"TCP","port":8944}]`)
			}
			if clusterType == "shoot" {
				metav1.SetMetaDataAnnotation(&obj.ObjectMeta, "networking.resources.gardener.cloud/from-all-webhook-targets-allowed-ports", `[{"protocol":"TCP","port":10250}]`)
				metav1.SetMetaDataAnnotation(&obj.ObjectMeta, "networking.resources.gardener.cloud/from-all-scrape-targets-allowed-ports", `[{"protocol":"TCP","port":8944}]`)
			}

			if topologyAwareRoutingEnabled {
				metav1.SetMetaDataAnnotation(&obj.ObjectMeta, "service.kubernetes.io/topology-aware-hints", "auto")
				metav1.SetMetaDataLabel(&obj.ObjectMeta, "endpoint-slice-hints.resources.gardener.cloud/consider", "true")
			}

			return obj
		}
		deploymentAdmissionControllerFor = func(withServiceAccount bool) *appsv1.Deployment {
			obj := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vpa-admission-controller",
					Namespace: namespace,
					Labels: map[string]string{
						"app":                 "vpa-admission-controller",
						"gardener.cloud/role": "vpa",
						"high-availability-config.resources.gardener.cloud/type": "server",
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas:             ptr.To[int32](1),
					RevisionHistoryLimit: ptr.To[int32](2),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "vpa-admission-controller",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app":                              "vpa-admission-controller",
								"gardener.cloud/role":              "vpa",
								"networking.gardener.cloud/to-dns": "allowed",
								"networking.resources.gardener.cloud/to-kube-apiserver-tcp-443": "allowed",
							},
						},
						Spec: corev1.PodSpec{
							PriorityClassName: valuesAdmissionController.PriorityClassName,
							Containers: []corev1.Container{{
								Name:            "admission-controller",
								Image:           imageAdmissionController,
								ImagePullPolicy: corev1.PullIfNotPresent,
								Args: []string{
									"--v=2",
									"--stderrthreshold=info",
									"--kube-api-qps=100",
									"--kube-api-burst=120",
									"--client-ca-file=/etc/tls-certs/bundle.crt",
									"--tls-cert-file=/etc/tls-certs/tls.crt",
									"--tls-private-key=/etc/tls-certs/tls.key",
									"--address=:8944",
									"--port=10250",
									"--register-webhook=false",
								},
								LivenessProbe: livenessProbeVpa,
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("10m"),
										corev1.ResourceMemory: resource.MustParse("30Mi"),
									},
								},
								SecurityContext: &corev1.SecurityContext{
									AllowPrivilegeEscalation: ptr.To(false),
								},
								Ports: []corev1.ContainerPort{
									{
										Name:          "metrics",
										ContainerPort: 8944,
									},
									{
										Name:          "server",
										ContainerPort: 10250,
									},
								},
								VolumeMounts: []corev1.VolumeMount{{
									Name:      "vpa-tls-certs",
									MountPath: "/etc/tls-certs",
									ReadOnly:  true,
								}},
							}},
							Volumes: []corev1.Volume{{
								Name: "vpa-tls-certs",
								VolumeSource: corev1.VolumeSource{
									Projected: &corev1.ProjectedVolumeSource{
										DefaultMode: ptr.To[int32](420),
										Sources: []corev1.VolumeProjection{
											{
												Secret: &corev1.SecretProjection{
													LocalObjectReference: corev1.LocalObjectReference{
														Name: "ca",
													},
													Items: []corev1.KeyToPath{{
														Key:  "bundle.crt",
														Path: "bundle.crt",
													}},
												},
											},
											{
												Secret: &corev1.SecretProjection{
													LocalObjectReference: corev1.LocalObjectReference{
														Name: "vpa-admission-controller-server",
													},
													Items: []corev1.KeyToPath{
														{
															Key:  "tls.crt",
															Path: "tls.crt",
														},
														{
															Key:  "tls.key",
															Path: "tls.key",
														},
													},
												},
											},
										},
									},
								},
							}},
						},
					},
				},
			}

			if withServiceAccount {
				obj.Spec.Template.Spec.ServiceAccountName = serviceAccountAdmissionController.Name
				obj.Spec.Template.Spec.Containers[0].Env = append(obj.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{
					Name: "NAMESPACE",
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: "metadata.namespace",
						},
					},
				})
			} else {
				obj.Labels["gardener.cloud/role"] = "controlplane"
				obj.Spec.Template.Spec.AutomountServiceAccountToken = ptr.To(false)
				obj.Spec.Template.Spec.Containers[0].Args = append(obj.Spec.Template.Spec.Containers[0].Args, "--kubeconfig="+pathGenericKubeconfig)

				Expect(gardenerutils.InjectGenericKubeconfig(obj, genericTokenKubeconfigSecretName, shootAccessSecretAdmissionController.Name)).To(Succeed())
			}

			return obj
		}
		podDisruptionBudgetAdmissionController = &policyv1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vpa-admission-controller",
				Namespace: namespace,
				Labels:    map[string]string{"gardener.cloud/role": "vpa"},
			},
			Spec: policyv1.PodDisruptionBudgetSpec{
				MaxUnavailable: &maxUnavailable,
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "vpa-admission-controller",
					},
				},
			},
		}
		vpaAdmissionController = &vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vpa-admission-controller",
				Namespace: namespace,
			},
			Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
				TargetRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       "vpa-admission-controller",
				},
				UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{UpdateMode: &vpaUpdateModeAuto},
				ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
					ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
						{
							ContainerName:    "*",
							ControlledValues: &vpaControlledValues,
						},
					},
				},
			},
		}
		serviceMonitorAdmissionController = &monitoringv1.ServiceMonitor{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "monitoring.coreos.com/v1",
				Kind:       "ServiceMonitor",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "seed-vpa-admission-controller",
				Namespace: namespace,
				Labels:    map[string]string{"prometheus": "seed"},
			},
			Spec: monitoringv1.ServiceMonitorSpec{
				Selector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app":                 "vpa-admission-controller",
						"gardener.cloud/role": "vpa",
					},
				},
				NamespaceSelector: monitoringv1.NamespaceSelector{Any: false},
				Endpoints: []monitoringv1.Endpoint{{
					Port: "metrics",
					RelabelConfigs: []monitoringv1.RelabelConfig{
						{
							Action:      "replace",
							Replacement: ptr.To("vpa-admission-controller"),
							TargetLabel: "job",
						},
						{
							SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_pod_container_port_name"},
							Regex:        "metrics",
							Action:       "keep",
						},
						{
							Action: "labelmap",
							Regex:  `__meta_kubernetes_pod_label_(.+)`,
						},
					},
				}},
			},
		}

		clusterRoleGeneralActor = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:vpa:target:actor",
				Labels: map[string]string{
					"gardener.cloud/role": "vpa",
				},
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"pods", "nodes", "limitranges"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"events"},
					Verbs:     []string{"get", "list", "watch", "create"},
				},
				{
					APIGroups: []string{"autoscaling.k8s.io"},
					Resources: []string{"verticalpodautoscalers"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{"coordination.k8s.io"},
					Resources: []string{"leases"},
					Verbs:     []string{"get", "list", "watch"},
				},
			},
		}
		clusterRoleBindingGeneralActor = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:vpa:target:actor",
				Labels: map[string]string{
					"gardener.cloud/role": "vpa",
				},
				Annotations: map[string]string{
					"resources.gardener.cloud/delete-on-invalid-update": "true",
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "gardener.cloud:vpa:target:actor",
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      "vpa-recommender",
					Namespace: namespace,
				},
				{
					Kind:      "ServiceAccount",
					Name:      "vpa-updater",
					Namespace: namespace,
				},
			},
		}
		clusterRoleGeneralTargetReader = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:vpa:target:target-reader",
				Labels: map[string]string{
					"gardener.cloud/role": "vpa",
				},
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{"*"},
					Resources: []string{"*/scale"},
					Verbs:     []string{"get", "watch"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"replicationcontrollers"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{"apps"},
					Resources: []string{"daemonsets", "deployments", "replicasets", "statefulsets"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{"batch"},
					Resources: []string{"jobs", "cronjobs"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{"druid.gardener.cloud"},
					Resources: []string{"etcds", "etcds/scale"},
					Verbs:     []string{"get", "list", "watch"},
				},
			},
		}
		clusterRoleBindingGeneralTargetReader = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:vpa:target:target-reader",
				Labels: map[string]string{
					"gardener.cloud/role": "vpa",
				},
				Annotations: map[string]string{
					"resources.gardener.cloud/delete-on-invalid-update": "true",
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "gardener.cloud:vpa:target:target-reader",
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      "vpa-admission-controller",
					Namespace: namespace,
				},
				{
					Kind:      "ServiceAccount",
					Name:      "vpa-recommender",
					Namespace: namespace,
				},
				{
					Kind:      "ServiceAccount",
					Name:      "vpa-updater",
					Namespace: namespace,
				},
			},
		}
		mutatingWebhookConfiguration = &admissionregistrationv1.MutatingWebhookConfiguration{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "zzz-vpa-webhook-config-target",
				Labels: map[string]string{"remediation.webhook.shoot.gardener.cloud/exclude": "true"},
				Annotations: map[string]string{constants.GardenerDescription: "The order in which MutatingWebhooks are " +
					"called is determined alphabetically. This webhook's name intentionally starts with 'zzz', such " +
					"that it is called after all other webhooks which inject containers. All containers injected by " +
					"webhooks that are called _after_ the vpa webhook will not be under control of vpa."},
			},
			Webhooks: []admissionregistrationv1.MutatingWebhook{{
				Name:                    "vpa.k8s.io",
				AdmissionReviewVersions: []string{"v1"},
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					URL: ptr.To(fmt.Sprintf("https://vpa-webhook.%s:443", namespace)),
				},
				FailurePolicy:      &webhookFailurePolicy,
				MatchPolicy:        &webhookMatchPolicy,
				ReinvocationPolicy: &webhookReinvocationPolicy,
				SideEffects:        &webhookSideEffects,
				TimeoutSeconds:     ptr.To[int32](10),
				Rules: []admissionregistrationv1.RuleWithOperations{
					{
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{""},
							APIVersions: []string{"v1"},
							Resources:   []string{"pods"},
							Scope:       &webhookScope,
						},
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
					},
					{
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{"autoscaling.k8s.io"},
							APIVersions: []string{"*"},
							Resources:   []string{"verticalpodautoscalers"},
							Scope:       &webhookScope,
						},
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
					},
				},
			}},
		}
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
	})

	Describe("#Deploy", func() {
		Context("cluster type seed", func() {
			BeforeEach(func() {
				vpa = New(c, namespace, sm, Values{
					ClusterType:              component.ClusterTypeSeed,
					Enabled:                  true,
					SecretNameServerCA:       secretNameCA,
					RuntimeKubernetesVersion: runtimeKubernetesVersion,
					AdmissionController:      valuesAdmissionController,
					Recommender:              valuesRecommender,
					Updater:                  valuesUpdater,
				})
				managedResourceName = "vpa"
			})

			Context("Different ServiceMonitor labels", func() {
				Context("when used with Garden cluster", func() {
					BeforeEach(func() {
						isGardenCluster := true
						vpa = vpaFor(component.ClusterTypeSeed, isGardenCluster)
						Expect(vpa.Deploy(ctx)).To(Succeed())
					})

					It("should label `admission-controller` with `prometheus=garden`", func() {
						serviceMonitorAdmissionController.ObjectMeta.Name = "garden-vpa-admission-controller"
						serviceMonitorAdmissionController.ObjectMeta.Labels = map[string]string{
							"prometheus": "garden",
						}
						Expect(managedResource).To(contain(serviceMonitorAdmissionController))
					})

					It("should label `recommender` with `prometheus=garden`", func() {
						serviceMonitorRecommender := serviceMonitorRecommenderFor(component.ClusterTypeSeed)
						serviceMonitorRecommender.ObjectMeta.Name = "garden-vpa-recommender"
						serviceMonitorRecommender.ObjectMeta.Labels = map[string]string{
							"prometheus": "garden",
						}
						Expect(managedResource).To(contain(serviceMonitorRecommender))
					})

				})

				Context("when used with non-Garden cluster", func() {
					BeforeEach(func() {
						isGardenCluster := false
						vpa = vpaFor(component.ClusterTypeSeed, isGardenCluster)
						Expect(vpa.Deploy(ctx)).To(Succeed())
					})

					It("should label `admission-controller` with `prometheus=seed`", func() {
						Expect(managedResource).To(contain(serviceMonitorAdmissionController))
					})

					It("should label `recommender` with `prometheus=seed`", func() {
						serviceMonitorRecommender := serviceMonitorRecommenderFor(component.ClusterTypeSeed)
						Expect(managedResource).To(contain(serviceMonitorRecommender))
					})

				})

			})

			Context("Different Kubernetes versions", func() {
				var expectedObjects []client.Object

				JustBeforeEach(func() {
					Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
					Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(BeNotFoundError())

					Expect(vpa.Deploy(ctx)).To(Succeed())

					Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					expectedMr := &resourcesv1alpha1.ManagedResource{
						ObjectMeta: metav1.ObjectMeta{
							Name:            managedResourceName,
							Namespace:       namespace,
							Labels:          map[string]string{"gardener.cloud/role": "seed-system-component"},
							ResourceVersion: "1",
						},
						Spec: resourcesv1alpha1.ManagedResourceSpec{
							Class: ptr.To("seed"),
							SecretRefs: []corev1.LocalObjectReference{{
								Name: managedResource.Spec.SecretRefs[0].Name,
							}},
							KeepObjects: ptr.To(false),
						},
					}
					utilruntime.Must(references.InjectAnnotations(expectedMr))
					Expect(managedResource).To(DeepEqual(expectedMr))

					managedResourceSecret.Name = managedResource.Spec.SecretRefs[0].Name
					Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
					Expect(managedResourceSecret.Immutable).To(Equal(ptr.To(true)))
					Expect(managedResourceSecret.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))
					Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))

					By("Verify vpa-updater resources")
					clusterRoleUpdater.Name = replaceTargetSubstrings(clusterRoleUpdater.Name)
					clusterRoleBindingUpdater.Name = replaceTargetSubstrings(clusterRoleBindingUpdater.Name)
					clusterRoleBindingUpdater.RoleRef.Name = replaceTargetSubstrings(clusterRoleBindingUpdater.RoleRef.Name)
					roleLeaderLockingUpdater.Name = replaceTargetSubstrings(roleLeaderLockingUpdater.Name)
					roleBindingLeaderLockingUpdater.Name = replaceTargetSubstrings(roleBindingLeaderLockingUpdater.Name)
					roleBindingLeaderLockingUpdater.RoleRef.Name = replaceTargetSubstrings(roleBindingLeaderLockingUpdater.RoleRef.Name)

					deploymentUpdater := deploymentUpdaterFor(true, nil, nil, nil, nil, nil, namespace)
					adaptNetworkPolicyLabelsForClusterTypeSeed(deploymentUpdater.Spec.Template.Labels)

					expectedObjects = []client.Object{
						serviceAccountUpdater,
						clusterRoleUpdater,
						clusterRoleBindingUpdater,
						roleLeaderLockingUpdater,
						roleBindingLeaderLockingUpdater,
						deploymentUpdater,
						vpaUpdater,
					}

					By("Verify vpa-recommender resources")
					clusterRoleRecommenderMetricsReader.Name = replaceTargetSubstrings(clusterRoleRecommenderMetricsReader.Name)
					clusterRoleBindingRecommenderMetricsReader.Name = replaceTargetSubstrings(clusterRoleBindingRecommenderMetricsReader.Name)
					clusterRoleBindingRecommenderMetricsReader.RoleRef.Name = replaceTargetSubstrings(clusterRoleBindingRecommenderMetricsReader.RoleRef.Name)
					clusterRoleRecommenderCheckpointActor.Name = replaceTargetSubstrings(clusterRoleRecommenderCheckpointActor.Name)
					clusterRoleBindingRecommenderCheckpointActor.Name = replaceTargetSubstrings(clusterRoleBindingRecommenderCheckpointActor.Name)
					clusterRoleBindingRecommenderCheckpointActor.RoleRef.Name = replaceTargetSubstrings(clusterRoleBindingRecommenderCheckpointActor.RoleRef.Name)
					clusterRoleRecommenderStatusActor.Name = replaceTargetSubstrings(clusterRoleRecommenderStatusActor.Name)
					clusterRoleBindingRecommenderStatusActor.Name = replaceTargetSubstrings(clusterRoleBindingRecommenderStatusActor.Name)
					clusterRoleBindingRecommenderStatusActor.RoleRef.Name = replaceTargetSubstrings(clusterRoleBindingRecommenderStatusActor.RoleRef.Name)
					roleLeaderLockingRecommender.Name = replaceTargetSubstrings(roleLeaderLockingRecommender.Name)
					roleBindingLeaderLockingRecommender.Name = replaceTargetSubstrings(roleBindingLeaderLockingRecommender.Name)
					roleBindingLeaderLockingRecommender.RoleRef.Name = replaceTargetSubstrings(roleBindingLeaderLockingRecommender.RoleRef.Name)

					deploymentRecommender := deploymentRecommenderFor(true, nil, nil, component.ClusterTypeSeed, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, namespace)
					adaptNetworkPolicyLabelsForClusterTypeSeed(deploymentRecommender.Spec.Template.Labels)

					expectedObjects = append(expectedObjects,
						serviceAccountRecommender,
						clusterRoleRecommenderMetricsReader,
						clusterRoleBindingRecommenderMetricsReader,
						clusterRoleRecommenderCheckpointActor,
						clusterRoleBindingRecommenderCheckpointActor,
						clusterRoleRecommenderStatusActor,
						clusterRoleBindingRecommenderStatusActor,
						roleLeaderLockingRecommender,
						roleBindingLeaderLockingRecommender,
						deploymentRecommender,
						serviceRecommenderFor(component.ClusterTypeSeed),
						serviceMonitorRecommenderFor(component.ClusterTypeSeed),
					)

					Expect(managedResourceSecret.Data).NotTo(HaveKey("verticalpodautoscaler__" + namespace + "__vpa-recommender.yaml"))

					By("Verify vpa-admission-controller resources")
					clusterRoleAdmissionController.Name = replaceTargetSubstrings(clusterRoleAdmissionController.Name)
					clusterRoleBindingAdmissionController.Name = replaceTargetSubstrings(clusterRoleBindingAdmissionController.Name)
					clusterRoleBindingAdmissionController.RoleRef.Name = replaceTargetSubstrings(clusterRoleBindingAdmissionController.RoleRef.Name)

					deploymentAdmissionController := deploymentAdmissionControllerFor(true)
					adaptNetworkPolicyLabelsForClusterTypeSeed(deploymentAdmissionController.Spec.Template.Labels)

					expectedObjects = append(expectedObjects,
						serviceAccountAdmissionController,
						clusterRoleAdmissionController,
						clusterRoleBindingAdmissionController,
						serviceAdmissionControllerFor(component.ClusterTypeSeed, false),
						deploymentAdmissionController,
						vpaAdmissionController,
						serviceMonitorAdmissionController,
					)

					By("Verify general resources")
					clusterRoleGeneralActor.Name = replaceTargetSubstrings(clusterRoleGeneralActor.Name)
					clusterRoleGeneralTargetReader.Name = replaceTargetSubstrings(clusterRoleGeneralTargetReader.Name)
					clusterRoleBindingGeneralActor.Name = replaceTargetSubstrings(clusterRoleBindingGeneralActor.Name)
					clusterRoleBindingGeneralActor.RoleRef.Name = replaceTargetSubstrings(clusterRoleBindingGeneralActor.RoleRef.Name)
					clusterRoleBindingGeneralTargetReader.Name = replaceTargetSubstrings(clusterRoleBindingGeneralTargetReader.Name)
					clusterRoleBindingGeneralTargetReader.RoleRef.Name = replaceTargetSubstrings(clusterRoleBindingGeneralTargetReader.RoleRef.Name)
					mutatingWebhookConfiguration.Name = strings.Replace(mutatingWebhookConfiguration.Name, "-target", "-source", -1)
					mutatingWebhookConfiguration.Webhooks[0].ClientConfig = admissionregistrationv1.WebhookClientConfig{
						Service: &admissionregistrationv1.ServiceReference{
							Name:      "vpa-webhook",
							Namespace: namespace,
							Port:      ptr.To[int32](443),
						},
					}

					expectedObjects = append(expectedObjects,
						clusterRoleGeneralActor,
						clusterRoleBindingGeneralActor,
						clusterRoleGeneralTargetReader,
						clusterRoleBindingGeneralTargetReader,
						mutatingWebhookConfiguration,
					)
				})

				Context("Kubernetes versions < 1.26", func() {
					It("should successfully deploy all resources", func() {
						expectedObjects = append(expectedObjects,
							podDisruptionBudgetUpdater,
							podDisruptionBudgetRecommender,
							podDisruptionBudgetAdmissionController,
						)
						Expect(managedResource).To(consistOf(expectedObjects...))
					})
				})

				Context("Kubernetes versions >= 1.26", func() {
					BeforeEach(func() {
						vpa = New(c, namespace, sm, Values{
							ClusterType:              component.ClusterTypeSeed,
							Enabled:                  true,
							SecretNameServerCA:       secretNameCA,
							RuntimeKubernetesVersion: semver.MustParse("1.26.0"),
							AdmissionController:      valuesAdmissionController,
							Recommender:              valuesRecommender,
							Updater:                  valuesUpdater,
						})
					})

					It("should successfully deploy all resources", func() {
						unhealthyPodEvictionPolicyAlwaysAllow := policyv1.AlwaysAllow
						podDisruptionBudgetUpdater.Spec.UnhealthyPodEvictionPolicy = &unhealthyPodEvictionPolicyAlwaysAllow
						podDisruptionBudgetRecommender.Spec.UnhealthyPodEvictionPolicy = &unhealthyPodEvictionPolicyAlwaysAllow
						podDisruptionBudgetAdmissionController.Spec.UnhealthyPodEvictionPolicy = &unhealthyPodEvictionPolicyAlwaysAllow
						expectedObjects = append(expectedObjects,
							podDisruptionBudgetUpdater,
							podDisruptionBudgetRecommender,
							podDisruptionBudgetAdmissionController,
						)
						Expect(managedResource).To(consistOf(expectedObjects...))
					})
				})
			})

			It("should successfully deploy with special configuration", func() {
				valuesRecommender.Interval = &metav1.Duration{Duration: 3 * time.Hour}
				valuesRecommender.RecommendationMarginFraction = ptr.To(float64(8.91))
				valuesRecommender.TargetCPUPercentile = ptr.To(float64(0.333))
				valuesRecommender.RecommendationLowerBoundCPUPercentile = ptr.To(float64(0.303))
				valuesRecommender.RecommendationUpperBoundCPUPercentile = ptr.To(float64(0.393))
				valuesRecommender.CPUHistogramDecayHalfLife = &metav1.Duration{Duration: 42 * time.Minute}
				valuesRecommender.TargetMemoryPercentile = ptr.To(float64(0.444))
				valuesRecommender.RecommendationLowerBoundCPUPercentile = ptr.To(float64(0.404))
				valuesRecommender.RecommendationUpperBoundCPUPercentile = ptr.To(float64(0.494))
				valuesRecommender.MemoryHistogramDecayHalfLife = &metav1.Duration{Duration: 1337 * time.Minute}
				valuesRecommender.MemoryAggregationInterval = &metav1.Duration{Duration: 42 * time.Minute}
				valuesRecommender.MemoryAggregationIntervalCount = ptr.To[int64](99)

				valuesUpdater.Interval = &metav1.Duration{Duration: 4 * time.Hour}
				valuesUpdater.EvictAfterOOMThreshold = &metav1.Duration{Duration: 5 * time.Hour}
				valuesUpdater.EvictionRateBurst = ptr.To[int32](1)
				valuesUpdater.EvictionRateLimit = ptr.To(float64(2.34))
				valuesUpdater.EvictionTolerance = ptr.To(float64(5.67))

				vpa = New(c, namespace, sm, Values{
					ClusterType:              component.ClusterTypeSeed,
					Enabled:                  true,
					SecretNameServerCA:       secretNameCA,
					RuntimeKubernetesVersion: runtimeKubernetesVersion,
					AdmissionController:      valuesAdmissionController,
					Recommender:              valuesRecommender,
					Updater:                  valuesUpdater,
				})

				Expect(vpa.Deploy(ctx)).To(Succeed())

				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				expectedMr := &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:            managedResourceName,
						Namespace:       namespace,
						Labels:          map[string]string{"gardener.cloud/role": "seed-system-component"},
						ResourceVersion: "1",
					},
					Spec: resourcesv1alpha1.ManagedResourceSpec{
						Class: ptr.To("seed"),
						SecretRefs: []corev1.LocalObjectReference{{
							Name: managedResource.Spec.SecretRefs[0].Name,
						}},
						KeepObjects: ptr.To(false),
					},
				}
				utilruntime.Must(references.InjectAnnotations(expectedMr))
				Expect(managedResource).To(DeepEqual(expectedMr))

				managedResourceSecret.Name = managedResource.Spec.SecretRefs[0].Name
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())

				deploymentUpdater := deploymentUpdaterFor(
					true,
					valuesUpdater.Interval,
					valuesUpdater.EvictAfterOOMThreshold,
					valuesUpdater.EvictionRateBurst,
					valuesUpdater.EvictionRateLimit,
					valuesUpdater.EvictionTolerance,
					namespace,
				)
				adaptNetworkPolicyLabelsForClusterTypeSeed(deploymentUpdater.Spec.Template.Labels)

				deploymentRecommender := deploymentRecommenderFor(
					true,
					valuesRecommender.Interval,
					valuesRecommender.RecommendationMarginFraction,
					component.ClusterTypeSeed,
					valuesRecommender.TargetCPUPercentile,
					valuesRecommender.RecommendationLowerBoundCPUPercentile,
					valuesRecommender.RecommendationUpperBoundCPUPercentile,
					valuesRecommender.CPUHistogramDecayHalfLife,
					valuesRecommender.TargetMemoryPercentile,
					valuesRecommender.RecommendationLowerBoundMemoryPercentile,
					valuesRecommender.RecommendationUpperBoundMemoryPercentile,
					valuesRecommender.MemoryHistogramDecayHalfLife,
					valuesRecommender.MemoryAggregationInterval,
					valuesRecommender.MemoryAggregationIntervalCount,
					namespace,
				)
				adaptNetworkPolicyLabelsForClusterTypeSeed(deploymentRecommender.Spec.Template.Labels)

				Expect(managedResource).To(contain(deploymentUpdater, deploymentRecommender))
				Expect(managedResourceSecret.Immutable).To(Equal(ptr.To(true)))
				Expect(managedResourceSecret.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))
			})
		})

		Context("cluster type shoot", func() {
			BeforeEach(func() {
				vpa = New(c, namespace, sm, Values{
					ClusterType:              component.ClusterTypeShoot,
					Enabled:                  true,
					SecretNameServerCA:       secretNameCA,
					RuntimeKubernetesVersion: runtimeKubernetesVersion,
					AdmissionController:      valuesAdmissionController,
					Recommender:              valuesRecommender,
					Updater:                  valuesUpdater,
				})
				managedResourceName = "shoot-core-vpa"
			})

			Context("Different ServiceMonitor labels", func() {

				Context("should always attach `prometheus=shoot` to `admission-controller`", func() {

					BeforeEach(func() {
						serviceMonitorAdmissionController.TypeMeta = metav1.TypeMeta{}
						serviceMonitorAdmissionController.ResourceVersion = "1"
						serviceMonitorAdmissionController.ObjectMeta.Name = "shoot-vpa-admission-controller"
						serviceMonitorAdmissionController.ObjectMeta.Labels = map[string]string{
							"prometheus": "shoot",
						}
					})

					It("when IsGardenCluster=true", func() {
						isGardenCluster := true
						vpa = vpaFor(component.ClusterTypeShoot, isGardenCluster)
						Expect(vpa.Deploy(ctx)).To(Succeed())

						serviceMonitor := &monitoringv1.ServiceMonitor{}
						Expect(c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "shoot-vpa-admission-controller"}, serviceMonitor)).To(Succeed())

						By("verify Prometheus label")
						Expect(serviceMonitor.Labels).To(Equal(serviceMonitorAdmissionController.ObjectMeta.Labels))

						By("verify object")
						Expect(serviceMonitor).To(Equal(serviceMonitorAdmissionController))
					})

					It("when IsGardenCluster=false", func() {
						isGardenCluster := false
						vpa = vpaFor(component.ClusterTypeShoot, isGardenCluster)
						Expect(vpa.Deploy(ctx)).To(Succeed())

						serviceMonitor := &monitoringv1.ServiceMonitor{}
						Expect(c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "shoot-vpa-admission-controller"}, serviceMonitor)).To(Succeed())

						By("verify Prometheus label")
						Expect(serviceMonitor.Labels).To(Equal(serviceMonitorAdmissionController.ObjectMeta.Labels))

						By("verify object")
						Expect(serviceMonitor).To(Equal(serviceMonitorAdmissionController))
					})
				})

				Context("should always attach `prometheus=shoot` to `recommender`", func() {
					var serviceMonitor = &monitoringv1.ServiceMonitor{}

					BeforeEach(func() {
						serviceMonitor = serviceMonitorRecommenderFor(component.ClusterTypeShoot)
						serviceMonitor.ResourceVersion = "1"

					})
					It("when IsGardenCluster=true", func() {
						isGardenCluster := true
						vpa = vpaFor(component.ClusterTypeShoot, isGardenCluster)
						Expect(vpa.Deploy(ctx)).To(Succeed())

						serviceMonitor := &monitoringv1.ServiceMonitor{}
						Expect(c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "shoot-vpa-recommender"}, serviceMonitor)).To(Succeed())

						By("verify Prometheus label")
						Expect(serviceMonitor.Labels).To(Equal(serviceMonitor.ObjectMeta.Labels))

						By("verify object")
						Expect(serviceMonitor).To(Equal(serviceMonitor))
					})

					It("when IsGardenCluster=false", func() {
						isGardenCluster := false
						vpa = vpaFor(component.ClusterTypeShoot, isGardenCluster)
						Expect(vpa.Deploy(ctx)).To(Succeed())

						serviceMonitor := &monitoringv1.ServiceMonitor{}
						Expect(c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "shoot-vpa-recommender"}, serviceMonitor)).To(Succeed())

						By("verify Prometheus label")
						Expect(serviceMonitor.Labels).To(Equal(serviceMonitor.ObjectMeta.Labels))

						By("verify object")
						Expect(serviceMonitor).To(Equal(serviceMonitor))
					})
				})

			})

			Context("Different Kubernetes versions", func() {
				JustBeforeEach(func() {
					Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())

					Expect(vpa.Deploy(ctx)).To(Succeed())

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

					managedResourceSecret.Name = managedResource.Spec.SecretRefs[0].Name
					Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
					Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))

					By("Verify vpa-updater application resources")
					clusterRoleBindingUpdater.Subjects[0].Namespace = "kube-system"
					roleLeaderLockingUpdater.Namespace = "kube-system"
					roleBindingLeaderLockingUpdater.Namespace = "kube-system"
					roleBindingLeaderLockingUpdater.Subjects[0].Namespace = "kube-system"

					Expect(managedResource).To(contain(
						clusterRoleUpdater,
						clusterRoleBindingUpdater,
						roleLeaderLockingUpdater,
						roleBindingLeaderLockingUpdater,
					))
					Expect(managedResourceSecret.Immutable).To(Equal(ptr.To(true)))
					Expect(managedResourceSecret.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))

					By("Verify vpa-updater runtime resources")
					secret := &corev1.Secret{}
					Expect(c.Get(ctx, client.ObjectKeyFromObject(shootAccessSecretUpdater), secret)).To(Succeed())
					shootAccessSecretUpdater.ResourceVersion = "1"
					Expect(secret).To(Equal(shootAccessSecretUpdater))

					deployment := &appsv1.Deployment{}
					Expect(c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "vpa-updater"}, deployment)).To(Succeed())
					deploymentUpdater := deploymentUpdaterFor(false, nil, nil, nil, nil, nil, "kube-system")
					deploymentUpdater.ResourceVersion = "1"
					Expect(deployment).To(Equal(deploymentUpdater))

					vpa := &vpaautoscalingv1.VerticalPodAutoscaler{}
					Expect(c.Get(ctx, client.ObjectKeyFromObject(vpaUpdater), vpa)).To(Succeed())
					vpaUpdater.ResourceVersion = "1"
					Expect(vpa).To(Equal(vpaUpdater))

					By("Verify vpa-recommender application resources")
					clusterRoleBindingRecommenderMetricsReader.Subjects[0].Namespace = "kube-system"
					clusterRoleBindingRecommenderCheckpointActor.Subjects[0].Namespace = "kube-system"
					clusterRoleBindingRecommenderStatusActor.Subjects[0].Namespace = "kube-system"
					roleLeaderLockingRecommender.Namespace = "kube-system"
					roleBindingLeaderLockingRecommender.Namespace = "kube-system"
					roleBindingLeaderLockingRecommender.Subjects[0].Namespace = "kube-system"

					Expect(managedResource).To(contain(
						clusterRoleRecommenderMetricsReader,
						clusterRoleBindingRecommenderMetricsReader,
						clusterRoleRecommenderCheckpointActor,
						clusterRoleBindingRecommenderCheckpointActor,
						clusterRoleRecommenderStatusActor,
						clusterRoleBindingRecommenderStatusActor,
						roleLeaderLockingRecommender,
						roleBindingLeaderLockingRecommender,
					))

					By("Verify vpa-recommender runtime resources")
					secret = &corev1.Secret{}
					Expect(c.Get(ctx, client.ObjectKeyFromObject(shootAccessSecretRecommender), secret)).To(Succeed())
					shootAccessSecretRecommender.ResourceVersion = "1"
					Expect(secret).To(Equal(shootAccessSecretRecommender))

					deployment = &appsv1.Deployment{}
					Expect(c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "vpa-recommender"}, deployment)).To(Succeed())
					deploymentRecommender := deploymentRecommenderFor(false, nil, nil, component.ClusterTypeShoot, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "kube-system")
					deploymentRecommender.ResourceVersion = "1"
					Expect(deployment).To(Equal(deploymentRecommender))

					service := &corev1.Service{}
					Expect(c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "vpa-recommender"}, service)).To(Succeed())
					serviceRecommender := serviceRecommenderFor(component.ClusterTypeShoot)
					serviceRecommender.ResourceVersion = "1"
					Expect(service).To(Equal(serviceRecommender))

					vpa = &vpaautoscalingv1.VerticalPodAutoscaler{}
					Expect(c.Get(ctx, client.ObjectKeyFromObject(vpaRecommender), vpa)).To(Succeed())
					vpaRecommender.ResourceVersion = "1"
					Expect(vpa).To(Equal(vpaRecommender))

					serviceMonitor := &monitoringv1.ServiceMonitor{}
					Expect(c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "shoot-vpa-recommender"}, serviceMonitor)).To(Succeed())
					serviceMonitorRecommender := serviceMonitorRecommenderFor(component.ClusterTypeShoot)
					serviceMonitorRecommender.ResourceVersion = "1"
					Expect(serviceMonitor).To(Equal(serviceMonitorRecommender))

					By("Verify vpa-admission-controller application resources")
					clusterRoleBindingAdmissionController.Subjects[0].Namespace = "kube-system"

					Expect(managedResource).To(contain(clusterRoleAdmissionController, clusterRoleBindingAdmissionController))

					By("Verify vpa-admission-controller runtime resources")
					secret = &corev1.Secret{}
					Expect(c.Get(ctx, client.ObjectKeyFromObject(shootAccessSecretAdmissionController), secret)).To(Succeed())
					shootAccessSecretAdmissionController.ResourceVersion = "1"
					Expect(secret).To(Equal(shootAccessSecretAdmissionController))

					service = &corev1.Service{}
					Expect(c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "vpa-webhook"}, service)).To(Succeed())
					serviceAdmissionController := serviceAdmissionControllerFor(component.ClusterTypeShoot, false)
					serviceAdmissionController.ResourceVersion = "1"
					Expect(service).To(Equal(serviceAdmissionController))

					deployment = &appsv1.Deployment{}
					Expect(c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "vpa-admission-controller"}, deployment)).To(Succeed())
					deploymentAdmissionController := deploymentAdmissionControllerFor(false)
					deploymentAdmissionController.ResourceVersion = "1"
					Expect(deployment).To(Equal(deploymentAdmissionController))

					vpa = &vpaautoscalingv1.VerticalPodAutoscaler{}
					Expect(c.Get(ctx, client.ObjectKeyFromObject(vpaAdmissionController), vpa)).To(Succeed())
					vpaAdmissionController.ResourceVersion = "1"
					Expect(vpa).To(Equal(vpaAdmissionController))

					By("Verify general application resources")
					clusterRoleBindingGeneralActor.Subjects[0].Namespace = "kube-system"
					clusterRoleBindingGeneralActor.Subjects[1].Namespace = "kube-system"
					clusterRoleBindingGeneralTargetReader.Subjects[0].Namespace = "kube-system"
					clusterRoleBindingGeneralTargetReader.Subjects[1].Namespace = "kube-system"
					clusterRoleBindingGeneralTargetReader.Subjects[2].Namespace = "kube-system"

					vpaCRD := &apiextensionsv1.CustomResourceDefinition{}
					Expect(yaml.Unmarshal(verticalPodAutoscalerCRD, vpaCRD)).To(Succeed())

					vpaChkPtCRD := &apiextensionsv1.CustomResourceDefinition{}
					Expect(yaml.Unmarshal(verticalPodAutoscalerCheckpointCRD, vpaChkPtCRD)).To(Succeed())

					Expect(managedResource).To(contain(
						clusterRoleGeneralActor,
						clusterRoleBindingGeneralActor,
						clusterRoleGeneralTargetReader,
						clusterRoleBindingGeneralTargetReader,
						mutatingWebhookConfiguration,
						vpaCRD,
						vpaChkPtCRD,
					))
				})

				Context("Kubernetes versions < 1.26", func() {
					It("should successfully deploy all resources", func() {
						expectedPodDisruptionBudgets := []*policyv1.PodDisruptionBudget{
							podDisruptionBudgetUpdater,
							podDisruptionBudgetRecommender,
							podDisruptionBudgetAdmissionController,
						}

						for _, expected := range expectedPodDisruptionBudgets {
							actual := &policyv1.PodDisruptionBudget{}
							Expect(c.Get(ctx, client.ObjectKeyFromObject(expected), actual)).To(Succeed())
							expected.ResourceVersion = "1"
							Expect(actual).To(Equal(expected))
						}
					})
				})

				Context("Kubernetes versions >= 1.26", func() {
					BeforeEach(func() {
						vpa = New(c, namespace, sm, Values{
							ClusterType:              component.ClusterTypeShoot,
							Enabled:                  true,
							SecretNameServerCA:       secretNameCA,
							RuntimeKubernetesVersion: semver.MustParse("1.26.0"),
							AdmissionController:      valuesAdmissionController,
							Recommender:              valuesRecommender,
							Updater:                  valuesUpdater,
						})
					})

					It("should successfully deploy all resources", func() {
						expectedPodDisruptionBudgets := []*policyv1.PodDisruptionBudget{
							podDisruptionBudgetUpdater,
							podDisruptionBudgetRecommender,
							podDisruptionBudgetAdmissionController,
						}

						for _, expected := range expectedPodDisruptionBudgets {
							actual := &policyv1.PodDisruptionBudget{}
							Expect(c.Get(ctx, client.ObjectKeyFromObject(expected), actual)).To(Succeed())
							expected.ResourceVersion = "1"
							unhealthyPodEvictionPolicyAlwaysAllow := policyv1.AlwaysAllow
							expected.Spec.UnhealthyPodEvictionPolicy = &unhealthyPodEvictionPolicyAlwaysAllow
							Expect(actual).To(Equal(expected))
						}
					})
				})
			})

			Context("when TopologyAwareRoutingEnabled=true", func() {
				It("should successfully deploy with expected vpa-webhook service annotations and labels", func() {
					valuesAdmissionController.TopologyAwareRoutingEnabled = true
					vpa = New(c, namespace, sm, Values{
						ClusterType:              component.ClusterTypeShoot,
						Enabled:                  true,
						SecretNameServerCA:       secretNameCA,
						RuntimeKubernetesVersion: runtimeKubernetesVersion,
						AdmissionController:      valuesAdmissionController,
						Recommender:              valuesRecommender,
						Updater:                  valuesUpdater,
					})

					Expect(vpa.Deploy(ctx)).To(Succeed())

					service := &corev1.Service{}
					Expect(c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "vpa-webhook"}, service)).To(Succeed())
					serviceAdmissionController := serviceAdmissionControllerFor(component.ClusterTypeShoot, true)
					serviceAdmissionController.ResourceVersion = "1"
					Expect(service).To(Equal(serviceAdmissionController))
				})
			})
		})
	})

	Describe("#Destroy", func() {
		Context("cluster type seed", func() {
			BeforeEach(func() {
				vpa = New(c, namespace, nil, Values{ClusterType: component.ClusterTypeSeed, RuntimeKubernetesVersion: runtimeKubernetesVersion})
				managedResourceName = "vpa"
			})

			It("should successfully destroy all resources", func() {
				Expect(c.Create(ctx, managedResource)).To(Succeed())
				Expect(c.Create(ctx, managedResourceSecret)).To(Succeed())

				Expect(vpa.Destroy(ctx)).To(Succeed())

				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(BeNotFoundError())
			})
		})

		Context("cluster type shoot", func() {
			BeforeEach(func() {
				vpa = New(c, namespace, nil, Values{ClusterType: component.ClusterTypeShoot, RuntimeKubernetesVersion: runtimeKubernetesVersion})
				managedResourceName = "shoot-core-vpa"
			})

			It("should successfully destroy all resources", func() {
				Expect(c.Create(ctx, managedResource)).To(Succeed())
				Expect(c.Create(ctx, managedResourceSecret)).To(Succeed())

				By("Create vpa-updater runtime resources")
				Expect(c.Create(ctx, deploymentUpdaterFor(true, nil, nil, nil, nil, nil, "kube-system"))).To(Succeed())
				Expect(c.Create(ctx, podDisruptionBudgetUpdater)).To(Succeed())
				Expect(c.Create(ctx, vpaUpdater)).To(Succeed())

				By("Create vpa-recommender runtime resources")
				Expect(c.Create(ctx, serviceRecommenderFor(component.ClusterTypeShoot))).To(Succeed())
				Expect(c.Create(ctx, deploymentRecommenderFor(true, nil, nil, component.ClusterTypeShoot, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "kube-system"))).To(Succeed())
				Expect(c.Create(ctx, podDisruptionBudgetRecommender)).To(Succeed())
				Expect(c.Create(ctx, vpaRecommender)).To(Succeed())
				Expect(c.Create(ctx, serviceMonitorRecommenderFor(component.ClusterTypeShoot))).To(Succeed())

				By("Create vpa-admission-controller runtime resources")
				Expect(c.Create(ctx, serviceAdmissionControllerFor(component.ClusterTypeSeed, false))).To(Succeed())
				Expect(c.Create(ctx, deploymentAdmissionControllerFor(true))).To(Succeed())
				Expect(c.Create(ctx, podDisruptionBudgetAdmissionController)).To(Succeed())
				Expect(c.Create(ctx, vpaAdmissionController)).To(Succeed())

				Expect(vpa.Destroy(ctx)).To(Succeed())

				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(BeNotFoundError())

				By("Verify vpa-updater runtime resources")
				Expect(c.Get(ctx, client.ObjectKeyFromObject(deploymentUpdaterFor(true, nil, nil, nil, nil, nil, "kube-system")), &appsv1.Deployment{})).To(BeNotFoundError())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(podDisruptionBudgetUpdater), &policyv1.PodDisruptionBudget{})).To(BeNotFoundError())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(vpaUpdater), &vpaautoscalingv1.VerticalPodAutoscaler{})).To(BeNotFoundError())

				By("Verify vpa-recommender runtime resources")
				Expect(c.Get(ctx, client.ObjectKeyFromObject(serviceRecommenderFor(component.ClusterTypeShoot)), &appsv1.Deployment{})).To(BeNotFoundError())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(deploymentRecommenderFor(true, nil, nil, component.ClusterTypeShoot, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "kube-system")), &appsv1.Deployment{})).To(BeNotFoundError())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(podDisruptionBudgetRecommender), &policyv1.PodDisruptionBudget{})).To(BeNotFoundError())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(vpaRecommender), &vpaautoscalingv1.VerticalPodAutoscaler{})).To(BeNotFoundError())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(serviceMonitorRecommenderFor(component.ClusterTypeShoot)), &monitoringv1.ServiceMonitor{})).To(BeNotFoundError())

				By("Verify vpa-admission-controller runtime resources")
				Expect(c.Get(ctx, client.ObjectKeyFromObject(serviceAdmissionControllerFor(component.ClusterTypeSeed, false)), &corev1.Service{})).To(BeNotFoundError())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(deploymentAdmissionControllerFor(true)), &appsv1.Deployment{})).To(BeNotFoundError())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(podDisruptionBudgetAdmissionController), &policyv1.PodDisruptionBudget{})).To(BeNotFoundError())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(vpaAdmissionController), &vpaautoscalingv1.VerticalPodAutoscaler{})).To(BeNotFoundError())
			})
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
			tests := func(managedResourceName string) {
				It("should fail because reading the ManagedResource fails", func() {
					Expect(vpa.Wait(ctx)).To(MatchError(ContainSubstring("not found")))
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

					Expect(vpa.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
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

					Expect(vpa.Wait(ctx)).To(Succeed())
				})
			}

			Context("cluster type seed", func() {
				BeforeEach(func() {
					vpa = New(c, namespace, nil, Values{ClusterType: component.ClusterTypeSeed})
				})

				tests("vpa")
			})

			Context("cluster type shoot", func() {
				BeforeEach(func() {
					vpa = New(c, namespace, nil, Values{ClusterType: component.ClusterTypeShoot})
				})

				tests("shoot-core-vpa")
			})
		})

		Describe("#WaitCleanup", func() {
			tests := func(managedResourceName string) {
				It("should fail when the wait for the managed resource deletion times out", func() {
					fakeOps.MaxAttempts = 2

					managedResource := &resourcesv1alpha1.ManagedResource{
						ObjectMeta: metav1.ObjectMeta{
							Name:      managedResourceName,
							Namespace: namespace,
						},
					}

					Expect(c.Create(ctx, managedResource)).To(Succeed())

					Expect(vpa.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
				})

				It("should not return an error when it's already removed", func() {
					Expect(vpa.WaitCleanup(ctx)).To(Succeed())
				})
			}

			Context("cluster type seed", func() {
				BeforeEach(func() {
					vpa = New(c, namespace, nil, Values{ClusterType: component.ClusterTypeSeed})
				})

				tests("vpa")
			})

			Context("cluster type shoot", func() {
				BeforeEach(func() {
					vpa = New(c, namespace, nil, Values{ClusterType: component.ClusterTypeShoot})
				})

				tests("shoot-core-vpa")
			})
		})
	})
})

func replaceTargetSubstrings(in string) string {
	return strings.Replace(in, ":target:", ":source:", -1)
}

func adaptNetworkPolicyLabelsForClusterTypeSeed(labels map[string]string) {
	delete(labels, "networking.resources.gardener.cloud/to-kube-apiserver-tcp-443")
	labels["networking.gardener.cloud/to-runtime-apiserver"] = "allowed"
}
