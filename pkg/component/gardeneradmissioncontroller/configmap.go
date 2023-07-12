// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gardeneradmissioncontroller

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"

	admissioncontrollerv1alpha1 "github.com/gardener/gardener/pkg/admissioncontroller/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/logger"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

var (
	admissionServerCodec runtime.Codec
)

const (
	dataConfigKey = "config.yaml"
)

func init() {
	admissionServerScheme := runtime.NewScheme()
	utilruntime.Must(admissioncontrollerv1alpha1.AddToScheme(admissionServerScheme))

	var (
		ser = json.NewSerializerWithOptions(json.DefaultMetaFactory, admissionServerScheme, admissionServerScheme, json.SerializerOptions{
			Yaml:   true,
			Pretty: false,
			Strict: false,
		})
		versions = schema.GroupVersions([]schema.GroupVersion{
			admissioncontrollerv1alpha1.SchemeGroupVersion,
		})
	)

	admissionServerCodec = serializer.NewCodecFactory(admissionServerScheme).CodecForVersions(ser, ser, versions, versions)
}

func (a admissioncontroller) admissionConfigConfigMap() (*corev1.ConfigMap, error) {
	admissionConfig := &admissioncontrollerv1alpha1.AdmissionControllerConfiguration{
		GardenClientConnection: componentbaseconfigv1alpha1.ClientConnectionConfiguration{
			QPS:        a.values.ClientConnection.QPS,
			Burst:      a.values.ClientConnection.Burst,
			Kubeconfig: gardenerutils.PathGenericKubeconfig,
		},
		LogLevel:  a.values.LogLevel,
		LogFormat: logger.FormatJSON,
		Server: admissioncontrollerv1alpha1.ServerConfiguration{
			Webhooks: admissioncontrollerv1alpha1.HTTPSServer{
				Server: admissioncontrollerv1alpha1.Server{Port: serverPort},
				TLS:    admissioncontrollerv1alpha1.TLSServer{ServerCertDir: volumeMountPathServerCert},
			},
			HealthProbes:                   &admissioncontrollerv1alpha1.Server{Port: healthzPort},
			Metrics:                        &admissioncontrollerv1alpha1.Server{Port: metricsPort},
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
		},
		Data: map[string]string{
			"config.yaml": string(data),
		},
	}

	utilruntime.Must(kubernetesutils.MakeUnique(configMap))
	return configMap, nil
}
