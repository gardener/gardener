// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package deployment

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/controlplane/konnectivity"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/secrets"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiserverv1alpha1 "k8s.io/apiserver/pkg/apis/apiserver/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/yaml"
)

var encoderAPIServerV1alpha1 = kubernetes.SeedCodec.EncoderForVersion(kubernetes.SeedSerializer, apiserverv1alpha1.SchemeGroupVersion)

func (k *kubeAPIServer) deployEgressSelectorConfigMap(ctx context.Context) (*string, error) {
	var (
		buf       = new(bytes.Buffer)
		sha256Hex string
	)

	connectionNameControlPlane := "controlplane"
	if versionConstraintK8sSmaller120.Check(k.shootKubernetesVersion) {
		connectionNameControlPlane = "master"
	}

	var clusterTransport apiserverv1alpha1.Transport
	if k.sniValues.SNIEnabled {
		clusterTransport = apiserverv1alpha1.Transport{
			TCP: &apiserverv1alpha1.TCPTransport{
				URL: fmt.Sprintf("https://%s:%d", konnectivity.ServiceName, konnectivity.ServerHTTPSPort),
				TLSConfig: &apiserverv1alpha1.TLSConfig{
					CABundle:   fmt.Sprintf("%s/%s", volumeMountPathCA, secrets.DataKeyCertificateCA),
					ClientKey:  fmt.Sprintf("%s/%s", volumeMountPathKonnectivityClientTLS, secrets.DataKeyPrivateKey),
					ClientCert: fmt.Sprintf("%s/%s", volumeMountPathKonnectivityClientTLS, secrets.DataKeyCertificate),
				},
			},
		}
	} else {
		clusterTransport = apiserverv1alpha1.Transport{
			UDS: &apiserverv1alpha1.UDSTransport{
				UDSName: fmt.Sprintf("%s/%s", volumeMountPathKonnectivityUDS, konnectivityUDSName),
			},
		}
	}

	egressSelectorConfig := apiserverv1alpha1.EgressSelectorConfiguration{
		EgressSelections: []apiserverv1alpha1.EgressSelection{
			{
				Name: "cluster",
				Connection: apiserverv1alpha1.Connection{
					ProxyProtocol: apiserverv1alpha1.ProtocolHTTPConnect,
					Transport:     &clusterTransport,
				},
			},
			{
				Name: connectionNameControlPlane,
				Connection: apiserverv1alpha1.Connection{
					ProxyProtocol: apiserverv1alpha1.ProtocolDirect,
				},
			},
			{
				Name: "etcd",
				Connection: apiserverv1alpha1.Connection{
					ProxyProtocol: apiserverv1alpha1.ProtocolDirect,
				},
			},
		},
	}

	if err := encoderAPIServerV1alpha1.Encode(&egressSelectorConfig, buf); err != nil {
		return nil, err
	}

	// will be mounted by the API server containers
	egressSelectorConfigMap := k.emptyConfigMap(cmNameKonnectivityEgressSelector)

	egressSelectorConfigMapSHA := *egressSelectorConfigMap
	egressSelectorConfigMapSHA.Data = map[string]string{
		fileNameKonnectivityEgressSelector: `---
` + buf.String(),
	}

	if _, err := controllerutil.CreateOrUpdate(ctx, k.seedClient.Client(), egressSelectorConfigMap, func() error {
		egressSelectorConfigMap.Data = egressSelectorConfigMapSHA.Data
		return nil
	}); err != nil {
		return nil, err
	}

	buf = new(bytes.Buffer)
	if err := encoderCoreV1.Encode(&egressSelectorConfigMapSHA, buf); err != nil {
		return nil, err
	}

	sha256Hex = utils.ComputeSHA256Hex(buf.Bytes())
	return &sha256Hex, nil
}

func (k *kubeAPIServer) deployAdmissionConfigMap(ctx context.Context, apiServerAdmissionPlugins []gardencorev1beta1.AdmissionPlugin) (*string, error) {
	var (
		buf                            = new(bytes.Buffer)
		sha256Hex                      string
		cmKubeApiserverAdmissionConfig = k.emptyConfigMap(cmNameAPIServerAdmissionConfig)
		admissionConfigData            = map[string]string{}
	)

	admissionConfiguration := apiserverv1alpha1.AdmissionConfiguration{
		Plugins: []apiserverv1alpha1.AdmissionPluginConfiguration{},
	}

	for _, plugin := range apiServerAdmissionPlugins {
		if plugin.Config == nil {
			continue
		}
		pluginConfigFileName := fmt.Sprintf("%s.yaml", strings.ToLower(plugin.Name))

		admissionConfiguration.Plugins = append(admissionConfiguration.Plugins, apiserverv1alpha1.AdmissionPluginConfiguration{
			Name: plugin.Name,
			Path: fmt.Sprintf("%s/%s", volumeMountPathAdmissionPluginConfig, pluginConfigFileName),
		})

		pluginConfig, err := yaml.JSONToYAML(plugin.Config.Raw)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal API server admission plugin configuration for plugin %q: %v", plugin.Name, err)
		}

		admissionConfigData[pluginConfigFileName] = `---
` + string(pluginConfig)
	}

	// encode AdmissionConfiguration
	if err := encoderAPIServerV1alpha1.Encode(&admissionConfiguration, buf); err != nil {
		return nil, fmt.Errorf("failed to encode the Shoot API server admission configuration config map (%s/%s): %v", k.seedNamespace, cmNameAPIServerAdmissionConfig, err)
	}

	admissionConfigData[fileNameAdmissionPluginConfiguration] = `---
` + buf.String()

	if _, err := controllerutil.CreateOrUpdate(ctx, k.seedClient.Client(), cmKubeApiserverAdmissionConfig, func() error {
		cmKubeApiserverAdmissionConfig.Data = admissionConfigData
		return nil
	}); err != nil {
		return nil, err
	}

	// encode config map to get SHA256
	buf = new(bytes.Buffer)
	if err := encoderCoreV1.Encode(cmKubeApiserverAdmissionConfig, buf); err != nil {
		return nil, fmt.Errorf("failed to encode the Shoot API server admission configuration config map (%s/%s): %v", k.seedNamespace, cmNameAPIServerAdmissionConfig, err)
	}
	sha256Hex = utils.ComputeSHA256Hex(buf.Bytes())
	return &sha256Hex, nil
}

func (k *kubeAPIServer) emptyConfigMap(name string) *corev1.ConfigMap {
	return &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: k.seedNamespace}}
}
