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

package deployment_test

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	versionutils "github.com/gardener/gardener/pkg/utils/version"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apiserverv1alpha1 "k8s.io/apiserver/pkg/apis/apiserver/v1alpha1"
	"sigs.k8s.io/yaml"
)

func expectEgressSelectorConfigMap(ctx context.Context, valuesProvider KubeAPIServerValuesProvider) *string {
	if !valuesProvider.IsKonnectivityTunnelEnabled() {
		return nil
	}

	mockSeedClient.EXPECT().Get(ctx, kutil.Key(defaultSeedNamespace, "kube-apiserver-egress-selector-configuration"), gomock.AssignableToTypeOf(&corev1.ConfigMap{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "foo"))

	connectionNameControlPlane := "controlplane"
	if ok, _ := versionutils.CompareVersions(valuesProvider.GetShootKubernetesVersion(), "<", "1.20"); ok {
		connectionNameControlPlane = "master"
	}

	var clusterTransport apiserverv1alpha1.Transport
	if valuesProvider.IsSNIEnabled() {
		clusterTransport = apiserverv1alpha1.Transport{
			TCP: &apiserverv1alpha1.TCPTransport{
				URL: "https://konnectivity-server:9443",
				TLSConfig: &apiserverv1alpha1.TLSConfig{
					CABundle:   "/srv/kubernetes/ca/ca.crt",
					ClientKey:  "/etc/srv/kubernetes/konnectivity-server-client-tls/tls.key",
					ClientCert: "/etc/srv/kubernetes/konnectivity-server-client-tls/tls.crt",
				},
			},
		}
	} else {
		clusterTransport = apiserverv1alpha1.Transport{
			UDS: &apiserverv1alpha1.UDSTransport{
				UDSName: "/etc/srv/kubernetes/konnectivity-server/konnectivity-server.socket",
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

	encoder := kubernetes.SeedCodec.EncoderForVersion(kubernetes.SeedSerializer, apiserverv1alpha1.SchemeGroupVersion)
	buf := new(bytes.Buffer)
	err := encoder.Encode(&egressSelectorConfig, buf)
	Expect(err).ToNot(HaveOccurred())

	// will be mounted by the API server containers
	expectedConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-apiserver-egress-selector-configuration",
			Namespace: defaultSeedNamespace,
		},
		Data: map[string]string{
			"egress-selector-configuration.yaml": `---
` + buf.String(),
		},
	}

	mockSeedClient.EXPECT().Create(ctx, expectedConfigMap).Times(1)
	sha := getSHA(expectedConfigMap)
	return &sha
}

func expectAdmissionConfigMap(ctx context.Context, valueProvider KubeAPIServerValuesProvider) string {
	apiServerConfig := valueProvider.GetAPIServerConfig()
	if apiServerConfig == nil || len(apiServerConfig.AdmissionPlugins) == 0 {
		return expectDefaultAdmissionConfigMap(ctx)
	}

	// only support one admission plugin in the test for simplicity reasons
	return expectAdmissionConfigMapWithPlugin(ctx, apiServerConfig.AdmissionPlugins[0])
}

func expectAdmissionConfigMapWithPlugin(ctx context.Context, plugin gardencorev1beta1.AdmissionPlugin) string {
	pluginName := plugin.Name
	pluginConfig := plugin.Config.Raw

	mockSeedClient.EXPECT().Get(ctx, kutil.Key(defaultSeedNamespace, "kube-apiserver-admission-config"), gomock.AssignableToTypeOf(&corev1.ConfigMap{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "foo"))

	plugins := fmt.Sprintf(`---
apiVersion: apiserver.k8s.io/v1alpha1
kind: AdmissionConfiguration
plugins:
- configuration: null
  name: %s
  path: /etc/kubernetes/admission/%s.yaml
`, pluginName, strings.ToLower(pluginName))

	conf, err := yaml.JSONToYAML(pluginConfig)
	Expect(err).ToNot(HaveOccurred())

	expectedPluginConfig := `---
` + string(conf)

	pluginConfigFilename := fmt.Sprintf("%s.yaml", strings.ToLower(pluginName))
	expectedAdmissionConfig := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-apiserver-admission-config",
			Namespace: defaultSeedNamespace,
		},
		Data: map[string]string{
			"admission-configuration.yaml": plugins,
			pluginConfigFilename:           expectedPluginConfig,
		},
	}
	mockSeedClient.EXPECT().Create(ctx, expectedAdmissionConfig).Times(1)

	return getSHA(expectedAdmissionConfig)
}

func expectDefaultAdmissionConfigMap(ctx context.Context) string {
	mockSeedClient.EXPECT().Get(ctx, kutil.Key(defaultSeedNamespace, "kube-apiserver-admission-config"), gomock.AssignableToTypeOf(&corev1.ConfigMap{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "foo"))

	expectedDefaultAdmissionConfig := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-apiserver-admission-config",
			Namespace: defaultSeedNamespace,
		},
		Data: map[string]string{
			"admission-configuration.yaml": `---
apiVersion: apiserver.k8s.io/v1alpha1
kind: AdmissionConfiguration
plugins: []
`,
		},
	}
	mockSeedClient.EXPECT().Create(ctx, expectedDefaultAdmissionConfig).Times(1)
	return getSHA(expectedDefaultAdmissionConfig)
}

func getSHA(config *corev1.ConfigMap) string {
	encoder := kubernetes.SeedCodec.EncoderForVersion(kubernetes.SeedSerializer, corev1.SchemeGroupVersion)
	buf := new(bytes.Buffer)
	err := encoder.Encode(config, buf)
	Expect(err).ToNot(HaveOccurred())
	sha256Hex := utils.ComputeSHA256Hex(buf.Bytes())
	return sha256Hex
}
