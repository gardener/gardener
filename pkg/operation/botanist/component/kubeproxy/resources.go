// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubeproxy

import (
	"fmt"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	kubeproxyconfigv1alpha1 "k8s.io/kube-proxy/config/v1alpha1"
	"k8s.io/utils/pointer"
)

var (
	//go:embed resources/conntrack-fix.sh
	conntrackFixScript string
	//go:embed resources/cleanup.sh
	cleanupScript string
)

func (k *kubeProxy) computeCentralResourcesData() (map[string][]byte, error) {
	componentConfigRaw, err := k.getRawComponentConfig()
	if err != nil {
		return nil, err
	}

	var (
		registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)

		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kube-proxy",
				Namespace: metav1.NamespaceSystem,
			},
			AutomountServiceAccountToken: pointer.Bool(false),
		}

		service = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceName,
				Namespace: metav1.NamespaceSystem,
				Labels:    getLabels(),
			},
			Spec: corev1.ServiceSpec{
				Type:      corev1.ServiceTypeClusterIP,
				ClusterIP: corev1.ClusterIPNone,
				Ports: []corev1.ServicePort{{
					Name:     portNameMetrics,
					Port:     int32(portMetrics),
					Protocol: corev1.ProtocolTCP,
				}},
				Selector: getLabels(),
			},
		}

		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kube-proxy",
				Namespace: metav1.NamespaceSystem,
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{dataKeyKubeconfig: k.values.Kubeconfig},
		}

		configMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ConfigNamePrefix,
				Namespace: metav1.NamespaceSystem,
			},
			Data: map[string]string{dataKeyConfig: componentConfigRaw},
		}
	)

	utilruntime.Must(kutil.MakeUnique(secret))
	utilruntime.Must(kutil.MakeUnique(configMap))

	return registry.AddAllAndSerialize(
		serviceAccount,
		service,
		secret,
		configMap,
	)
}

func (k *kubeProxy) computePoolResourcesData(pool WorkerPool) (map[string][]byte, error) {
	var (
		registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)
	)

	return registry.AddAllAndSerialize()
}

func getLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp:  v1beta1constants.LabelKubernetes,
		v1beta1constants.LabelRole: v1beta1constants.LabelProxy,
	}
}

func (k *kubeProxy) getRawComponentConfig() (string, error) {
	config := &kubeproxyconfigv1alpha1.KubeProxyConfiguration{
		ClientConnection: componentbaseconfigv1alpha1.ClientConnectionConfiguration{
			Kubeconfig: volumeMountPathKubeconfig + "/" + dataKeyKubeconfig,
		},
		MetricsBindAddress: fmt.Sprintf("0.0.0.0:%d", portMetrics),
		Mode:               k.getMode(),
		Conntrack: kubeproxyconfigv1alpha1.KubeProxyConntrackConfiguration{
			MaxPerCore: pointer.Int32(524288),
		},
		FeatureGates: k.values.FeatureGates,
	}

	if !k.values.IPVSEnabled && k.values.PodNetworkCIDR != nil {
		config.ClusterCIDR = *k.values.PodNetworkCIDR
	}

	return NewConfigCodec().Encode(config)
}

func (k *kubeProxy) getMode() kubeproxyconfigv1alpha1.ProxyMode {
	if k.values.IPVSEnabled {
		return "ipvs"
	}
	return "iptables"
}
