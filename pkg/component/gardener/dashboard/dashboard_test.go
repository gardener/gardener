// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package dashboard_test

import (
	"context"
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"golang.org/x/crypto/bcrypt"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
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

		image                 = "gardener-dashboard-image:latest"
		apiServerURL          = "api.local.gardener.cloud"
		logLevel              = "debug"
		ingressValues         = IngressValues{Domains: []string{"first", "second"}}
		enableTokenLogin      bool
		terminal              *TerminalValues
		oidc                  *OIDCValues
		gitHub                *operatorv1alpha1.DashboardGitHub
		frontendConfigMapName *string
		assetsConfigMapName   *string

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
		sessionSecretPrevious     *corev1.Secret
		configMap                 *corev1.ConfigMap
		deployment                *appsv1.Deployment
		service                   *corev1.Service
		podDisruptionBudget       *policyv1.PodDisruptionBudget
		vpa                       *vpaautoscalingv1.VerticalPodAutoscaler
		ingress                   *networkingv1.Ingress
		serviceMonitor            *monitoringv1.ServiceMonitor

		clusterRole                      *rbacv1.ClusterRole
		clusterRoleBinding               *rbacv1.ClusterRoleBinding
		serviceAccountTerminal           *corev1.ServiceAccount
		clusterRoleTerminalProjectMember *rbacv1.ClusterRole
		clusterRoleBindingTerminal       *rbacv1.ClusterRoleBinding
		roleGitHub                       *rbacv1.Role
		roleBindingGitHub                *rbacv1.RoleBinding
	)

	BeforeEach(func() {
		enableTokenLogin = true
		terminal = nil
		oidc = nil
		gitHub = nil
		frontendConfigMapName = nil
		assetsConfigMapName = nil
		sessionSecretPrevious = nil

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
					"rotation-strategy":             "keepold",
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
				"auth":     []byte("admin:$2y$05$VV/caJeJ0XEza7sc5hHib.uppkej805AYCGAKbSCbZwPz6INJy07G"),
			},
		}
		configMap = func(enableTokenLogin bool, terminal *TerminalValues, oidc *OIDCValues, ingressDomains []string, gitHub *operatorv1alpha1.DashboardGitHub, frontendConfigMapName *string) *corev1.ConfigMap {
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
apiServerUrl: https://` + apiServerURL + `
maxRequestBodySize: 500kb
readinessProbe:
  periodSeconds: 10
unreachableSeeds:
  matchLabels:
    seed.gardener.cloud/network: private
`

			if terminal != nil {
				configRaw += `contentSecurityPolicy:
  connectSrc:
    - '''self'''`

				for _, host := range terminal.AllowedHosts {
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
frontend:
  features:
    terminalEnabled: true
`
			}

			if oidc != nil {
				configRaw += `oidc:
  issuer: ` + oidc.IssuerURL + `
  sessionLifetime: 43200
  redirect_uris:`

				for _, domain := range ingressDomains {
					configRaw += `
    - https://dashboard.` + domain + `/auth/callback`
				}

				configRaw += `
  scope: ` + strings.Join(append([]string{"openid", "email"}, oidc.AdditionalScopes...), " ") + `
  rejectUnauthorized: true
  public:
    clientId: ` + oidc.ClientIDPublic + `
    usePKCE: true
`
			}

			if gitHub != nil {
				configRaw += `gitHub:
  apiUrl: ` + gitHub.APIURL + `
  org: ` + gitHub.Organisation + `
  repository: ` + gitHub.Repository

				if gitHub.PollInterval != nil {
					configRaw += `
  pollIntervalSeconds: ` + fmt.Sprintf("%d", int64(gitHub.PollInterval.Duration.Seconds()))
				}

				configRaw += `
  syncThrottleSeconds: 20
  syncConcurrency: 10
`
			}

			if frontendConfigMapName != nil {
				configRaw += `frontend:
  branding:
    some: branding
  foo:
    bar: baz
  landingPageUrl: landing-page-url
  themes:
    some: themes
`
			}

			loginTypes := "null"
			if enableTokenLogin && oidc == nil {
				loginTypes = `["token"]`
			} else if enableTokenLogin && oidc != nil {
				loginTypes = `["oidc","token"]`
			} else if !enableTokenLogin && oidc != nil {
				loginTypes = `["oidc"]`
			}

			frontend := ``
			if frontendConfigMapName != nil {
				frontend = `,"landingPageUrl":"landing-page-url","branding":{"some":"branding"},"themes":{"some":"themes"}`
			}

			loginConfigRaw := `{"loginTypes":` + loginTypes + frontend + `}`

			obj.Data["config.yaml"] = configRaw
			obj.Data["login-config.json"] = loginConfigRaw
			utilruntime.Must(kubernetesutils.MakeUnique(obj))
			return obj
		}(enableTokenLogin, terminal, oidc, ingressValues.Domains, gitHub, frontendConfigMapName)
		deployment = func(oidc *OIDCValues, gitHub *operatorv1alpha1.DashboardGitHub, assetsConfigMapName *string) *appsv1.Deployment {
			obj := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "gardener-dashboard",
					Namespace: namespace,
					Labels: map[string]string{
						"app":  "gardener",
						"role": "dashboard",
						"high-availability-config.resources.gardener.cloud/type": "server",
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
								"networking.gardener.cloud/to-private-networks":                                "allowed",
								"networking.resources.gardener.cloud/to-virtual-garden-kube-apiserver-tcp-443": "allowed",
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
									SecurityContext: &corev1.SecurityContext{
										AllowPrivilegeEscalation: ptr.To(false),
									},
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      "gardener-dashboard-sessionsecret",
											MountPath: "/etc/gardener-dashboard/secrets/session/sessionSecret",
											SubPath:   "sessionSecret",
										},
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
									Name: "gardener-dashboard-sessionsecret",
									VolumeSource: corev1.VolumeSource{
										Secret: &corev1.SecretVolumeSource{
											SecretName:  sessionSecret.Name,
											DefaultMode: ptr.To[int32](0640),
											Items: []corev1.KeyToPath{{
												Key:  "password",
												Path: "sessionSecret",
											}},
										},
									},
								},
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

			if sessionSecretPrevious != nil {
				obj.Spec.Template.Spec.Volumes = append(obj.Spec.Template.Spec.Volumes, corev1.Volume{
					Name: "gardener-dashboard-sessionsecret-previous",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName:  sessionSecretPrevious.Name,
							DefaultMode: ptr.To[int32](0640),
							Items: []corev1.KeyToPath{{
								Key:  "password",
								Path: "sessionSecretPrevious",
							}},
						},
					},
				})
				obj.Spec.Template.Spec.Containers[0].VolumeMounts = append(obj.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
					Name:      "gardener-dashboard-sessionsecret-previous",
					MountPath: "/etc/gardener-dashboard/secrets/session/sessionSecretPrevious",
					SubPath:   "sessionSecretPrevious",
				})
			}

			if oidc != nil {
				obj.Spec.Template.Spec.Volumes = append(obj.Spec.Template.Spec.Volumes, corev1.Volume{
					Name: "gardener-dashboard-oidc",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName:  oidc.SecretRef.Name,
							DefaultMode: ptr.To[int32](0640),
						},
					},
				})
				obj.Spec.Template.Spec.Containers[0].VolumeMounts = append(obj.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
					Name:      "gardener-dashboard-oidc",
					MountPath: "/etc/gardener-dashboard/secrets/oidc",
				})
			}

			if gitHub != nil {
				obj.Spec.Template.Spec.Volumes = append(obj.Spec.Template.Spec.Volumes, corev1.Volume{
					Name: "gardener-dashboard-github",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName:  gitHub.SecretRef.Name,
							DefaultMode: ptr.To[int32](0640),
						},
					},
				})
				obj.Spec.Template.Spec.Containers[0].VolumeMounts = append(obj.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
					Name:      "gardener-dashboard-github",
					MountPath: "/etc/gardener-dashboard/secrets/github",
				})
			}

			if assetsConfigMapName != nil {
				metav1.SetMetaDataAnnotation(&obj.Spec.Template.ObjectMeta, "checksum-configmap-"+*assetsConfigMapName, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855")
				obj.Spec.Template.Spec.Volumes = append(obj.Spec.Template.Spec.Volumes, corev1.Volume{
					Name:         "gardener-dashboard-assets",
					VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: *assetsConfigMapName}}},
				})
				obj.Spec.Template.Spec.Containers[0].VolumeMounts = append(obj.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
					Name:      "gardener-dashboard-assets",
					MountPath: "/app/public/static/assets",
				})
			}

			utilruntime.Must(gardener.InjectGenericKubeconfig(obj, "generic-token-kubeconfig", "shoot-access-gardener-dashboard"))
			utilruntime.Must(references.InjectAnnotations(obj))
			return obj
		}(oidc, gitHub, assetsConfigMapName)
		service = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener-dashboard",
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "gardener",
					"role": "dashboard",
				},
				Annotations: map[string]string{
					"networking.resources.gardener.cloud/from-all-garden-scrape-targets-allowed-ports": `[{"protocol":"TCP","port":9050}]`,
				},
			},
			Spec: corev1.ServiceSpec{
				Type:     corev1.ServiceTypeClusterIP,
				Selector: GetLabels(),
				Ports: []corev1.ServicePort{
					{
						Name:       "http",
						Port:       8080,
						Protocol:   corev1.ProtocolTCP,
						TargetPort: intstr.FromInt32(8080),
					},
					{
						Name:       "metrics",
						Port:       9050,
						Protocol:   corev1.ProtocolTCP,
						TargetPort: intstr.FromInt32(9050),
					},
				},
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
				MaxUnavailable: ptr.To(intstr.FromInt32(1)),
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
							ContainerName:    "*",
							ControlledValues: ptr.To(vpaautoscalingv1.ContainerControlledValuesRequestsOnly),
							MinAllowed: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("10m"),
								corev1.ResourceMemory: resource.MustParse("64Mi"),
							},
						},
					},
				},
			},
		}
		ingress = &networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener-dashboard",
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "gardener",
					"role": "dashboard",
				},
				Annotations: map[string]string{
					"nginx.ingress.kubernetes.io/ssl-redirect":          "true",
					"nginx.ingress.kubernetes.io/use-port-in-redirects": "true",
				},
			},
			Spec: networkingv1.IngressSpec{
				IngressClassName: ptr.To("nginx-ingress-gardener"),
				TLS: []networkingv1.IngressTLS{{
					SecretName: "gardener-dashboard-tls",
					Hosts:      []string{"dashboard.first", "dashboard.second"},
				}},
				Rules: []networkingv1.IngressRule{
					{
						Host: "dashboard.first",
						IngressRuleValue: networkingv1.IngressRuleValue{
							HTTP: &networkingv1.HTTPIngressRuleValue{
								Paths: []networkingv1.HTTPIngressPath{{
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "gardener-dashboard",
											Port: networkingv1.ServiceBackendPort{Number: 8080},
										},
									},
									Path:     "/",
									PathType: ptr.To(networkingv1.PathTypePrefix),
								}},
							},
						},
					},
					{
						Host: "dashboard.second",
						IngressRuleValue: networkingv1.IngressRuleValue{
							HTTP: &networkingv1.HTTPIngressRuleValue{
								Paths: []networkingv1.HTTPIngressPath{{
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "gardener-dashboard",
											Port: networkingv1.ServiceBackendPort{Number: 8080},
										},
									},
									Path:     "/",
									PathType: ptr.To(networkingv1.PathTypePrefix),
								}},
							},
						},
					},
				},
			},
		}
		serviceMonitor = &monitoringv1.ServiceMonitor{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "garden-gardener-dashboard",
				Namespace: namespace,
				Labels:    map[string]string{"prometheus": "garden"},
			},
			Spec: monitoringv1.ServiceMonitorSpec{
				Selector: metav1.LabelSelector{MatchLabels: map[string]string{
					"app":  "gardener",
					"role": "dashboard",
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
		roleGitHub = &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener.cloud:system:dashboard-github-webhook",
				Namespace: "garden",
				Labels: map[string]string{
					"app":  "gardener",
					"role": "dashboard",
				},
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups:     []string{"coordination.k8s.io"},
					Resources:     []string{"leases"},
					ResourceNames: []string{"gardener-dashboard-github-webhook"},
					Verbs:         []string{"get", "patch", "watch", "list"},
				},
				{
					APIGroups: []string{"coordination.k8s.io"},
					Resources: []string{"leases"},
					Verbs:     []string{"create"},
				},
			},
		}
		roleBindingGitHub = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener.cloud:system:dashboard-github-webhook",
				Namespace: "garden",
				Labels: map[string]string{
					"app":  "gardener",
					"role": "dashboard",
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Role",
				Name:     "gardener.cloud:system:dashboard-github-webhook",
			},
			Subjects: []rbacv1.Subject{{
				Kind:      "ServiceAccount",
				Name:      "gardener-dashboard",
				Namespace: "kube-system",
			}},
		}

		values = Values{
			LogLevel:              logLevel,
			Image:                 image,
			APIServerURL:          apiServerURL,
			EnableTokenLogin:      enableTokenLogin,
			Terminal:              terminal,
			OIDC:                  oidc,
			Ingress:               ingressValues,
			GitHub:                gitHub,
			FrontendConfigMapName: frontendConfigMapName,
			AssetsConfigMapName:   assetsConfigMapName,
		}
		deployer = New(fakeClient, namespace, fakeSecretManager, values)

		By("Create secrets managed outside of this package for whose secretsmanager.Get() will be called")
		Expect(fakeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "generic-token-kubeconfig", Namespace: namespace}})).To(Succeed())
	})

	Describe("#Deploy", func() {
		Context("resources generation", func() {
			var expectedRuntimeObjects, expectedVirtualObjects []client.Object

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
				expectedRuntimeObjects = []client.Object{
					configMap,
					deployment,
					service,
					podDisruptionBudget,
					vpa,
					ingress,
					serviceMonitor,
				}
				expectedVirtualObjects = []client.Object{
					clusterRole,
					clusterRoleBinding,
					serviceAccountTerminal,
					clusterRoleBindingTerminal,
					clusterRoleTerminalProjectMember,
				}

				managedResourceSecretVirtual.Name = expectedVirtualMr.Spec.SecretRefs[0].Name
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretVirtual), managedResourceSecretVirtual)).To(Succeed())
				Expect(managedResourceSecretRuntime.Type).To(Equal(corev1.SecretTypeOpaque))
				Expect(managedResourceSecretRuntime.Immutable).To(Equal(ptr.To(true)))
				Expect(managedResourceSecretRuntime.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))

				Expect(managedResourceSecretVirtual.Type).To(Equal(corev1.SecretTypeOpaque))
				Expect(managedResourceSecretVirtual.Immutable).To(Equal(ptr.To(true)))
				Expect(managedResourceSecretVirtual.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))
			})

			It("should successfully deploy all resources", func() {
				Expect(managedResourceRuntime).To(consistOf(expectedRuntimeObjects...))
				Expect(managedResourceVirtual).To(consistOf(expectedVirtualObjects...))
			})

			When("previous session secret found", func() {
				BeforeEach(func() {
					sessionSecretPrevious = &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "gardener-dashboard-session-secret-old",
							Namespace: namespace,
							Labels: map[string]string{
								"manager-identity":              "fake",
								"name":                          "gardener-dashboard-session-secret",
								"rotation-strategy":             "keepold",
								"checksum-of-config":            "5743303071195020433",
								"last-rotation-initiation-time": "",
								"managed-by":                    "secrets-manager",
							},
						},
						Type:      corev1.SecretTypeOpaque,
						Immutable: ptr.To(true),
						Data: map[string][]byte{
							"password": []byte("____________previous____________"),
							"username": []byte("admin"),
							"auth":     []byte("admin:$2a$12$nufeOsvYvptwZo4y3SIbmeBKnrBK/w5aBy6HtFAd6VCepQvJ4BNdG"),
						},
					}
					Expect(fakeClient.Create(ctx, sessionSecretPrevious)).To(Succeed())
				})

				It("should successfully deploy all resources", func() {
					Expect(managedResourceRuntime).To(consistOf(expectedRuntimeObjects...))
					Expect(managedResourceVirtual).To(consistOf(expectedVirtualObjects...))
				})
			})

			When("token login is disabled", func() {
				BeforeEach(func() {
					enableTokenLogin = false
				})

				It("should successfully deploy all resources", func() {
					Expect(managedResourceRuntime).To(consistOf(expectedRuntimeObjects...))
					Expect(managedResourceVirtual).To(consistOf(expectedVirtualObjects...))
				})
			})

			When("terminal is configured", func() {
				BeforeEach(func() {
					terminal = &TerminalValues{
						DashboardTerminal: operatorv1alpha1.DashboardTerminal{
							Container: operatorv1alpha1.DashboardTerminalContainer{
								Image:       "some-image:latest",
								Description: ptr.To("cool image"),
							},
							AllowedHosts: []string{"first", "second"},
						},
						GardenTerminalSeedHost: "terminal-host",
					}
				})

				It("should successfully deploy all resources", func() {
					Expect(managedResourceRuntime).To(consistOf(expectedRuntimeObjects...))
					Expect(managedResourceVirtual).To(consistOf(expectedVirtualObjects...))
				})
			})

			When("oidc is configured", func() {
				BeforeEach(func() {
					oidc = &OIDCValues{
						DashboardOIDC: operatorv1alpha1.DashboardOIDC{
							AdditionalScopes: []string{"first", "second"},
							SecretRef:        corev1.LocalObjectReference{Name: "some-oidc-secret"},
						},
						IssuerURL:      "http://issuer",
						ClientIDPublic: "public-client",
					}

					Expect(fakeClient.Create(ctx, &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{Name: oidc.DashboardOIDC.SecretRef.Name, Namespace: namespace},
						Data:       map[string][]byte{"client_id": []byte("id"), "client_secret": []byte("secret")},
					})).To(Succeed())
				})

				It("should successfully deploy all resources", func() {
					Expect(managedResourceRuntime).To(consistOf(expectedRuntimeObjects...))
					Expect(managedResourceVirtual).To(consistOf(expectedVirtualObjects...))
				})
			})

			When("github is configured", func() {
				BeforeEach(func() {
					gitHub = &operatorv1alpha1.DashboardGitHub{
						APIURL:       "api-url",
						Organisation: "org",
						Repository:   "repo",
						SecretRef:    corev1.LocalObjectReference{Name: "some-github-secret"},
					}
				})

				JustBeforeEach(func() {
					expectedVirtualObjects = append(expectedVirtualObjects,
						roleGitHub,
						roleBindingGitHub,
					)
				})

				Context("with webhook secret", func() {
					BeforeEach(func() {
						Expect(fakeClient.Create(ctx, &corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{Name: gitHub.SecretRef.Name, Namespace: namespace},
							Data:       map[string][]byte{"authentication.token": []byte("token"), "webhookSecret": []byte("webhookSecret")},
						})).To(Succeed())
					})

					It("should successfully deploy all resources", func() {
						Expect(managedResourceRuntime).To(consistOf(expectedRuntimeObjects...))
						Expect(managedResourceVirtual).To(consistOf(expectedVirtualObjects...))
					})
				})

				Context("without webhook secret and poll interval", func() {
					BeforeEach(func() {
						gitHub.PollInterval = &metav1.Duration{Duration: 10 * time.Minute}

						Expect(fakeClient.Create(ctx, &corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{Name: gitHub.SecretRef.Name, Namespace: namespace},
							Data:       map[string][]byte{"authentication.token": []byte("token")},
						})).To(Succeed())
					})

					It("should successfully deploy all resources", func() {
						Expect(managedResourceRuntime).To(consistOf(expectedRuntimeObjects...))
						Expect(managedResourceVirtual).To(consistOf(expectedVirtualObjects...))
					})
				})
			})

			When("frontend is configured", func() {
				BeforeEach(func() {
					frontendConfigMapName = ptr.To("frontend")

					Expect(fakeClient.Create(ctx, &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: *frontendConfigMapName, Namespace: namespace},
						Data: map[string]string{"frontend-config.yaml": `
foo:
  bar: baz
landingPageUrl: landing-page-url
branding:
  some: branding
themes:
  some: themes
`}},
					)).To(Succeed())

				})

				It("should successfully deploy all resources", func() {
					Expect(managedResourceRuntime).To(consistOf(expectedRuntimeObjects...))
					Expect(managedResourceVirtual).To(consistOf(expectedVirtualObjects...))
				})
			})

			When("assets are configured", func() {
				BeforeEach(func() {
					assetsConfigMapName = ptr.To("assets")

					Expect(fakeClient.Create(ctx, &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: *assetsConfigMapName, Namespace: namespace},
						Data: map[string]string{"assets-config.yaml": `
assets:
  foo: YmFy
  bar: Zm9vCg==
`}},
					)).To(Succeed())

				})

				It("should successfully deploy all resources", func() {
					Expect(managedResourceRuntime).To(consistOf(expectedRuntimeObjects...))
					Expect(managedResourceVirtual).To(consistOf(expectedVirtualObjects...))
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
					Expect(actualSessionSecret.ObjectMeta).To(Equal(sessionSecret.ObjectMeta))
					Expect(actualSessionSecret.Type).To(Equal(sessionSecret.Type))
					Expect(actualSessionSecret.Immutable).To(Equal(sessionSecret.Immutable))
					Expect(actualSessionSecret.Data["username"]).To(Equal(sessionSecret.Data["username"]))
					Expect(actualSessionSecret.Data["password"]).To(Equal(sessionSecret.Data["password"]))
					hashedPassword := strings.TrimPrefix(string(actualSessionSecret.Data["auth"]), string(sessionSecret.Data["username"])+":")
					Expect(bcrypt.CompareHashAndPassword([]byte(hashedPassword), sessionSecret.Data["password"])).To(Succeed())
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
