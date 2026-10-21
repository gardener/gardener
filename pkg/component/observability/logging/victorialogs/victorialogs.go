// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package victorialogs

import (
	"context"
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	victoriametricsv1 "github.com/VictoriaMetrics/operator/api/operator/v1"
	victoriametricsv1beta1 "github.com/VictoriaMetrics/operator/api/operator/v1beta1"
	pvcautoscalerv1alpha1 "github.com/gardener/pvc-autoscaler/api/autoscaling/v1alpha1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	istioapiannotation "istio.io/api/annotation"
	istioapinetworkingv1beta1 "istio.io/api/networking/v1beta1"
	istionetworkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/autoscaling/pvcautoscaler"
	"github.com/gardener/gardener/pkg/component/networking/istiobasicauthserver"
	"github.com/gardener/gardener/pkg/component/observability/logging/victorialogs/constants"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/garden"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/seed"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/shoot"
	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/istio"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const (
	name = "victoria-logs"

	timeoutWaitForManagedResources = 2 * time.Minute

	vlServerTLSVolumeName = "vl-server-tls"
	vlServerTLSMountPath  = "/etc/victorialogs/tls"
)

// Values is the values for VictoriaLogs configurations.
type Values struct {
	// ImageRepository is the VictoriaLogs image repository.
	ImageRepository string
	// ImageTag is the VictoriaLogs image tag. May include an appended digest
	// in the form "<tag>@sha256:...".
	ImageTag string
	// Storage is the disk storage capacity of VictoriaLogs.
	// If not set, a default of 30Gi will be used.
	Storage *resource.Quantity
	// IsGardenCluster specifies whether VictoriaLogs is being deployed in a cluster registered as a Garden.
	IsGardenCluster bool
	// ClusterType is the type of the cluster where VictoriaLogs is deployed (Seed or Shoot).
	ClusterType component.ClusterType
	// Replicas is the number of VictoriaLogs replicas.
	Replicas int32
	// PriorityClassName is the name of the priority class for the VictoriaLogs pods.
	PriorityClassName string
	// PVCAutoscaler configures whether and how the VictoriaLogs PVC is autoscaled.
	PVCAutoscaling PVCAutoscalingConfig
	// SecretNameServerCA is the name of the CA secret used to sign the server TLS cert.
	SecretNameServerCA string
	// ExternalExposure contains configuration for exposing this VictoriaLogs instance via a VirtualService resource.
	ExternalExposure *ExposureValues
}

// ExposureValues contains configuration for exposing this VictoriaLogs instance via a VirtualService resource.
type ExposureValues struct {
	// AuthSecretName is the name of the auth secret.
	AuthSecretName string
	// AuthSecretManaged indicates whether the auth secret is managed by the secrets manager (referenced by its base
	// name) or used verbatim.
	AuthSecretManaged bool
	// Host is the hostname under which the VictoriaLogs instance should be exposed.
	Host string
	// IstioIngressGatewayLabels are the labels identifying the corresponding istio ingress gateway.
	IstioIngressGatewayLabels map[string]string
	// IstioIngressGatewayNamespace is the namespace of the corresponding istio ingress gateway. The Gateway,
	// VirtualService and DestinationRule are exported to this namespace only, and the TLS certificate is copied there.
	IstioIngressGatewayNamespace string
	// WildcardCertSecretName is name of a secret containing the wildcard TLS certificate which is issued for the
	// ingress domain. If not provided, a self-signed server certificate will be created.
	WildcardCertSecretName *string
}

// PVCAutoscalingConfig configures whether and up to what capacity the VictoriaLogs PVC is autoscaled.
type PVCAutoscalingConfig struct {
	// Enabled controls whether the component creates a PersistentVolumeClaimAutoscaler resource.
	Enabled bool
	// MaxCapacity is the upper bound up to which the PVC may be scaled.
	MaxCapacity resource.Quantity
}

type victoriaLogs struct {
	client         client.Client
	namespace      string
	secretsManager secretsmanager.Interface
	values         Values
}

// New creates a new instance of VictoriaLogs deployer.
func New(
	client client.Client,
	namespace string,
	secretsManager secretsmanager.Interface,
	values Values,
) component.DeployWaiter {
	return &victoriaLogs{
		client:         client,
		namespace:      namespace,
		secretsManager: secretsManager,
		values:         values,
	}
}

func (v *victoriaLogs) Deploy(ctx context.Context) error {
	// TODO(rrhubenov): Remove this check once https://github.com/VictoriaMetrics/operator/pull/2401 is merged
	// and we update to the release that includes it. Until then, the VictoriaMetrics operator cannot handle a
	// digest-only image reference, so we reject it here. A digest-only tag has the form "<algorithm>:<hex>"
	// (e.g. "sha256:...", "sha512:...") whereas a tag+digest combined form contains a "@" separator.
	if !strings.Contains(v.values.ImageTag, "@") {
		for _, algorithmPrefix := range []string{"sha256:", "sha512:"} {
			if strings.HasPrefix(v.values.ImageTag, algorithmPrefix) {
				return fmt.Errorf("digest-only image reference %q is not supported yet", v.values.ImageRepository+"@"+v.values.ImageTag)
			}
		}
	}

	registry := managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)

	if v.values.SecretNameServerCA == "" {
		return fmt.Errorf("secretNameServerCA must be set")
	}

	serverTLSSecret, err := v.secretsManager.Generate(ctx, &secrets.CertificateSecretConfig{
		Name:                        "victoria-logs-server-tls",
		CommonName:                  kubernetesutils.FQDNForService(constants.ServiceName, v.namespace),
		DNSNames:                    kubernetesutils.DNSNamesForService(constants.ServiceName, v.namespace),
		CertType:                    secrets.ServerCert,
		SkipPublishingCACertificate: true,
	}, secretsmanager.SignedByCA(v.values.SecretNameServerCA), secretsmanager.Rotate(secretsmanager.InPlace))
	if err != nil {
		return fmt.Errorf("failed to generate victoria-logs-server-tls secret: %w", err)
	}

	resources := []client.Object{
		v.vlSingle(serverTLSSecret.Name),
		v.getVPA(),
		v.getServiceMonitor(),
		v.getPrometheusRule(),
	}

	istioObjects, err := v.istioResources(ctx)
	if err != nil {
		return err
	}
	resources = append(resources, istioObjects...)

	if v.values.PVCAutoscaling.Enabled {
		resources = append(resources, v.getPVCA(v.values.PVCAutoscaling))
	}

	serializedResources, err := registry.AddAllAndSerialize(resources...)
	if err != nil {
		return err
	}

	return managedresources.CreateForSeedWithLabels(ctx, v.client, v.namespace, constants.ManagedResourceNameRuntime, false, map[string]string{v1beta1constants.LabelCareConditionType: v1beta1constants.ObservabilityComponentsHealthy}, serializedResources)
}

func (v *victoriaLogs) Destroy(ctx context.Context) error {
	return managedresources.DeleteForSeed(ctx, v.client, v.namespace, constants.ManagedResourceNameRuntime)
}

func (v *victoriaLogs) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeoutWaitForManagedResources)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, v.client, v.namespace, constants.ManagedResourceNameRuntime)
}

func (v *victoriaLogs) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeoutWaitForManagedResources)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, v.client, v.namespace, constants.ManagedResourceNameRuntime)
}

func (v *victoriaLogs) vlSingle(vlServerTLSSecretName string) *victoriametricsv1.VLSingle {
	storage := resource.MustParse("30Gi")
	if v.values.Storage != nil {
		storage = *v.values.Storage
	}

	vlSingle := &victoriametricsv1.VLSingle{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.VLSingleResourceName,
			Namespace: v.namespace,
		},
		Spec: victoriametricsv1.VLSingleSpec{
			PodMetadata: &victoriametricsv1beta1.EmbeddedObjectMetadata{
				Labels: getLabels(),
			},
			CommonAppsParams: victoriametricsv1beta1.CommonAppsParams{
				DisableSelfServiceScrape: new(true),
				UseStrictSecurity:        new(true),
				Image: victoriametricsv1beta1.Image{
					Repository: v.values.ImageRepository,
					Tag:        v.values.ImageTag,
				},
				Port: strconv.Itoa(constants.VictoriaLogsPort),
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("10m"),
						corev1.ResourceMemory: resource.MustParse("100M"),
					},
				},
				UseDefaultResources: new(false),
				ReplicaCount:        new(v.values.Replicas),
				PriorityClassName:   v.values.PriorityClassName,
				// Port order matters => do not reorder
				ExtraArgs: map[string]string{
					"httpListenAddr": fmt.Sprintf(":%d,:%d", constants.VictoriaLogsPort, constants.VictoriaLogsHttpPort),
					"tls":            "true,false",
					"tlsCertFile":    path.Join(vlServerTLSMountPath, secrets.DataKeyCertificate),
					"tlsKeyFile":     path.Join(vlServerTLSMountPath, secrets.DataKeyPrivateKey),
				},
				Volumes: []corev1.Volume{{
					Name: vlServerTLSVolumeName,
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{SecretName: vlServerTLSSecretName},
					},
				}},
				VolumeMounts: []corev1.VolumeMount{{
					Name:      vlServerTLSVolumeName,
					MountPath: vlServerTLSMountPath,
					ReadOnly:  true,
				}},
			},
			RetentionPeriod: "15d",
			Storage: &corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: storage,
					},
				},
			},
			// This annotation tells the VictoriaMetrics operator not to attempt resizing the PVC itself,
			// if the CR's storage field is mutated. PVC resizing is instead owned by the dedicated
			// pvc-autoscaler component. If pvc-autoscaler gets disabled, the PVC will not be resized
			// to the storage size specified in the VLSingle CR as long as this annotation is still present.
			StorageMetadata: victoriametricsv1beta1.EmbeddedObjectMetadata{
				Annotations: map[string]string{
					"operator.victoriametrics.com/pvc-allow-volume-expansion": "false",
				},
			},
			ServiceSpec: &victoriametricsv1beta1.AdditionalServiceSpec{
				EmbeddedObjectMetadata: victoriametricsv1beta1.EmbeddedObjectMetadata{
					Name:        constants.ServiceName,
					Annotations: v.serviceAnnotations(),
				},
				// Exposing both ports. The http port is left for extension compatibility reasons.
				// For more information look up constants.VictoriaLogsHttpPort
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Name:       "https",
							Port:       constants.VictoriaLogsPort,
							TargetPort: intstr.FromInt32(constants.VictoriaLogsPort),
							Protocol:   corev1.ProtocolTCP,
						},
						{
							Name:       "http",
							Port:       constants.VictoriaLogsHttpPort,
							TargetPort: intstr.FromInt32(constants.VictoriaLogsHttpPort),
							Protocol:   corev1.ProtocolTCP,
						},
					},
				},
			},
		},
	}

	// Add network policy annotations to allow Prometheus scraping based on cluster type.
	// ManagedMetadata propagates annotations to all objects created by the VM operator, including the Service.
	managedAnnotations := make(map[string]string)
	switch v.values.ClusterType {
	case component.ClusterTypeSeed:
		if v.values.IsGardenCluster {
			managedAnnotations[resourcesv1alpha1.NetworkPolicyFromPolicyAnnotationPrefix+v1beta1constants.LabelNetworkPolicyGardenScrapeTargets+resourcesv1alpha1.NetworkPolicyFromPolicyAnnotationSuffix] = fmt.Sprintf(`[{"protocol":"TCP","port":%d}]`, constants.VictoriaLogsPort)
		} else {
			managedAnnotations[resourcesv1alpha1.NetworkPolicyFromPolicyAnnotationPrefix+v1beta1constants.LabelNetworkPolicySeedScrapeTargets+resourcesv1alpha1.NetworkPolicyFromPolicyAnnotationSuffix] = fmt.Sprintf(`[{"protocol":"TCP","port":%d}]`, constants.VictoriaLogsPort)
		}
	case component.ClusterTypeShoot:
		managedAnnotations[resourcesv1alpha1.NetworkPolicyFromPolicyAnnotationPrefix+v1beta1constants.LabelNetworkPolicyScrapeTargets+resourcesv1alpha1.NetworkPolicyFromPolicyAnnotationSuffix] = fmt.Sprintf(`[{"protocol":"TCP","port":%d}]`, constants.VictoriaLogsPort)
		managedAnnotations[resourcesv1alpha1.NetworkingPodLabelSelectorNamespaceAlias] = v1beta1constants.LabelNetworkPolicyShootNamespaceAlias
		managedAnnotations[resourcesv1alpha1.NetworkingNamespaceSelectors] = `[{"matchLabels":{"kubernetes.io/metadata.name":"garden"}}]`
	}
	if len(managedAnnotations) > 0 {
		vlSingle.Spec.ManagedMetadata = &victoriametricsv1beta1.ManagedObjectsMetadata{
			Annotations: managedAnnotations,
		}
	}

	return vlSingle
}

func getLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp:                      constants.VLSingleResourceName,
		v1beta1constants.LabelRole:                     v1beta1constants.LabelObservability,
		v1beta1constants.GardenRole:                    v1beta1constants.GardenRoleObservability,
		v1beta1constants.LabelObservabilityApplication: constants.VLSingleResourceName,
	}
}

func (v *victoriaLogs) serviceAnnotations() map[string]string {
	if v.values.ExternalExposure == nil {
		return nil
	}
	return map[string]string{
		istioapiannotation.NetworkingExportTo.Name: v.values.ExternalExposure.IstioIngressGatewayNamespace,
	}
}

func (v *victoriaLogs) getVPA() *vpaautoscalingv1.VerticalPodAutoscaler {
	return &vpaautoscalingv1.VerticalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "victoria-logs-vpa",
			Namespace: v.namespace,
			Labels:    getLabels(),
		},
		Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
			TargetRef: &autoscalingv1.CrossVersionObjectReference{
				Kind:       "Deployment",
				Name:       "vlsingle-" + constants.VLSingleResourceName,
				APIVersion: appsv1.SchemeGroupVersion.String(),
			},
			UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
				UpdateMode: new(vpaautoscalingv1.UpdateModeInPlaceOrRecreate),
			},
			ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
				ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
					{
						ContainerName:    "vlsingle",
						ControlledValues: new(vpaautoscalingv1.ContainerControlledValuesRequestsOnly),
					},
				},
			},
		},
	}
}

func (v *victoriaLogs) getPVCA(pvcAutoscaling PVCAutoscalingConfig) *pvcautoscalerv1alpha1.PersistentVolumeClaimAutoscaler {
	return &pvcautoscalerv1alpha1.PersistentVolumeClaimAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.VLSingleResourceName,
			Namespace: v.namespace,
			Labels:    getLabels(),
		},
		Spec: pvcautoscalerv1alpha1.PersistentVolumeClaimAutoscalerSpec{
			AutoscalerName: v.getAutoscalerName(),
			TargetRef: autoscalingv1.CrossVersionObjectReference{
				APIVersion: appsv1.SchemeGroupVersion.String(),
				Kind:       "Deployment",
				Name:       "vlsingle-" + constants.VLSingleResourceName,
			},
			VolumePolicies: []pvcautoscalerv1alpha1.VolumePolicy{
				{
					MaxCapacity: pvcAutoscaling.MaxCapacity,
					ScaleUp: &pvcautoscalerv1alpha1.ScalingRules{
						UtilizationThresholdPercent: new(70),
						StepPercent:                 new(10),
						MinStepAbsolute:             new(resource.MustParse("2Gi")),
					},
				},
			},
		},
	}
}

func (v *victoriaLogs) getPrometheusLabel() string {
	if v.values.ClusterType == component.ClusterTypeSeed {
		if v.values.IsGardenCluster {
			return garden.Label
		}
		return seed.Label
	}
	return shoot.Label
}

// getAutoscalerName returns the name of the pvc-autoscaler instance that manages the VictoriaLogs PVCA. It must match
// the --autoscaler-name flag of the pvc-autoscaler instance running in the cluster where VictoriaLogs is deployed. The
// empty string denotes the default (unnamed) instance running in seeds.
func (v *victoriaLogs) getAutoscalerName() string {
	if v.values.IsGardenCluster {
		return pvcautoscaler.GardenAutoscalerName
	}
	return ""
}

func (v *victoriaLogs) getServiceMonitor() *monitoringv1.ServiceMonitor {
	return &monitoringv1.ServiceMonitor{
		ObjectMeta: monitoringutils.ConfigObjectMeta(name, v.namespace, v.getPrometheusLabel()),
		Spec: monitoringv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name":                          "vlsingle",
					"app.kubernetes.io/instance":                      constants.VLSingleResourceName,
					"app.kubernetes.io/component":                     "monitoring",
					"managed-by":                                      "vm-operator",
					"operator.victoriametrics.com/additional-service": "managed",
				},
			},
			Endpoints: []monitoringv1.Endpoint{{
				Port: "https",
				RelabelConfigs: []monitoringv1.RelabelConfig{
					{
						Action:      "replace",
						Replacement: new(name),
						TargetLabel: "job",
					},
					{
						Action: "labelmap",
						Regex:  `__meta_kubernetes_service_label_(.+)`,
					},
				},
				Scheme: new(monitoringv1.SchemeHTTPS),
				HTTPConfigWithProxyAndTLSFiles: monitoringv1.HTTPConfigWithProxyAndTLSFiles{
					HTTPConfigWithTLSFiles: monitoringv1.HTTPConfigWithTLSFiles{
						TLSConfig: &monitoringv1.TLSConfig{SafeTLSConfig: monitoringv1.SafeTLSConfig{InsecureSkipVerify: new(true)}},
					},
				},
			}},
		},
	}
}

func (v *victoriaLogs) getPrometheusRule() *monitoringv1.PrometheusRule {
	description := "There are no VictoriaLogs pods running on seed: {{ .ExternalLabels.seed }}. No logs will be collected."
	if v.values.ClusterType == component.ClusterTypeShoot {
		description = "There are no VictoriaLogs pods running. No logs will be collected."
	}

	return &monitoringv1.PrometheusRule{
		ObjectMeta: monitoringutils.ConfigObjectMeta(name, v.namespace, v.getPrometheusLabel()),
		Spec: monitoringv1.PrometheusRuleSpec{
			Groups: []monitoringv1.RuleGroup{{
				Name: "victoria-logs.rules",
				Rules: []monitoringv1.Rule{{
					Alert: "VictoriaLogsDown",
					Expr:  intstr.FromString(`absent(up{job="victoria-logs"} == 1)`),
					For:   new(monitoringv1.Duration("30m")),
					Labels: map[string]string{
						"service":    "logging",
						"severity":   "warning",
						"type":       "seed",
						"visibility": "operator",
					},
					Annotations: map[string]string{
						"description": description,
						"summary":     "VictoriaLogs is down",
					},
				}},
			}},
		},
	}
}

func (v *victoriaLogs) istioResources(ctx context.Context) ([]client.Object, error) {
	if v.values.ExternalExposure == nil {
		return nil, nil
	}

	var (
		ingressNamespace = v.values.ExternalExposure.IstioIngressGatewayNamespace
		gatewayName      = name
	)

	switch {
	case v.values.IsGardenCluster:
		gatewayName = name + "-garden"
	case v.values.ClusterType == component.ClusterTypeSeed:
		gatewayName = name + "-seed"
	}

	tlsSecretName := ptr.Deref(v.values.ExternalExposure.WildcardCertSecretName, "")
	if tlsSecretName == "" {
		// On purpose creating a separate server certificate dedicated to the ingress
		ingressTLSSecret, err := v.secretsManager.Generate(ctx, &secrets.CertificateSecretConfig{
			Name:                        name + "-ingress-tls",
			CommonName:                  name,
			Organization:                []string{"gardener.cloud:logging:ingress"},
			DNSNames:                    []string{v.values.ExternalExposure.Host},
			CertType:                    secrets.ServerCert,
			Validity:                    new(v1beta1constants.IngressTLSCertificateValidity),
			SkipPublishingCACertificate: true,
		}, secretsmanager.SignedByCA(v.values.SecretNameServerCA))
		if err != nil {
			return nil, fmt.Errorf("failed to generate ingress TLS secret: %w", err)
		}
		tlsSecretName = ingressTLSSecret.Name
	}

	// Istio expects the secret in the istio ingress gateway namespace => copy certificate to istio namespace
	tlsSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tlsSecretName,
			Namespace: v.namespace,
		},
	}
	if err := v.client.Get(ctx, client.ObjectKeyFromObject(tlsSecret), tlsSecret); err != nil {
		return nil, fmt.Errorf("failed to get TLS secret %q: %w", tlsSecretName, err)
	}

	tlsSecretInIstioNamespace := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s-%s", v.namespace, name, tlsSecretName),
			Namespace: ingressNamespace,
			Labels:    getLabels(),
		},
		Data: tlsSecret.Data,
	}

	// Fetch the CA bundle to allow Istio to verify the VictoriaLogs server cert
	caBundle, found := v.secretsManager.Get(v.values.SecretNameServerCA)
	if !found {
		return nil, fmt.Errorf("the CA trust bundle for %q was not found", v.values.SecretNameServerCA)
	}

	caBundleInIstioNamespace := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s-%s", v.namespace, name, caBundle.Name),
			Namespace: v.values.ExternalExposure.IstioIngressGatewayNamespace,
			Labels:    getLabels(),
		},
		Data: map[string][]byte{
			"cacert": caBundle.Data[secrets.DataKeyCertificateBundle],
		},
	}

	gateway := &istionetworkingv1beta1.Gateway{ObjectMeta: metav1.ObjectMeta{Name: gatewayName, Namespace: v.namespace}}
	if err := istio.GatewayWithTLSTermination(
		gateway,
		getLabels(),
		v.values.ExternalExposure.IstioIngressGatewayLabels,
		[]string{v.values.ExternalExposure.Host},
		tlsSecretInIstioNamespace.Name,
	)(); err != nil {
		return nil, fmt.Errorf("failed to create gateway resource: %w", err)
	}

	destinationHost := kubernetesutils.FQDNForService(constants.ServiceName, v.namespace)
	virtualService := &istionetworkingv1beta1.VirtualService{ObjectMeta: metav1.ObjectMeta{Name: gatewayName, Namespace: v.namespace}}
	if err := istio.VirtualServiceForTLSTermination(
		virtualService,
		utils.MergeStringMaps(getLabels(), istiobasicauthserver.BasicAuthLabels(v.values.IsGardenCluster, v.values.ExternalExposure.AuthSecretName, v.values.ExternalExposure.AuthSecretManaged)),
		[]string{ingressNamespace},
		[]string{v.values.ExternalExposure.Host},
		gatewayName,
		constants.VictoriaLogsPort,
		destinationHost,
		"",
		"",
	)(); err != nil {
		return nil, fmt.Errorf("failed to create virtual service resource: %w", err)
	}

	// Allowlist: only /select/logsql/query is accessible; everything else returns 403.
	virtualService.Spec.Http = append(
		[]*istioapinetworkingv1beta1.HTTPRoute{
			{
				Name: "query",
				Match: []*istioapinetworkingv1beta1.HTTPMatchRequest{{
					Uri: &istioapinetworkingv1beta1.StringMatch{
						MatchType: &istioapinetworkingv1beta1.StringMatch_Prefix{
							Prefix: "/select/logsql/query",
						},
					},
				}},
				Route: []*istioapinetworkingv1beta1.HTTPRouteDestination{{
					Destination: &istioapinetworkingv1beta1.Destination{
						Host: destinationHost,
						Port: &istioapinetworkingv1beta1.PortSelector{Number: constants.VictoriaLogsPort},
					},
				}},
			},
			{
				DirectResponse: &istioapinetworkingv1beta1.HTTPDirectResponse{
					Status: 403,
				},
			},
		},
		virtualService.Spec.Http...,
	)

	destinationRule := &istionetworkingv1beta1.DestinationRule{ObjectMeta: metav1.ObjectMeta{Name: gatewayName, Namespace: v.namespace}}
	if err := istio.DestinationRuleWithLocalityPreferenceAndTLS(
		destinationRule,
		getLabels(),
		[]string{v.values.ExternalExposure.IstioIngressGatewayNamespace},
		destinationHost,
		&istioapinetworkingv1beta1.ClientTLSSettings{
			Mode:           istioapinetworkingv1beta1.ClientTLSSettings_SIMPLE,
			CredentialName: caBundleInIstioNamespace.Name,
			Sni:            destinationHost,
		},
	)(); err != nil {
		return nil, fmt.Errorf("failed to create destination rule resource: %w", err)
	}

	return []client.Object{tlsSecretInIstioNamespace, caBundleInIstioNamespace, gateway, virtualService, destinationRule}, nil
}
