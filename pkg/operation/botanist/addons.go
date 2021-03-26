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

package botanist

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/gardener/gardener/charts"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/controllerutils"
	netpol "github.com/gardener/gardener/pkg/operation/botanist/addons/networkpolicy"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/dns"
	"github.com/gardener/gardener/pkg/operation/common"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/secrets"
	versionutils "github.com/gardener/gardener/pkg/utils/version"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// SecretLabelKeyManagedResource is a key for a label on a secret with the value 'managed-resource'.
	SecretLabelKeyManagedResource = "managed-resource"
	// gardenerRestartedAtKey is annotation key for timestamp used to restart components.
	gardenerRestartedAtKey = "gardener.cloud/restarted-at"
)

// GenerateKubernetesDashboardConfig generates the values which are required to render the chart of
// the kubernetes-dashboard properly.
func (b *Botanist) GenerateKubernetesDashboardConfig() (map[string]interface{}, error) {
	var (
		enabled = gardencorev1beta1helper.KubernetesDashboardEnabled(b.Shoot.Info.Spec.Addons)
		values  = map[string]interface{}{}
	)

	if b.APIServerSNIEnabled() {
		values["kubeAPIServerHost"] = b.outOfClusterAPIServerFQDN()
	}

	if enabled && b.Shoot.Info.Spec.Addons.KubernetesDashboard.AuthenticationMode != nil {
		values["authenticationMode"] = *b.Shoot.Info.Spec.Addons.KubernetesDashboard.AuthenticationMode
	}

	return common.GenerateAddonConfig(values, enabled), nil
}

// EnsureIngressDNSRecord deploys the nginx ingress DNSEntry and DNSOwner resources.
func (b *Botanist) EnsureIngressDNSRecord(ctx context.Context) error {
	if b.NeedsExternalDNS() && !b.Shoot.HibernationEnabled && gardencorev1beta1helper.NginxIngressEnabled(b.Shoot.Info.Spec.Addons) {
		if b.isRestorePhase() {
			return dnsRestoreDeployer{
				entry: b.Shoot.Components.Extensions.DNS.NginxEntry,
				owner: b.Shoot.Components.Extensions.DNS.NginxOwner,
			}.Deploy(ctx)
		}

		return component.OpWaiter(
			b.Shoot.Components.Extensions.DNS.NginxOwner,
			b.Shoot.Components.Extensions.DNS.NginxEntry,
		).Deploy(ctx)
	}

	return component.OpWaiter(
		b.Shoot.Components.Extensions.DNS.NginxEntry,
		b.Shoot.Components.Extensions.DNS.NginxOwner,
	).Deploy(ctx)
}

// DestroyIngressDNSRecord destroys the nginx ingress DNSEntry and DNSOwner resources.
func (b *Botanist) DestroyIngressDNSRecord(ctx context.Context) error {
	return component.OpDestroyAndWait(
		b.Shoot.Components.Extensions.DNS.NginxEntry,
		b.Shoot.Components.Extensions.DNS.NginxOwner,
	).Destroy(ctx)
}

// MigrateIngressDNSRecord destroys the nginx ingress DNSEntry and DNSOwner resources,
// without removing the entry from the DNS provider.
func (b *Botanist) MigrateIngressDNSRecord(ctx context.Context) error {
	return component.OpDestroy(
		b.Shoot.Components.Extensions.DNS.NginxOwner,
		b.Shoot.Components.Extensions.DNS.NginxEntry,
	).Destroy(ctx)
}

// DefaultNginxIngressDNSEntry returns a Deployer which removes existing nginx ingress DNSEntry.
func (b *Botanist) DefaultNginxIngressDNSEntry(seedClient client.Client) component.DeployWaiter {
	return component.OpDestroy(dns.NewEntry(
		b.Logger,
		seedClient,
		b.Shoot.SeedNamespace,
		&dns.EntryValues{
			Name: common.ShootDNSIngressName,
			TTL:  *b.Config.Controllers.Shoot.DNSEntryTTLSeconds,
		},
		nil,
	))
}

// DefaultNginxIngressDNSOwner returns DeployWaiter which removes the nginx ingress DNSOwner.
func (b *Botanist) DefaultNginxIngressDNSOwner(seedClient client.Client) component.DeployWaiter {
	return component.OpDestroy(dns.NewOwner(
		seedClient,
		b.Shoot.SeedNamespace,
		&dns.OwnerValues{
			Name: common.ShootDNSIngressName,
		},
	))
}

// SetNginxIngressAddress sets the IP address of the API server's LoadBalancer.
func (b *Botanist) SetNginxIngressAddress(address string, seedClient client.Client) {
	if b.NeedsExternalDNS() && !b.Shoot.HibernationEnabled && gardencorev1beta1helper.NginxIngressEnabled(b.Shoot.Info.Spec.Addons) {
		ownerID := *b.Shoot.Info.Status.ClusterIdentity + "-" + common.ShootDNSIngressName
		b.Shoot.Components.Extensions.DNS.NginxOwner = dns.NewOwner(
			seedClient,
			b.Shoot.SeedNamespace,
			&dns.OwnerValues{
				Name:    common.ShootDNSIngressName,
				Active:  pointer.BoolPtr(true),
				OwnerID: ownerID,
			},
		)
		b.Shoot.Components.Extensions.DNS.NginxEntry = dns.NewEntry(
			b.Logger,
			seedClient,
			b.Shoot.SeedNamespace,
			&dns.EntryValues{
				Name:    common.ShootDNSIngressName,
				DNSName: b.Shoot.GetIngressFQDN("*"),
				Targets: []string{address},
				OwnerID: ownerID,
				TTL:     *b.Config.Controllers.Shoot.DNSEntryTTLSeconds,
			},
			nil,
		)
	}
}

// GenerateNginxIngressConfig generates the values which are required to render the chart of
// the nginx-ingress properly.
func (b *Botanist) GenerateNginxIngressConfig() (map[string]interface{}, error) {
	var (
		enabled = gardencorev1beta1helper.NginxIngressEnabled(b.Shoot.Info.Spec.Addons)
		values  map[string]interface{}
	)

	if enabled {
		values = map[string]interface{}{
			"controller": map[string]interface{}{
				"customConfig": b.Shoot.Info.Spec.Addons.NginxIngress.Config,
				"service": map[string]interface{}{
					"loadBalancerSourceRanges": b.Shoot.Info.Spec.Addons.NginxIngress.LoadBalancerSourceRanges,
					"externalTrafficPolicy":    *b.Shoot.Info.Spec.Addons.NginxIngress.ExternalTrafficPolicy,
				},
			},
		}

		if b.APIServerSNIEnabled() {
			values["kubeAPIServerHost"] = b.outOfClusterAPIServerFQDN()
		}
	}

	return common.GenerateAddonConfig(values, enabled), nil
}

// DeployManagedResourceForAddons deploys all the ManagedResource CRDs for the gardener-resource-manager.
func (b *Botanist) DeployManagedResourceForAddons(ctx context.Context) error {
	for name, chartRenderFunc := range map[string]func(context.Context) (*chartrenderer.RenderedChart, error){
		common.ManagedResourceShootCoreName: b.generateCoreAddonsChart,
		common.ManagedResourceAddonsName:    b.generateOptionalAddonsChart,
	} {
		renderedChart, err := chartRenderFunc(ctx)
		if err != nil {
			return fmt.Errorf("error rendering %q chart: %+v", name, err)
		}

		if err := common.DeployManagedResourceForShoot(ctx, b.K8sSeedClient.Client(), name, b.Shoot.SeedNamespace, false, renderedChart.AsSecretData()); err != nil {
			return err
		}
	}

	return nil
}

// generateCoreAddonsChart renders the gardener-resource-manager configuration for the core addons. After that it
// creates a ManagedResource CRD that references the rendered manifests and creates it.
func (b *Botanist) generateCoreAddonsChart(ctx context.Context) (*chartrenderer.RenderedChart, error) {
	var (
		kasFQDN         = b.outOfClusterAPIServerFQDN()
		kubeProxySecret = b.Secrets["kube-proxy"]
		global          = map[string]interface{}{
			"kubernetesVersion": b.Shoot.Info.Spec.Kubernetes.Version,
			"podNetwork":        b.Shoot.Networks.Pods.String(),
			"vpaEnabled":        b.Shoot.WantsVerticalPodAutoscaler,
		}
		coreDNSConfig = map[string]interface{}{
			"nodeNetwork": b.Shoot.GetNodeNetwork(),
			"service": map[string]interface{}{
				"clusterDNS": b.Shoot.Networks.CoreDNS.String(),
				// TODO: resolve conformance test issue before changing:
				// https://github.com/kubernetes/kubernetes/blob/master/test/e2e/network/dns.go#L44
				"domain": map[string]interface{}{
					"clusterDomain": gardencorev1beta1.DefaultDomain,
				},
			},
		}
		nodeLocalDNSConfig = map[string]interface{}{
			"domain": gardencorev1beta1.DefaultDomain,
		}

		podSecurityPolicies = map[string]interface{}{
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
		verticalPodAutoscaler = map[string]interface{}{
			"clusterType": "shoot",
			"admissionController": map[string]interface{}{
				"enableServiceAccount": false,
				"controlNamespace":     b.Shoot.SeedNamespace,
			},
			"exporter":    map[string]interface{}{"enableServiceAccount": false},
			"recommender": map[string]interface{}{"enableServiceAccount": false},
			"updater":     map[string]interface{}{"enableServiceAccount": false},
		}

		shootInfo = map[string]interface{}{
			"projectName":       b.Garden.Project.Name,
			"shootName":         b.Shoot.Info.Name,
			"provider":          b.Shoot.Info.Spec.Provider.Type,
			"region":            b.Shoot.Info.Spec.Region,
			"kubernetesVersion": b.Shoot.Info.Spec.Kubernetes.Version,
			"podNetwork":        b.Shoot.Networks.Pods.String(),
			"serviceNetwork":    b.Shoot.Networks.Services.String(),
			"maintenanceBegin":  b.Shoot.Info.Spec.Maintenance.TimeWindow.Begin,
			"maintenanceEnd":    b.Shoot.Info.Spec.Maintenance.TimeWindow.End,
		}
		nodeExporterConfig        = map[string]interface{}{}
		blackboxExporterConfig    = map[string]interface{}{}
		nodeProblemDetectorConfig = map[string]interface{}{}
		networkPolicyConfig       = netpol.ShootNetworkPolicyValues{
			Enabled: true,
			NodeLocalDNS: netpol.NodeLocalDNSValues{
				Enabled:          b.Shoot.NodeLocalDNSEnabled,
				KubeDNSClusterIP: b.Shoot.Networks.CoreDNS.String(),
			},
		}

		nodeNetwork = b.Shoot.GetNodeNetwork()
	)

	if b.Shoot.IPVSEnabled() {
		networkPolicyConfig.NodeLocalDNS.KubeDNSClusterIP = NodeLocalIPVSAddress
	}

	if b.APIServerSNIEnabled() {
		coreDNSConfig["kubeAPIServerHost"] = kasFQDN
		nodeProblemDetectorConfig["env"] = []interface{}{
			map[string]interface{}{
				"name":  "KUBERNETES_SERVICE_HOST",
				"value": kasFQDN,
			},
		}
	}

	if _, ok := b.Secrets[common.VPASecretName]; ok {
		verticalPodAutoscaler["admissionController"].(map[string]interface{})["caCert"] = b.Secrets[common.VPASecretName].Data[secrets.DataKeyCertificateCA]
	}

	proxyConfig := b.Shoot.Info.Spec.Kubernetes.KubeProxy
	if proxyConfig != nil {
		kubeProxyConfig["featureGates"] = proxyConfig.FeatureGates
	}

	if domain := b.Shoot.ExternalClusterDomain; domain != nil {
		shootInfo["domain"] = *domain
	}
	var extensions []string
	for extensionType := range b.Shoot.Components.Extensions.Extension.Extensions() {
		extensions = append(extensions, extensionType)
	}
	shootInfo["extensions"] = strings.Join(extensions, ",")

	coreDNSRestartTimestamp, err := b.getCoreDNSRestartTimestamp(ctx)
	if err != nil {
		return nil, err
	}
	if len(coreDNSRestartTimestamp) != 0 {
		coreDNSConfig["deployment"] = map[string]interface{}{
			"spec": map[string]interface{}{
				"podAnnotations": map[string]interface{}{
					gardenerRestartedAtKey: coreDNSRestartTimestamp,
				},
			},
		}
	}

	coreDNS, err := b.InjectShootShootImages(coreDNSConfig, charts.ImageNameCoredns)
	if err != nil {
		return nil, err
	}

	// The node-local-dns interface cannot bind the kube-dns cluster IP since the interface
	// used for IPVS load-balancing already uses this address.
	if b.Shoot.IPVSEnabled() {
		nodeLocalDNSConfig["clusterDNS"] = b.Shoot.Networks.CoreDNS.String()
	} else {
		nodeLocalDNSConfig["dnsServer"] = b.Shoot.Networks.CoreDNS.String()
	}

	nodelocalDNS, err := b.InjectShootShootImages(nodeLocalDNSConfig, charts.ImageNameNodeLocalDns)
	if err != nil {
		return nil, err
	}

	nodeProblemDetector, err := b.InjectShootShootImages(nodeProblemDetectorConfig, charts.ImageNameNodeProblemDetector)
	if err != nil {
		return nil, err
	}

	kubeProxy, err := b.InjectShootShootImages(kubeProxyConfig, charts.ImageNameKubeProxy, charts.ImageNameAlpine)
	if err != nil {
		return nil, err
	}

	nodeExporter, err := b.InjectShootShootImages(nodeExporterConfig, charts.ImageNameNodeExporter)
	if err != nil {
		return nil, err
	}
	blackboxExporter, err := b.InjectShootShootImages(blackboxExporterConfig, charts.ImageNameBlackboxExporter)
	if err != nil {
		return nil, err
	}

	apiserverProxyConfig := map[string]interface{}{
		"advertiseIPAddress": b.APIServerClusterIP,
		"proxySeedServer": map[string]interface{}{
			"host": kasFQDN,
			"port": "8443",
		},
		"webhook": map[string]interface{}{
			"caBundle": b.Secrets[v1beta1constants.SecretNameCACluster].Data[secrets.DataKeyCertificateCA],
		},
		"podMutatorEnabled": b.APIServerSNIPodMutatorEnabled(),
	}

	apiserverProxy, err := b.InjectShootShootImages(apiserverProxyConfig, charts.ImageNameApiserverProxySidecar, charts.ImageNameApiserverProxy)
	if err != nil {
		return nil, err
	}

	if nodeNetwork != nil {
		shootInfo["nodeNetwork"] = *nodeNetwork
	}

	values := map[string]interface{}{
		"global":                 global,
		"coredns":                coreDNS,
		"node-local-dns":         common.GenerateAddonConfig(nodelocalDNS, b.Shoot.NodeLocalDNSEnabled),
		"kube-apiserver-kubelet": common.GenerateAddonConfig(nil, true),
		"apiserver-proxy":        common.GenerateAddonConfig(apiserverProxy, b.APIServerSNIEnabled()),
		"kube-proxy":             common.GenerateAddonConfig(kubeProxy, true),
		"monitoring": common.GenerateAddonConfig(map[string]interface{}{
			"node-exporter":     nodeExporter,
			"blackbox-exporter": blackboxExporter,
		}, b.Shoot.Purpose != gardencorev1beta1.ShootPurposeTesting),
		"network-policies":        networkPolicyConfig,
		"node-problem-detector":   common.GenerateAddonConfig(nodeProblemDetector, true),
		"podsecuritypolicies":     common.GenerateAddonConfig(podSecurityPolicies, true),
		"shoot-info":              common.GenerateAddonConfig(shootInfo, true),
		"vertical-pod-autoscaler": common.GenerateAddonConfig(verticalPodAutoscaler, b.Shoot.WantsVerticalPodAutoscaler),
		"cluster-identity":        map[string]interface{}{"clusterIdentity": b.Shoot.Info.Status.ClusterIdentity},
	}

	shootClient := b.K8sShootClient.Client()

	if b.Shoot.KonnectivityTunnelEnabled {
		konnectivityAgentConfig := map[string]interface{}{
			"proxyHost": gutil.GetAPIServerDomain(b.Shoot.InternalClusterDomain),
			"podAnnotations": map[string]interface{}{
				"checksum/secret-konnectivity-agent": b.CheckSums["konnectivity-agent"],
			},
		}

		// Konnectivity agent related values
		konnectivityAgent, err := b.InjectShootShootImages(konnectivityAgentConfig, charts.ImageNameKonnectivityAgent)
		if err != nil {
			return nil, err
		}

		values["konnectivity-agent"] = common.GenerateAddonConfig(konnectivityAgent, true)

		// TODO: remove when konnectivity tunnel is the default tunneling method for all shoots.
		secret, err := common.GetSecretFromSecretRef(ctx, shootClient, &corev1.SecretReference{Namespace: metav1.NamespaceSystem, Name: "vpn-shoot"})
		if err != nil && !apierrors.IsNotFound(err) {
			return nil, err
		}

		if secret != nil {
			if err := b.K8sShootClient.Client().Delete(ctx, secret); err != nil {
				return nil, err
			}
		}
	} else {
		var (
			vpnTLSAuthSecret = b.Secrets["vpn-seed-tlsauth"]
			vpnShootSecret   = b.Secrets["vpn-shoot"]
			vpnShootConfig   = map[string]interface{}{
				"podNetwork":     b.Shoot.Networks.Pods.String(),
				"serviceNetwork": b.Shoot.Networks.Services.String(),
				"tlsAuth":        vpnTLSAuthSecret.Data["vpn.tlsauth"],
				"vpnShootSecretData": map[string]interface{}{
					"ca":     vpnShootSecret.Data["ca.crt"],
					"tlsCrt": vpnShootSecret.Data["tls.crt"],
					"tlsKey": vpnShootSecret.Data["tls.key"],
				},
				"podAnnotations": map[string]interface{}{
					"checksum/secret-vpn-shoot": b.CheckSums["vpn-shoot"],
				},
			}
		)

		// OpenVPN related values
		if openvpnDiffieHellmanSecret, ok := b.Secrets[v1beta1constants.GardenRoleOpenVPNDiffieHellman]; ok {
			vpnShootConfig["diffieHellmanKey"] = openvpnDiffieHellmanSecret.Data["dh2048.pem"]
		}

		if nodeNetwork != nil {
			vpnShootConfig["nodeNetwork"] = *nodeNetwork
		}

		vpnShoot, err := b.InjectShootShootImages(vpnShootConfig, charts.ImageNameVpnShoot)
		if err != nil {
			return nil, err
		}

		values["vpn-shoot"] = common.GenerateAddonConfig(vpnShoot, true)
	}

	return b.K8sShootClient.ChartRenderer().Render(filepath.Join(charts.Path, "shoot-core", "components"), "shoot-core", metav1.NamespaceSystem, values)
}

// generateOptionalAddonsChart renders the gardener-resource-manager chart for the optional addons. After that it
// creates a ManagedResource CRD that references the rendered manifests and creates it.
func (b *Botanist) generateOptionalAddonsChart(_ context.Context) (*chartrenderer.RenderedChart, error) {
	global := map[string]interface{}{
		"vpaEnabled": b.Shoot.WantsVerticalPodAutoscaler,
	}

	kubernetesDashboardConfig, err := b.GenerateKubernetesDashboardConfig()
	if err != nil {
		return nil, err
	}
	kubernetesDashboardImagesToInject := []string{charts.ImageNameKubernetesDashboard}

	k8sVersionLessThan116, err := versionutils.CompareVersions(b.Shoot.Info.Spec.Kubernetes.Version, "<", "1.16")
	if err != nil {
		return nil, err
	}
	if !k8sVersionLessThan116 {
		kubernetesDashboardImagesToInject = append(kubernetesDashboardImagesToInject, charts.ImageNameKubernetesDashboardMetricsScraper)
	}

	kubernetesDashboard, err := b.InjectShootShootImages(kubernetesDashboardConfig, kubernetesDashboardImagesToInject...)
	if err != nil {
		return nil, err
	}

	nginxIngressConfig, err := b.GenerateNginxIngressConfig()
	if err != nil {
		return nil, err
	}
	nginxIngress, err := b.InjectShootShootImages(nginxIngressConfig, charts.ImageNameNginxIngressController, charts.ImageNameIngressDefaultBackend)
	if err != nil {
		return nil, err
	}

	return b.K8sShootClient.ChartRenderer().Render(filepath.Join(charts.Path, "shoot-addons"), "addons", metav1.NamespaceSystem, map[string]interface{}{
		"global":               global,
		"kubernetes-dashboard": kubernetesDashboard,
		"nginx-ingress":        nginxIngress,
	})
}

// outOfClusterAPIServerFQDN returns the Fully Qualified Domain Name of the apiserver
// with dot "." suffix. It'll prevent extra requests to the DNS in case the record is not
// available.
func (b *Botanist) outOfClusterAPIServerFQDN() string {
	return fmt.Sprintf("%s.", b.Shoot.ComputeOutOfClusterAPIServerAddress(b.APIServerAddress, true))
}

// getCoreDNSRestartTimestamp returns a timestamp that can potentially restart the CoreDNS deployment.
func (b *Botanist) getCoreDNSRestartTimestamp(ctx context.Context) (string, error) {
	if controllerutils.HasTask(b.Shoot.Info.Annotations, common.ShootTaskRestartCoreAddons) {
		return time.Now().UTC().Format(time.RFC3339), nil
	}

	coreDNSDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.CoreDNSDeploymentName,
			Namespace: metav1.NamespaceSystem,
		},
	}
	if err := b.K8sShootClient.Client().Get(ctx, client.ObjectKeyFromObject(coreDNSDeployment), coreDNSDeployment); err != nil {
		if apierrors.IsNotFound(err) {
			return "", nil
		}
		return "", err
	}

	val, ok := coreDNSDeployment.Spec.Template.ObjectMeta.Annotations[gardenerRestartedAtKey]
	if !ok {
		return "", nil
	}
	return val, nil
}
