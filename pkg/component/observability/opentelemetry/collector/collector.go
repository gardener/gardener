// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"context"
	"fmt"
	"strconv"
	"time"

	otelv1beta1 "github.com/open-telemetry/opentelemetry-operator/apis/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	valiconstants "github.com/gardener/gardener/pkg/component/observability/logging/vali/constants"
	collectorconstants "github.com/gardener/gardener/pkg/component/observability/opentelemetry/collector/constants"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const (
	managedResourceName              = "opentelemetry-collector"
	otelCollectorConfigName          = "opentelemetry-collector-config"
	kubeRBACProxyName                = "kube-rbac-proxy"
	volumeMountPathGenericKubeconfig = "/var/run/secrets/gardener.cloud/shoot/generic-kubeconfig"
	timeoutWaitForManagedResources   = 2 * time.Minute
)

// Values is the values for otel-collector configurations
type Values struct {
	// Image is the collector image.
	Image              string
	KubeRBACProxyImage string
	WithRBACProxy      bool
}

type otelCollector struct {
	client         client.Client
	namespace      string
	values         Values
	lokiEndpoint   string
	secretsManager secretsmanager.Interface
}

// Interface is the interface for the OtelCollector deployer.
type Interface interface {
	component.DeployWaiter
	WithAuthenticationProxy(bool)
}

// New creates a new instance of otel-collector deployer.
func New(
	client client.Client,
	namespace string,
	values Values,
	lokiEndpoint string,
	secretsManager secretsmanager.Interface,
) Interface {
	return &otelCollector{
		client:         client,
		namespace:      namespace,
		values:         values,
		lokiEndpoint:   lokiEndpoint,
		secretsManager: secretsManager,
	}
}

func (o *otelCollector) WithAuthenticationProxy(b bool) {
	o.values.WithRBACProxy = b
}

func (o *otelCollector) newKubeRBACProxyShootAccessSecret() *gardenerutils.AccessSecret {
	return gardenerutils.NewShootAccessSecret(kubeRBACProxyName, o.namespace)
}

func (o *otelCollector) Deploy(ctx context.Context) error {
	var (
		genericTokenKubeconfigSecretName string
		kubeRBACProxyShootAccessSecret   = o.newKubeRBACProxyShootAccessSecret()
	)

	if err := kubeRBACProxyShootAccessSecret.Reconcile(ctx, o.client); err != nil {
		return err
	}

	genericTokenKubeconfigSecret, found := o.secretsManager.Get(v1beta1constants.SecretNameGenericTokenKubeconfig)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameGenericTokenKubeconfig)
	}
	genericTokenKubeconfigSecretName = genericTokenKubeconfigSecret.Name

	registry := managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)

	serializedResources, err := registry.AddAllAndSerialize(o.openTelemetryCollector(o.namespace, o.lokiEndpoint, genericTokenKubeconfigSecretName))
	if err != nil {
		return err
	}

	return managedresources.CreateForSeedWithLabels(ctx, o.client, o.namespace, managedResourceName, false, map[string]string{v1beta1constants.LabelCareConditionType: v1beta1constants.ObservabilityComponentsHealthy}, serializedResources)
}

func (o *otelCollector) Destroy(ctx context.Context) error {
	return managedresources.DeleteForSeed(ctx, o.client, o.namespace, managedResourceName)
}

func (o *otelCollector) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeoutWaitForManagedResources)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, o.client, o.namespace, managedResourceName)
}

func (o *otelCollector) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeoutWaitForManagedResources)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, o.client, o.namespace, managedResourceName)
}

func (o *otelCollector) openTelemetryCollector(namespace, lokiEndpoint, genericTokenKubeconfigSecretName string) *otelv1beta1.OpenTelemetryCollector {
	var (
		volume = corev1.Volume{
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
									Key:  secrets.DataKeyKubeconfig,
									Path: secrets.DataKeyKubeconfig,
								}},
								Optional: ptr.To(false),
							},
						},
						{
							Secret: &corev1.SecretProjection{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "shoot-access-" + kubeRBACProxyName,
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
		}

		volumeMount = corev1.VolumeMount{
			Name:      volume.Name,
			MountPath: volumeMountPathGenericKubeconfig,
			ReadOnly:  true,
		}
	)

	obj := &otelv1beta1.OpenTelemetryCollector{
		ObjectMeta: metav1.ObjectMeta{
			Name:      collectorconstants.OpenTelemetryCollectorResourceName,
			Namespace: namespace,
			Labels:    getLabels(),
		},
		Spec: otelv1beta1.OpenTelemetryCollectorSpec{
			Mode:            "deployment",
			UpgradeStrategy: "none",
			OpenTelemetryCommonFields: otelv1beta1.OpenTelemetryCommonFields{
				Image: o.values.Image,
				SecurityContext: &corev1.SecurityContext{
					AllowPrivilegeEscalation: ptr.To(false),
				},
				Volumes: []corev1.Volume{volume},
				Ports: []otelv1beta1.PortsSpec{
					{
						ServicePort: corev1.ServicePort{
							Name: kubeRBACProxyName,
							Port: collectorconstants.KubeRBACProxyPort,
						},
					},
				},
			},
			Config: otelv1beta1.Config{
				Receivers: otelv1beta1.AnyConfig{
					Object: map[string]any{
						"loki": map[string]any{
							"protocols": map[string]any{
								"http": map[string]any{
									"endpoint": "0.0.0.0:" + strconv.Itoa(collectorconstants.PushPort),
								},
							},
						},
					},
				},
				Processors: &otelv1beta1.AnyConfig{
					Object: map[string]any{
						"batch": map[string]any{
							"timeout": "10s",
						},
						"attributes/labels": map[string]any{
							"actions": []any{
								map[string]any{
									"key":    "loki.attribute.labels",
									"value":  "job, unit, nodename, origin, pod_name, container_name, origin, namespace_name, nodename, gardener_cloud_role",
									"action": "insert",
								},
								map[string]any{
									"key":    "loki.format",
									"value":  "logfmt",
									"action": "insert",
								},
							},
						},
					},
				},
				Exporters: otelv1beta1.AnyConfig{
					Object: map[string]any{
						"loki": map[string]any{
							"endpoint": lokiEndpoint,
						},
					},
				},
				Service: otelv1beta1.Service{
					Pipelines: map[string]*otelv1beta1.Pipeline{
						"logs": {
							Exporters: []string{
								"loki",
							},
							Receivers: []string{
								"loki",
							},
							Processors: []string{
								"attributes/labels",
								"batch",
							},
						},
					},
				},
			},
		},
	}

	if o.values.WithRBACProxy {
		obj.Spec.AdditionalContainers = append(obj.Spec.AdditionalContainers,
			corev1.Container{
				Name:  kubeRBACProxyName,
				Image: o.values.KubeRBACProxyImage,
				Args: []string{
					fmt.Sprintf("--insecure-listen-address=0.0.0.0:%d", collectorconstants.KubeRBACProxyPort),
					fmt.Sprintf("--upstream=http://127.0.0.1:%d/", collectorconstants.PushPort),
					"--kubeconfig=" + volumeMountPathGenericKubeconfig + "/kubeconfig",
					"--logtostderr=true",
					"--v=6",
				},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("5m"),
						corev1.ResourceMemory: resource.MustParse("30Mi"),
					},
				},
				SecurityContext: &corev1.SecurityContext{
					AllowPrivilegeEscalation: ptr.To(false),
					RunAsUser:                ptr.To[int64](65532),
					RunAsGroup:               ptr.To[int64](65534),
					RunAsNonRoot:             ptr.To(true),
					ReadOnlyRootFilesystem:   ptr.To(true),
				},
				Ports: []corev1.ContainerPort{
					{
						Name:          kubeRBACProxyName,
						ContainerPort: collectorconstants.KubeRBACProxyPort,
						Protocol:      corev1.ProtocolTCP,
					},
				},
				VolumeMounts: []corev1.VolumeMount{
					volumeMount,
				},
			},
		)
	}

	return obj
}

func getLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelRole:  v1beta1constants.LabelObservability,
		v1beta1constants.GardenRole: v1beta1constants.GardenRoleObservability,
		gardenerutils.NetworkPolicyLabel(valiconstants.ServiceName, valiconstants.ValiPort): v1beta1constants.LabelNetworkPolicyAllowed,
		v1beta1constants.LabelNetworkPolicyToDNS:                                            v1beta1constants.LabelNetworkPolicyAllowed,
		v1beta1constants.LabelNetworkPolicyToRuntimeAPIServer:                               v1beta1constants.LabelNetworkPolicyAllowed,
		v1beta1constants.LabelObservabilityApplication:                                      "opentelemetry-collector",
	}
}
