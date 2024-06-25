// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllermanager

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"k8s.io/utils/ptr"

	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/logger"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

const (
	configMapControllerManagerPrefix = "gardener-controller-manager-config"
	dataConfigKey                    = "config.yaml"
)

var controllerManagerCodec runtime.Codec

func init() {
	controllerManagerScheme := runtime.NewScheme()
	utilruntime.Must(controllermanagerconfigv1alpha1.AddToScheme(controllerManagerScheme))

	var (
		ser = json.NewSerializerWithOptions(json.DefaultMetaFactory, controllerManagerScheme, controllerManagerScheme, json.SerializerOptions{
			Yaml:   true,
			Pretty: false,
			Strict: false,
		})
		versions = schema.GroupVersions([]schema.GroupVersion{
			controllermanagerconfigv1alpha1.SchemeGroupVersion,
		})
	)

	controllerManagerCodec = serializer.NewCodecFactory(controllerManagerScheme).CodecForVersions(ser, ser, versions, versions)
}

func (g *gardenerControllerManager) configMapControllerManagerConfig() (*corev1.ConfigMap, error) {
	controllerManagerConfig := &controllermanagerconfigv1alpha1.ControllerManagerConfiguration{
		GardenClientConnection: componentbaseconfigv1alpha1.ClientConnectionConfiguration{
			QPS:        100,
			Burst:      130,
			Kubeconfig: gardenerutils.PathGenericKubeconfig,
		},
		Controllers: controllermanagerconfigv1alpha1.ControllerManagerControllerConfiguration{
			ControllerRegistration: &controllermanagerconfigv1alpha1.ControllerRegistrationControllerConfiguration{
				ConcurrentSyncs: ptr.To(20),
			},
			Project: &controllermanagerconfigv1alpha1.ProjectControllerConfiguration{
				ConcurrentSyncs: ptr.To(20),
				Quotas:          g.values.Quotas,
				// TODO: replace this hardcoded configuration with proper fields in the Garden API
				StaleExpirationTimeDays: ptr.To(6000),
			},
			SecretBinding: &controllermanagerconfigv1alpha1.SecretBindingControllerConfiguration{
				ConcurrentSyncs: ptr.To(20),
			},
			CredentialsBinding: &controllermanagerconfigv1alpha1.CredentialsBindingControllerConfiguration{
				ConcurrentSyncs: ptr.To(20),
			},
			Seed: &controllermanagerconfigv1alpha1.SeedControllerConfiguration{
				ConcurrentSyncs:    ptr.To(20),
				ShootMonitorPeriod: &metav1.Duration{Duration: 300 * time.Second},
			},
			SeedExtensionsCheck: &controllermanagerconfigv1alpha1.SeedExtensionsCheckControllerConfiguration{
				ConditionThresholds: []controllermanagerconfigv1alpha1.ConditionThreshold{{
					Duration: metav1.Duration{Duration: 1 * time.Minute},
					Type:     "ExtensionsReady",
				}},
			},
			SeedBackupBucketsCheck: &controllermanagerconfigv1alpha1.SeedBackupBucketsCheckControllerConfiguration{
				ConditionThresholds: []controllermanagerconfigv1alpha1.ConditionThreshold{{
					Duration: metav1.Duration{Duration: 1 * time.Minute},
					Type:     "BackupBucketsReady",
				}},
			},
			Event: &controllermanagerconfigv1alpha1.EventControllerConfiguration{
				ConcurrentSyncs:   ptr.To(10),
				TTLNonShootEvents: &metav1.Duration{Duration: 2 * time.Hour},
			},
			ShootMaintenance: controllermanagerconfigv1alpha1.ShootMaintenanceControllerConfiguration{
				ConcurrentSyncs: ptr.To(20),
			},
			ShootReference: &controllermanagerconfigv1alpha1.ShootReferenceControllerConfiguration{
				ConcurrentSyncs: ptr.To(20),
			},
		},
		LeaderElection: &componentbaseconfigv1alpha1.LeaderElectionConfiguration{
			LeaderElect:       ptr.To(true),
			ResourceName:      controllermanagerconfigv1alpha1.ControllerManagerDefaultLockObjectName,
			ResourceNamespace: metav1.NamespaceSystem,
		},
		LogLevel:  g.values.LogLevel,
		LogFormat: logger.FormatJSON,
		Server: controllermanagerconfigv1alpha1.ServerConfiguration{
			HealthProbes: &controllermanagerconfigv1alpha1.Server{Port: probePort},
			Metrics:      &controllermanagerconfigv1alpha1.Server{Port: metricsPort},
		},
		FeatureGates: g.values.FeatureGates,
	}

	data, err := runtime.Encode(controllerManagerCodec, controllerManagerConfig)
	if err != nil {
		return nil, err
	}

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapControllerManagerPrefix,
			Namespace: g.namespace,
			Labels:    GetLabels(),
		},
		Data: map[string]string{
			dataConfigKey: string(data),
		},
	}

	utilruntime.Must(kubernetesutils.MakeUnique(configMap))
	return configMap, nil
}
