// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package admissioncontroller

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"

	admissioncontrollerconfigv1alpha1 "github.com/gardener/gardener/pkg/admissioncontroller/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/logger"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

const dataConfigKey = "config.yaml"

var admissionServerCodec runtime.Codec

func init() {
	admissionServerScheme := runtime.NewScheme()
	utilruntime.Must(admissioncontrollerconfigv1alpha1.AddToScheme(admissionServerScheme))

	var (
		ser = json.NewSerializerWithOptions(json.DefaultMetaFactory, admissionServerScheme, admissionServerScheme, json.SerializerOptions{
			Yaml:   true,
			Pretty: false,
			Strict: false,
		})
		versions = schema.GroupVersions([]schema.GroupVersion{
			admissioncontrollerconfigv1alpha1.SchemeGroupVersion,
		})
	)

	admissionServerCodec = serializer.NewCodecFactory(admissionServerScheme).CodecForVersions(ser, ser, versions, versions)
}

func (a *gardenerAdmissionController) admissionConfigConfigMap() (*corev1.ConfigMap, error) {
	admissionConfig := &admissioncontrollerconfigv1alpha1.AdmissionControllerConfiguration{
		GardenClientConnection: componentbaseconfigv1alpha1.ClientConnectionConfiguration{
			QPS:        100,
			Burst:      130,
			Kubeconfig: gardenerutils.PathGenericKubeconfig,
		},
		LogLevel:  a.values.LogLevel,
		LogFormat: logger.FormatJSON,
		Server: admissioncontrollerconfigv1alpha1.ServerConfiguration{
			Webhooks: admissioncontrollerconfigv1alpha1.HTTPSServer{
				Server: admissioncontrollerconfigv1alpha1.Server{Port: serverPort},
				TLS:    admissioncontrollerconfigv1alpha1.TLSServer{ServerCertDir: volumeMountPathServerCert},
			},
			HealthProbes:                   &admissioncontrollerconfigv1alpha1.Server{Port: probePort},
			Metrics:                        &admissioncontrollerconfigv1alpha1.Server{Port: metricsPort},
			ResourceAdmissionConfiguration: a.values.ResourceAdmissionConfiguration,
		},
	}

	data, err := runtime.Encode(admissionServerCodec, admissionConfig)
	if err != nil {
		return nil, err
	}

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DeploymentName,
			Namespace: a.namespace,
			Labels:    GetLabels(),
		},
		Data: map[string]string{
			dataConfigKey: string(data),
		},
	}

	utilruntime.Must(kubernetesutils.MakeUnique(configMap))
	return configMap, nil
}
