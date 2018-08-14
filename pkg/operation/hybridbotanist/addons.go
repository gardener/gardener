// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package hybridbotanist

import (
	"path/filepath"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/apis/garden/v1beta1/helper"
	"github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// generateCoreAddonsChart renders the kube-addon-manager configuration for the core addons. It will be
// stored as a Secret (as it may contain credentials) and mounted into the Pod. The configuration contains
// specially labelled Kubernetes manifests which will be created and periodically reconciled.
func (b *HybridBotanist) generateCoreAddonsChart() (*chartrenderer.RenderedChart, error) {
	var (
		kubeProxySecret  = b.Secrets["kube-proxy"]
		vpnShootSecret   = b.Secrets["vpn-shoot"]
		vpnTLSAuthSecret = b.Secrets["vpn-seed-tlsauth"]
		global           = map[string]interface{}{
			"podNetwork": b.Shoot.GetPodNetwork(),
		}
		calicoConfig = map[string]interface{}{
			"cloudProvider": b.Shoot.CloudProvider,
		}

		kubeDNSConfig = map[string]interface{}{
			"clusterDNS": common.ComputeClusterIP(b.Shoot.GetServiceNetwork(), 10),
			// TODO: resolve conformance test issue before changing:
			// https://github.com/kubernetes/kubernetes/blob/master/test/e2e/network/dns.go#L44
			"domain": gardenv1beta1.DefaultDomain,
		}
		kubeProxyConfig = map[string]interface{}{
			"kubeconfig": kubeProxySecret.Data["kubeconfig"],
			"podAnnotations": map[string]interface{}{
				"checksum/secret-kube-proxy": b.CheckSums["kube-proxy"],
			},
		}
		vpnShootConfig = map[string]interface{}{
			"podNetwork":     b.Shoot.GetPodNetwork(),
			"serviceNetwork": b.Shoot.GetServiceNetwork(),
			"nodeNetwork":    b.Shoot.GetNodeNetwork(),
			"tlsAuth":        vpnTLSAuthSecret.Data["vpn.tlsauth"],
			"podAnnotations": map[string]interface{}{
				"checksum/secret-vpn-shoot": b.CheckSums["vpn-shoot"],
			},
		}
		nodeExporterConfig = map[string]interface{}{}
	)

	proxyConfig := b.Shoot.Info.Spec.Kubernetes.KubeProxy
	if proxyConfig != nil {
		kubeProxyConfig["featureGates"] = proxyConfig.FeatureGates
	}

	if openvpnDiffieHellmanSecret, ok := b.Secrets[common.GardenRoleOpenVPNDiffieHellman]; ok {
		vpnShootConfig["diffieHellmanKey"] = openvpnDiffieHellmanSecret.Data["dh2048.pem"]
	}

	calico, err := b.Botanist.InjectImages(calicoConfig, b.K8sShootClient.Version(), map[string]string{"calico-node": "calico-node", "calico-cni": "calico-cni", "calico-typha": "calico-typha"})
	if err != nil {
		return nil, err
	}
	kubeDNS, err := b.Botanist.InjectImages(kubeDNSConfig, b.K8sShootClient.Version(), map[string]string{"kube-dns": "kube-dns", "kube-dns-dnsmasq": "kube-dns-dnsmasq", "kube-dns-sidecar": "kube-dns-sidecar", "kube-dns-autoscaler": "cluster-proportional-autoscaler"})
	if err != nil {
		return nil, err
	}
	kubeProxy, err := b.Botanist.InjectImages(kubeProxyConfig, b.K8sShootClient.Version(), map[string]string{"hyperkube": "hyperkube"})
	if err != nil {
		return nil, err
	}
	vpnShoot, err := b.Botanist.InjectImages(vpnShootConfig, b.K8sShootClient.Version(), map[string]string{"vpn-shoot": "vpn-shoot"})
	if err != nil {
		return nil, err
	}
	nodeExporter, err := b.Botanist.InjectImages(nodeExporterConfig, b.K8sShootClient.Version(), map[string]string{"node-exporter": "node-exporter"})
	if err != nil {
		return nil, err
	}

	if _, err := b.K8sShootClient.CreateSecret(metav1.NamespaceSystem, "vpn-shoot", corev1.SecretTypeOpaque, vpnShootSecret.Data, true); err != nil {
		return nil, err
	}

	return b.ChartShootRenderer.Render(filepath.Join(common.ChartPath, "shoot-core"), "shoot-core", metav1.NamespaceSystem, map[string]interface{}{
		"global":     global,
		"kube-dns":   kubeDNS,
		"kube-proxy": kubeProxy,
		"vpn-shoot":  vpnShoot,
		"calico":     calico,
		"monitoring": map[string]interface{}{
			"node-exporter": nodeExporter,
		},
	})
}

// generateOptionalAddonsChart renders the kube-addon-manager chart for the optional addons. It
// will be stored as a Secret (as it may contain credentials) and mounted into the Pod. The configuration
// contains specially labelled Kubernetes manifests which will be created and periodically reconciled.
func (b *HybridBotanist) generateOptionalAddonsChart() (*chartrenderer.RenderedChart, error) {
	clusterAutoscalerConfig, err := b.Botanist.GenerateClusterAutoscalerConfig()
	if err != nil {
		return nil, err
	}
	heapsterConfig, err := b.Botanist.GenerateHeapsterConfig()
	if err != nil {
		return nil, err
	}
	helmTillerConfig, err := b.Botanist.GenerateHelmTillerConfig()
	if err != nil {
		return nil, err
	}
	kubeLegoConfig, err := b.Botanist.GenerateKubeLegoConfig()
	if err != nil {
		return nil, err
	}
	kube2IAMConfig, err := b.ShootCloudBotanist.GenerateKube2IAMConfig()
	if err != nil {
		return nil, err
	}
	kubernetesDashboardConfig, err := b.Botanist.GenerateKubernetesDashboardConfig()
	if err != nil {
		return nil, err
	}
	monocularConfig, err := b.Botanist.GenerateMonocularConfig()
	if err != nil {
		return nil, err
	}
	nginxIngressConfig, err := b.ShootCloudBotanist.GenerateNginxIngressConfig()
	if err != nil {
		return nil, err
	}
	if b.Shoot.NginxIngressEnabled() {
		nginxIngressConfig = utils.MergeMaps(nginxIngressConfig, map[string]interface{}{
			"controller": map[string]interface{}{
				"service": map[string]interface{}{
					"loadBalancerSourceRanges": b.Shoot.Info.Spec.Addons.NginxIngress.LoadBalancerSourceRanges,
				},
			},
		})

		if shootUsedAsSeed, _, _ := helper.IsUsedAsSeed(b.Shoot.Info); shootUsedAsSeed {
			nginxIngressConfig = utils.MergeMaps(nginxIngressConfig, map[string]interface{}{
				"controller": map[string]interface{}{
					"resources": map[string]interface{}{
						"limits": map[string]interface{}{
							"cpu":    "500m",
							"memory": "1024Mi",
						},
					},
				},
			})
		}
	}

	heapster, err := b.Botanist.InjectImages(heapsterConfig, b.K8sShootClient.Version(), map[string]string{"heapster": "heapster", "heapster-nanny": "addon-resizer"})
	if err != nil {
		return nil, err
	}
	helmTiller, err := b.Botanist.InjectImages(helmTillerConfig, b.K8sShootClient.Version(), map[string]string{"helm-tiller": "helm-tiller"})
	if err != nil {
		return nil, err
	}
	kubeLego, err := b.Botanist.InjectImages(kubeLegoConfig, b.K8sShootClient.Version(), map[string]string{"kube-lego": "kube-lego"})
	if err != nil {
		return nil, err
	}
	kube2IAM, err := b.Botanist.InjectImages(kube2IAMConfig, b.K8sShootClient.Version(), map[string]string{"kube2iam": "kube2iam"})
	if err != nil {
		return nil, err
	}
	kubernetesDashboard, err := b.Botanist.InjectImages(kubernetesDashboardConfig, b.K8sShootClient.Version(), map[string]string{"kubernetes-dashboard": "kubernetes-dashboard"})
	if err != nil {
		return nil, err
	}
	monocular, err := b.Botanist.InjectImages(monocularConfig, b.K8sShootClient.Version(), map[string]string{"monocular-api": "monocular-api", "monocular-ui": "monocular-ui", "busybox": "busybox"})
	if err != nil {
		return nil, err
	}
	nginxIngress, err := b.Botanist.InjectImages(nginxIngressConfig, b.K8sShootClient.Version(), map[string]string{"nginx-ingress-controller": "nginx-ingress-controller", "ingress-default-backend": "ingress-default-backend"})
	if err != nil {
		return nil, err
	}

	return b.ChartShootRenderer.Render(filepath.Join(common.ChartPath, "shoot-addons"), "addons", metav1.NamespaceSystem, map[string]interface{}{
		"cluster-autoscaler":   clusterAutoscalerConfig,
		"heapster":             heapster,
		"helm-tiller":          helmTiller,
		"kube-lego":            kubeLego,
		"kube2iam":             kube2IAM,
		"kubernetes-dashboard": kubernetesDashboard,
		"monocular":            monocular,
		"nginx-ingress":        nginxIngress,
	})
}

// generateAdmissionControlsChart renders the kube-addon-manager configuration for the admission control
// extensions. It will be stored as a ConfigMap and mounted into the Pod. The configuration contains
// specially labelled Kubernetes manifests which will be created and periodically reconciled.
func (b *HybridBotanist) generateAdmissionControlsChart() (*chartrenderer.RenderedChart, error) {
	config, err := b.ShootCloudBotanist.GenerateAdmissionControlConfig()
	if err != nil {
		return nil, err
	}

	return b.ChartShootRenderer.Render(filepath.Join(common.ChartPath, "shoot-admission-controls"), "admission-controls", metav1.NamespaceSystem, config)
}
