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
	"fmt"
	"strings"

	"github.com/gardener/gardener/pkg/operation/botanist/component/vpnseedserver"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretutils "github.com/gardener/gardener/pkg/utils/secrets"
	versionutils "github.com/gardener/gardener/pkg/utils/version"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	apiserverv1alpha1 "k8s.io/apiserver/pkg/apis/apiserver/v1alpha1"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
)

const (
	configMapAdmissionNamePrefix = "kube-apiserver-admission-config"
	configMapAdmissionDataKey    = "admission-configuration.yaml"

	configMapAuditPolicyNamePrefix = "audit-policy-config"
	configMapAuditPolicyDataKey    = "audit-policy.yaml"

	configMapEgressSelectorNamePrefix = "kube-apiserver-egress-selector-config"
	configMapEgressSelectorDataKey    = "egress-selector-configuration.yaml"
)

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
	defaultPolicy := &auditv1.Policy{
		Rules: []auditv1.PolicyRule{
			{Level: auditv1.LevelNone},
		},
	}

	data, err := runtime.Encode(codec, defaultPolicy)
	if err != nil {
		return err
	}
	policy := string(data)

	if k.values.Audit != nil && k.values.Audit.Policy != nil {
		policy = *k.values.Audit.Policy
	}

	configMap.Data = map[string]string{configMapAuditPolicyDataKey: policy}
	utilruntime.Must(kutil.MakeUnique(configMap))

	return kutil.IgnoreAlreadyExists(k.client.Client().Create(ctx, configMap))
}

func (k *kubeAPIServer) reconcileConfigMapEgressSelector(ctx context.Context, configMap *corev1.ConfigMap) error {
	if !k.values.VPN.ReversedVPNEnabled {
		// We don't delete the confimap here as we don't know its name (as it's unique). Instead, we rely on the usual
		// garbage collection for unique secrets/configmaps.
		return nil
	}

	egressSelectionControlPlaneName := "controlplane"
	if versionutils.ConstraintK8sLess120.Check(k.values.Version) {
		egressSelectionControlPlaneName = "master"
	}

	egressSelectorConfig := &apiserverv1alpha1.EgressSelectorConfiguration{
		EgressSelections: []apiserverv1alpha1.EgressSelection{
			{
				Name: "cluster",
				Connection: apiserverv1alpha1.Connection{
					ProxyProtocol: apiserverv1alpha1.ProtocolHTTPConnect,
					Transport: &apiserverv1alpha1.Transport{
						TCP: &apiserverv1alpha1.TCPTransport{
							URL: fmt.Sprintf("https://%s:%d", vpnseedserver.ServiceName, vpnseedserver.EnvoyPort),
							TLSConfig: &apiserverv1alpha1.TLSConfig{
								CABundle:   fmt.Sprintf("%s/%s", volumeMountPathCAVPN, secretutils.DataKeyCertificateBundle),
								ClientCert: fmt.Sprintf("%s/%s", volumeMountPathHTTPProxy, secretutils.DataKeyCertificate),
								ClientKey:  fmt.Sprintf("%s/%s", volumeMountPathHTTPProxy, secretutils.DataKeyPrivateKey),
							},
						},
					},
				},
			},
			{
				Name:       egressSelectionControlPlaneName,
				Connection: apiserverv1alpha1.Connection{ProxyProtocol: apiserverv1alpha1.ProtocolDirect},
			},
			{
				Name:       "etcd",
				Connection: apiserverv1alpha1.Connection{ProxyProtocol: apiserverv1alpha1.ProtocolDirect},
			},
		},
	}

	data, err := runtime.Encode(codec, egressSelectorConfig)
	if err != nil {
		return err
	}

	configMap.Data = map[string]string{configMapEgressSelectorDataKey: string(data)}
	utilruntime.Must(kutil.MakeUnique(configMap))

	return kutil.IgnoreAlreadyExists(k.client.Client().Create(ctx, configMap))
}
