// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package resourcemanager_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"go.uber.org/mock/gomock"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/gardener/resourcemanager"
	resourcemanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/resourcemanager/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

var _ = Describe("ResourceManager", func() {
	var (
		ctrl            *gomock.Controller
		c               *mockclient.MockClient
		fakeClient      client.Client
		sm              secretsmanager.Interface
		resourceManager Interface

		ctx                               = context.TODO()
		deployNamespace                   = "fake-ns"
		fakeErr                           = fmt.Errorf("fake error")
		image                             = "fake-image"
		replicas                    int32 = 2
		healthPort                  int32 = 8081
		metricsPort                 int32 = 8080
		serverPort                  int32 = 10250
		binPackingSchedulingProfile       = gardencorev1beta1.SchedulingProfileBinPacking

		// optional configuration
		clusterIdentity                    = "foo"
		secretNameServer                   = "gardener-resource-manager-server"
		secretMountPathServer              = "/etc/gardener-resource-manager-tls"
		secretMountPathRootCA              = "/etc/gardener-resource-manager-root-ca"
		secretMountPathConfig              = "/etc/gardener-resource-manager-config"
		secretMountPathAPIAccess           = "/var/run/secrets/kubernetes.io/serviceaccount"
		secrets                            Secrets
		alwaysUpdate                       = true
		concurrentSyncs                    = 20
		genericTokenKubeconfigSecretName   = "generic-token-kubeconfig"
		clusterRoleName                    = "gardener-resource-manager-seed"
		healthSyncPeriod                   = metav1.Duration{Duration: time.Minute}
		maxConcurrentHealthWorkers         = 20
		maxConcurrentTokenRequestorWorkers = 21
		maxConcurrentCSRApproverWorkers    = 24
		maxConcurrentNetworkPolicyWorkers  = 25
		resourceClass                      = "fake-ResourceClass"
		watchedNamespace                   = "fake-ns"
		targetDisableCache                 = true
		maxUnavailable                     = intstr.FromInt32(1)
		failurePolicyFail                  = admissionregistrationv1.Fail
		failurePolicyIgnore                = admissionregistrationv1.Ignore
		reinvocationPolicyNever            = admissionregistrationv1.NeverReinvocationPolicy
		matchPolicyExact                   = admissionregistrationv1.Exact
		matchPolicyEquivalent              = admissionregistrationv1.Equivalent
		sideEffect                         = admissionregistrationv1.SideEffectClassNone
		priorityClassName                  = v1beta1constants.PriorityClassNameSeedSystemCritical
		ingressControllerSelector          = &resourcemanagerconfigv1alpha1.IngressControllerSelector{
			Namespace:   "foo",
			PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{"bar": "baz"}},
		}
		additionalNetworkPolicyNamespaceSelectors = []metav1.LabelSelector{{MatchLabels: map[string]string{"foo": "bar"}}}
		kubernetesServiceHost                     = "some-host"
		targetNamespaces                          = []string{"foo", "bar"}

		allowAll                                             []rbacv1.PolicyRule
		allowManagedResources                                []rbacv1.PolicyRule
		allowMachines                                        []rbacv1.PolicyRule
		cfg                                                  Values
		clusterRole                                          *rbacv1.ClusterRole
		clusterRoleBinding                                   *rbacv1.ClusterRoleBinding
		configMap                                            *corev1.ConfigMap
		deployment                                           *appsv1.Deployment
		defaultNotReadyTolerationSeconds                     *int64
		defaultUnreachableTolerationSeconds                  *int64
		configMapFor                                         func(watchedNamespace *string, responsibilityMode ResponsibilityMode, isWorkerless, bootstrapControlPlaneNode bool) *corev1.ConfigMap
		deploymentFor                                        func(configMapName string, targetClusterDiffersFromSourceCluster bool, secretNameBootstrapKubeconfig *string, bootstrapControlPlaneNode bool) *appsv1.Deployment
		defaultLabels                                        map[string]string
		roleBinding                                          *rbacv1.RoleBinding
		role                                                 *rbacv1.Role
		secret                                               *corev1.Secret
		service                                              *corev1.Service
		serviceAccount                                       *corev1.ServiceAccount
		pdb                                                  *policyv1.PodDisruptionBudget
		serviceMonitorFor                                    func(string) *monitoringv1.ServiceMonitor
		vpa                                                  *vpaautoscalingv1.VerticalPodAutoscaler
		mutatingWebhookConfigurationFor                      func(responsibilityMode ResponsibilityMode, bootstrapControlPlaneNode bool) *admissionregistrationv1.MutatingWebhookConfiguration
		mutatingWebhookConfigurationYAML                     func() string
		clusterRoleBindingTargetYAML                         string
		validatingWebhookConfiguration                       *admissionregistrationv1.ValidatingWebhookConfiguration
		managedResourceSecret                                *corev1.Secret
		managedResource                                      *resourcesv1alpha1.ManagedResource
		matchLabelKeysInPodTopologySpreadFeatureGateDisabled bool
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).Build()
		sm = fakesecretsmanager.New(fakeClient, deployNamespace)

		By("Create secrets managed outside of this package for whose secretsmanager.Get() will be called")
		Expect(fakeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca", Namespace: deployNamespace}})).To(Succeed())
		Expect(fakeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "generic-token-kubeconfig", Namespace: deployNamespace}})).To(Succeed())

		secrets = Secrets{}
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
		allowMachines = []rbacv1.PolicyRule{
			{
				APIGroups: []string{"machine.sapcloud.io"},
				Resources: []string{"machines"},
				Verbs:     []string{"get", "list", "watch"},
			},
		}
		defaultLabels = map[string]string{
			v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
			v1beta1constants.LabelApp:   "gardener-resource-manager",
		}

		defaultNotReadyTolerationSeconds = ptr.To[int64](60)
		defaultUnreachableTolerationSeconds = ptr.To[int64](120)

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
			Rules: append(allowManagedResources, allowMachines...)}
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

		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shoot-access-gardener-resource-manager",
				Namespace: deployNamespace,
				Annotations: map[string]string{
					"serviceaccount.resources.gardener.cloud/name":                      "gardener-resource-manager",
					"serviceaccount.resources.gardener.cloud/namespace":                 "kube-system",
					"serviceaccount.resources.gardener.cloud/token-expiration-duration": "24h",
				},
				Labels: map[string]string{
					"resources.gardener.cloud/purpose": "token-requestor",
					"resources.gardener.cloud/class":   "shoot",
				},
				ResourceVersion: "0",
			},
			Type: corev1.SecretTypeOpaque,
		}

		service = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener-resource-manager",
				Namespace: deployNamespace,
				Labels:    defaultLabels,
				Annotations: map[string]string{
					"networking.resources.gardener.cloud/from-all-scrape-targets-allowed-ports":  `[{"protocol":"TCP","port":8080}]`,
					"networking.resources.gardener.cloud/from-all-webhook-targets-allowed-ports": `[{"protocol":"TCP","port":10250}]`,
				},
			},
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
						TargetPort: intstr.FromInt32(serverPort),
						Protocol:   corev1.ProtocolTCP,
					},
				},
			},
		}
		cfg = Values{
			AlwaysUpdate:                                     &alwaysUpdate,
			ClusterIdentity:                                  &clusterIdentity,
			ConcurrentSyncs:                                  &concurrentSyncs,
			DefaultNotReadyToleration:                        defaultNotReadyTolerationSeconds,
			DefaultUnreachableToleration:                     defaultUnreachableTolerationSeconds,
			NetworkPolicyAdditionalNamespaceSelectors:        additionalNetworkPolicyNamespaceSelectors,
			NetworkPolicyControllerIngressControllerSelector: ingressControllerSelector,
			HealthSyncPeriod:                                 &healthSyncPeriod,
			KubernetesServiceHost:                            &kubernetesServiceHost,
			Image:                                            image,
			MaxConcurrentHealthWorkers:                       &maxConcurrentHealthWorkers,
			MaxConcurrentTokenRequestorWorkers:               &maxConcurrentTokenRequestorWorkers,
			MaxConcurrentCSRApproverWorkers:                  &maxConcurrentCSRApproverWorkers,
			MaxConcurrentNetworkPolicyWorkers:                &maxConcurrentNetworkPolicyWorkers,
			PriorityClassName:                                priorityClassName,
			Replicas:                                         &replicas,
			ResourceClass:                                    &resourceClass,
			SecretNameServerCA:                               "ca",
			SystemComponentTolerations: []corev1.Toleration{
				{Key: "a"},
				{Key: "b"},
				{Key: "c"},
			},
			ResponsibilityMode:                  ForTarget,
			TargetDisableCache:                  &targetDisableCache,
			WatchedNamespace:                    &watchedNamespace,
			SchedulingProfile:                   &binPackingSchedulingProfile,
			DefaultSeccompProfileEnabled:        false,
			EndpointSliceHintsEnabled:           false,
			PodTopologySpreadConstraintsEnabled: true,
			LogLevel:                            "info",
			LogFormat:                           "json",
			Zones:                               []string{"a", "b"},
			ManagedResourceLabels:               map[string]string{"foo": "bar"},
		}
		resourceManager = New(c, deployNamespace, sm, cfg)
		resourceManager.SetSecrets(secrets)

		matchLabelKeysInPodTopologySpreadFeatureGateDisabled = true
		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{Name: "gardener-resource-manager",
				Namespace: deployNamespace,
				Labels:    defaultLabels,
			},
			AutomountServiceAccountToken: ptr.To(false),
		}

		configMapFor = func(watchedNamespace *string, responsibilityMode ResponsibilityMode, isWorkerless, bootstrapControlPlaneNode bool) *corev1.ConfigMap {
			configMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "gardener-resource-manager",
					Namespace: deployNamespace,
				},
			}

			config := &resourcemanagerconfigv1alpha1.ResourceManagerConfiguration{
				LeaderElection: componentbaseconfigv1alpha1.LeaderElectionConfiguration{
					LeaderElect:       ptr.To(true),
					ResourceName:      "gardener-resource-manager",
					ResourceNamespace: deployNamespace,
				},
				Server: resourcemanagerconfigv1alpha1.ServerConfiguration{
					HealthProbes: &resourcemanagerconfigv1alpha1.Server{
						Port: int(healthPort),
					},
					Metrics: &resourcemanagerconfigv1alpha1.Server{
						Port: int(metricsPort),
					},
					Webhooks: resourcemanagerconfigv1alpha1.HTTPSServer{
						Server: resourcemanagerconfigv1alpha1.Server{
							Port: int(serverPort),
						},
						TLS: resourcemanagerconfigv1alpha1.TLSServer{
							ServerCertDir: secretMountPathServer,
						},
					},
				},
				LogLevel:  "info",
				LogFormat: "json",
				Controllers: resourcemanagerconfigv1alpha1.ResourceManagerControllerConfiguration{
					ClusterID:     &clusterIdentity,
					ResourceClass: &resourceClass,
					GarbageCollector: resourcemanagerconfigv1alpha1.GarbageCollectorControllerConfig{
						Enabled:    true,
						SyncPeriod: &metav1.Duration{Duration: 12 * time.Hour},
					},
					Health: resourcemanagerconfigv1alpha1.HealthControllerConfig{
						ConcurrentSyncs: &maxConcurrentHealthWorkers,
						SyncPeriod:      &healthSyncPeriod,
					},
					CSRApprover: resourcemanagerconfigv1alpha1.CSRApproverControllerConfig{
						Enabled:         !isWorkerless,
						ConcurrentSyncs: &maxConcurrentCSRApproverWorkers,
					},
					ManagedResource: resourcemanagerconfigv1alpha1.ManagedResourceControllerConfig{
						ConcurrentSyncs: &concurrentSyncs,
						SyncPeriod:      &metav1.Duration{Duration: time.Hour},
						AlwaysUpdate:    &alwaysUpdate,
					},
					TokenRequestor: resourcemanagerconfigv1alpha1.TokenRequestorControllerConfig{
						Enabled:         true,
						ConcurrentSyncs: &maxConcurrentTokenRequestorWorkers,
					},
					NodeCriticalComponents: resourcemanagerconfigv1alpha1.NodeCriticalComponentsControllerConfig{
						Enabled: false,
					},
				},
				Webhooks: resourcemanagerconfigv1alpha1.ResourceManagerWebhookConfiguration{
					HighAvailabilityConfig: resourcemanagerconfigv1alpha1.HighAvailabilityConfigWebhookConfig{
						Enabled:                             !isWorkerless && !bootstrapControlPlaneNode,
						DefaultNotReadyTolerationSeconds:    defaultNotReadyTolerationSeconds,
						DefaultUnreachableTolerationSeconds: defaultUnreachableTolerationSeconds,
					},
					KubernetesServiceHost: resourcemanagerconfigv1alpha1.KubernetesServiceHostWebhookConfig{
						Enabled: !isWorkerless,
						Host:    kubernetesServiceHost,
					},
					PodSchedulerName: resourcemanagerconfigv1alpha1.PodSchedulerNameWebhookConfig{
						Enabled: false,
					},
					PodTopologySpreadConstraints: resourcemanagerconfigv1alpha1.PodTopologySpreadConstraintsWebhookConfig{
						Enabled: !isWorkerless && matchLabelKeysInPodTopologySpreadFeatureGateDisabled,
					},
					ProjectedTokenMount: resourcemanagerconfigv1alpha1.ProjectedTokenMountWebhookConfig{
						Enabled: !isWorkerless,
					},
					SystemComponentsConfig: resourcemanagerconfigv1alpha1.SystemComponentsConfigWebhookConfig{
						Enabled: false,
					},
				},
			}

			if watchedNamespace != nil {
				config.SourceClientConnection.Namespaces = []string{*watchedNamespace}
				config.Controllers.CSRApprover.MachineNamespace = *watchedNamespace
			}

			if responsibilityMode == ForTarget {
				config.TargetClientConnection = &resourcemanagerconfigv1alpha1.ClientConnection{
					ClientConnectionConfiguration: componentbaseconfigv1alpha1.ClientConnectionConfiguration{
						Kubeconfig: gardenerutils.PathGenericKubeconfig,
					},
					Namespaces: targetNamespaces,
				}

				config.Webhooks.PodSchedulerName = resourcemanagerconfigv1alpha1.PodSchedulerNameWebhookConfig{
					Enabled:       !isWorkerless,
					SchedulerName: ptr.To("bin-packing-scheduler"),
				}
			} else {
				config.Controllers.NetworkPolicy = resourcemanagerconfigv1alpha1.NetworkPolicyControllerConfig{
					Enabled:         true,
					ConcurrentSyncs: &maxConcurrentNetworkPolicyWorkers,
					NamespaceSelectors: []metav1.LabelSelector{
						{MatchLabels: map[string]string{"gardener.cloud/role": "shoot"}},
						{MatchLabels: map[string]string{"gardener.cloud/role": "extension"}},
						{MatchLabels: map[string]string{"gardener.cloud/role": "istio-system"}},
						{MatchLabels: map[string]string{"gardener.cloud/role": "istio-ingress"}},
						{MatchExpressions: []metav1.LabelSelectorRequirement{{Key: "handler.exposureclass.gardener.cloud/name", Operator: metav1.LabelSelectorOpExists}}},
						{MatchLabels: map[string]string{"kubernetes.io/metadata.name": "garden"}},
						{MatchLabels: map[string]string{"foo": "bar"}},
					},
					IngressControllerSelector: ingressControllerSelector,
				}
				config.Webhooks.CRDDeletionProtection.Enabled = true
				config.Webhooks.ExtensionValidation.Enabled = true
				config.Webhooks.SeccompProfile.Enabled = true
			}

			if responsibilityMode == ForSource {
				config.Webhooks.EndpointSliceHints.Enabled = true
			}

			if responsibilityMode == ForTarget || responsibilityMode == ForSourceAndTarget {
				config.Webhooks.SystemComponentsConfig = resourcemanagerconfigv1alpha1.SystemComponentsConfigWebhookConfig{
					Enabled: !isWorkerless,
					NodeSelector: map[string]string{
						"worker.gardener.cloud/system-components": "true",
					},
					PodNodeSelector: map[string]string{
						"worker.gardener.cloud/system-components": "true",
					},
					PodTolerations: []corev1.Toleration{
						{Key: "a"},
						{Key: "b"},
						{Key: "c"},
					},
				}
				config.Controllers.NodeCriticalComponents.Enabled = !isWorkerless
			}

			data, err := runtime.Encode(codec, config)
			Expect(err).NotTo(HaveOccurred())

			configMap.Data = map[string]string{"config.yaml": string(data)}
			utilruntime.Must(kubernetesutils.MakeUnique(configMap))

			return configMap
		}

		deploymentFor = func(
			configMapName string,
			targetClusterDiffersFromSourceCluster bool,
			secretNameBootstrapKubeconfig *string,
			bootstrapControlPlaneNode bool,
		) *appsv1.Deployment {
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      v1beta1constants.DeploymentNameGardenerResourceManager,
					Namespace: deployNamespace,
					Labels:    defaultLabels,
				},
				Spec: appsv1.DeploymentSpec{
					Replicas:             &replicas,
					RevisionHistoryLimit: ptr.To[int32](2),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "gardener-resource-manager",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"projected-token-mount.resources.gardener.cloud/skip":           "true",
								"networking.gardener.cloud/to-dns":                              "allowed",
								"networking.gardener.cloud/to-runtime-apiserver":                "allowed",
								"networking.resources.gardener.cloud/to-kube-apiserver-tcp-443": "allowed",
								v1beta1constants.GardenRole:                                     v1beta1constants.GardenRoleControlPlane,
								v1beta1constants.LabelApp:                                       "gardener-resource-manager",
							},
						},
						Spec: corev1.PodSpec{
							PriorityClassName: priorityClassName,
							SecurityContext: &corev1.PodSecurityContext{
								SeccompProfile: &corev1.SeccompProfile{
									Type: corev1.SeccompProfileTypeRuntimeDefault,
								},
							},
							ServiceAccountName: "gardener-resource-manager",
							Containers: []corev1.Container{
								{
									Args:            []string{"--config=/etc/gardener-resource-manager-config/config.yaml"},
									Image:           image,
									ImagePullPolicy: corev1.PullIfNotPresent,
									LivenessProbe: &corev1.Probe{
										ProbeHandler: corev1.ProbeHandler{
											HTTPGet: &corev1.HTTPGetAction{
												Path:   "/healthz",
												Scheme: "HTTP",
												Port:   intstr.FromInt32(healthPort),
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
									ReadinessProbe: &corev1.Probe{
										ProbeHandler: corev1.ProbeHandler{
											HTTPGet: &corev1.HTTPGetAction{
												Path:   "/readyz",
												Scheme: "HTTP",
												Port:   intstr.FromInt32(healthPort),
											},
										},
										InitialDelaySeconds: 10,
									},
									Resources: corev1.ResourceRequirements{
										Requests: corev1.ResourceList{
											corev1.ResourceCPU:    resource.MustParse("5m"),
											corev1.ResourceMemory: resource.MustParse("30M"),
										},
									},
									SecurityContext: &corev1.SecurityContext{
										AllowPrivilegeEscalation: ptr.To(false),
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
											MountPath: secretMountPathConfig,
											Name:      "config",
											ReadOnly:  true,
										},
									},
								},
							},
							Tolerations: []corev1.Toleration{
								{
									Key:               corev1.TaintNodeNotReady,
									Operator:          corev1.TolerationOpExists,
									Effect:            corev1.TaintEffectNoExecute,
									TolerationSeconds: defaultNotReadyTolerationSeconds,
								},
								{
									Key:               corev1.TaintNodeUnreachable,
									Operator:          corev1.TolerationOpExists,
									Effect:            corev1.TaintEffectNoExecute,
									TolerationSeconds: defaultUnreachableTolerationSeconds,
								},
							},
							Volumes: []corev1.Volume{
								{
									Name: "kube-api-access-gardener",
									VolumeSource: corev1.VolumeSource{
										Projected: &corev1.ProjectedVolumeSource{
											DefaultMode: ptr.To[int32](420),
											Sources: []corev1.VolumeProjection{
												{
													ServiceAccountToken: &corev1.ServiceAccountTokenProjection{
														ExpirationSeconds: ptr.To[int64](43200),
														Path:              "token",
													},
												},
												{
													ConfigMap: &corev1.ConfigMapProjection{
														LocalObjectReference: corev1.LocalObjectReference{
															Name: "kube-root-ca.crt",
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
											DefaultMode: ptr.To[int32](420),
										},
									},
								},
								{
									Name: "config",
									VolumeSource: corev1.VolumeSource{
										ConfigMap: &corev1.ConfigMapVolumeSource{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: configMapName,
											},
										},
									},
								},
							},
						},
					},
				},
			}

			if bootstrapControlPlaneNode {
				deployment.Spec.Template.Spec.Tolerations = append(deployment.Spec.Template.Spec.Tolerations,
					corev1.Toleration{Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule},
					corev1.Toleration{Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoExecute},
				)
				deployment.Spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{{Name: "KUBERNETES_SERVICE_HOST", Value: "localhost"}}
				deployment.Spec.Template.Spec.HostNetwork = true
			} else {
				deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
					MountPath: secretMountPathRootCA,
					Name:      "root-ca",
					ReadOnly:  true,
				})
				deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, corev1.Volume{
					Name: "root-ca",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName:  "ca",
							DefaultMode: ptr.To[int32](420),
						},
					},
				})
			}

			if secretNameBootstrapKubeconfig != nil {
				deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
					Name:      "kubeconfig-bootstrap",
					MountPath: "/var/run/secrets/gardener.cloud/shoot/generic-kubeconfig",
					ReadOnly:  true,
				})
				deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, corev1.Volume{
					Name: "kubeconfig-bootstrap",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName:  *secretNameBootstrapKubeconfig,
							DefaultMode: ptr.To[int32](420),
						},
					},
				})
			} else if !bootstrapControlPlaneNode {
				deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
					Name:      "kubeconfig",
					MountPath: "/var/run/secrets/gardener.cloud/shoot/generic-kubeconfig",
					ReadOnly:  true,
				})
				deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, corev1.Volume{
					Name: "kubeconfig",
					VolumeSource: corev1.VolumeSource{
						Projected: &corev1.ProjectedVolumeSource{
							DefaultMode: ptr.To[int32](420),
							Sources: []corev1.VolumeProjection{
								{
									Secret: &corev1.SecretProjection{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: genericTokenKubeconfigSecretName,
										},
										Items: []corev1.KeyToPath{{
											Key:  "kubeconfig",
											Path: "kubeconfig",
										}},
										Optional: ptr.To(false),
									},
								},
								{
									Secret: &corev1.SecretProjection{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "shoot-access-gardener-resource-manager",
										},
										Items: []corev1.KeyToPath{{
											Key:  resourcesv1alpha1.DataKeyToken,
											Path: resourcesv1alpha1.DataKeyToken,
										}},
										Optional: ptr.To(false),
									},
								},
							},
						},
					},
				})
			}

			utilruntime.Must(references.InjectAnnotations(deployment))

			if targetClusterDiffersFromSourceCluster {
				deployment.Labels = utils.MergeStringMaps(deployment.Labels, map[string]string{
					"high-availability-config.resources.gardener.cloud/type": "server",
				})
			} else {
				deployment.Labels = utils.MergeStringMaps(deployment.Labels, map[string]string{
					"high-availability-config.resources.gardener.cloud/skip": "true",
				})

				deployment.Spec.Template.Spec.TopologySpreadConstraints = []corev1.TopologySpreadConstraint{
					{
						MaxSkew:           1,
						TopologyKey:       "kubernetes.io/hostname",
						WhenUnsatisfiable: "ScheduleAnyway",
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
								v1beta1constants.LabelApp:   "gardener-resource-manager",
							},
						},
						MatchLabelKeys: []string{"pod-template-hash"},
					},
					{
						MaxSkew:           1,
						MinDomains:        ptr.To[int32](2),
						TopologyKey:       "topology.kubernetes.io/zone",
						WhenUnsatisfiable: "DoNotSchedule",
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
								v1beta1constants.LabelApp:   "gardener-resource-manager",
							},
						},
						MatchLabelKeys: []string{"pod-template-hash"},
					},
				}
			}

			return deployment
		}
		vpa = &vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener-resource-manager-vpa",
				Namespace: deployNamespace,
				Labels:    defaultLabels,
			},
			Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
				TargetRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       "gardener-resource-manager",
				},
				UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
					UpdateMode: ptr.To(vpaautoscalingv1.UpdateModeAuto),
				},
				ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
					ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{{
						ContainerName:    vpaautoscalingv1.DefaultContainerResourcePolicy,
						ControlledValues: ptr.To(vpaautoscalingv1.ContainerControlledValuesRequestsOnly),
					}},
				},
			},
		}
		pdb = &policyv1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener-resource-manager",
				Namespace: deployNamespace,
				Labels:    defaultLabels,
			},
			Spec: policyv1.PodDisruptionBudgetSpec{
				MaxUnavailable: &maxUnavailable,
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
						v1beta1constants.LabelApp:   "gardener-resource-manager",
					},
				},
				UnhealthyPodEvictionPolicy: ptr.To(policyv1.AlwaysAllow),
			},
		}
		serviceMonitorFor = func(prometheusLabel string) *monitoringv1.ServiceMonitor {
			return &monitoringv1.ServiceMonitor{
				ObjectMeta: metav1.ObjectMeta{
					Name:      prometheusLabel + "-gardener-resource-manager",
					Namespace: deployNamespace,
					Labels:    map[string]string{"prometheus": prometheusLabel},
				},
				Spec: monitoringv1.ServiceMonitorSpec{
					Selector: metav1.LabelSelector{MatchLabels: map[string]string{
						"app": "gardener-resource-manager",
					}},
					Endpoints: []monitoringv1.Endpoint{{
						Port: "metrics",
						RelabelConfigs: []monitoringv1.RelabelConfig{{
							Action: "labelmap",
							Regex:  `__meta_kubernetes_service_label_(.+)`,
						}},
					}},
				},
			}
		}

		mutatingWebhookConfigurationFor = func(responsibilityMode ResponsibilityMode, bootstrapControlPlaneNode bool) *admissionregistrationv1.MutatingWebhookConfiguration {
			obj := &admissionregistrationv1.MutatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "gardener-resource-manager",
					Namespace: deployNamespace,
					Labels: map[string]string{
						"app": "gardener-resource-manager",
						"remediation.webhook.shoot.gardener.cloud/exclude": "true",
					},
				},
				Webhooks: []admissionregistrationv1.MutatingWebhook{
					{
						Name: "projected-token-mount.resources.gardener.cloud",
						Rules: []admissionregistrationv1.RuleWithOperations{{
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"pods"},
							},
							Operations: []admissionregistrationv1.OperationType{"CREATE"},
						}},
						NamespaceSelector: &metav1.LabelSelector{
							MatchExpressions: []metav1.LabelSelectorRequirement{{
								Key:      "gardener.cloud/purpose",
								Operator: metav1.LabelSelectorOpNotIn,
								Values:   []string{"kube-system", "kubernetes-dashboard"},
							}},
						},
						ObjectSelector: &metav1.LabelSelector{
							MatchExpressions: []metav1.LabelSelectorRequirement{
								{
									Key:      "projected-token-mount.resources.gardener.cloud/skip",
									Operator: metav1.LabelSelectorOpDoesNotExist,
								},
								{
									Key:      "app",
									Operator: metav1.LabelSelectorOpNotIn,
									Values:   []string{"gardener-resource-manager"},
								},
							},
						},
						ClientConfig: admissionregistrationv1.WebhookClientConfig{
							Service: &admissionregistrationv1.ServiceReference{
								Name:      "gardener-resource-manager",
								Namespace: deployNamespace,
								Path:      ptr.To("/webhooks/mount-projected-service-account-token"),
							},
						},
						AdmissionReviewVersions: []string{"v1beta1", "v1"},
						FailurePolicy:           &failurePolicyFail,
						MatchPolicy:             &matchPolicyExact,
						SideEffects:             &sideEffect,
						TimeoutSeconds:          ptr.To[int32](10),
					},
				},
			}

			if !bootstrapControlPlaneNode {
				obj.Webhooks = append(obj.Webhooks, admissionregistrationv1.MutatingWebhook{
					Name: "high-availability-config.resources.gardener.cloud",
					Rules: []admissionregistrationv1.RuleWithOperations{
						{
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"apps"},
								APIVersions: []string{"v1"},
								Resources:   []string{"deployments", "statefulsets"},
							},
							Operations: []admissionregistrationv1.OperationType{"CREATE", "UPDATE"},
						},
						{
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"autoscaling"},
								APIVersions: []string{"v2beta1", "v2"},
								Resources:   []string{"horizontalpodautoscalers"},
							},
							Operations: []admissionregistrationv1.OperationType{"CREATE", "UPDATE"},
						},
					},
					NamespaceSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{{
							Key:      "gardener.cloud/purpose",
							Operator: metav1.LabelSelectorOpNotIn,
							Values:   []string{"kube-system", "kubernetes-dashboard"},
						}},
						MatchLabels: map[string]string{
							"high-availability-config.resources.gardener.cloud/consider": "true",
						},
					},
					ObjectSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      "high-availability-config.resources.gardener.cloud/skip",
								Operator: metav1.LabelSelectorOpDoesNotExist,
							},
						},
					},
					ClientConfig: admissionregistrationv1.WebhookClientConfig{
						Service: &admissionregistrationv1.ServiceReference{
							Name:      "gardener-resource-manager",
							Namespace: deployNamespace,
							Path:      ptr.To("/webhooks/high-availability-config"),
						},
					},
					AdmissionReviewVersions: []string{"v1beta1", "v1"},
					FailurePolicy:           &failurePolicyFail,
					MatchPolicy:             &matchPolicyEquivalent,
					SideEffects:             &sideEffect,
					TimeoutSeconds:          ptr.To[int32](10),
				})
			}

			obj.Webhooks = append(obj.Webhooks,
				admissionregistrationv1.MutatingWebhook{
					Name: "seccomp-profile.resources.gardener.cloud",
					Rules: []admissionregistrationv1.RuleWithOperations{{
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{""},
							APIVersions: []string{"v1"},
							Resources:   []string{"pods"},
						},
						Operations: []admissionregistrationv1.OperationType{"CREATE"},
					}},
					NamespaceSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{{
							Key:      "gardener.cloud/purpose",
							Operator: metav1.LabelSelectorOpNotIn,
							Values:   []string{"kube-system", "kubernetes-dashboard"},
						}},
					},
					ObjectSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      "seccompprofile.resources.gardener.cloud/skip",
								Operator: metav1.LabelSelectorOpDoesNotExist,
							},
							{
								Key:      "app",
								Operator: metav1.LabelSelectorOpNotIn,
								Values:   []string{"gardener-resource-manager"},
							},
						},
					},
					ClientConfig: admissionregistrationv1.WebhookClientConfig{
						Service: &admissionregistrationv1.ServiceReference{
							Name:      "gardener-resource-manager",
							Namespace: deployNamespace,
							Path:      ptr.To("/webhooks/seccomp-profile"),
						},
					},
					AdmissionReviewVersions: []string{"v1beta1", "v1"},
					FailurePolicy:           &failurePolicyFail,
					MatchPolicy:             &matchPolicyExact,
					SideEffects:             &sideEffect,
					TimeoutSeconds:          ptr.To[int32](10),
				},
				admissionregistrationv1.MutatingWebhook{
					Name: "kubernetes-service-host.resources.gardener.cloud",
					Rules: []admissionregistrationv1.RuleWithOperations{{
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{""},
							APIVersions: []string{"v1"},
							Resources:   []string{"pods"},
						},
						Operations: []admissionregistrationv1.OperationType{"CREATE"},
					}},
					NamespaceSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{{
							Key:      resourcesv1alpha1.KubernetesServiceHostInject,
							Operator: metav1.LabelSelectorOpNotIn,
							Values:   []string{"disable"},
						}},
					},
					ObjectSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      resourcesv1alpha1.KubernetesServiceHostInject,
								Operator: metav1.LabelSelectorOpNotIn,
								Values:   []string{"disable"},
							},
						},
					},
					ClientConfig: admissionregistrationv1.WebhookClientConfig{
						Service: &admissionregistrationv1.ServiceReference{
							Name:      "gardener-resource-manager",
							Namespace: deployNamespace,
							Path:      ptr.To("/webhooks/kubernetes-service-host"),
						},
					},
					AdmissionReviewVersions: []string{"v1beta1", "v1"},
					ReinvocationPolicy:      &reinvocationPolicyNever,
					FailurePolicy:           &failurePolicyIgnore,
					MatchPolicy:             &matchPolicyExact,
					SideEffects:             &sideEffect,
					TimeoutSeconds:          ptr.To[int32](2),
				},
			)

			if responsibilityMode == ForSource {
				obj.Webhooks = append(obj.Webhooks,
					admissionregistrationv1.MutatingWebhook{
						Name: "endpoint-slice-hints.resources.gardener.cloud",
						Rules: []admissionregistrationv1.RuleWithOperations{{
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"discovery.k8s.io"},
								APIVersions: []string{"v1"},
								Resources:   []string{"endpointslices"},
							},
							Operations: []admissionregistrationv1.OperationType{"CREATE", "UPDATE"},
						}},
						NamespaceSelector: &metav1.LabelSelector{
							MatchExpressions: []metav1.LabelSelectorRequirement{{
								Key:      "gardener.cloud/purpose",
								Operator: metav1.LabelSelectorOpNotIn,
								Values:   []string{"kube-system", "kubernetes-dashboard"},
							}},
						},
						ObjectSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"endpoint-slice-hints.resources.gardener.cloud/consider": "true",
							},
						},
						ClientConfig: admissionregistrationv1.WebhookClientConfig{
							Service: &admissionregistrationv1.ServiceReference{
								Name:      "gardener-resource-manager",
								Namespace: deployNamespace,
								Path:      ptr.To("/webhooks/endpoint-slice-hints"),
							},
						},
						AdmissionReviewVersions: []string{"v1beta1", "v1"},
						FailurePolicy:           &failurePolicyFail,
						MatchPolicy:             &matchPolicyEquivalent,
						SideEffects:             &sideEffect,
						TimeoutSeconds:          ptr.To[int32](10),
					},
				)
			} else if responsibilityMode == ForSourceAndTarget {
				obj.Webhooks = append(obj.Webhooks,
					admissionregistrationv1.MutatingWebhook{
						Name: "system-components-config.resources.gardener.cloud",
						Rules: []admissionregistrationv1.RuleWithOperations{{
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"pods"},
							},
							Operations: []admissionregistrationv1.OperationType{"CREATE"},
						}},
						NamespaceSelector: &metav1.LabelSelector{
							MatchExpressions: []metav1.LabelSelectorRequirement{{
								Key:      "gardener.cloud/purpose",
								Operator: metav1.LabelSelectorOpNotIn,
								Values:   []string{"kube-system", "kubernetes-dashboard"},
							}},
						},
						ObjectSelector: &metav1.LabelSelector{
							MatchExpressions: []metav1.LabelSelectorRequirement{{
								Key:      "system-components-config.resources.gardener.cloud/skip",
								Operator: metav1.LabelSelectorOpDoesNotExist,
							}},
						},
						ClientConfig: admissionregistrationv1.WebhookClientConfig{
							Service: &admissionregistrationv1.ServiceReference{
								Name:      "gardener-resource-manager",
								Namespace: deployNamespace,
								Path:      ptr.To("/webhooks/system-components-config"),
							},
						},
						AdmissionReviewVersions: []string{"v1beta1", "v1"},
						FailurePolicy:           &failurePolicyFail,
						MatchPolicy:             &matchPolicyExact,
						SideEffects:             &sideEffect,
						TimeoutSeconds:          ptr.To[int32](10),
					},
				)
			}

			obj.Webhooks = append(obj.Webhooks, admissionregistrationv1.MutatingWebhook{
				Name: "pod-topology-spread-constraints.resources.gardener.cloud",
				Rules: []admissionregistrationv1.RuleWithOperations{{
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{""},
						APIVersions: []string{"v1"},
						Resources:   []string{"pods"},
					},
					Operations: []admissionregistrationv1.OperationType{"CREATE"},
				}},
				NamespaceSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{{
						Key:      "gardener.cloud/purpose",
						Operator: metav1.LabelSelectorOpNotIn,
						Values:   []string{"kube-system", "kubernetes-dashboard"},
					}},
				},
				ObjectSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      "app",
							Operator: metav1.LabelSelectorOpNotIn,
							Values:   []string{"gardener-resource-manager"},
						},
						{
							Key:      "topology-spread-constraints.resources.gardener.cloud/skip",
							Operator: metav1.LabelSelectorOpDoesNotExist,
						},
					},
				},
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					Service: &admissionregistrationv1.ServiceReference{
						Name:      "gardener-resource-manager",
						Namespace: deployNamespace,
						Path:      ptr.To("/webhooks/pod-topology-spread-constraints"),
					},
				},
				AdmissionReviewVersions: []string{"v1beta1", "v1"},
				FailurePolicy:           &failurePolicyFail,
				MatchPolicy:             &matchPolicyExact,
				SideEffects:             &sideEffect,
				TimeoutSeconds:          ptr.To[int32](10),
			})

			return obj
		}

		mutatingWebhookConfigurationYAML = func() string {
			out := `apiVersion: admissionregistration.k8s.io/v1
kind: MutatingWebhookConfiguration
metadata:
  creationTimestamp: null
  labels:
    app: gardener-resource-manager
  name: gardener-resource-manager-shoot
  namespace: fake-ns
webhooks:
- admissionReviewVersions:
  - v1beta1
  - v1
  clientConfig:
    url: https://gardener-resource-manager.` + deployNamespace + `:443/webhooks/mount-projected-service-account-token
  failurePolicy: Fail
  matchPolicy: Exact
  name: projected-token-mount.resources.gardener.cloud
  namespaceSelector:
    matchExpressions:
    - key: gardener.cloud/purpose
      operator: In
      values:
      - kube-system
      - kubernetes-dashboard
  objectSelector:
    matchExpressions:
    - key: projected-token-mount.resources.gardener.cloud/skip
      operator: DoesNotExist
    - key: app
      operator: NotIn
      values:
      - gardener-resource-manager
  rules:
  - apiGroups:
    - ""
    apiVersions:
    - v1
    operations:
    - CREATE
    resources:
    - pods
  sideEffects: None
  timeoutSeconds: 10
- admissionReviewVersions:
  - v1beta1
  - v1
  clientConfig:
    url: https://gardener-resource-manager.` + deployNamespace + `:443/webhooks/high-availability-config
  failurePolicy: Fail
  matchPolicy: Equivalent
  name: high-availability-config.resources.gardener.cloud
  namespaceSelector:
    matchExpressions:
    - key: gardener.cloud/purpose
      operator: In
      values:
      - kube-system
      - kubernetes-dashboard
    matchLabels:
      high-availability-config.resources.gardener.cloud/consider: "true"
  objectSelector:
    matchExpressions:
    - key: high-availability-config.resources.gardener.cloud/skip
      operator: DoesNotExist
    matchLabels:
      resources.gardener.cloud/managed-by: gardener
  rules:
  - apiGroups:
    - apps
    apiVersions:
    - v1
    operations:
    - CREATE
    - UPDATE
    resources:
    - deployments
    - statefulsets
  - apiGroups:
    - autoscaling
    apiVersions:
    - v2beta1
    - v2
    operations:
    - CREATE
    - UPDATE
    resources:
    - horizontalpodautoscalers
  sideEffects: None
  timeoutSeconds: 10
- admissionReviewVersions:
  - v1beta1
  - v1
  clientConfig:
    url: https://gardener-resource-manager.` + deployNamespace + `:443/webhooks/default-pod-scheduler-name
  failurePolicy: Ignore
  matchPolicy: Exact
  name: pod-scheduler-name.resources.gardener.cloud
  namespaceSelector: {}
  objectSelector: {}
  rules:
  - apiGroups:
    - ""
    apiVersions:
    - v1
    operations:
    - CREATE
    resources:
    - pods
  sideEffects: None
  timeoutSeconds: 10
- admissionReviewVersions:
  - v1beta1
  - v1
  clientConfig:
    url: https://gardener-resource-manager.` + deployNamespace + `:443/webhooks/kubernetes-service-host
  failurePolicy: Ignore
  matchPolicy: Exact
  name: kubernetes-service-host.resources.gardener.cloud
  namespaceSelector:
    matchExpressions:
    - key: apiserver-proxy.networking.gardener.cloud/inject
      operator: NotIn
      values:
      - disable
  objectSelector:
    matchExpressions:
    - key: apiserver-proxy.networking.gardener.cloud/inject
      operator: NotIn
      values:
      - disable
  reinvocationPolicy: Never
  rules:
  - apiGroups:
    - ""
    apiVersions:
    - v1
    operations:
    - CREATE
    resources:
    - pods
  sideEffects: None
  timeoutSeconds: 2
- admissionReviewVersions:
  - v1beta1
  - v1
  clientConfig:
    url: https://gardener-resource-manager.` + deployNamespace + `:443/webhooks/system-components-config
  failurePolicy: Fail
  matchPolicy: Exact
  name: system-components-config.resources.gardener.cloud
  namespaceSelector:
    matchExpressions:
    - key: gardener.cloud/purpose
      operator: In
      values:
      - kube-system
      - kubernetes-dashboard
  objectSelector:
    matchExpressions:
    - key: system-components-config.resources.gardener.cloud/skip
      operator: DoesNotExist
    matchLabels:
      resources.gardener.cloud/managed-by: gardener
  rules:
  - apiGroups:
    - ""
    apiVersions:
    - v1
    operations:
    - CREATE
    resources:
    - pods
  sideEffects: None
  timeoutSeconds: 10`
			if matchLabelKeysInPodTopologySpreadFeatureGateDisabled {
				out += `
- admissionReviewVersions:
  - v1beta1
  - v1
  clientConfig:
    url: https://gardener-resource-manager.` + deployNamespace + `:443/webhooks/pod-topology-spread-constraints
  failurePolicy: Fail
  matchPolicy: Exact
  name: pod-topology-spread-constraints.resources.gardener.cloud
  namespaceSelector:
    matchExpressions:
    - key: gardener.cloud/purpose
      operator: In
      values:
      - kube-system
      - kubernetes-dashboard
  objectSelector:
    matchExpressions:
    - key: app
      operator: NotIn
      values:
      - gardener-resource-manager
    - key: topology-spread-constraints.resources.gardener.cloud/skip
      operator: DoesNotExist
    matchLabels:
      resources.gardener.cloud/managed-by: gardener
  rules:
  - apiGroups:
    - ""
    apiVersions:
    - v1
    operations:
    - CREATE
    resources:
    - pods
  sideEffects: None
  timeoutSeconds: 10
`
			}

			return out
		}

		clusterRoleBindingTargetYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  annotations:
    resources.gardener.cloud/keep-object: "true"
  creationTimestamp: null
  name: gardener.cloud:target:resource-manager
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
- kind: ServiceAccount
  name: gardener-resource-manager
  namespace: kube-system
`

		validatingWebhookConfiguration = &admissionregistrationv1.ValidatingWebhookConfiguration{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener-resource-manager",
				Namespace: deployNamespace,
				Labels: map[string]string{
					"app": "gardener-resource-manager",
					"remediation.webhook.shoot.gardener.cloud/exclude": "true",
				},
			},
			Webhooks: []admissionregistrationv1.ValidatingWebhook{
				{
					Name: "crd-deletion-protection.resources.gardener.cloud",
					Rules: []admissionregistrationv1.RuleWithOperations{{
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{"apiextensions.k8s.io"},
							APIVersions: []string{"v1beta1", "v1"},
							Resources:   []string{"customresourcedefinitions"},
						},
						Operations: []admissionregistrationv1.OperationType{"DELETE"},
					}},
					FailurePolicy:     &failurePolicyFail,
					NamespaceSelector: &metav1.LabelSelector{},
					ObjectSelector:    &metav1.LabelSelector{MatchLabels: map[string]string{"gardener.cloud/deletion-protected": "true"}},
					ClientConfig: admissionregistrationv1.WebhookClientConfig{
						Service: &admissionregistrationv1.ServiceReference{
							Name:      "gardener-resource-manager",
							Namespace: deployNamespace,
							Path:      ptr.To("/webhooks/validate-crd-deletion"),
						},
					},
					AdmissionReviewVersions: []string{"v1beta1", "v1"},
					MatchPolicy:             &matchPolicyExact,
					SideEffects:             &sideEffect,
					TimeoutSeconds:          ptr.To[int32](10),
				},
				{
					Name: "cr-deletion-protection.resources.gardener.cloud",
					Rules: []admissionregistrationv1.RuleWithOperations{
						{
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"druid.gardener.cloud"},
								APIVersions: []string{"v1alpha1"},
								Resources: []string{
									"etcds",
								},
							},
							Operations: []admissionregistrationv1.OperationType{"DELETE"},
						},
						{
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"extensions.gardener.cloud"},
								APIVersions: []string{"v1alpha1"},
								Resources: []string{
									"backupbuckets",
									"backupentries",
									"bastions",
									"containerruntimes",
									"controlplanes",
									"dnsrecords",
									"extensions",
									"infrastructures",
									"networks",
									"operatingsystemconfigs",
									"workers",
								},
							},
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Delete},
						},
					},
					FailurePolicy:     &failurePolicyFail,
					NamespaceSelector: &metav1.LabelSelector{},
					ClientConfig: admissionregistrationv1.WebhookClientConfig{
						Service: &admissionregistrationv1.ServiceReference{
							Name:      "gardener-resource-manager",
							Namespace: deployNamespace,
							Path:      ptr.To("/webhooks/validate-crd-deletion"),
						},
					},
					AdmissionReviewVersions: []string{"v1beta1", "v1"},
					MatchPolicy:             &matchPolicyExact,
					SideEffects:             &sideEffect,
					TimeoutSeconds:          ptr.To[int32](10),
				},
				{
					Name: "validation.extensions.backupbuckets.resources.gardener.cloud",
					Rules: []admissionregistrationv1.RuleWithOperations{
						{
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"extensions.gardener.cloud"},
								APIVersions: []string{"v1alpha1"},
								Resources:   []string{"backupbuckets"},
							},
							Operations: []admissionregistrationv1.OperationType{"CREATE", "UPDATE"},
						},
					},
					FailurePolicy:     &failurePolicyFail,
					NamespaceSelector: &metav1.LabelSelector{},
					ClientConfig: admissionregistrationv1.WebhookClientConfig{
						Service: &admissionregistrationv1.ServiceReference{
							Name:      "gardener-resource-manager",
							Namespace: deployNamespace,
							Path:      ptr.To("/validate-extensions-gardener-cloud-v1alpha1-backupbucket"),
						},
					},
					AdmissionReviewVersions: []string{"v1beta1", "v1"},
					MatchPolicy:             &matchPolicyExact,
					SideEffects:             &sideEffect,
					TimeoutSeconds:          ptr.To[int32](10),
				},
				{
					Name: "validation.extensions.backupentries.resources.gardener.cloud",
					Rules: []admissionregistrationv1.RuleWithOperations{
						{
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"extensions.gardener.cloud"},
								APIVersions: []string{"v1alpha1"},
								Resources:   []string{"backupentries"},
							},
							Operations: []admissionregistrationv1.OperationType{"CREATE", "UPDATE"},
						},
					},
					FailurePolicy:     &failurePolicyFail,
					NamespaceSelector: &metav1.LabelSelector{},
					ClientConfig: admissionregistrationv1.WebhookClientConfig{
						Service: &admissionregistrationv1.ServiceReference{
							Name:      "gardener-resource-manager",
							Namespace: deployNamespace,
							Path:      ptr.To("/validate-extensions-gardener-cloud-v1alpha1-backupentry"),
						},
					},
					AdmissionReviewVersions: []string{"v1beta1", "v1"},
					MatchPolicy:             &matchPolicyExact,
					SideEffects:             &sideEffect,
					TimeoutSeconds:          ptr.To[int32](10),
				},
				{
					Name: "validation.extensions.bastions.resources.gardener.cloud",
					Rules: []admissionregistrationv1.RuleWithOperations{
						{
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"extensions.gardener.cloud"},
								APIVersions: []string{"v1alpha1"},
								Resources:   []string{"bastions"},
							},
							Operations: []admissionregistrationv1.OperationType{"CREATE", "UPDATE"},
						},
					},
					FailurePolicy:     &failurePolicyFail,
					NamespaceSelector: &metav1.LabelSelector{},
					ClientConfig: admissionregistrationv1.WebhookClientConfig{
						Service: &admissionregistrationv1.ServiceReference{
							Name:      "gardener-resource-manager",
							Namespace: deployNamespace,
							Path:      ptr.To("/validate-extensions-gardener-cloud-v1alpha1-bastion"),
						},
					},
					AdmissionReviewVersions: []string{"v1beta1", "v1"},
					MatchPolicy:             &matchPolicyExact,
					SideEffects:             &sideEffect,
					TimeoutSeconds:          ptr.To[int32](10),
				},
				{
					Name: "validation.extensions.containerruntimes.resources.gardener.cloud",
					Rules: []admissionregistrationv1.RuleWithOperations{
						{
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"extensions.gardener.cloud"},
								APIVersions: []string{"v1alpha1"},
								Resources:   []string{"containerruntimes"},
							},
							Operations: []admissionregistrationv1.OperationType{"CREATE", "UPDATE"},
						},
					},
					FailurePolicy:     &failurePolicyFail,
					NamespaceSelector: &metav1.LabelSelector{},
					ClientConfig: admissionregistrationv1.WebhookClientConfig{
						Service: &admissionregistrationv1.ServiceReference{
							Name:      "gardener-resource-manager",
							Namespace: deployNamespace,
							Path:      ptr.To("/validate-extensions-gardener-cloud-v1alpha1-containerruntime"),
						},
					},
					AdmissionReviewVersions: []string{"v1beta1", "v1"},
					MatchPolicy:             &matchPolicyExact,
					SideEffects:             &sideEffect,
					TimeoutSeconds:          ptr.To[int32](10),
				},
				{
					Name: "validation.extensions.controlplanes.resources.gardener.cloud",
					Rules: []admissionregistrationv1.RuleWithOperations{
						{
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"extensions.gardener.cloud"},
								APIVersions: []string{"v1alpha1"},
								Resources:   []string{"controlplanes"},
							},
							Operations: []admissionregistrationv1.OperationType{"CREATE", "UPDATE"},
						},
					},
					FailurePolicy:     &failurePolicyFail,
					NamespaceSelector: &metav1.LabelSelector{},
					ClientConfig: admissionregistrationv1.WebhookClientConfig{
						Service: &admissionregistrationv1.ServiceReference{
							Name:      "gardener-resource-manager",
							Namespace: deployNamespace,
							Path:      ptr.To("/validate-extensions-gardener-cloud-v1alpha1-controlplane"),
						},
					},
					AdmissionReviewVersions: []string{"v1beta1", "v1"},
					MatchPolicy:             &matchPolicyExact,
					SideEffects:             &sideEffect,
					TimeoutSeconds:          ptr.To[int32](10),
				},
				{
					Name: "validation.extensions.dnsrecords.resources.gardener.cloud",
					Rules: []admissionregistrationv1.RuleWithOperations{
						{
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"extensions.gardener.cloud"},
								APIVersions: []string{"v1alpha1"},
								Resources:   []string{"dnsrecords"},
							},
							Operations: []admissionregistrationv1.OperationType{"CREATE", "UPDATE"},
						},
					},
					FailurePolicy:     &failurePolicyFail,
					NamespaceSelector: &metav1.LabelSelector{},
					ClientConfig: admissionregistrationv1.WebhookClientConfig{
						Service: &admissionregistrationv1.ServiceReference{
							Name:      "gardener-resource-manager",
							Namespace: deployNamespace,
							Path:      ptr.To("/validate-extensions-gardener-cloud-v1alpha1-dnsrecord"),
						},
					},
					AdmissionReviewVersions: []string{"v1beta1", "v1"},
					MatchPolicy:             &matchPolicyExact,
					SideEffects:             &sideEffect,
					TimeoutSeconds:          ptr.To[int32](10),
				},
				{
					Name: "validation.extensions.etcds.resources.gardener.cloud",
					Rules: []admissionregistrationv1.RuleWithOperations{
						{
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"druid.gardener.cloud"},
								APIVersions: []string{"v1alpha1"},
								Resources:   []string{"etcds"},
							},
							Operations: []admissionregistrationv1.OperationType{"CREATE", "UPDATE"},
						},
					},
					FailurePolicy:     &failurePolicyFail,
					NamespaceSelector: &metav1.LabelSelector{},
					ClientConfig: admissionregistrationv1.WebhookClientConfig{
						Service: &admissionregistrationv1.ServiceReference{
							Name:      "gardener-resource-manager",
							Namespace: deployNamespace,
							Path:      ptr.To("/validate-druid-gardener-cloud-v1alpha1-etcd"),
						},
					},
					AdmissionReviewVersions: []string{"v1beta1", "v1"},
					MatchPolicy:             &matchPolicyExact,
					SideEffects:             &sideEffect,
					TimeoutSeconds:          ptr.To[int32](10),
				},
				{
					Name: "validation.extensions.extensions.resources.gardener.cloud",
					Rules: []admissionregistrationv1.RuleWithOperations{
						{
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"extensions.gardener.cloud"},
								APIVersions: []string{"v1alpha1"},
								Resources:   []string{"extensions"},
							},
							Operations: []admissionregistrationv1.OperationType{"CREATE", "UPDATE"},
						},
					},
					FailurePolicy:     &failurePolicyFail,
					NamespaceSelector: &metav1.LabelSelector{},
					ClientConfig: admissionregistrationv1.WebhookClientConfig{
						Service: &admissionregistrationv1.ServiceReference{
							Name:      "gardener-resource-manager",
							Namespace: deployNamespace,
							Path:      ptr.To("/validate-extensions-gardener-cloud-v1alpha1-extension"),
						},
					},
					AdmissionReviewVersions: []string{"v1beta1", "v1"},
					MatchPolicy:             &matchPolicyExact,
					SideEffects:             &sideEffect,
					TimeoutSeconds:          ptr.To[int32](10),
				},
				{
					Name: "validation.extensions.infrastructures.resources.gardener.cloud",
					Rules: []admissionregistrationv1.RuleWithOperations{
						{
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"extensions.gardener.cloud"},
								APIVersions: []string{"v1alpha1"},
								Resources:   []string{"infrastructures"},
							},
							Operations: []admissionregistrationv1.OperationType{"CREATE", "UPDATE"},
						},
					},
					FailurePolicy:     &failurePolicyFail,
					NamespaceSelector: &metav1.LabelSelector{},
					ClientConfig: admissionregistrationv1.WebhookClientConfig{
						Service: &admissionregistrationv1.ServiceReference{
							Name:      "gardener-resource-manager",
							Namespace: deployNamespace,
							Path:      ptr.To("/validate-extensions-gardener-cloud-v1alpha1-infrastructure"),
						},
					},
					AdmissionReviewVersions: []string{"v1beta1", "v1"},
					MatchPolicy:             &matchPolicyExact,
					SideEffects:             &sideEffect,
					TimeoutSeconds:          ptr.To[int32](10),
				},
				{
					Name: "validation.extensions.networks.resources.gardener.cloud",
					Rules: []admissionregistrationv1.RuleWithOperations{
						{
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"extensions.gardener.cloud"},
								APIVersions: []string{"v1alpha1"},
								Resources:   []string{"networks"},
							},
							Operations: []admissionregistrationv1.OperationType{"CREATE", "UPDATE"},
						},
					},
					FailurePolicy:     &failurePolicyFail,
					NamespaceSelector: &metav1.LabelSelector{},
					ClientConfig: admissionregistrationv1.WebhookClientConfig{
						Service: &admissionregistrationv1.ServiceReference{
							Name:      "gardener-resource-manager",
							Namespace: deployNamespace,
							Path:      ptr.To("/validate-extensions-gardener-cloud-v1alpha1-network"),
						},
					},
					AdmissionReviewVersions: []string{"v1beta1", "v1"},
					MatchPolicy:             &matchPolicyExact,
					SideEffects:             &sideEffect,
					TimeoutSeconds:          ptr.To[int32](10),
				},
				{
					Name: "validation.extensions.operatingsystemconfigs.resources.gardener.cloud",
					Rules: []admissionregistrationv1.RuleWithOperations{
						{
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"extensions.gardener.cloud"},
								APIVersions: []string{"v1alpha1"},
								Resources:   []string{"operatingsystemconfigs"},
							},
							Operations: []admissionregistrationv1.OperationType{"CREATE", "UPDATE"},
						},
					},
					FailurePolicy:     &failurePolicyFail,
					NamespaceSelector: &metav1.LabelSelector{},
					ClientConfig: admissionregistrationv1.WebhookClientConfig{
						Service: &admissionregistrationv1.ServiceReference{
							Name:      "gardener-resource-manager",
							Namespace: deployNamespace,
							Path:      ptr.To("/validate-extensions-gardener-cloud-v1alpha1-operatingsystemconfig"),
						},
					},
					AdmissionReviewVersions: []string{"v1beta1", "v1"},
					MatchPolicy:             &matchPolicyExact,
					SideEffects:             &sideEffect,
					TimeoutSeconds:          ptr.To[int32](10),
				},
				{
					Name: "validation.extensions.workers.resources.gardener.cloud",
					Rules: []admissionregistrationv1.RuleWithOperations{
						{
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"extensions.gardener.cloud"},
								APIVersions: []string{"v1alpha1"},
								Resources:   []string{"workers"},
							},
							Operations: []admissionregistrationv1.OperationType{"CREATE", "UPDATE"},
						},
					},
					FailurePolicy:     &failurePolicyFail,
					NamespaceSelector: &metav1.LabelSelector{},
					ClientConfig: admissionregistrationv1.WebhookClientConfig{
						Service: &admissionregistrationv1.ServiceReference{
							Name:      "gardener-resource-manager",
							Namespace: deployNamespace,
							Path:      ptr.To("/validate-extensions-gardener-cloud-v1alpha1-worker"),
						},
					},
					AdmissionReviewVersions: []string{"v1beta1", "v1"},
					MatchPolicy:             &matchPolicyExact,
					SideEffects:             &sideEffect,
					TimeoutSeconds:          ptr.To[int32](10),
				},
			},
		}

		compressedData, err := test.BrotliCompressionForManifests(mutatingWebhookConfigurationYAML(), clusterRoleBindingTargetYAML)
		Expect(err).NotTo(HaveOccurred())

		managedResourceSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "managedresource-shoot-core-gardener-resource-manager",
				Namespace: deployNamespace,
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"data.yaml.br": compressedData,
			},
		}
		utilruntime.Must(kubernetesutils.MakeUnique(managedResourceSecret))

		managedResource = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shoot-core-gardener-resource-manager",
				Namespace: deployNamespace,
				Labels: map[string]string{
					"origin": "gardener",
					"foo":    "bar",
				},
			},
			Spec: resourcesv1alpha1.ManagedResourceSpec{
				SecretRefs: []corev1.LocalObjectReference{
					{Name: managedResourceSecret.Name},
				},
				InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
				KeepObjects:  ptr.To(false),
			},
		}
		utilruntime.Must(references.InjectAnnotations(managedResource))
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Deploy", func() {
		Context("target cluster != source cluster; watched namespace is set", func() {
			JustBeforeEach(func() {
				role.Namespace = watchedNamespace
				configMap = configMapFor(&watchedNamespace, ForTarget, false, false)
				deployment = deploymentFor(configMap.Name, true, nil, false)
				cfg.TargetNamespaces = targetNamespaces
				resourceManager = New(c, deployNamespace, sm, cfg)
				resourceManager.SetSecrets(secrets)
			})

			Context("should successfully deploy all resources (w/ shoot access secret)", func() {
				BeforeEach(func() {
					gomock.InOrder(
						c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: secret.Name}, gomock.AssignableToTypeOf(&corev1.Secret{})).
							Do(func(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) {
								obj.SetResourceVersion("0")
							}),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()).
							Do(func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
								Expect(obj).To(DeepEqual(secret))
							}),
						c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "gardener-resource-manager"}, gomock.AssignableToTypeOf(&corev1.ServiceAccount{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.ServiceAccount{}), gomock.Any()).
							Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
								Expect(obj).To(DeepEqual(serviceAccount))
							}),
						c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.ConfigMap{})).
							Do(func(_ context.Context, obj *corev1.ConfigMap, _ ...client.CreateOption) {
								Expect(obj).To(DeepEqual(configMap))
							}),
						c.EXPECT().Get(ctx, client.ObjectKey{Namespace: watchedNamespace, Name: "gardener-resource-manager"}, gomock.AssignableToTypeOf(&rbacv1.Role{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&rbacv1.Role{}), gomock.Any()).
							Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
								Expect(obj).To(DeepEqual(role))
							}),
						c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "gardener-resource-manager"}, gomock.AssignableToTypeOf(&rbacv1.RoleBinding{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&rbacv1.RoleBinding{}), gomock.Any()).
							Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
								Expect(obj).To(DeepEqual(roleBinding))
							}),
						c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "gardener-resource-manager"}, gomock.AssignableToTypeOf(&corev1.Service{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Service{}), gomock.Any()).
							Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
								Expect(obj).To(DeepEqual(service))
							}),
						c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "gardener-resource-manager"}, gomock.AssignableToTypeOf(&appsv1.Deployment{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{}), gomock.Any()).
							Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
								Expect(obj).To(DeepEqual(deployment))
							}),
						c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: pdb.Name}, gomock.AssignableToTypeOf(&policyv1.PodDisruptionBudget{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&policyv1.PodDisruptionBudget{}), gomock.Any()).
							Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
								Expect(obj).To(DeepEqual(pdb))
							}),
						c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "gardener-resource-manager-vpa"}, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{}), gomock.Any()).
							Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
								Expect(obj).To(DeepEqual(vpa))
							}),
						c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "shoot-gardener-resource-manager"}, gomock.AssignableToTypeOf(&monitoringv1.ServiceMonitor{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&monitoringv1.ServiceMonitor{}), gomock.Any()).
							Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
								Expect(obj).To(DeepEqual(serviceMonitorFor("shoot")))
							}),
					)
				})

				Context("MatchLabelKeysInPodTopologySpread feature gate is disabled (PodTopologySpreadConstraints webhook should be enabled)", func() {
					JustBeforeEach(func() {
						matchLabelKeysInPodTopologySpreadFeatureGateDisabled = true
						cfg.PodTopologySpreadConstraintsEnabled = true
						configMap = configMapFor(&watchedNamespace, ForTarget, false, false)
						deployment = deploymentFor(configMap.Name, true, nil, false)

						resourceManager = New(c, deployNamespace, sm, cfg)
						resourceManager.SetSecrets(secrets)

						compressedData, err := test.BrotliCompressionForManifests(mutatingWebhookConfigurationYAML(), clusterRoleBindingTargetYAML)
						Expect(err).NotTo(HaveOccurred())

						managedResourceSecret = &corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "managedresource-shoot-core-gardener-resource-manager",
								Namespace: deployNamespace,
							},
							Type: corev1.SecretTypeOpaque,
							Data: map[string][]byte{
								"data.yaml.br": compressedData,
							},
						}
						utilruntime.Must(kubernetesutils.MakeUnique(managedResourceSecret))

						managedResource.Spec.SecretRefs = []corev1.LocalObjectReference{
							{Name: managedResourceSecret.Name},
						}

						utilruntime.Must(references.InjectAnnotations(managedResource))
					})

					It("should successfully deploy all resources (w/ shoot access secret)", func() {
						gomock.InOrder(
							c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: managedResourceSecret.Name}, gomock.AssignableToTypeOf(&corev1.Secret{})),
							c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Do(func(_ context.Context, obj client.Object, _ ...client.UpdateOption) {
								Expect(obj).To(DeepEqual(managedResourceSecret))
							}),
							c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "shoot-core-gardener-resource-manager"}, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
							c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Do(func(_ context.Context, obj client.Object, _ ...client.UpdateOption) {
								Expect(obj).To(DeepEqual(managedResource))
							}),
						)

						Expect(resourceManager.Deploy(ctx)).To(Succeed())
					})
				})

				Context("MatchLabelKeysInPodTopologySpread feature gate is enabled (PodTopologySpreadConstraints webhook should be disabled)", func() {
					JustBeforeEach(func() {
						matchLabelKeysInPodTopologySpreadFeatureGateDisabled = false
						cfg.PodTopologySpreadConstraintsEnabled = false

						configMap = configMapFor(&watchedNamespace, ForTarget, false, false)
						deployment = deploymentFor(configMap.Name, true, nil, false)

						resourceManager = New(c, deployNamespace, sm, cfg)
						resourceManager.SetSecrets(secrets)

						compressedData, err := test.BrotliCompressionForManifests(mutatingWebhookConfigurationYAML(), clusterRoleBindingTargetYAML)
						Expect(err).NotTo(HaveOccurred())

						managedResourceSecret = &corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "managedresource-shoot-core-gardener-resource-manager",
								Namespace: deployNamespace,
							},
							Type: corev1.SecretTypeOpaque,
							Data: map[string][]byte{
								"data.yaml.br": compressedData,
							},
						}
						utilruntime.Must(kubernetesutils.MakeUnique(managedResourceSecret))

						managedResource.Spec.SecretRefs = []corev1.LocalObjectReference{
							{Name: managedResourceSecret.Name},
						}

						utilruntime.Must(references.InjectAnnotations(managedResource))
					})

					It("should successfully deploy all resources (w/ shoot access secret)", func() {
						gomock.InOrder(
							c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: managedResourceSecret.Name}, gomock.AssignableToTypeOf(&corev1.Secret{})),
							c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Do(func(_ context.Context, obj client.Object, _ ...client.UpdateOption) {
								Expect(obj).To(DeepEqual(managedResourceSecret))
							}),
							c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "shoot-core-gardener-resource-manager"}, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
							c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Do(func(_ context.Context, obj client.Object, _ ...client.UpdateOption) {
								Expect(obj).To(DeepEqual(managedResource))
							}),
						)

						Expect(resourceManager.Deploy(ctx)).To(Succeed())
					})
				})
			})

			Context("should successfully deploy all resources (w/ bootstrap kubeconfig)", func() {
				It("should successfully deploy all resources (w/ bootstrap kubeconfig)", func() {
					secretNameBootstrapKubeconfig := "bootstrap-kubeconfig"

					secrets.BootstrapKubeconfig = &component.Secret{Name: secretNameBootstrapKubeconfig}

					resourceManager = New(c, deployNamespace, sm, cfg)
					resourceManager.SetSecrets(secrets)

					deployment = deploymentFor(configMap.Name, true, &secretNameBootstrapKubeconfig, false)

					gomock.InOrder(
						c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: secret.Name}, gomock.AssignableToTypeOf(&corev1.Secret{})).
							Do(func(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) {
								obj.SetResourceVersion("0")
							}),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()).
							Do(func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
								Expect(obj).To(DeepEqual(secret))
							}),
						c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "gardener-resource-manager"}, gomock.AssignableToTypeOf(&corev1.ServiceAccount{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.ServiceAccount{}), gomock.Any()).
							Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
								Expect(obj).To(DeepEqual(serviceAccount))
							}),
						c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.ConfigMap{})).
							Do(func(_ context.Context, obj *corev1.ConfigMap, _ ...client.CreateOption) {
								Expect(obj).To(DeepEqual(configMap))
							}),
						c.EXPECT().Get(ctx, client.ObjectKey{Namespace: watchedNamespace, Name: "gardener-resource-manager"}, gomock.AssignableToTypeOf(&rbacv1.Role{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&rbacv1.Role{}), gomock.Any()).
							Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
								Expect(obj).To(DeepEqual(role))
							}),
						c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "gardener-resource-manager"}, gomock.AssignableToTypeOf(&rbacv1.RoleBinding{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&rbacv1.RoleBinding{}), gomock.Any()).
							Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
								Expect(obj).To(DeepEqual(roleBinding))
							}),
						c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "gardener-resource-manager"}, gomock.AssignableToTypeOf(&corev1.Service{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Service{}), gomock.Any()).
							Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
								Expect(obj).To(DeepEqual(service))
							}),
						c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "gardener-resource-manager"}, gomock.AssignableToTypeOf(&appsv1.Deployment{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{}), gomock.Any()).
							Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
								Expect(obj).To(DeepEqual(deployment))
							}),
						c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: pdb.Name}, gomock.AssignableToTypeOf(&policyv1.PodDisruptionBudget{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&policyv1.PodDisruptionBudget{}), gomock.Any()).
							Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
								Expect(obj).To(DeepEqual(pdb))
							}),
						c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "gardener-resource-manager-vpa"}, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{}), gomock.Any()).
							Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
								Expect(obj).To(DeepEqual(vpa))
							}),
						c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "shoot-gardener-resource-manager"}, gomock.AssignableToTypeOf(&monitoringv1.ServiceMonitor{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&monitoringv1.ServiceMonitor{}), gomock.Any()).
							Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
								Expect(obj).To(DeepEqual(serviceMonitorFor("shoot")))
							}),
						c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: managedResourceSecret.Name}, gomock.AssignableToTypeOf(&corev1.Secret{})),
						c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Do(func(_ context.Context, obj client.Object, _ ...client.UpdateOption) {
							Expect(obj).To(DeepEqual(managedResourceSecret))
						}),
						c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "shoot-core-gardener-resource-manager"}, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
						c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Do(func(_ context.Context, obj client.Object, _ ...client.UpdateOption) {
							Expect(obj).To(DeepEqual(managedResource))
						}),
					)

					Expect(resourceManager.Deploy(ctx)).To(Succeed())
				})
			})
		})

		Context("target cluster != source cluster, watched namespace is nil", func() {
			BeforeEach(func() {
				clusterRole.Rules = allowManagedResources
				cfg.ResponsibilityMode = ForTarget
				cfg.TargetNamespaces = targetNamespaces
				cfg.WatchedNamespace = nil
				configMap = configMapFor(nil, ForTarget, false, false)
				deployment = deploymentFor(configMap.Name, true, nil, false)

				resourceManager = New(c, deployNamespace, sm, cfg)
				resourceManager.SetSecrets(secrets)
			})

			It("should deploy a ClusterRole allowing access to mr related resources", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: secret.Name}, gomock.AssignableToTypeOf(&corev1.Secret{})).
						Do(func(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) {
							obj.SetResourceVersion("0")
						}),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()).
						Do(func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(secret))
						}),
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "gardener-resource-manager"}, gomock.AssignableToTypeOf(&corev1.ServiceAccount{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.ServiceAccount{}), gomock.Any()).
						Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(serviceAccount))
						}),
					c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.ConfigMap{})).
						Do(func(_ context.Context, obj *corev1.ConfigMap, _ ...client.CreateOption) {
							Expect(obj).To(DeepEqual(configMap))
						}),
					c.EXPECT().Get(ctx, client.ObjectKey{Name: clusterRoleName}, gomock.AssignableToTypeOf(&rbacv1.ClusterRole{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRole{}), gomock.Any()).
						Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(clusterRole))
						}),
					c.EXPECT().Get(ctx, client.ObjectKey{Name: clusterRoleName}, gomock.AssignableToTypeOf(&rbacv1.ClusterRoleBinding{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRoleBinding{}), gomock.Any()).
						Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(clusterRoleBinding))
						}),
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "gardener-resource-manager"}, gomock.AssignableToTypeOf(&corev1.Service{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Service{}), gomock.Any()).
						Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(service))
						}),
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "gardener-resource-manager"}, gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{}), gomock.Any()).
						Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(deployment))
						}),
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: pdb.Name}, gomock.AssignableToTypeOf(&policyv1.PodDisruptionBudget{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&policyv1.PodDisruptionBudget{}), gomock.Any()).
						Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(pdb))
						}),
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "gardener-resource-manager-vpa"}, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{}), gomock.Any()).
						Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(vpa))
						}),
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "shoot-gardener-resource-manager"}, gomock.AssignableToTypeOf(&monitoringv1.ServiceMonitor{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&monitoringv1.ServiceMonitor{}), gomock.Any()).
						Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(serviceMonitorFor("shoot")))
						}),
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: managedResourceSecret.Name}, gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Do(func(_ context.Context, obj client.Object, _ ...client.UpdateOption) {
						Expect(obj).To(DeepEqual(managedResourceSecret))
					}),
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "shoot-core-gardener-resource-manager"}, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Do(func(_ context.Context, obj client.Object, _ ...client.UpdateOption) {
						Expect(obj).To(DeepEqual(managedResource))
					}),
				)
				Expect(resourceManager.Deploy(ctx)).To(Succeed())
			})

			It("should fail because the ClusterRole can not be created", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: secret.Name}, gomock.AssignableToTypeOf(&corev1.Secret{})).
						Do(func(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) {
							obj.SetResourceVersion("0")
						}),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()),
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "gardener-resource-manager"}, gomock.AssignableToTypeOf(&corev1.ServiceAccount{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.ServiceAccount{}), gomock.Any()),
					c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.ConfigMap{})),
					c.EXPECT().Get(ctx, client.ObjectKey{Name: clusterRoleName}, gomock.AssignableToTypeOf(&rbacv1.ClusterRole{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRole{}), gomock.Any()).Return(fakeErr),
				)

				Expect(resourceManager.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the ClusterRoleBinding can not be created", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: secret.Name}, gomock.AssignableToTypeOf(&corev1.Secret{})).
						Do(func(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) {
							obj.SetResourceVersion("0")
						}),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()),
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "gardener-resource-manager"}, gomock.AssignableToTypeOf(&corev1.ServiceAccount{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.ServiceAccount{}), gomock.Any()),
					c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.ConfigMap{})),
					c.EXPECT().Get(ctx, client.ObjectKey{Name: clusterRoleName}, gomock.AssignableToTypeOf(&rbacv1.ClusterRole{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRole{}), gomock.Any()),
					c.EXPECT().Get(ctx, client.ObjectKey{Name: clusterRoleName}, gomock.AssignableToTypeOf(&rbacv1.ClusterRoleBinding{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRoleBinding{}), gomock.Any()).Return(fakeErr),
				)

				Expect(resourceManager.Deploy(ctx)).To(MatchError(fakeErr))
			})
		})

		Context("target cluster != source cluster, workerless shoot", func() {
			JustBeforeEach(func() {
				clusterRole.Rules = allowManagedResources
				cfg.ResponsibilityMode = ForTarget
				cfg.TargetNamespaces = targetNamespaces
				cfg.WatchedNamespace = nil
				cfg.IsWorkerless = true
				configMap = configMapFor(nil, ForTarget, true, false)
				deployment = deploymentFor(configMap.Name, true, nil, false)

				resourceManager = New(c, deployNamespace, sm, cfg)
				resourceManager.SetSecrets(secrets)
			})

			It("should disable controllers and webhooks properly in resource manager configuration", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: secret.Name}, gomock.AssignableToTypeOf(&corev1.Secret{})).
						Do(func(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) {
							obj.SetResourceVersion("0")
						}),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()).
						Do(func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(secret))
						}),
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "gardener-resource-manager"}, gomock.AssignableToTypeOf(&corev1.ServiceAccount{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.ServiceAccount{}), gomock.Any()).
						Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(serviceAccount))
						}),
					c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.ConfigMap{})).
						Do(func(_ context.Context, obj *corev1.ConfigMap, _ ...client.CreateOption) {
							Expect(obj).To(DeepEqual(configMap))
						}),
					c.EXPECT().Get(ctx, client.ObjectKey{Name: clusterRoleName}, gomock.AssignableToTypeOf(&rbacv1.ClusterRole{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRole{}), gomock.Any()).
						Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(clusterRole))
						}),
					c.EXPECT().Get(ctx, client.ObjectKey{Name: clusterRoleName}, gomock.AssignableToTypeOf(&rbacv1.ClusterRoleBinding{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRoleBinding{}), gomock.Any()).
						Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(clusterRoleBinding))
						}),
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "gardener-resource-manager"}, gomock.AssignableToTypeOf(&corev1.Service{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Service{}), gomock.Any()).
						Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(service))
						}),
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "gardener-resource-manager"}, gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{}), gomock.Any()).
						Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(deployment))
						}),
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: pdb.Name}, gomock.AssignableToTypeOf(&policyv1.PodDisruptionBudget{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&policyv1.PodDisruptionBudget{}), gomock.Any()).
						Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(pdb))
						}),
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "gardener-resource-manager-vpa"}, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{}), gomock.Any()).
						Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(vpa))
						}),
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "shoot-gardener-resource-manager"}, gomock.AssignableToTypeOf(&monitoringv1.ServiceMonitor{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&monitoringv1.ServiceMonitor{}), gomock.Any()).
						Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(serviceMonitorFor("shoot")))
						}),
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: managedResourceSecret.Name}, gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Do(func(_ context.Context, obj client.Object, _ ...client.UpdateOption) {
						Expect(obj).To(DeepEqual(managedResourceSecret))
					}),
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "shoot-core-gardener-resource-manager"}, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Do(func(_ context.Context, obj client.Object, _ ...client.UpdateOption) {
						Expect(obj).To(Equal(managedResource))
					}),
				)
				Expect(resourceManager.Deploy(ctx)).To(Succeed())
			})
		})

		Context("target cluster = source cluster", func() {
			BeforeEach(func() {
				clusterRole.Rules = allowAll
				delete(service.Annotations, "networking.resources.gardener.cloud/from-all-scrape-targets-allowed-ports")
				delete(service.Annotations, "networking.resources.gardener.cloud/from-all-webhook-targets-allowed-ports")
				service.Annotations["networking.resources.gardener.cloud/from-all-seed-scrape-targets-allowed-ports"] = `[{"protocol":"TCP","port":8080}]`
				service.Annotations["networking.resources.gardener.cloud/from-world-to-ports"] = `[{"protocol":"TCP","port":10250}]`
				configMap = configMapFor(&watchedNamespace, ForSource, false, false)
				deployment = deploymentFor(configMap.Name, false, nil, false)

				deployment.Spec.Template.Spec.Volumes = deployment.Spec.Template.Spec.Volumes[:len(deployment.Spec.Template.Spec.Volumes)-2]
				deployment.Spec.Template.Spec.Containers[0].VolumeMounts = deployment.Spec.Template.Spec.Containers[0].VolumeMounts[:len(deployment.Spec.Template.Spec.Containers[0].VolumeMounts)-2]
				deployment.Spec.Template.Labels["gardener.cloud/role"] = "seed"
				pdb.Spec.Selector.MatchLabels["gardener.cloud/role"] = "seed"
				for i := range deployment.Spec.Template.Spec.TopologySpreadConstraints {
					deployment.Spec.Template.Spec.TopologySpreadConstraints[i].LabelSelector.MatchLabels["gardener.cloud/role"] = "seed"
				}

				// Remove controlplane label from resources
				delete(serviceAccount.Labels, v1beta1constants.GardenRole)
				delete(clusterRole.Labels, v1beta1constants.GardenRole)
				delete(clusterRoleBinding.Labels, v1beta1constants.GardenRole)
				delete(service.Labels, v1beta1constants.GardenRole)
				delete(deployment.Labels, v1beta1constants.GardenRole)
				delete(vpa.Labels, v1beta1constants.GardenRole)
				delete(pdb.Labels, v1beta1constants.GardenRole)
				// Remove networking label from deployment template
				delete(deployment.Spec.Template.Labels, "networking.resources.gardener.cloud/to-kube-apiserver-tcp-443")

				utilruntime.Must(references.InjectAnnotations(deployment))

				cfg.DefaultSeccompProfileEnabled = true
				cfg.EndpointSliceHintsEnabled = true
				cfg.SchedulingProfile = nil
				cfg.ResponsibilityMode = ForSource
				resourceManager = New(c, deployNamespace, sm, cfg)
				resourceManager.SetSecrets(secrets)
			})

			It("should deploy a cluster role allowing all access", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, client.ObjectKey{Name: "managedresources.resources.gardener.cloud"}, gomock.AssignableToTypeOf(&apiextensionsv1.CustomResourceDefinition{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&apiextensionsv1.CustomResourceDefinition{}), gomock.Any()),
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "gardener-resource-manager"}, gomock.AssignableToTypeOf(&corev1.ServiceAccount{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.ServiceAccount{}), gomock.Any()).
						Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(serviceAccount))
						}),
					c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.ConfigMap{})).
						Do(func(_ context.Context, obj *corev1.ConfigMap, _ ...client.CreateOption) {
							Expect(obj).To(DeepEqual(configMap))
						}),
					c.EXPECT().Get(ctx, client.ObjectKey{Name: clusterRoleName}, gomock.AssignableToTypeOf(&rbacv1.ClusterRole{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRole{}), gomock.Any()).
						Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(clusterRole))
						}),
					c.EXPECT().Get(ctx, client.ObjectKey{Name: clusterRoleName}, gomock.AssignableToTypeOf(&rbacv1.ClusterRoleBinding{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRoleBinding{}), gomock.Any()).
						Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(clusterRoleBinding))
						}),
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "gardener-resource-manager"}, gomock.AssignableToTypeOf(&corev1.Service{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Service{}), gomock.Any()).
						Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(service))
						}),
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "gardener-resource-manager"}, gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{}), gomock.Any()).
						Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(deployment))
						}),
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: pdb.Name}, gomock.AssignableToTypeOf(&policyv1.PodDisruptionBudget{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&policyv1.PodDisruptionBudget{}), gomock.Any()).
						Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(pdb))
						}),
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "gardener-resource-manager-vpa"}, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{}), gomock.Any()).
						Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(vpa))
						}),
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "seed-gardener-resource-manager"}, gomock.AssignableToTypeOf(&monitoringv1.ServiceMonitor{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&monitoringv1.ServiceMonitor{}), gomock.Any()).
						Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(serviceMonitorFor("seed")))
						}),
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "gardener-resource-manager"}, gomock.AssignableToTypeOf(&admissionregistrationv1.MutatingWebhookConfiguration{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&admissionregistrationv1.MutatingWebhookConfiguration{}), gomock.Any()).
						Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(mutatingWebhookConfigurationFor(ForSource, false)))
						}),
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "gardener-resource-manager"}, gomock.AssignableToTypeOf(&admissionregistrationv1.ValidatingWebhookConfiguration{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&admissionregistrationv1.ValidatingWebhookConfiguration{}), gomock.Any()).
						Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(validatingWebhookConfiguration))
						}),
				)
				Expect(resourceManager.Deploy(ctx)).To(Succeed())
			})
		})

		Context("responsibility mode 'source and target'", func() {
			BeforeEach(func() {
				clusterRole.Rules = allowAll
				delete(service.Annotations, "networking.resources.gardener.cloud/from-all-scrape-targets-allowed-ports")
				delete(service.Annotations, "networking.resources.gardener.cloud/from-all-webhook-targets-allowed-ports")
				service.Annotations["networking.resources.gardener.cloud/from-all-seed-scrape-targets-allowed-ports"] = `[{"protocol":"TCP","port":8080}]`
				service.Annotations["networking.resources.gardener.cloud/from-world-to-ports"] = `[{"protocol":"TCP","port":10250}]`
				configMap = configMapFor(&watchedNamespace, ForSourceAndTarget, false, false)
				deployment = deploymentFor(configMap.Name, false, nil, false)

				deployment.Spec.Template.Spec.Volumes = deployment.Spec.Template.Spec.Volumes[:len(deployment.Spec.Template.Spec.Volumes)-2]
				deployment.Spec.Template.Spec.Containers[0].VolumeMounts = deployment.Spec.Template.Spec.Containers[0].VolumeMounts[:len(deployment.Spec.Template.Spec.Containers[0].VolumeMounts)-2]
				deployment.Spec.Template.Labels["gardener.cloud/role"] = "seed"
				pdb.Spec.Selector.MatchLabels["gardener.cloud/role"] = "seed"
				for i := range deployment.Spec.Template.Spec.TopologySpreadConstraints {
					deployment.Spec.Template.Spec.TopologySpreadConstraints[i].LabelSelector.MatchLabels["gardener.cloud/role"] = "seed"
				}

				// Remove controlplane label from resources
				delete(serviceAccount.Labels, v1beta1constants.GardenRole)
				delete(clusterRole.Labels, v1beta1constants.GardenRole)
				delete(clusterRoleBinding.Labels, v1beta1constants.GardenRole)
				delete(service.Labels, v1beta1constants.GardenRole)
				delete(deployment.Labels, v1beta1constants.GardenRole)
				delete(vpa.Labels, v1beta1constants.GardenRole)
				delete(pdb.Labels, v1beta1constants.GardenRole)
				// Remove networking label from deployment template
				delete(deployment.Spec.Template.Labels, "networking.resources.gardener.cloud/to-kube-apiserver-tcp-443")

				utilruntime.Must(references.InjectAnnotations(deployment))

				cfg.DefaultSeccompProfileEnabled = true
				cfg.SchedulingProfile = nil
				cfg.ResponsibilityMode = ForSourceAndTarget
				resourceManager = New(c, deployNamespace, sm, cfg)
				resourceManager.SetSecrets(secrets)
			})

			It("should deploy the expected resources", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, client.ObjectKey{Name: "managedresources.resources.gardener.cloud"}, gomock.AssignableToTypeOf(&apiextensionsv1.CustomResourceDefinition{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&apiextensionsv1.CustomResourceDefinition{}), gomock.Any()),
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "gardener-resource-manager"}, gomock.AssignableToTypeOf(&corev1.ServiceAccount{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.ServiceAccount{}), gomock.Any()).
						Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(serviceAccount))
						}),
					c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.ConfigMap{})).
						Do(func(_ context.Context, obj *corev1.ConfigMap, _ ...client.CreateOption) {
							Expect(obj).To(DeepEqual(configMap))
						}),
					c.EXPECT().Get(ctx, client.ObjectKey{Name: clusterRoleName}, gomock.AssignableToTypeOf(&rbacv1.ClusterRole{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRole{}), gomock.Any()).
						Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(clusterRole))
						}),
					c.EXPECT().Get(ctx, client.ObjectKey{Name: clusterRoleName}, gomock.AssignableToTypeOf(&rbacv1.ClusterRoleBinding{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRoleBinding{}), gomock.Any()).
						Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(clusterRoleBinding))
						}),
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "gardener-resource-manager"}, gomock.AssignableToTypeOf(&corev1.Service{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Service{}), gomock.Any()).
						Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(service))
						}),
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "gardener-resource-manager"}, gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{}), gomock.Any()).
						Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(deployment))
						}),
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: pdb.Name}, gomock.AssignableToTypeOf(&policyv1.PodDisruptionBudget{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&policyv1.PodDisruptionBudget{}), gomock.Any()).
						Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(pdb))
						}),
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "gardener-resource-manager-vpa"}, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{}), gomock.Any()).
						Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(vpa))
						}),
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "seed-gardener-resource-manager"}, gomock.AssignableToTypeOf(&monitoringv1.ServiceMonitor{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&monitoringv1.ServiceMonitor{}), gomock.Any()).
						Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(serviceMonitorFor("seed")))
						}),
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "gardener-resource-manager"}, gomock.AssignableToTypeOf(&admissionregistrationv1.MutatingWebhookConfiguration{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&admissionregistrationv1.MutatingWebhookConfiguration{}), gomock.Any()).
						Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(mutatingWebhookConfigurationFor(ForSourceAndTarget, false)))
						}),
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "gardener-resource-manager"}, gomock.AssignableToTypeOf(&admissionregistrationv1.ValidatingWebhookConfiguration{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&admissionregistrationv1.ValidatingWebhookConfiguration{}), gomock.Any()).
						Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(validatingWebhookConfiguration))
						}),
				)
				Expect(resourceManager.Deploy(ctx)).To(Succeed())
			})

			When("bootstrap control plane node is set to true", func() {
				BeforeEach(func() {
					configMap = configMapFor(&watchedNamespace, ForSourceAndTarget, false, true)
					cfg.BootstrapControlPlaneNode = true
					resourceManager = New(c, deployNamespace, sm, cfg)
					resourceManager.SetSecrets(secrets)

					deployment = deploymentFor(configMap.Name, false, nil, true)

					deployment.Spec.Template.Labels["gardener.cloud/role"] = "seed"
					for i := range deployment.Spec.Template.Spec.TopologySpreadConstraints {
						deployment.Spec.Template.Spec.TopologySpreadConstraints[i].LabelSelector.MatchLabels["gardener.cloud/role"] = "seed"
					}
					delete(deployment.Spec.Template.Labels, "networking.resources.gardener.cloud/to-kube-apiserver-tcp-443")
				})

				It("should deploy the expected resources", func() {
					gomock.InOrder(
						c.EXPECT().Get(ctx, client.ObjectKey{Name: "managedresources.resources.gardener.cloud"}, gomock.AssignableToTypeOf(&apiextensionsv1.CustomResourceDefinition{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&apiextensionsv1.CustomResourceDefinition{}), gomock.Any()),
						c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "gardener-resource-manager"}, gomock.AssignableToTypeOf(&corev1.ServiceAccount{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.ServiceAccount{}), gomock.Any()).
							Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
								Expect(obj).To(DeepEqual(serviceAccount))
							}),
						c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.ConfigMap{})).
							Do(func(_ context.Context, obj *corev1.ConfigMap, _ ...client.CreateOption) {
								Expect(obj).To(DeepEqual(configMap))
							}),
						c.EXPECT().Get(ctx, client.ObjectKey{Name: clusterRoleName}, gomock.AssignableToTypeOf(&rbacv1.ClusterRole{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRole{}), gomock.Any()).
							Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
								Expect(obj).To(DeepEqual(clusterRole))
							}),
						c.EXPECT().Get(ctx, client.ObjectKey{Name: clusterRoleName}, gomock.AssignableToTypeOf(&rbacv1.ClusterRoleBinding{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRoleBinding{}), gomock.Any()).
							Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
								Expect(obj).To(DeepEqual(clusterRoleBinding))
							}),
						c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "gardener-resource-manager"}, gomock.AssignableToTypeOf(&corev1.Service{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Service{}), gomock.Any()).
							Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
								Expect(obj).To(DeepEqual(service))
							}),
						c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "gardener-resource-manager"}, gomock.AssignableToTypeOf(&appsv1.Deployment{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{}), gomock.Any()).
							Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
								Expect(obj).To(DeepEqual(deployment))
							}),
						c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: pdb.Name}, gomock.AssignableToTypeOf(&policyv1.PodDisruptionBudget{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&policyv1.PodDisruptionBudget{}), gomock.Any()).
							Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
								Expect(obj).To(DeepEqual(pdb))
							}),
						c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "gardener-resource-manager-vpa"}, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{}), gomock.Any()).
							Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
								Expect(obj).To(DeepEqual(vpa))
							}),
						c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "seed-gardener-resource-manager"}, gomock.AssignableToTypeOf(&monitoringv1.ServiceMonitor{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&monitoringv1.ServiceMonitor{}), gomock.Any()).
							Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
								Expect(obj).To(DeepEqual(serviceMonitorFor("seed")))
							}),
						c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "gardener-resource-manager"}, gomock.AssignableToTypeOf(&admissionregistrationv1.MutatingWebhookConfiguration{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&admissionregistrationv1.MutatingWebhookConfiguration{}), gomock.Any()).
							Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
								Expect(obj).To(DeepEqual(mutatingWebhookConfigurationFor(ForSourceAndTarget, true)))
							}),
						c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "gardener-resource-manager"}, gomock.AssignableToTypeOf(&admissionregistrationv1.ValidatingWebhookConfiguration{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&admissionregistrationv1.ValidatingWebhookConfiguration{}), gomock.Any()).
							Do(func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) {
								Expect(obj).To(DeepEqual(validatingWebhookConfiguration))
							}),
					)
					Expect(resourceManager.Deploy(ctx)).To(Succeed())
				})
			})
		})
	})

	Describe("#Destroy", func() {
		Context("target differs from source cluster", func() {
			JustBeforeEach(func() {
				resourceManager = New(c, deployNamespace, sm, cfg)
			})

			Context("should delete all created resources", func() {
				It("should delete all created resources", func() {
					gomock.InOrder(
						c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "shoot-core-gardener-resource-manager"}, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
						c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "managedresource-shoot-core-gardener-resource-manager"}}),
						c.EXPECT().Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "shoot-core-gardener-resource-manager"}}),
						c.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: deployNamespace, Name: "shoot-core-gardener-resource-manager"}, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),
						c.EXPECT().Delete(ctx, &policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
						c.EXPECT().Delete(ctx, &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager-vpa"}}),
						c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
						c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
						c.EXPECT().Delete(ctx, &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
						c.EXPECT().Delete(ctx, &monitoringv1.ServiceMonitor{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "shoot-gardener-resource-manager", Labels: map[string]string{"prometheus": "shoot"}}}),
						c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: secret.Name}}),
						c.EXPECT().Delete(ctx, &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
						c.EXPECT().Delete(ctx, &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					)

					Expect(resourceManager.Destroy(ctx)).To(Succeed())
				})
			})

			It("should fail because the managed resource cannot be deleted", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "shoot-core-gardener-resource-manager"}, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
					c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "managedresource-shoot-core-gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "shoot-core-gardener-resource-manager"}}).Return(fakeErr),
				)

				Expect(resourceManager.Destroy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the managed resource secret cannot be deleted", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "shoot-core-gardener-resource-manager"}, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
					c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "managedresource-shoot-core-gardener-resource-manager"}}).Return(fakeErr),
				)

				Expect(resourceManager.Destroy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the pdb cannot be deleted", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "shoot-core-gardener-resource-manager"}, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
					c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "managedresource-shoot-core-gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "shoot-core-gardener-resource-manager"}}),
					c.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: deployNamespace, Name: "shoot-core-gardener-resource-manager"}, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),
					c.EXPECT().Delete(ctx, &policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}).Return(fakeErr),
				)

				Expect(resourceManager.Destroy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the vpa cannot be deleted", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "shoot-core-gardener-resource-manager"}, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
					c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "managedresource-shoot-core-gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "shoot-core-gardener-resource-manager"}}),
					c.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: deployNamespace, Name: "shoot-core-gardener-resource-manager"}, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),
					c.EXPECT().Delete(ctx, &policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager-vpa"}}).Return(fakeErr),
				)

				Expect(resourceManager.Destroy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the deployment cannot be deleted", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "shoot-core-gardener-resource-manager"}, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
					c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "managedresource-shoot-core-gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "shoot-core-gardener-resource-manager"}}),
					c.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: deployNamespace, Name: "shoot-core-gardener-resource-manager"}, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),
					c.EXPECT().Delete(ctx, &policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager-vpa"}}),
					c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}).Return(fakeErr),
				)

				Expect(resourceManager.Destroy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the service cannot be deleted", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "shoot-core-gardener-resource-manager"}, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
					c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "managedresource-shoot-core-gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "shoot-core-gardener-resource-manager"}}),
					c.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: deployNamespace, Name: "shoot-core-gardener-resource-manager"}, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),
					c.EXPECT().Delete(ctx, &policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager-vpa"}}),
					c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}).Return(fakeErr),
				)

				Expect(resourceManager.Destroy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the service account cannot be deleted", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "shoot-core-gardener-resource-manager"}, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
					c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "managedresource-shoot-core-gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "shoot-core-gardener-resource-manager"}}),
					c.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: deployNamespace, Name: "shoot-core-gardener-resource-manager"}, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),
					c.EXPECT().Delete(ctx, &policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager-vpa"}}),
					c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}).Return(fakeErr),
				)

				Expect(resourceManager.Destroy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the service monitor cannot be deleted", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "shoot-core-gardener-resource-manager"}, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
					c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "managedresource-shoot-core-gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "shoot-core-gardener-resource-manager"}}),
					c.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: deployNamespace, Name: "shoot-core-gardener-resource-manager"}, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),
					c.EXPECT().Delete(ctx, &policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager-vpa"}}),
					c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &monitoringv1.ServiceMonitor{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "shoot-gardener-resource-manager", Labels: map[string]string{"prometheus": "shoot"}}}).Return(fakeErr),
				)

				Expect(resourceManager.Destroy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the secret cannot be deleted", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "shoot-core-gardener-resource-manager"}, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
					c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "managedresource-shoot-core-gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "shoot-core-gardener-resource-manager"}}),
					c.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: deployNamespace, Name: "shoot-core-gardener-resource-manager"}, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),
					c.EXPECT().Delete(ctx, &policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager-vpa"}}),
					c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &monitoringv1.ServiceMonitor{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "shoot-gardener-resource-manager", Labels: map[string]string{"prometheus": "shoot"}}}),
					c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: secret.Name}}).Return(fakeErr),
				)

				Expect(resourceManager.Destroy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the role cannot be deleted", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "shoot-core-gardener-resource-manager"}, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
					c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "managedresource-shoot-core-gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "shoot-core-gardener-resource-manager"}}),
					c.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: deployNamespace, Name: "shoot-core-gardener-resource-manager"}, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),
					c.EXPECT().Delete(ctx, &policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager-vpa"}}),
					c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &monitoringv1.ServiceMonitor{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "shoot-gardener-resource-manager", Labels: map[string]string{"prometheus": "shoot"}}}),
					c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: secret.Name}}),
					c.EXPECT().Delete(ctx, &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}).Return(fakeErr),
				)

				Expect(resourceManager.Destroy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the role binding cannot be deleted", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: deployNamespace, Name: "shoot-core-gardener-resource-manager"}, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
					c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "managedresource-shoot-core-gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "shoot-core-gardener-resource-manager"}}),
					c.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: deployNamespace, Name: "shoot-core-gardener-resource-manager"}, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),
					c.EXPECT().Delete(ctx, &policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager-vpa"}}),
					c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &monitoringv1.ServiceMonitor{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "shoot-gardener-resource-manager", Labels: map[string]string{"prometheus": "shoot"}}}),
					c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: secret.Name}}),
					c.EXPECT().Delete(ctx, &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}).Return(fakeErr),
				)

				Expect(resourceManager.Destroy(ctx)).To(MatchError(fakeErr))
			})
		})

		Context("target equals source cluster", func() {
			BeforeEach(func() {
				cfg.ResponsibilityMode = ForSource
				cfg.WatchedNamespace = nil
				resourceManager = New(c, deployNamespace, sm, cfg)
			})

			It("should delete all created resources", func() {
				gomock.InOrder(
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&apiextensionsv1.CustomResourceDefinition{}), gomock.Any()),
					c.EXPECT().Delete(ctx, &apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "managedresources.resources.gardener.cloud"}}),
					c.EXPECT().Delete(ctx, &admissionregistrationv1.MutatingWebhookConfiguration{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &admissionregistrationv1.ValidatingWebhookConfiguration{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: clusterRoleName}}),
					c.EXPECT().Delete(ctx, &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: clusterRoleName}}),
					c.EXPECT().Delete(ctx, &policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager-vpa"}}),
					c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &monitoringv1.ServiceMonitor{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "seed-gardener-resource-manager", Labels: map[string]string{"prometheus": "seed"}}}),
				)

				Expect(resourceManager.Destroy(ctx)).To(Succeed())
			})
		})

		Context("responsibility mode 'source and target'", func() {
			BeforeEach(func() {
				cfg.ResponsibilityMode = ForSourceAndTarget
				cfg.WatchedNamespace = nil
				resourceManager = New(c, deployNamespace, sm, cfg)
			})

			It("should delete all created resources", func() {
				gomock.InOrder(
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&apiextensionsv1.CustomResourceDefinition{}), gomock.Any()),
					c.EXPECT().Delete(ctx, &apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "managedresources.resources.gardener.cloud"}}),
					c.EXPECT().Delete(ctx, &admissionregistrationv1.MutatingWebhookConfiguration{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &admissionregistrationv1.ValidatingWebhookConfiguration{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: clusterRoleName}}),
					c.EXPECT().Delete(ctx, &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: clusterRoleName}}),
					c.EXPECT().Delete(ctx, &policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager-vpa"}}),
					c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "gardener-resource-manager"}}),
					c.EXPECT().Delete(ctx, &monitoringv1.ServiceMonitor{ObjectMeta: metav1.ObjectMeta{Namespace: deployNamespace, Name: "seed-gardener-resource-manager", Labels: map[string]string{"prometheus": "seed"}}}),
				)

				Expect(resourceManager.Destroy(ctx)).To(Succeed())
			})
		})
	})

	Describe("#Wait", func() {
		BeforeEach(func() {
			configMap = configMapFor(&watchedNamespace, ForTarget, false, false)
			deployment = deploymentFor(configMap.Name, false, nil, false)
			resourceManager = New(fakeClient, deployNamespace, nil, cfg)
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

	Describe("#SetReplicas, #GetReplicas", func() {
		It("should set and return the replicas", func() {
			resourceManager = New(nil, "", nil, Values{})
			Expect(resourceManager.GetReplicas()).To(BeZero())

			resourceManager.SetReplicas(&replicas)
			Expect(resourceManager.GetReplicas()).To(PointTo(Equal(replicas)))
		})
	})
})

var (
	scheme *runtime.Scheme
	codec  runtime.Codec
)

func init() {
	scheme = runtime.NewScheme()
	utilruntime.Must(resourcemanagerconfigv1alpha1.AddToScheme(scheme))

	var (
		ser = json.NewSerializerWithOptions(json.DefaultMetaFactory, scheme, scheme, json.SerializerOptions{
			Yaml:   true,
			Pretty: false,
			Strict: false,
		})
		versions = schema.GroupVersions([]schema.GroupVersion{
			resourcemanagerconfigv1alpha1.SchemeGroupVersion,
		})
	)

	codec = serializer.NewCodecFactory(scheme).CodecForVersions(ser, ser, versions, versions)
}
