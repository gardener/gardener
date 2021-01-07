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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	netpol "github.com/gardener/gardener/pkg/operation/botanist/addons/networkpolicy"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/extensions/dns"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/secrets"
	versionutils "github.com/gardener/gardener/pkg/utils/version"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SecretLabelKeyManagedResource is a key for a label on a secret with the value 'managed-resource'.
const SecretLabelKeyManagedResource = "managed-resource"

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
	return component.OpDestroy(dns.NewDNSEntry(
		&dns.EntryValues{
			Name: common.ShootDNSIngressName,
			TTL:  *b.Config.Controllers.Shoot.DNSEntryTTLSeconds,
		},
		b.Shoot.SeedNamespace,
		b.K8sSeedClient.ChartApplier(),
		b.ChartsRootPath,
		b.Logger,
		seedClient,
		nil,
	))
}

// DefaultNginxIngressDNSOwner returns DeployWaiter which removes the nginx ingress DNSOwner.
func (b *Botanist) DefaultNginxIngressDNSOwner(seedClient client.Client) component.DeployWaiter {
	return component.OpDestroy(dns.NewDNSOwner(
		&dns.OwnerValues{
			Name: common.ShootDNSIngressName,
		},
		b.Shoot.SeedNamespace,
		b.K8sSeedClient.ChartApplier(),
		b.ChartsRootPath,
		seedClient,
	))
}

// SetNginxIngressAddress sets the IP address of the API server's LoadBalancer.
func (b *Botanist) SetNginxIngressAddress(address string, seedClient client.Client) {
	if b.NeedsExternalDNS() && !b.Shoot.HibernationEnabled && gardencorev1beta1helper.NginxIngressEnabled(b.Shoot.Info.Spec.Addons) {
		ownerID := *b.Shoot.Info.Status.ClusterIdentity + "-" + common.ShootDNSIngressName
		b.Shoot.Components.Extensions.DNS.NginxOwner = dns.NewDNSOwner(
			&dns.OwnerValues{
				Name:    common.ShootDNSIngressName,
				Active:  true,
				OwnerID: ownerID,
			},
			b.Shoot.SeedNamespace,
			b.K8sSeedClient.ChartApplier(),
			b.ChartsRootPath,
			seedClient,
		)
		b.Shoot.Components.Extensions.DNS.NginxEntry = dns.NewDNSEntry(
			&dns.EntryValues{
				Name:    common.ShootDNSIngressName,
				DNSName: b.Shoot.GetIngressFQDN("*"),
				Targets: []string{address},
				OwnerID: ownerID,
				TTL:     *b.Config.Controllers.Shoot.DNSEntryTTLSeconds,
			},
			b.Shoot.SeedNamespace,
			b.K8sSeedClient.ChartApplier(),
			b.ChartsRootPath,
			b.Logger,
			seedClient,
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

// DeployManagedResources deploys all the ManagedResource CRDs for the gardener-resource-manager.
func (b *Botanist) DeployManagedResources(ctx context.Context) error {
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

	return b.deployCloudConfigExecutionManagedResource(ctx)
}

// deployCloudConfigExecutionManagedResource creates the cloud config managed resource that contains:
// 1. A secret containing the dedicated cloud config execution script for each worker group
// 2. A secret containing some shared RBAC policies for downloading the cloud config execution script
func (b *Botanist) deployCloudConfigExecutionManagedResource(ctx context.Context) error {
	var (
		managedResourceName = "shoot-cloud-config-execution"
		secretLabels        = map[string]string{
			SecretLabelKeyManagedResource: managedResourceName,
		}
		wantedSecretNames = sets.NewString()
	)

	cloudConfigCharts := map[string]func() (*chartrenderer.RenderedChart, error){
		"shoot-cloud-config-rbac": b.generateCloudConfigRBACChart,
	}

	bootstrapTokenSecret, err := kutil.ComputeBootstrapToken(ctx, b.K8sShootClient.Client(), utils.ComputeSHA256Hex([]byte(time.Now().Format("2006-01-02")))[:6], "A bootstrap token generated by Gardener.", 48*time.Hour)
	if err != nil {
		return fmt.Errorf("error computing bootstrap token for shoot cloud config: %+v", err)
	}

	//  for each worker pool add a secret containing the cloud config execution script
	for _, worker := range b.Shoot.Info.Spec.Provider.Workers {
		name := fmt.Sprintf("shoot-cloud-config-execution-%s", worker.Name)
		cloudConfigCharts[name] = b.getGenerateCloudConfigExecutionChartFunc(name, worker, bootstrapTokenSecret)
	}

	cloudConfigManagedResource := common.NewManagedResourceForShoot(b.K8sSeedClient.Client(), managedResourceName, b.Shoot.SeedNamespace, false)

	// reconcile secrets and reference them to the ManagedResource
	fns := make([]flow.TaskFn, 0, len(cloudConfigCharts))
	for name, renderChartFunc := range cloudConfigCharts {
		renderedChart, err := renderChartFunc()
		if err != nil {
			return fmt.Errorf("error rendering %q chart: %+v", name, err)
		}

		secretName, secret := common.NewManagedResourceSecret(b.K8sSeedClient.Client(), name, b.Shoot.SeedNamespace)
		cloudConfigManagedResource.WithSecretRef(secretName)
		wantedSecretNames.Insert(secretName)

		fns = append(fns, func(ctx context.Context) error {
			return secret.
				WithKeyValues(renderedChart.AsSecretData()).
				WithLabels(map[string]string{SecretLabelKeyManagedResource: managedResourceName}).
				Reconcile(ctx)
		})
	}

	if err := flow.Parallel(fns...)(ctx); err != nil {
		return err
	}
	if err := cloudConfigManagedResource.Reconcile(ctx); err != nil {
		return err
	}

	return b.deleteStaleSecretsMatchLabel(ctx, secretLabels, wantedSecretNames)
}

// deleteStaleSecretsMatchLabel deletes the stale secrets that match the labels but are not used anymore
func (b *Botanist) deleteStaleSecretsMatchLabel(ctx context.Context, labels map[string]string, wantedSecretNames sets.String) error {
	c := b.K8sSeedClient.Client()

	secretList := &corev1.SecretList{}
	if err := c.List(ctx, secretList, client.InNamespace(b.Shoot.SeedNamespace), client.MatchingLabels(labels)); err != nil {
		return err
	}

	fns := make([]flow.TaskFn, 0, meta.LenList(secretList))
	for _, secret := range secretList.Items {
		if !wantedSecretNames.Has(secret.Name) {
			toDelete := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secret.Name,
					Namespace: secret.Namespace,
				},
			}
			fns = append(fns, func(ctx context.Context) error {
				return client.IgnoreNotFound(c.Delete(ctx, toDelete, kubernetes.DefaultDeleteOptions...))
			})
		}
	}
	return flow.Parallel(fns...)(ctx)
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
	for extensionType := range b.Shoot.Extensions {
		extensions = append(extensions, extensionType)
	}
	shootInfo["extensions"] = strings.Join(extensions, ",")

	coreDNS, err := b.InjectShootShootImages(coreDNSConfig, common.CoreDNSImageName)
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

	nodelocalDNS, err := b.InjectShootShootImages(nodeLocalDNSConfig, common.NodeLocalDNSImageName)
	if err != nil {
		return nil, err
	}

	nodeProblemDetector, err := b.InjectShootShootImages(nodeProblemDetectorConfig, common.NodeProblemDetectorImageName)
	if err != nil {
		return nil, err
	}

	kubeProxy, err := b.InjectShootShootImages(kubeProxyConfig, common.KubeProxyImageName, common.AlpineImageName)
	if err != nil {
		return nil, err
	}

	nodeExporter, err := b.InjectShootShootImages(nodeExporterConfig, common.NodeExporterImageName)
	if err != nil {
		return nil, err
	}
	blackboxExporter, err := b.InjectShootShootImages(blackboxExporterConfig, common.BlackboxExporterImageName)
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

	apiserverProxy, err := b.InjectShootShootImages(apiserverProxyConfig, common.APIServerProxySidecarImageName, common.APIServerProxyImageName)
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
			"proxyHost": common.GetAPIServerDomain(b.Shoot.InternalClusterDomain),
			"podAnnotations": map[string]interface{}{
				"checksum/secret-konnectivity-agent": b.CheckSums["konnectivity-agent"],
			},
		}

		// Konnectivity agent related values
		konnectivityAgent, err := b.InjectShootShootImages(konnectivityAgentConfig, common.KonnectivityAgentImageName)
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
		if openvpnDiffieHellmanSecret, ok := b.Secrets[common.GardenRoleOpenVPNDiffieHellman]; ok {
			vpnShootConfig["diffieHellmanKey"] = openvpnDiffieHellmanSecret.Data["dh2048.pem"]
		}

		if nodeNetwork != nil {
			vpnShootConfig["nodeNetwork"] = *nodeNetwork
		}

		vpnShoot, err := b.InjectShootShootImages(vpnShootConfig, common.VPNShootImageName)
		if err != nil {
			return nil, err
		}

		values["vpn-shoot"] = common.GenerateAddonConfig(vpnShoot, true)
	}

	return b.K8sShootClient.ChartRenderer().Render(filepath.Join(common.ChartPath, "shoot-core", "components"), "shoot-core", metav1.NamespaceSystem, values)
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
	kubernetesDashboardImagesToInject := []string{common.KubernetesDashboardImageName}

	k8sVersionLessThan116, err := versionutils.CompareVersions(b.Shoot.Info.Spec.Kubernetes.Version, "<", "1.16")
	if err != nil {
		return nil, err
	}
	if !k8sVersionLessThan116 {
		kubernetesDashboardImagesToInject = append(kubernetesDashboardImagesToInject, common.KubernetesDashboardMetricsScraperImageName)
	}

	kubernetesDashboard, err := b.InjectShootShootImages(kubernetesDashboardConfig, kubernetesDashboardImagesToInject...)
	if err != nil {
		return nil, err
	}

	nginxIngressConfig, err := b.GenerateNginxIngressConfig()
	if err != nil {
		return nil, err
	}
	nginxIngress, err := b.InjectShootShootImages(nginxIngressConfig, common.NginxIngressControllerImageName, common.IngressDefaultBackendImageName)
	if err != nil {
		return nil, err
	}

	return b.K8sShootClient.ChartRenderer().Render(filepath.Join(common.ChartPath, "shoot-addons"), "addons", metav1.NamespaceSystem, map[string]interface{}{
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
