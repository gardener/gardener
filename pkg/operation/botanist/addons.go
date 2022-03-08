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

	"github.com/gardener/gardener/charts"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1helper "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/controllerutils"
	netpol "github.com/gardener/gardener/pkg/operation/botanist/addons/networkpolicy"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/dns"
	extensionsdnsrecord "github.com/gardener/gardener/pkg/operation/botanist/component/extensions/dnsrecord"
	"github.com/gardener/gardener/pkg/operation/botanist/component/nodelocaldns"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils/images"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/secrets"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		Name:              b.Shoot.GetInfo().Name + "-" + common.ShootDNSIngressName,
		SecretName:        DNSRecordSecretPrefix + "-" + b.Shoot.GetInfo().Name + "-" + v1beta1constants.DNSRecordExternalName,
		Namespace:         b.Shoot.SeedNamespace,
		TTL:               b.Config.Controllers.Shoot.DNSEntryTTLSeconds,
		AnnotateOperation: controllerutils.HasTask(b.Shoot.GetInfo().Annotations, v1beta1constants.ShootTaskDeployDNSRecordIngress) || b.isRestorePhase(),
	}

	// Set component values even if the nginx-ingress addons is not enabled.
	if b.NeedsExternalDNS() {
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

	return nil
}

// generateCoreAddonsChart renders the gardener-resource-manager configuration for the core addons. After that it
// creates a ManagedResource CRD that references the rendered manifests and creates it.
func (b *Botanist) generateCoreAddonsChart(ctx context.Context) (*chartrenderer.RenderedChart, error) {
	var (
		kasFQDN = b.outOfClusterAPIServerFQDN()
		global  = map[string]interface{}{
			"kubernetesVersion": b.Shoot.GetInfo().Spec.Kubernetes.Version,
			"podNetwork":        b.Shoot.Networks.Pods.String(),
			"vpaEnabled":        b.Shoot.WantsVerticalPodAutoscaler,
		}

		podSecurityPolicies = map[string]interface{}{
			"allowPrivilegedContainers": *b.Shoot.GetInfo().Spec.Kubernetes.AllowPrivilegedContainers,
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
		nodeExporterConfig     = map[string]interface{}{}
		blackboxExporterConfig = map[string]interface{}{}
		networkPolicyConfig    = netpol.ShootNetworkPolicyValues{
			Enabled: true,
			NodeLocalDNS: netpol.NodeLocalDNSValues{
				Enabled:          b.Shoot.NodeLocalDNSEnabled,
				KubeDNSClusterIP: b.Shoot.Networks.CoreDNS.String(),
			},
		}

		nodeNetwork = b.Shoot.GetInfo().Spec.Networking.Nodes
	)

	if b.Shoot.IPVSEnabled() {
		networkPolicyConfig.NodeLocalDNS.KubeDNSClusterIP = nodelocaldns.IPVSAddress
	}

	if vpaSecret := b.LoadSecret(common.VPASecretName); vpaSecret != nil {
		verticalPodAutoscaler["application"].(map[string]interface{})["admissionController"].(map[string]interface{})["caCert"] = vpaSecret.Data[secrets.DataKeyCertificateCA]
	}

	workerPools, err := b.computeWorkerPoolsForKubeProxy(ctx)
	if err != nil {
		return nil, err
	}
	var kubeProxyWorkerPools []map[string]string
	for _, obj := range workerPools {
		kubeProxyWorkerPools = append(kubeProxyWorkerPools, map[string]string{
			"name":              obj.Name,
			"kubernetesVersion": obj.KubernetesVersion,
			"kubeProxyImage":    obj.Image,
		})
	}
	kubeProxy := map[string]interface{}{
		"workerPools": kubeProxyWorkerPools,
	}

	if domain := b.Shoot.ExternalClusterDomain; domain != nil {
		shootInfo["domain"] = *domain
	}
	var extensions []string
	for extensionType := range b.Shoot.Components.Extensions.Extension.Extensions() {
		extensions = append(extensions, extensionType)
	}
	shootInfo["extensions"] = strings.Join(extensions, ",")

	nodeExporter, err := b.InjectShootShootImages(nodeExporterConfig, images.ImageNameNodeExporter)
	if err != nil {
		return nil, err
	}
	blackboxExporter, err := b.InjectShootShootImages(blackboxExporterConfig, images.ImageNameBlackboxExporter)
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

	apiserverProxy, err := b.InjectShootShootImages(apiserverProxyConfig, images.ImageNameApiserverProxySidecar, images.ImageNameApiserverProxy)
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
		"node-local-dns":         common.GenerateAddonConfig(nil, b.Shoot.NodeLocalDNSEnabled),
		"kube-apiserver-kubelet": common.GenerateAddonConfig(nil, true),
		"apiserver-proxy":        common.GenerateAddonConfig(apiserverProxy, b.APIServerSNIEnabled()),
		"kube-proxy":             common.GenerateAddonConfig(kubeProxy, gardencorev1beta1helper.KubeProxyEnabled(b.Shoot.GetInfo().Spec.Kubernetes.KubeProxy)),
		"monitoring": common.GenerateAddonConfig(map[string]interface{}{
			"node-exporter":     nodeExporter,
			"blackbox-exporter": blackboxExporter,
		}, b.Shoot.Purpose != gardencorev1beta1.ShootPurposeTesting),
		"network-policies":        networkPolicyConfig,
		"node-problem-detector":   common.GenerateAddonConfig(nil, true),
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
	kubernetesDashboardImagesToInject := []string{
		images.ImageNameKubernetesDashboard,
		images.ImageNameKubernetesDashboardMetricsScraper,
	}

	kubernetesDashboard, err := b.InjectShootShootImages(kubernetesDashboardConfig, kubernetesDashboardImagesToInject...)
	if err != nil {
		return nil, err
	}

	nginxIngressConfig, err := b.GenerateNginxIngressConfig()
	if err != nil {
		return nil, err
	}
	nginxIngress, err := b.InjectShootShootImages(nginxIngressConfig, images.ImageNameNginxIngressController, images.ImageNameIngressDefaultBackend)
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
