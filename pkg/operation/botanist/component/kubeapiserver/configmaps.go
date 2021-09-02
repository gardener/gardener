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

package kubeapiserver

import (
	"context"
	"strconv"
	"strings"

	"github.com/gardener/gardener/pkg/operation/botanist/component/vpnseedserver"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretutils "github.com/gardener/gardener/pkg/utils/secrets"
	versionutils "github.com/gardener/gardener/pkg/utils/version"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	apiserverv1alpha1 "k8s.io/apiserver/pkg/apis/apiserver/v1alpha1"
)

const (
	configMapAdmissionNamePrefix = "kube-apiserver-admission-config"
	configMapAdmissionDataKey    = "admission-configuration.yaml"

	configMapAuditPolicyNamePrefix = "audit-policy-config"
	configMapAuditPolicyDataKey    = "audit-policy.yaml"

	configMapEgressSelectorNamePrefix = "kube-apiserver-egress-selector-config"
	configMapEgressSelectorDataKey    = "egress-selector-configuration.yaml"
)

var (
	scheme *runtime.Scheme
	codec  runtime.Codec
)

func init() {
	scheme = runtime.NewScheme()
	utilruntime.Must(apiserverv1alpha1.AddToScheme(scheme))

	var (
		ser = json.NewSerializerWithOptions(json.DefaultMetaFactory, scheme, scheme, json.SerializerOptions{
			Yaml:   true,
			Pretty: false,
			Strict: false,
		})
		versions = schema.GroupVersions([]schema.GroupVersion{apiserverv1alpha1.SchemeGroupVersion})
	)

	codec = serializer.NewCodecFactory(scheme).CodecForVersions(ser, ser, versions, versions)
}

func (k *kubeAPIServer) emptyConfigMap(name string) *corev1.ConfigMap {
	return &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: k.namespace}}
}

func (k *kubeAPIServer) reconcileConfigMapAdmission(ctx context.Context, configMap *corev1.ConfigMap) error {
	configMap.Data = map[string]string{}

	admissionConfig := &apiserverv1alpha1.AdmissionConfiguration{}
	for _, plugin := range k.values.AdmissionPlugins {
		if plugin.Config == nil {
			continue
		}

		admissionConfig.Plugins = append(admissionConfig.Plugins, apiserverv1alpha1.AdmissionPluginConfiguration{
			Name: plugin.Name,
			Path: volumeMountPathAdmissionConfiguration + "/" + admissionPluginsConfigFilename(plugin.Name),
		})

		configMap.Data[admissionPluginsConfigFilename(plugin.Name)] = string(plugin.Config.Raw)
	}

	data, err := runtime.Encode(codec, admissionConfig)
	if err != nil {
		return err
	}
	configMap.Data[configMapAdmissionDataKey] = string(data)

	utilruntime.Must(kutil.MakeUnique(configMap))

	return kutil.IgnoreAlreadyExists(k.client.Client().Create(ctx, configMap))
}

func admissionPluginsConfigFilename(name string) string {
	return strings.ToLower(name) + ".yaml"
}

func (k *kubeAPIServer) reconcileConfigMapAuditPolicy(ctx context.Context, configMap *corev1.ConfigMap) error {
	policy := `---
apiVersion: audit.k8s.io/v1
kind: Policy
rules:
- level: None
`

	if k.values.Audit != nil && k.values.Audit.Policy != nil {
		policy = *k.values.Audit.Policy
	}

	configMap.Data = map[string]string{configMapAuditPolicyDataKey: policy}
	utilruntime.Must(kutil.MakeUnique(configMap))

	return kutil.IgnoreAlreadyExists(k.client.Client().Create(ctx, configMap))
}

func (k *kubeAPIServer) reconcileConfigMapEgressSelector(ctx context.Context, configMap *corev1.ConfigMap) error {
	if !k.values.ReversedVPNEnabled {
		// We don't delete the confimap here as we don't know its name (as it's unique). Instead, we rely on the usual
		// garbage collection for unique secrets/configmaps.
		return nil
	}

	egressSelectionControlPlaneName := "controlplane"
	if versionutils.ConstraintK8sLess120.Check(k.values.Version) {
		egressSelectionControlPlaneName = "master"
	}

	configMap.Data = map[string]string{configMapEgressSelectorDataKey: `---
apiVersion: apiserver.k8s.io/v1alpha1
  kind: EgressSelectorConfiguration
  egressSelections:
  - name: cluster
    connection:
      proxyProtocol: HTTPConnect
      transport:
        tcp:
          url: https://` + vpnseedserver.ServiceName + `:` + strconv.Itoa(vpnseedserver.EnvoyPort) + `
          tlsConfig:
            caBundle: ` + volumeMountPathHTTPProxy + `/` + secretutils.DataKeyCertificateCA + `
            clientCert: ` + volumeMountPathHTTPProxy + `/` + secretutils.DataKeyCertificate + `
            clientKey: ` + volumeMountPathHTTPProxy + `/` + secretutils.DataKeyPrivateKey + `
  - name: ` + egressSelectionControlPlaneName + `
    connection:
      proxyProtocol: Direct
  - name: etcd
    connection:
      proxyProtocol: Direct
`}
	utilruntime.Must(kutil.MakeUnique(configMap))

	return kutil.IgnoreAlreadyExists(k.client.Client().Create(ctx, configMap))
}
