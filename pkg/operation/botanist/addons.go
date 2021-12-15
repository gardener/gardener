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
	"strconv"
	"strings"

	"github.com/gardener/gardener/charts"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1helper "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/chartrenderer"
	netpol "github.com/gardener/gardener/pkg/operation/botanist/addons/networkpolicy"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/dns"
	extensionsdnsrecord "github.com/gardener/gardener/pkg/operation/botanist/component/extensions/dnsrecord"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/secrets"
	secretutils "github.com/gardener/gardener/pkg/utils/secrets"
	versionutils "github.com/gardener/gardener/pkg/utils/version"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// SecretLabelKeyManagedResource is a key for a label on a secret with the value 'managed-resource'.
	SecretLabelKeyManagedResource = "managed-resource"
)

// GenerateKubernetesDashboardConfig generates the values which are required to render the chart of
// the kubernetes-dashboard properly.
func (b *Botanist) GenerateKubernetesDashboardConfig() (map[string]interface{}, error) {
	var (
		enabled = gardencorev1beta1helper.KubernetesDashboardEnabled(b.Shoot.GetInfo().Spec.Addons)
		values  = map[string]interface{}{}
	)

	if b.APIServerSNIEnabled() {
		values["kubeAPIServerHost"] = b.outOfClusterAPIServerFQDN()
	}

	if enabled && b.Shoot.GetInfo().Spec.Addons.KubernetesDashboard.AuthenticationMode != nil {
		values["authenticationMode"] = *b.Shoot.GetInfo().Spec.Addons.KubernetesDashboard.AuthenticationMode
	}

	return common.GenerateAddonConfig(values, enabled), nil
}

// DeployIngressDNS deploys the nginx ingress DNSEntry and DNSOwner resources.
func (b *Botanist) DeployIngressDNS(ctx context.Context) error {
	if b.NeedsIngressDNS() {
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

// DestroyIngressDNS destroys the nginx ingress DNSEntry and DNSOwner resources.
func (b *Botanist) DestroyIngressDNS(ctx context.Context) error {
	return component.OpDestroyAndWait(
		b.Shoot.Components.Extensions.DNS.NginxEntry,
		b.Shoot.Components.Extensions.DNS.NginxOwner,
	).Destroy(ctx)
}

// MigrateIngressDNS destroys the nginx ingress DNSEntry and DNSOwner resources,
// without removing the entry from the DNS provider.
func (b *Botanist) MigrateIngressDNS(ctx context.Context) error {
	return component.OpDestroyAndWait(
		b.Shoot.Components.Extensions.DNS.NginxOwner,
		b.Shoot.Components.Extensions.DNS.NginxEntry,
	).Destroy(ctx)
}

// DefaultNginxIngressDNSEntry returns a Deployer which removes existing nginx ingress DNSEntry.
func (b *Botanist) DefaultNginxIngressDNSEntry() component.DeployWaiter {
	return component.OpDestroy(dns.NewEntry(
		b.Logger,
		b.K8sSeedClient.Client(),
		b.Shoot.SeedNamespace,
		&dns.EntryValues{
			Name: common.ShootDNSIngressName,
			TTL:  *b.Config.Controllers.Shoot.DNSEntryTTLSeconds,
		},
	))
}

// DefaultNginxIngressDNSOwner returns DeployWaiter which removes the nginx ingress DNSOwner.
func (b *Botanist) DefaultNginxIngressDNSOwner() component.DeployWaiter {
	return component.OpDestroy(dns.NewOwner(
		b.K8sSeedClient.Client(),
		b.Shoot.SeedNamespace,
		&dns.OwnerValues{
			Name: common.ShootDNSIngressName,
		},
	))
}

// NeedsIngressDNS returns true if the Shoot cluster needs ingress DNS.
func (b *Botanist) NeedsIngressDNS() bool {
	return b.NeedsExternalDNS() && gardencorev1beta1helper.NginxIngressEnabled(b.Shoot.GetInfo().Spec.Addons)
}

// DefaultIngressDNSRecord creates the default deployer for the ingress DNSRecord resource.
func (b *Botanist) DefaultIngressDNSRecord() extensionsdnsrecord.Interface {
	values := &extensionsdnsrecord.Values{
		Name:       b.Shoot.GetInfo().Name + "-" + common.ShootDNSIngressName,
		SecretName: DNSRecordSecretPrefix + "-" + b.Shoot.GetInfo().Name + "-" + DNSExternalName,
		Namespace:  b.Shoot.SeedNamespace,
		TTL:        b.Config.Controllers.Shoot.DNSEntryTTLSeconds,
	}
	if b.NeedsIngressDNS() {
		values.Type = b.Shoot.ExternalDomain.Provider
		if b.Shoot.ExternalDomain.Zone != "" {
			values.Zone = &b.Shoot.ExternalDomain.Zone
		}
		values.SecretData = b.Shoot.ExternalDomain.SecretData
		values.DNSName = b.Shoot.GetIngressFQDN("*")
	}
	return extensionsdnsrecord.New(
		b.Logger,
		b.K8sSeedClient.Client(),
		values,
		extensionsdnsrecord.DefaultInterval,
		extensionsdnsrecord.DefaultSevereThreshold,
		extensionsdnsrecord.DefaultTimeout,
	)
}

// DeployOrDestroyIngressDNSRecord deploys, restores, or destroys the ingress DNSRecord and waits for the operation to complete.
func (b *Botanist) DeployOrDestroyIngressDNSRecord(ctx context.Context) error {
	if b.NeedsIngressDNS() {
		return b.deployIngressDNSRecord(ctx)
	}
	return b.DestroyIngressDNSRecord(ctx)
}

// deployIngressDNSRecord deploys or restores the ingress DNSRecord and waits for the operation to complete.
func (b *Botanist) deployIngressDNSRecord(ctx context.Context) error {
	if err := b.deployOrRestoreDNSRecord(ctx, b.Shoot.Components.Extensions.IngressDNSRecord); err != nil {
		return err
	}
	return b.Shoot.Components.Extensions.IngressDNSRecord.Wait(ctx)
}

// DestroyIngressDNSRecord destroys the ingress DNSRecord and waits for the operation to complete.
func (b *Botanist) DestroyIngressDNSRecord(ctx context.Context) error {
	if err := b.Shoot.Components.Extensions.IngressDNSRecord.Destroy(ctx); err != nil {
		return err
	}
	return b.Shoot.Components.Extensions.IngressDNSRecord.WaitCleanup(ctx)
}

// MigrateIngressDNSRecord migrates the ingress DNSRecord and waits for the operation to complete.
func (b *Botanist) MigrateIngressDNSRecord(ctx context.Context) error {
	if err := b.Shoot.Components.Extensions.IngressDNSRecord.Migrate(ctx); err != nil {
		return err
	}
	return b.Shoot.Components.Extensions.IngressDNSRecord.WaitMigrate(ctx)
}

// SetNginxIngressAddress sets the IP address of the API server's LoadBalancer.
func (b *Botanist) SetNginxIngressAddress(address string, seedClient client.Client) {
	if b.NeedsIngressDNS() {
		ownerID := *b.Shoot.GetInfo().Status.ClusterIdentity + "-" + common.ShootDNSIngressName
		b.Shoot.Components.Extensions.DNS.NginxOwner = dns.NewOwner(
			seedClient,
			b.Shoot.SeedNamespace,
			&dns.OwnerValues{
				Name:    common.ShootDNSIngressName,
				Active:  pointer.Bool(true),
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
		)

		b.Shoot.Components.Extensions.IngressDNSRecord.SetRecordType(extensionsv1alpha1helper.GetDNSRecordType(address))
		b.Shoot.Components.Extensions.IngressDNSRecord.SetValues([]string{address})
	}
}

// GenerateNginxIngressConfig generates the values which are required to render the chart of
// the nginx-ingress properly.
func (b *Botanist) GenerateNginxIngressConfig() (map[string]interface{}, error) {
	var (
		enabled = gardencorev1beta1helper.NginxIngressEnabled(b.Shoot.GetInfo().Spec.Addons)
		values  map[string]interface{}
	)

	if enabled {
		values = map[string]interface{}{
			"controller": map[string]interface{}{
				"customConfig": b.Shoot.GetInfo().Spec.Addons.NginxIngress.Config,
				"service": map[string]interface{}{
					"loadBalancerSourceRanges": b.Shoot.GetInfo().Spec.Addons.NginxIngress.LoadBalancerSourceRanges,
					"externalTrafficPolicy":    *b.Shoot.GetInfo().Spec.Addons.NginxIngress.ExternalTrafficPolicy,
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

		if err := managedresources.CreateForShoot(ctx, b.K8sSeedClient.Client(), b.Shoot.SeedNamespace, name, false, renderedChart.AsSecretData()); err != nil {
			return err
		}
	}

	// TODO(rfranzke): Remove in a future release.
	return kutil.DeleteObject(ctx, b.K8sSeedClient.Client(), &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "kube-proxy", Namespace: b.Shoot.SeedNamespace}})
}

// generateCoreAddonsChart renders the gardener-resource-manager configuration for the core addons. After that it
// creates a ManagedResource CRD that references the rendered manifests and creates it.
func (b *Botanist) generateCoreAddonsChart(ctx context.Context) (*chartrenderer.RenderedChart, error) {
	kubeProxyKubeconfig, err := runtime.Encode(clientcmdlatest.Codec, kutil.NewKubeconfig(
		b.Shoot.SeedNamespace,
		b.Shoot.ComputeOutOfClusterAPIServerAddress(b.APIServerAddress, true),
		b.LoadSecret(v1beta1constants.SecretNameCACluster).Data[secretutils.DataKeyCertificateCA],
		clientcmdv1.AuthInfo{TokenFile: "/var/run/secrets/kubernetes.io/serviceaccount/token"},
	))
	if err != nil {
		return nil, err
	}

	var (
		kasFQDN = b.outOfClusterAPIServerFQDN()
		global  = map[string]interface{}{
			"kubernetesVersion": b.Shoot.GetInfo().Spec.Kubernetes.Version,
			"podNetwork":        b.Shoot.Networks.Pods.String(),
			"vpaEnabled":        b.Shoot.WantsVerticalPodAutoscaler,
		}
		nodeLocalDNSConfig = map[string]interface{}{
			"domain": gardencorev1beta1.DefaultDomain,
		}

		podSecurityPolicies = map[string]interface{}{
			"allowPrivilegedContainers": *b.Shoot.GetInfo().Spec.Kubernetes.AllowPrivilegedContainers,
		}
		kubeProxyConfig = map[string]interface{}{
			"kubeconfig": kubeProxyKubeconfig,
			"podAnnotations": map[string]interface{}{
				"checksum/secret-kube-proxy": b.LoadCheckSum("kube-proxy"),
			},
			"enableIPVS": b.Shoot.IPVSEnabled(),
			"podNetwork": b.Shoot.Networks.Pods.String(),
			"vpaEnabled": b.Shoot.WantsVerticalPodAutoscaler,
		}
		verticalPodAutoscaler = map[string]interface{}{
			"application": map[string]interface{}{
				"clusterType": "shoot",
				"admissionController": map[string]interface{}{
					"createServiceAccount": false,
					"controlNamespace":     b.Shoot.SeedNamespace,
				},
				"exporter":    map[string]interface{}{"createServiceAccount": false},
				"recommender": map[string]interface{}{"createServiceAccount": false},
				"updater":     map[string]interface{}{"createServiceAccount": false},
			},
		}

		shootInfo = map[string]interface{}{
			"projectName":       b.Garden.Project.Name,
			"shootName":         b.Shoot.GetInfo().Name,
			"provider":          b.Shoot.GetInfo().Spec.Provider.Type,
			"region":            b.Shoot.GetInfo().Spec.Region,
			"kubernetesVersion": b.Shoot.GetInfo().Spec.Kubernetes.Version,
			"podNetwork":        b.Shoot.Networks.Pods.String(),
			"serviceNetwork":    b.Shoot.Networks.Services.String(),
			"maintenanceBegin":  b.Shoot.GetInfo().Spec.Maintenance.TimeWindow.Begin,
			"maintenanceEnd":    b.Shoot.GetInfo().Spec.Maintenance.TimeWindow.End,
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

		nodeNetwork = b.Shoot.GetInfo().Spec.Networking.Nodes
	)

	if b.Shoot.IPVSEnabled() {
		networkPolicyConfig.NodeLocalDNS.KubeDNSClusterIP = common.NodeLocalIPVSAddress
	}

	if b.APIServerSNIEnabled() {
		nodeProblemDetectorConfig["env"] = []interface{}{
			map[string]interface{}{
				"name":  "KUBERNETES_SERVICE_HOST",
				"value": kasFQDN,
			},
		}
	}

	if vpaSecret := b.LoadSecret(common.VPASecretName); vpaSecret != nil {
		verticalPodAutoscaler["application"].(map[string]interface{})["admissionController"].(map[string]interface{})["caCert"] = vpaSecret.Data[secrets.DataKeyCertificateCA]
	}

	proxyConfig := b.Shoot.GetInfo().Spec.Kubernetes.KubeProxy
	kubeProxyEnabled := true
	if proxyConfig != nil {
		kubeProxyConfig["featureGates"] = proxyConfig.FeatureGates
		if proxyConfig.Enabled != nil {
			kubeProxyEnabled = *proxyConfig.Enabled
		}
	}

	workerPoolKubeProxyImages := make(map[string]workerPoolKubeProxyImage)

	for _, worker := range b.Shoot.GetInfo().Spec.Provider.Workers {
		kubernetesVersion, err := gardencorev1beta1helper.CalculateEffectiveKubernetesVersion(b.Shoot.KubernetesVersion, worker.Kubernetes)
		if err != nil {
			return nil, err
		}

		image, err := b.ImageVector.FindImage(charts.ImageNameKubeProxy, imagevector.RuntimeVersion(kubernetesVersion.String()), imagevector.TargetVersion(kubernetesVersion.String()))
		if err != nil {
			return nil, err
		}

		key := workerPoolKubeProxyImagesKey(worker.Name, kubernetesVersion.String())
		workerPoolKubeProxyImages[key] = workerPoolKubeProxyImage{worker.Name, kubernetesVersion.String(), image.String()}
	}

	nodeList := &corev1.NodeList{}
	if err := b.K8sShootClient.Client().List(ctx, nodeList); err != nil {
		return nil, err
	}

	for _, node := range nodeList.Items {
		poolName, ok1 := node.Labels[v1beta1constants.LabelWorkerPool]
		kubernetesVersion, ok2 := node.Labels[v1beta1constants.LabelWorkerKubernetesVersion]
		if !ok1 || !ok2 {
			continue
		}

		image, err := b.ImageVector.FindImage(charts.ImageNameKubeProxy, imagevector.RuntimeVersion(kubernetesVersion), imagevector.TargetVersion(kubernetesVersion))
		if err != nil {
			return nil, err
		}

		key := workerPoolKubeProxyImagesKey(poolName, kubernetesVersion)
		workerPoolKubeProxyImages[key] = workerPoolKubeProxyImage{poolName, kubernetesVersion, image.String()}
	}

	var workerPools []map[string]string

	// TODO(rfranzke): Delete this in a future version.
	{
		kubeProxyImage, err := b.ImageVector.FindImage(charts.ImageNameKubeProxy, imagevector.RuntimeVersion(b.ShootVersion()), imagevector.TargetVersion(b.ShootVersion()))
		if err != nil {
			return nil, err
		}

		workerPools = append(workerPools, map[string]string{
			"name":              "",
			"kubernetesVersion": b.Shoot.GetInfo().Spec.Kubernetes.Version,
			"kubeProxyImage":    kubeProxyImage.String(),
		})
	}

	for _, obj := range workerPoolKubeProxyImages {
		workerPools = append(workerPools, map[string]string{
			"name":              obj.poolName,
			"kubernetesVersion": obj.kubernetesVersion,
			"kubeProxyImage":    obj.image,
		})
	}
	kubeProxyConfig["workerPools"] = workerPools

	if domain := b.Shoot.ExternalClusterDomain; domain != nil {
		shootInfo["domain"] = *domain
	}
	var extensions []string
	for extensionType := range b.Shoot.Components.Extensions.Extension.Extensions() {
		extensions = append(extensions, extensionType)
	}
	shootInfo["extensions"] = strings.Join(extensions, ",")

	// The node-local-dns interface cannot bind the kube-dns cluster IP since the interface
	// used for IPVS load-balancing already uses this address.
	if b.Shoot.IPVSEnabled() {
		nodeLocalDNSConfig["clusterDNS"] = b.Shoot.Networks.CoreDNS.String()
	} else {
		nodeLocalDNSConfig["dnsServer"] = b.Shoot.Networks.CoreDNS.String()
	}

	nodeLocalDNSForceTcpToClusterDNS := true
	if forceTcp, err := strconv.ParseBool(b.Shoot.GetInfo().Annotations[v1beta1constants.AnnotationNodeLocalDNSForceTcpToClusterDns]); err == nil {
		nodeLocalDNSForceTcpToClusterDNS = forceTcp
	}
	nodeLocalDNSConfig["forceTcpToClusterDNS"] = nodeLocalDNSForceTcpToClusterDNS
	nodeLocalDNSForceTcpToUpstreamDNS := true
	if forceTcp, err := strconv.ParseBool(b.Shoot.GetInfo().Annotations[v1beta1constants.AnnotationNodeLocalDNSForceTcpToUpstreamDns]); err == nil {
		nodeLocalDNSForceTcpToUpstreamDNS = forceTcp
	}
	nodeLocalDNSConfig["forceTcpToUpstreamDNS"] = nodeLocalDNSForceTcpToUpstreamDNS

	nodelocalDNS, err := b.InjectShootShootImages(nodeLocalDNSConfig, charts.ImageNameNodeLocalDns)
	if err != nil {
		return nil, err
	}

	nodeProblemDetector, err := b.InjectShootShootImages(nodeProblemDetectorConfig, charts.ImageNameNodeProblemDetector)
	if err != nil {
		return nil, err
	}

	kubeProxy, err := b.InjectShootShootImages(kubeProxyConfig, charts.ImageNameAlpine)
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
			"caBundle": b.LoadSecret(v1beta1constants.SecretNameCACluster).Data[secrets.DataKeyCertificateCA],
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
		"coredns":                common.GenerateAddonConfig(nil, true),
		"vpn-shoot":              common.GenerateAddonConfig(nil, true),
		"node-local-dns":         common.GenerateAddonConfig(nodelocalDNS, b.Shoot.NodeLocalDNSEnabled),
		"kube-apiserver-kubelet": common.GenerateAddonConfig(nil, true),
		"apiserver-proxy":        common.GenerateAddonConfig(apiserverProxy, b.APIServerSNIEnabled()),
		"kube-proxy":             common.GenerateAddonConfig(kubeProxy, kubeProxyEnabled),
		"monitoring": common.GenerateAddonConfig(map[string]interface{}{
			"node-exporter":     nodeExporter,
			"blackbox-exporter": blackboxExporter,
		}, b.Shoot.Purpose != gardencorev1beta1.ShootPurposeTesting),
		"network-policies":        networkPolicyConfig,
		"node-problem-detector":   common.GenerateAddonConfig(nodeProblemDetector, true),
		"podsecuritypolicies":     common.GenerateAddonConfig(podSecurityPolicies, true),
		"shoot-info":              common.GenerateAddonConfig(shootInfo, true),
		"vertical-pod-autoscaler": common.GenerateAddonConfig(verticalPodAutoscaler, b.Shoot.WantsVerticalPodAutoscaler),
		"cluster-identity":        map[string]interface{}{"clusterIdentity": b.Shoot.GetInfo().Status.ClusterIdentity},
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

	k8sVersionLessThan116, err := versionutils.CompareVersions(b.Shoot.GetInfo().Spec.Kubernetes.Version, "<", "1.16")
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

type workerPoolKubeProxyImage struct {
	poolName          string
	kubernetesVersion string
	image             string
}

func workerPoolKubeProxyImagesKey(poolName, kubernetesVersion string) string {
	return poolName + "@" + kubernetesVersion
}
