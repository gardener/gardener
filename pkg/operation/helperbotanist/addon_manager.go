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
		"CloudConfigContent":       cloudConfig.Files,
		"CoreAddonsContent":        coreAddons.Files,
		"AdmissionControlsContent": admissionControls.Files,
		"OptionalAddonsContent":    optionalAddons.Files,
		"PodAnnotations": map[string]interface{}{
			"checksum/secret-kube-addon-manager": b.CheckSums[name],
		},
	}

	return b.ApplyChartSeed(filepath.Join(common.ChartPath, "seed-controlplane", "charts", name), name, b.Shoot.SeedNamespace, defaultValues, nil)
}

// generateCloudConfigChart renders the kube-addon-manager configuration for the cloud config user data.
// It will be stored as a Secret and mounted into the Pod. The configuration contains
// specially labelled Kubernetes manifests which will be created and periodically reconciled.
func (b *HelperBotanist) generateCloudConfigChart() (*chartrenderer.RenderedChart, error) {
	var (
		kubeletSecret = b.Secrets["kubelet"]
		cloudProvider = map[string]interface{}{
			"name": b.CloudBotanist.GetCloudProviderName(),
		}
		serviceNetwork = b.Shoot.GetServiceNetwork()
		userDataConfig = b.CloudBotanist.GenerateCloudConfigUserDataConfig()
	)

	if userDataConfig.CloudConfig {
		cloudProviderConfig, err := b.CloudBotanist.GenerateCloudProviderConfig()
		if err != nil {
			return nil, err
		}
		cloudProvider["config"] = cloudProviderConfig
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
	calico, err := b.CloudBotanist.GenerateCalicoConfig()
	if err != nil {
		return nil, err
	}

	var (
		kubeProxySecret  = b.Secrets["kube-proxy"]
		sshKeyPairSecret = b.Secrets["vpn-ssh-keypair"]
		global           = map[string]interface{}{
			"PodNetwork": b.Shoot.GetPodNetwork(),
		}
		kubeDNS = map[string]interface{}{
			"ClusterDNS": common.ComputeClusterIP(b.Shoot.GetServiceNetwork(), 10),
		}
		kubeProxy = map[string]interface{}{
			"Kubeconfig":   kubeProxySecret.Data["kubeconfig"],
			"FeatureGates": b.Shoot.Info.Spec.Kubernetes.KubeProxy.FeatureGates,
		}
		vpnShoot = map[string][]byte{
			"authorizedKeys": sshKeyPairSecret.Data["id_rsa.pub"],
		}
		rbac = map[string]interface{}{
			"enabled": b.Shoot.CloudProvider == gardenv1beta1.CloudProviderGCP,
		}
	)

	return b.ChartShootRenderer.Render(filepath.Join(common.ChartPath, "shoot-core"), "shoot-core", metav1.NamespaceSystem, map[string]interface{}{
		"global":     global,
		"kube-dns":   kubeDNS,
		"kube-proxy": kubeProxy,
		"vpn-shoot":  vpnShoot,
		"calico":     calico,
		"rbac":       rbac,
	})
}

// generateOptionalAddonsChart renders the kube-addon-manager chart for the optional addons. It
// will be stored as a Secret (as it may contain credentials) and mounted into the Pod. The configuration
// contains specially labelled Kubernetes manifests which will be created and periodically reconciled.
func (b *HelperBotanist) generateOptionalAddonsChart() (*chartrenderer.RenderedChart, error) {
	clusterAutoscaler, err := b.CloudBotanist.GenerateClusterAutoscalerConfig()
	if err != nil {
		return nil, err
	}
	heapster, err := b.Botanist.GenerateHeapsterConfig()
	if err != nil {
		return nil, err
	}
	helmTiller, err := b.Botanist.GenerateHelmTillerConfig()
	if err != nil {
		return nil, err
	}
	kubeLego, err := b.Botanist.GenerateKubeLegoConfig()
	if err != nil {
		return nil, err
	}
	kube2IAM, err := b.CloudBotanist.GenerateKube2IAMConfig()
	if err != nil {
		return nil, err
	}
	kubernetesDashboard, err := b.Botanist.GenerateKubernetesDashboardConfig()
	if err != nil {
		return nil, err
	}
	monocular, err := b.Botanist.GenerateMonocularConfig()
	if err != nil {
		return nil, err
	}
	nginxIngress, err := b.CloudBotanist.GenerateNginxIngressConfig()
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
	config, err := b.CloudBotanist.GenerateAdmissionControlConfig()
	if err != nil {
		return nil, err
	}

	return b.ChartShootRenderer.Render(filepath.Join(common.ChartPath, "shoot-admission-controls"), "admission-controls", metav1.NamespaceSystem, config)
}
