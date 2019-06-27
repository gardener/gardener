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
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gardener/gardener-resource-manager/pkg/manager"
	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/secrets"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeployManagedResources deploys all the ManagedResource CRDs for the gardener-resource-manager.
func (b *HybridBotanist) DeployManagedResources(ctx context.Context) error {
	type chartRenderFunc func() (*chartrenderer.RenderedChart, error)

	var (
		labels = map[string]string{
			common.ShootNoCleanup: "true",
		}
		charts = map[string]chartRenderFunc{
			"storageclasses":               b.generateStorageClassesChart,
			"shoot-cloud-config-execution": b.generateCloudConfigExecutionChart,
			"shoot-core":                   b.generateCoreAddonsChart,
			"addons":                       b.generateOptionalAddonsChart,
		}
	)

	for name, renderFunc := range charts {
		renderedChart, err := renderFunc()
		if err != nil {
			return fmt.Errorf("error rendering %q chart: %+v", name, err)
		}

		data := make(map[string][]byte, len(renderedChart.Files()))
		for fileName, fileContent := range renderedChart.Files() {
			data[strings.Replace(fileName, "/", "_", -1)] = []byte(fileContent)
		}

		secretName := "managedresource-" + name

		if err := manager.
			NewSecret(b.K8sSeedClient.Client()).
			WithNamespacedName(b.Shoot.SeedNamespace, secretName).
			WithKeyValues(data).
			Reconcile(ctx); err != nil {
			return err
		}

		if err := manager.
			NewManagedResource(b.K8sSeedClient.Client()).
			WithNamespacedName(b.Shoot.SeedNamespace, name).
			WithSecretRef(secretName).
			WithInjectedLabels(labels).
			Reconcile(ctx); err != nil {
			return err
		}
	}

	return nil
}

// generateCoreAddonsChart renders the gardener-resource-manager configuration for the core addons. After that it
// creates a ManagedResource CRD that references the rendered manifests and creates it.
func (b *HybridBotanist) generateCoreAddonsChart() (*chartrenderer.RenderedChart, error) {
	var (
		kubeProxySecret  = b.Secrets["kube-proxy"]
		vpnShootSecret   = b.Secrets["vpn-shoot"]
		vpnTLSAuthSecret = b.Secrets["vpn-seed-tlsauth"]
		global           = map[string]interface{}{
			"kubernetesVersion": b.Shoot.Info.Spec.Kubernetes.Version,
			"podNetwork":        b.Shoot.GetPodNetwork(),
		}
		calicoConfig = map[string]interface{}{
			"cloudProvider": b.Shoot.CloudProvider,
		}
		coreDNSConfig = map[string]interface{}{
			"service": map[string]interface{}{
				"clusterDNS": common.ComputeClusterIP(b.Shoot.GetServiceNetwork(), 10),
				// TODO: resolve conformance test issue before changing:
				// https://github.com/kubernetes/kubernetes/blob/master/test/e2e/network/dns.go#L44
				"domain": map[string]interface{}{
					"clusterDomain": gardenv1beta1.DefaultDomain,
				},
			},
		}
		clusterAutoscaler = map[string]interface{}{
			"enabled": b.Shoot.WantsClusterAutoscaler,
		}
		podsecuritypolicies = map[string]interface{}{
			"allowPrivilegedContainers": *b.Shoot.Info.Spec.Kubernetes.AllowPrivilegedContainers,
		}
		kubeProxyConfig = map[string]interface{}{
			"kubeconfig":        kubeProxySecret.Data["kubeconfig"],
			"kubernetesVersion": b.Shoot.Info.Spec.Kubernetes.Version,
			"podAnnotations": map[string]interface{}{
				"checksum/secret-kube-proxy": b.CheckSums["kube-proxy"],
			},
			"enableIPVS": b.Shoot.IPVSEnabled(),
		}
		metricsServerConfig = map[string]interface{}{
			"tls": map[string]interface{}{
				"caBundle": b.Secrets[gardencorev1alpha1.SecretNameCAMetricsServer].Data[secrets.DataKeyCertificateCA],
			},
			"secret": map[string]interface{}{
				"data": b.Secrets["metrics-server"].Data,
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
		nodeExporterConfig     = map[string]interface{}{}
		blackboxExporterConfig = map[string]interface{}{}
	)

	proxyConfig := b.Shoot.Info.Spec.Kubernetes.KubeProxy
	if proxyConfig != nil {
		kubeProxyConfig["featureGates"] = proxyConfig.FeatureGates
	}

	if openvpnDiffieHellmanSecret, ok := b.Secrets[common.GardenRoleOpenVPNDiffieHellman]; ok {
		vpnShootConfig["diffieHellmanKey"] = openvpnDiffieHellmanSecret.Data["dh2048.pem"]
	}

	calico, err := b.InjectShootShootImages(calicoConfig, common.CalicoNodeImageName, common.CalicoCNIImageName, common.CalicoTyphaImageName, common.CalicoKubeControllersImageName)
	if err != nil {
		return nil, err
	}

	coreDNS, err := b.InjectShootShootImages(coreDNSConfig, common.CoreDNSImageName)
	if err != nil {
		return nil, err
	}

	kubeProxy, err := b.InjectShootShootImages(kubeProxyConfig, common.HyperkubeImageName, common.AlpineImageName)
	if err != nil {
		return nil, err
	}

	metricsServer, err := b.InjectShootShootImages(metricsServerConfig, common.MetricsServerImageName)
	if err != nil {
		return nil, err
	}

	vpnShoot, err := b.InjectShootShootImages(vpnShootConfig, common.VPNShootImageName)
	if err != nil {
		return nil, err
	}
	vpnShootCloudSpecific, err := b.ShootCloudBotanist.GenerateVPNShootConfig()
	if err != nil {
		return nil, err
	}
	vpnShoot = utils.MergeMaps(vpnShoot, vpnShootCloudSpecific)

	nodeExporter, err := b.InjectShootShootImages(nodeExporterConfig, common.NodeExporterImageName)
	if err != nil {
		return nil, err
	}
	blackboxExporter, err := b.InjectShootShootImages(blackboxExporterConfig, common.BlackboxExporterImageName)
	if err != nil {
		return nil, err
	}

	csiPlugin, err := b.ShootCloudBotanist.GenerateCSIConfig()
	if err != nil {
		return nil, err
	}

	newVpnShootSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vpn-shoot",
			Namespace: metav1.NamespaceSystem,
		},
	}
	if err := kutil.CreateOrUpdate(context.TODO(), b.K8sShootClient.Client(), newVpnShootSecret, func() error {
		newVpnShootSecret.Type = corev1.SecretTypeOpaque
		newVpnShootSecret.Data = vpnShootSecret.Data
		return nil
	}); err != nil {
		return nil, err
	}

	return b.ChartApplierShoot.Render(filepath.Join(common.ChartPath, "shoot-core"), "shoot-core", metav1.NamespaceSystem, map[string]interface{}{
		"global":              global,
		"cluster-autoscaler":  clusterAutoscaler,
		"podsecuritypolicies": podsecuritypolicies,
		"coredns":             coreDNS,
		fmt.Sprintf("csi-%s", b.ShootCloudBotanist.GetCloudProviderName()): csiPlugin,
		"kube-proxy":     kubeProxy,
		"vpn-shoot":      vpnShoot,
		"calico":         calico,
		"metrics-server": metricsServer,
		"monitoring": map[string]interface{}{
			"node-exporter":     nodeExporter,
			"blackbox-exporter": blackboxExporter,
		},
	})
}

// generateOptionalAddonsChart renders the gardener-resource-manager chart for the optional addons. After that it
// creates a ManagedResource CRD that references the rendered manifests and creates it.
func (b *HybridBotanist) generateOptionalAddonsChart() (*chartrenderer.RenderedChart, error) {
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

		if b.ShootedSeed != nil {
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

	kubeLego, err := b.InjectShootShootImages(kubeLegoConfig, common.KubeLegoImageName)
	if err != nil {
		return nil, err
	}
	kube2IAM, err := b.InjectShootShootImages(kube2IAMConfig, common.Kube2IAMImageName)
	if err != nil {
		return nil, err
	}
	kubernetesDashboard, err := b.InjectShootShootImages(kubernetesDashboardConfig, common.KubernetesDashboardImageName)
	if err != nil {
		return nil, err
	}
	nginxIngress, err := b.InjectShootShootImages(nginxIngressConfig, common.NginxIngressControllerImageName, common.IngressDefaultBackendImageName)
	if err != nil {
		return nil, err
	}

	return b.ChartApplierShoot.Render(filepath.Join(common.ChartPath, "shoot-addons"), "addons", metav1.NamespaceSystem, map[string]interface{}{
		"kube-lego":            kubeLego,
		"kube2iam":             kube2IAM,
		"kubernetes-dashboard": kubernetesDashboard,
		"nginx-ingress":        nginxIngress,
	})
}

// generateStorageClassesChart renders the gardener-resource-manager configuration for the storage classes. After that it
// creates a ManagedResource CRD that references the rendered manifests and creates it.
func (b *HybridBotanist) generateStorageClassesChart() (*chartrenderer.RenderedChart, error) {
	config, err := b.ShootCloudBotanist.GenerateStorageClassesConfig()
	if err != nil {
		return nil, err
	}

	return b.ChartApplierShoot.Render(filepath.Join(common.ChartPath, "shoot-storageclasses"), "storageclasses", metav1.NamespaceSystem, config)
}
