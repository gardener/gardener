// Copyright 2018 The Gardener Authors.
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

package helperbotanist

import (
	"path/filepath"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/operation/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeployKubeAddonManager deploys the Kubernetes Addon Manager which will use labelled Kubernetes resources in order
// to ensure that they exist in a cluster/reconcile them in case somebody changed something.
func (b *HelperBotanist) DeployKubeAddonManager() error {
	name := "kube-addon-manager"
	cloudConfig, err := b.generateCloudConfigChart()
	if err != nil {
		return err
	}
	coreAddons, err := b.generateCoreAddonsChart()
	if err != nil {
		return err
	}
	admissionControls, err := b.generateAdmissionControlsChart()
	if err != nil {
		return err
	}
	optionalAddons, err := b.generateOptionalAddonsChart()
	if err != nil {
		return err
	}

	defaultValues := map[string]interface{}{
		"cloudConfigContent":       cloudConfig.Files,
		"coreAddonsContent":        coreAddons.Files,
		"admissionControlsContent": admissionControls.Files,
		"optionalAddonsContent":    optionalAddons.Files,
		"podAnnotations": map[string]interface{}{
			"checksum/secret-kube-addon-manager": b.CheckSums[name],
		},
	}

	values, err := b.Botanist.InjectImages(defaultValues, b.K8sSeedClient.Version(), map[string]string{"kube-addon-manager": "kube-addon-manager"})
	if err != nil {
		return err
	}

	return b.ApplyChartSeed(filepath.Join(common.ChartPath, "seed-controlplane", "charts", name), name, b.Shoot.SeedNamespace, values, nil)
}

// generateCloudConfigChart renders the kube-addon-manager configuration for the cloud config user data.
// It will be stored as a Secret and mounted into the Pod. The configuration contains
// specially labelled Kubernetes manifests which will be created and periodically reconciled.
func (b *HelperBotanist) generateCloudConfigChart() (*chartrenderer.RenderedChart, error) {
	var (
		kubeletSecret = b.Secrets["kubelet"]
		cloudProvider = map[string]interface{}{
			"name": b.ShootCloudBotanist.GetCloudProviderName(),
		}
		serviceNetwork = b.Shoot.GetServiceNetwork()
		userDataConfig = b.ShootCloudBotanist.GenerateCloudConfigUserDataConfig()
	)

	if userDataConfig.CloudConfig {
		cloudProviderConfig, err := b.ShootCloudBotanist.GenerateCloudProviderConfig()
		if err != nil {
			return nil, err
		}
		cloudProvider["config"] = cloudProviderConfig
	}

	hyperKube, err := b.ImageVector.FindImage("hyperkube", b.Shoot.Info.Spec.Kubernetes.Version)
	if err != nil {
		return nil, err
	}

	config := map[string]interface{}{
		"cloudProvider": cloudProvider,
		"kubernetes": map[string]interface{}{
			"caCert":     string(kubeletSecret.Data["ca.crt"]),
			"clusterDNS": common.ComputeClusterIP(serviceNetwork, 10),
			"kubelet": map[string]interface{}{
				"kubeconfig":    string(kubeletSecret.Data["kubeconfig"]),
				"networkPlugin": userDataConfig.NetworkPlugin,
				"parameters":    userDataConfig.KubeletParameters,
				"featureGates":  b.Shoot.Info.Spec.Kubernetes.Kubelet.FeatureGates,
			},
			"nonMasqueradeCIDR": common.ComputeNonMasqueradeCIDR(serviceNetwork),
			"version":           b.Shoot.Info.Spec.Kubernetes.Version,
		},
		"images": map[string]interface{}{
			"hyperkube": hyperKube.String(),
		},
		"workers": userDataConfig.WorkerNames,
	}

	if userDataConfig.CABundle != "" {
		config["caBundle"] = userDataConfig.CABundle
	}

	return b.ChartShootRenderer.Render(filepath.Join(common.ChartPath, "shoot-cloud-config"), "shoot-cloud-config", metav1.NamespaceSystem, config)
}

// generateCoreAddonsChart renders the kube-addon-manager configuration for the core addons. It will be
// stored as a Secret (as it may contain credentials) and mounted into the Pod. The configuration contains
// specially labelled Kubernetes manifests which will be created and periodically reconciled.
func (b *HelperBotanist) generateCoreAddonsChart() (*chartrenderer.RenderedChart, error) {
	var (
		kubeProxySecret  = b.Secrets["kube-proxy"]
		sshKeyPairSecret = b.Secrets["vpn-ssh-keypair"]
		global           = map[string]interface{}{
			"podNetwork": b.Shoot.GetPodNetwork(),
		}
		rbac = map[string]interface{}{
			"enabled": b.Shoot.CloudProvider == gardenv1beta1.CloudProviderGCP,
		}

		kubeDNSConfig = map[string]interface{}{
			"clusterDNS": common.ComputeClusterIP(b.Shoot.GetServiceNetwork(), 10),
		}
		kubeProxyConfig = map[string]interface{}{
			"kubeconfig":   kubeProxySecret.Data["kubeconfig"],
			"featureGates": b.Shoot.Info.Spec.Kubernetes.KubeProxy.FeatureGates,
		}
		vpnShootConfig = map[string]interface{}{
			"authorizedKeys": sshKeyPairSecret.Data["id_rsa.pub"],
		}
		nodeExporterConfig = map[string]interface{}{}
	)

	calicoConfig, err := b.ShootCloudBotanist.GenerateCalicoConfig()
	if err != nil {
		return nil, err
	}

	calico, err := b.Botanist.InjectImages(calicoConfig, b.K8sShootClient.Version(), map[string]string{"calico-node": "calico-node", "calico-cni": "calico-cni"})
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

	return b.ChartShootRenderer.Render(filepath.Join(common.ChartPath, "shoot-core"), "shoot-core", metav1.NamespaceSystem, map[string]interface{}{
		"global":     global,
		"kube-dns":   kubeDNS,
		"kube-proxy": kubeProxy,
		"vpn-shoot":  vpnShoot,
		"calico":     calico,
		"rbac":       rbac,
		"monitoring": map[string]interface{}{
			"node-exporter": nodeExporter,
		},
	})
}

// generateOptionalAddonsChart renders the kube-addon-manager chart for the optional addons. It
// will be stored as a Secret (as it may contain credentials) and mounted into the Pod. The configuration
// contains specially labelled Kubernetes manifests which will be created and periodically reconciled.
func (b *HelperBotanist) generateOptionalAddonsChart() (*chartrenderer.RenderedChart, error) {
	clusterAutoscalerConfig, err := b.ShootCloudBotanist.GenerateClusterAutoscalerConfig()
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

	clusterAutoscaler, err := b.Botanist.InjectImages(clusterAutoscalerConfig, b.K8sShootClient.Version(), map[string]string{"hyperkube": "hyperkube", "cluster-autoscaler": "cluster-autoscaler"})
	if err != nil {
		return nil, err
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
	monocular, err := b.Botanist.InjectImages(monocularConfig, b.K8sShootClient.Version(), map[string]string{"monocular-api": "monocular-api", "monocular-ui": "monocular-ui", "monocular-prerender": "monocular-prerender", "busybox": "busybox"})
	if err != nil {
		return nil, err
	}
	nginxIngress, err := b.Botanist.InjectImages(nginxIngressConfig, b.K8sShootClient.Version(), map[string]string{"nginx-ingress-controller": "nginx-ingress-controller", "ingress-default-backend": "ingress-default-backend", "vts-ingress-exporter": "vts-ingress-exporter"})
	if err != nil {
		return nil, err
	}

	return b.ChartShootRenderer.Render(filepath.Join(common.ChartPath, "shoot-addons"), "addons", metav1.NamespaceSystem, map[string]interface{}{
		"cluster-autoscaler":   clusterAutoscaler,
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
func (b *HelperBotanist) generateAdmissionControlsChart() (*chartrenderer.RenderedChart, error) {
	config, err := b.ShootCloudBotanist.GenerateAdmissionControlConfig()
	if err != nil {
		return nil, err
	}

	return b.ChartShootRenderer.Render(filepath.Join(common.ChartPath, "shoot-admission-controls"), "admission-controls", metav1.NamespaceSystem, config)
}
