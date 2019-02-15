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
	"fmt"
	"path/filepath"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/chartrenderer"
	controllermanagerfeatures "github.com/gardener/gardener/pkg/controllermanager/features"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/secrets"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
			"kubeconfig": kubeProxySecret.Data["kubeconfig"],
			"podAnnotations": map[string]interface{}{
				"checksum/secret-kube-proxy": b.CheckSums["kube-proxy"],
			},
		}
		metricsServerConfig = map[string]interface{}{
			"tls": map[string]interface{}{
				"caBundle": b.Secrets["ca-metrics-server"].Data[secrets.DataKeyCertificateCA],
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

	calico, err := b.Botanist.ImageVector.InjectImages(calicoConfig, b.ShootVersion(), b.ShootVersion(), common.CalicoNodeImageName, common.CalicoCNIImageName, common.CalicoTyphaImageName)
	if err != nil {
		return nil, err
	}

	coreDNS, err := b.Botanist.ImageVector.InjectImages(coreDNSConfig, b.ShootVersion(), b.ShootVersion(), common.CoreDNSImageName)
	if err != nil {
		return nil, err
	}

	kubeProxy, err := b.Botanist.ImageVector.InjectImages(kubeProxyConfig, b.ShootVersion(), b.ShootVersion(), common.HyperkubeImageName)
	if err != nil {
		return nil, err
	}

	metricsServer, err := b.Botanist.ImageVector.InjectImages(metricsServerConfig, b.ShootVersion(), b.ShootVersion(), common.MetricsServerImageName)
	if err != nil {
		return nil, err
	}

	vpnShoot, err := b.Botanist.ImageVector.InjectImages(vpnShootConfig, b.ShootVersion(), b.ShootVersion(), common.VPNShootImageName)
	if err != nil {
		return nil, err
	}

	nodeExporter, err := b.Botanist.ImageVector.InjectImages(nodeExporterConfig, b.ShootVersion(), b.ShootVersion(), common.NodeExporterImageName)
	if err != nil {
		return nil, err
	}
	blackboxExporter, err := b.Botanist.ImageVector.InjectImages(blackboxExporterConfig, b.ShootVersion(), b.ShootVersion(), common.BlackboxExporterImageName)
	if err != nil {
		return nil, err
	}

	csiPlugin, err := b.ShootCloudBotanist.GenerateCSIConfig()
	if err != nil {
		return nil, err
	}

	if _, err := b.K8sShootClient.CreateSecret(metav1.NamespaceSystem, "vpn-shoot", corev1.SecretTypeOpaque, vpnShootSecret.Data, true); err != nil {
		return nil, err
	}

	ccmConfig := map[string]interface{}{}
	if cfg := b.ShootCloudBotanist.GenerateCloudConfigUserDataConfig(); cfg != nil {
		ccmConfig["enableCSI"] = cfg.EnableCSI
	}

	return b.ChartShootRenderer.Render(filepath.Join(common.ChartPath, "shoot-core"), "shoot-core", metav1.NamespaceSystem, map[string]interface{}{
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
		"cloud-controller-manager": ccmConfig,
		"cert-broker": map[string]interface{}{
			"enabled": controllermanagerfeatures.FeatureGate.Enabled(features.CertificateManagement),
		},
	})
}

// generateOptionalAddonsChart renders the kube-addon-manager chart for the optional addons. It
// will be stored as a Secret (as it may contain credentials) and mounted into the Pod. The configuration
// contains specially labelled Kubernetes manifests which will be created and periodically reconciled.
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

	kubeLego, err := b.Botanist.ImageVector.InjectImages(kubeLegoConfig, b.ShootVersion(), b.ShootVersion(), common.KubeLegoImageName)
	if err != nil {
		return nil, err
	}
	kube2IAM, err := b.Botanist.ImageVector.InjectImages(kube2IAMConfig, b.ShootVersion(), b.ShootVersion(), common.Kube2IAMImageName)
	if err != nil {
		return nil, err
	}
	kubernetesDashboard, err := b.Botanist.ImageVector.InjectImages(kubernetesDashboardConfig, b.ShootVersion(), b.ShootVersion(), common.KubernetesDashboardImageName)
	if err != nil {
		return nil, err
	}
	nginxIngress, err := b.Botanist.ImageVector.InjectImages(nginxIngressConfig, b.ShootVersion(), b.ShootVersion(), common.NginxIngressControllerImageName, common.IngressDefaultBackendImageName)
	if err != nil {
		return nil, err
	}

	// From https://github.com/kubernetes/kubernetes/blob/677f740adf61f9c56d0719eacabfeae3b0787256/cluster/addons/addon-manager/README.md:
	// "Addons with label addonmanager.kubernetes.io/mode=EnsureExists will be checked for existence only. Users can edit these addons as they want. In particular:"
	// "* Addon will only be created/re-created with the given template file when there is no instance of the resource with that name."
	// "* Addon will not be deleted when the manifest file is deleted from the $ADDON_PATH."
	// --> As we used the 'addonmanager.kubernetes.io/mode=EnsureExists' label for the Heapster deployment in previous versions we have to delete it ourselves now.
	//     This behavior can be removed in a future release.
	heapsterDeployments, err := b.K8sShootClient.ListDeployments(metav1.NamespaceSystem, metav1.ListOptions{
		LabelSelector: "chart=heapster-0.1.1,origin=gardener",
	})
	if err != nil {
		return nil, err
	}
	for _, deployment := range heapsterDeployments.Items {
		if err := b.K8sShootClient.DeleteDeployment(metav1.NamespaceSystem, deployment.Name); err != nil && !apierrors.IsNotFound(err) {
			return nil, err
		}
	}

	return b.ChartShootRenderer.Render(filepath.Join(common.ChartPath, "shoot-addons"), "addons", metav1.NamespaceSystem, map[string]interface{}{
		"kube-lego":            kubeLego,
		"kube2iam":             kube2IAM,
		"kubernetes-dashboard": kubernetesDashboard,
		"nginx-ingress":        nginxIngress,
	})
}

// generateStorageClassesChart renders the kube-addon-manager configuration for the storage classes.
// It will be stored as a ConfigMap and mounted into the Pod. The configuration contains specially labelled
// Kubernetes manifests which will be created and periodically reconciled.
func (b *HybridBotanist) generateStorageClassesChart() (*chartrenderer.RenderedChart, error) {
	config, err := b.ShootCloudBotanist.GenerateStorageClassesConfig()
	if err != nil {
		return nil, err
	}

	return b.ChartShootRenderer.Render(filepath.Join(common.ChartPath, "shoot-storageclasses"), "storageclasses", metav1.NamespaceSystem, config)
}
