// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
	"net"
	"slices"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/component-base/featuregate"
	"k8s.io/utils/ptr"

	admissioncontrollerconfigv1alpha1 "github.com/gardener/gardener/pkg/admissioncontroller/apis/config/v1alpha1"
	admissioncontrollervalidation "github.com/gardener/gardener/pkg/admissioncontroller/apis/config/v1alpha1/validation"
	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencoreinstall "github.com/gardener/gardener/pkg/apis/core/install"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	gardencorevalidation "github.com/gardener/gardener/pkg/apis/core/validation"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	operatorv1alpha1conversion "github.com/gardener/gardener/pkg/apis/operator/v1alpha1/conversion"
	"github.com/gardener/gardener/pkg/apis/operator/v1alpha1/helper"
	sharedcomponent "github.com/gardener/gardener/pkg/component/shared"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/gardener/operator"
	cidrvalidation "github.com/gardener/gardener/pkg/utils/validation/cidr"
	featuresvalidation "github.com/gardener/gardener/pkg/utils/validation/features"
	"github.com/gardener/gardener/pkg/utils/validation/kubernetesversion"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
	plugin "github.com/gardener/gardener/plugin/pkg"
)

var (
	gardenCoreScheme *runtime.Scheme
	// TODO(AleksandarSavchev): Remove this variable and the associated validation after support for Kubernetes v1.33 is dropped.
	forbiddenETCDEncryptionKeyShootOperationsWithK8s134 = sets.New(
		v1beta1constants.OperationRotateETCDEncryptionKeyStart,
		v1beta1constants.OperationRotateETCDEncryptionKeyComplete,
	)
)

func init() {
	gardenCoreScheme = runtime.NewScheme()
	utilruntime.Must(gardencoreinstall.AddToScheme(gardenCoreScheme))
	utilruntime.Must(admissioncontrollerconfigv1alpha1.AddToScheme(gardenCoreScheme))
}

// ValidateGarden contains functionality for performing extended validation of a Garden object which is not possible
// with standard CRD validation, see https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#validation-rules.
func ValidateGarden(garden *operatorv1alpha1.Garden, extensions []operatorv1alpha1.Extension) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateOperation(garden.Annotations[v1beta1constants.GardenerOperation], garden, field.NewPath("metadata", "annotations"))...)
	allErrs = append(allErrs, validateDNS(garden.Spec.DNS, field.NewPath("spec", "dns"))...)
	allErrs = append(allErrs, validateExtensions(garden.Spec.Extensions, extensions, field.NewPath("spec", "extensions"))...)
	allErrs = append(allErrs, validateRuntimeCluster(garden.Spec.DNS, garden.Spec.RuntimeCluster, helper.HighAvailabilityEnabled(garden), field.NewPath("spec", "runtimeCluster"))...)
	allErrs = append(allErrs, validateVirtualCluster(garden.Spec.DNS, garden.Spec.VirtualCluster, garden.Spec.RuntimeCluster, field.NewPath("spec", "virtualCluster"))...)

	return allErrs
}

// ValidateGardenUpdate contains functionality for performing extended validation of a Garden object under update which
// is not possible with standard CRD validation, see https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#validation-rules.
func ValidateGardenUpdate(oldGarden, newGarden *operatorv1alpha1.Garden, extensions []operatorv1alpha1.Extension) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateRuntimeClusterUpdate(oldGarden, newGarden)...)
	allErrs = append(allErrs, validateVirtualClusterUpdate(oldGarden, newGarden)...)
	allErrs = append(allErrs, ValidateGarden(newGarden, extensions)...)

	return allErrs
}

func validateRuntimeClusterUpdate(oldGarden, newGarden *operatorv1alpha1.Garden) field.ErrorList {
	var (
		allErrs           = field.ErrorList{}
		oldRuntimeCluster = oldGarden.Spec.RuntimeCluster
		newRuntimeCluster = newGarden.Spec.RuntimeCluster
		fldPath           = field.NewPath("spec", "runtimeCluster")
	)

	// First domain is immutable.
	// Keep the first value immutable because components like the Gardener Discovery Server and Workload Identity depend on it.
	if len(oldRuntimeCluster.Ingress.Domains) > 0 && len(newRuntimeCluster.Ingress.Domains) > 0 {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(oldRuntimeCluster.Ingress.Domains[0].Name, newRuntimeCluster.Ingress.Domains[0].Name, fldPath.Child("ingress", "domains").Index(0))...)
	}

	for _, n := range []struct {
		new, old []string
		name     string
	}{
		{newRuntimeCluster.Networking.Nodes, oldRuntimeCluster.Networking.Nodes, "nodes"},
		{newRuntimeCluster.Networking.Pods, oldRuntimeCluster.Networking.Pods, "pods"},
		{newRuntimeCluster.Networking.Services, oldRuntimeCluster.Networking.Services, "services"},
	} {
		if len(n.new) < len(n.old) {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("networking", n.name), n.name+" cannot be removed"))
		}
		for i := min(len(n.new), len(n.old)) - 1; i >= 0; i-- {
			allErrs = append(allErrs, apivalidation.ValidateImmutableField(n.new[i], n.old[i], fldPath.Child("networking", n.name).Index(i))...)
		}
	}

	return allErrs
}

func validateVirtualClusterUpdate(oldGarden, newGarden *operatorv1alpha1.Garden) field.ErrorList {
	var (
		allErrs           = field.ErrorList{}
		oldVirtualCluster = oldGarden.Spec.VirtualCluster
		newVirtualCluster = newGarden.Spec.VirtualCluster
		fldPath           = field.NewPath("spec", "virtualCluster")
	)

	// First domain is immutable. Changing this would incompatibly change the service account issuer in the cluster, ref https://github.com/gardener/gardener/blob/17ff592e734131ef746560641bdcdec3bcfce0f1/pkg/component/kubeapiserver/deployment.go#L585C8-L585C8
	// Note: We can consider supporting this scenario in the future but would need to re-issue all service account tokens during the reconcile run.
	if len(oldVirtualCluster.DNS.Domains) > 0 && len(newVirtualCluster.DNS.Domains) > 0 {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(oldVirtualCluster.DNS.Domains[0].Name, newVirtualCluster.DNS.Domains[0].Name, fldPath.Child("dns", "domains").Index(0).Child("name"))...)
	}

	if oldVirtualCluster.ETCD != nil && oldVirtualCluster.ETCD.Main != nil && oldVirtualCluster.ETCD.Main.Backup != nil &&
		newVirtualCluster.ETCD != nil && newVirtualCluster.ETCD.Main != nil {
		fldBackup := fldPath.Child("etcd", "main", "backup")
		if newVirtualCluster.ETCD.Main.Backup != nil {
			allErrs = append(allErrs, apivalidation.ValidateImmutableField(oldVirtualCluster.ETCD.Main.Backup.BucketName, newVirtualCluster.ETCD.Main.Backup.BucketName, fldBackup.Child("bucketName"))...)
		}
		if newVirtualCluster.ETCD.Main.Backup == nil {
			allErrs = append(allErrs, field.Forbidden(fldBackup, "backup must not be deactivated if it was set before"))
		}
	}

	if oldVirtualCluster.ControlPlane != nil && oldVirtualCluster.ControlPlane.HighAvailability != nil &&
		(newVirtualCluster.ControlPlane == nil || newVirtualCluster.ControlPlane.HighAvailability == nil) {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(oldVirtualCluster.ControlPlane, newVirtualCluster.ControlPlane, fldPath.Child("controlPlane", "highAvailability"))...)
	}

	allErrs = append(allErrs, gardencorevalidation.ValidateKubernetesVersionUpdate(newVirtualCluster.Kubernetes.Version, oldVirtualCluster.Kubernetes.Version, false, fldPath.Child("kubernetes", "version"))...)
	allErrs = append(allErrs, validateEncryptionConfigUpdate(oldGarden, newGarden)...)

	if len(newVirtualCluster.Networking.Services) < len(oldVirtualCluster.Networking.Services) {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("networking", "services"), "services cannot be removed"))
	}
	for i := min(len(newVirtualCluster.Networking.Services), len(oldVirtualCluster.Networking.Services)) - 1; i >= 0; i-- {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newVirtualCluster.Networking.Services[i], oldVirtualCluster.Networking.Services[i], fldPath.Child("networking", "services").Index(i))...)
	}

	return allErrs
}

func validateDNS(dns *operatorv1alpha1.DNSManagement, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if dns != nil {
		names := sets.New[string]()
		for i, provider := range dns.Providers {
			if names.Has(provider.Name) {
				allErrs = append(allErrs, field.Duplicate(fldPath.Child("providers").Index(i), provider.Name))
			}
			names.Insert()
		}
	}

	return allErrs
}

func validateRuntimeCluster(dns *operatorv1alpha1.DNSManagement, runtimeCluster operatorv1alpha1.RuntimeCluster, virtualClusterHAEnabled bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateDomains(dns, runtimeCluster.Ingress.Domains, fldPath.Child("ingress", "domains"))...)
	allErrs = append(allErrs, validateRuntimeClusterNetworking(runtimeCluster.Networking, fldPath.Child("networking"))...)
	allErrs = append(allErrs, validateRuntimeClusterSettings(runtimeCluster, virtualClusterHAEnabled, fldPath.Child("settings"))...)

	return allErrs
}

func validateDomains(dns *operatorv1alpha1.DNSManagement, domains []operatorv1alpha1.DNSDomain, path *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	names := sets.New[string]()
	for i, domain := range domains {
		allErrs = append(allErrs, gardencorevalidation.ValidateDNS1123Subdomain(domain.Name, path.Index(i).Child("name"))...)
		if names.Has(domain.Name) {
			allErrs = append(allErrs, field.Duplicate(path.Index(i).Child("name"), domain.Name))
		}
		names.Insert(domain.Name)
		if dns != nil {
			if domain.Provider != nil {
				if !hasProvider(dns, *domain.Provider) {
					allErrs = append(allErrs, field.Invalid(path.Index(i).Child("provider"), *domain.Provider, "provider name not found in .spec.dns.providers"))
				}
			} else {
				allErrs = append(allErrs, field.Required(path.Index(i).Child("provider"), "provider name must be set if `.spec.dns` is set"))
			}
		}
	}

	return allErrs
}

func validateRuntimeClusterNetworking(runtimeNetworking operatorv1alpha1.RuntimeNetworking, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for i, nodes := range runtimeNetworking.Nodes {
		if _, _, err := net.ParseCIDR(nodes); err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("nodes").Index(i), nodes, "cannot parse node network cidr of runtime cluster: "+err.Error()))
		}
	}
	for i, pods := range runtimeNetworking.Pods {
		if _, _, err := net.ParseCIDR(pods); err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("pods").Index(i), pods, "cannot parse pod network cidr of runtime cluster: "+err.Error()))
		}
	}
	for i, services := range runtimeNetworking.Services {
		if _, _, err := net.ParseCIDR(services); err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("services").Index(i), services, "cannot parse service network cidr of runtime cluster: "+err.Error()))
		}
	}

	// In case any IP ranges are incorrect, there is no benefit in checking for intersections.
	if len(allErrs) > 0 {
		return allErrs
	}

	for i, nodes := range runtimeNetworking.Nodes {
		for _, services := range runtimeNetworking.Services {
			if cidrvalidation.NetworksIntersect(nodes, services) {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("nodes").Index(i), nodes, "node network of runtime cluster intersects with service network of runtime cluster"))
			}
		}
		for _, pods := range runtimeNetworking.Pods {
			if cidrvalidation.NetworksIntersect(nodes, pods) {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("nodes").Index(i), nodes, "node network of runtime cluster intersects with pod network of runtime cluster"))
			}
		}
	}
	for i, pods := range runtimeNetworking.Pods {
		for _, services := range runtimeNetworking.Services {
			if cidrvalidation.NetworksIntersect(pods, services) {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("pods").Index(i), services, "pod network of runtime cluster intersects with service network of runtime cluster"))
			}
		}
	}

	return allErrs
}

func validateRuntimeClusterSettings(runtimeCluster operatorv1alpha1.RuntimeCluster, virtualClusterHAEnabled bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if runtimeCluster.Settings == nil {
		return allErrs
	}

	if runtimeCluster.Settings.VerticalPodAutoscaler != nil {
		allErrs = append(allErrs, featuresvalidation.ValidateVpaFeatureGates(runtimeCluster.Settings.VerticalPodAutoscaler.FeatureGates, fldPath.Child("verticalPodAutoscaler", "featureGates"))...)
		allErrs = append(allErrs, gardencorevalidation.ValidateVerticalPodAutoscalerMaxAllowed(runtimeCluster.Settings.VerticalPodAutoscaler.MaxAllowed, fldPath.Child("verticalPodAutoscaler"))...)
	}

	if helper.TopologyAwareRoutingEnabled(runtimeCluster.Settings) {
		if len(runtimeCluster.Provider.Zones) <= 1 {
			allErrs = append(allErrs, field.Forbidden(field.NewPath("spec", "runtimeCluster", "settings", "topologyAwareRouting", "enabled"), "topology-aware routing can only be enabled on multi-zone garden runtime cluster (with at least two zones in spec.provider.zones)"))
		}
		if !virtualClusterHAEnabled {
			allErrs = append(allErrs, field.Forbidden(field.NewPath("spec", "runtimeCluster", "settings", "topologyAwareRouting", "enabled"), "topology-aware routing can only be enabled when virtual cluster's high-availability is enabled"))
		}
	}

	return allErrs
}

func validateVirtualCluster(dns *operatorv1alpha1.DNSManagement, virtualCluster operatorv1alpha1.VirtualCluster, runtimeCluster operatorv1alpha1.RuntimeCluster, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateDomains(dns, virtualCluster.DNS.Domains, fldPath.Child("dns", "domains"))...)

	if virtualCluster.ETCD != nil && virtualCluster.ETCD.Main != nil {
		allErrs = append(allErrs, validateETCDAutoscaling(virtualCluster.ETCD.Main.Autoscaling, corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("300M")}, fldPath.Child("etcd", "main", "autoscaling"))...)
	}

	if virtualCluster.ETCD != nil && virtualCluster.ETCD.Events != nil {
		allErrs = append(allErrs, validateETCDAutoscaling(virtualCluster.ETCD.Events.Autoscaling, corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("60M")}, fldPath.Child("etcd", "events", "autoscaling"))...)
	}

	if err := kubernetesversion.CheckIfSupported(virtualCluster.Kubernetes.Version); err != nil {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("kubernetes", "version"), virtualCluster.Kubernetes.Version, kubernetesversion.SupportedVersions))
	}

	if kubeAPIServer := virtualCluster.Kubernetes.KubeAPIServer; kubeAPIServer != nil {
		path := fldPath.Child("kubernetes", "kubeAPIServer")

		if kubeAPIServer.KubeAPIServerConfig != nil {
			coreKubeAPIServerConfig := &gardencore.KubeAPIServerConfig{}
			if err := gardenCoreScheme.Convert(kubeAPIServer.KubeAPIServerConfig, coreKubeAPIServerConfig, nil); err != nil {
				allErrs = append(allErrs, field.InternalError(path, err))
			}

			allErrs = append(allErrs, gardencorevalidation.ValidateKubeAPIServer(coreKubeAPIServerConfig, virtualCluster.Kubernetes.Version, true, gardenerutils.DefaultGroupResourcesForEncryption(), path)...)
		}

		// The API server domain of the virtual cluster which is derived from the primary (immutable) DNS name does not
		// match any SNI domain pattern. It is used by gardenlets to connect to the virtual cluster API server, and they
		// expect a certificate signed by the Garden CA.
		if kubeAPIServer.SNI != nil && len(virtualCluster.DNS.Domains) > 0 {
			for i, domainPattern := range kubeAPIServer.SNI.DomainPatterns {
				apiServerDomain := v1beta1helper.GetAPIServerDomain(virtualCluster.DNS.Domains[0].Name)
				if len(helper.GetAPIServerSNIDomains([]string{apiServerDomain}, operatorv1alpha1.SNI{DomainPatterns: []string{domainPattern}})) > 0 {
					allErrs = append(allErrs, field.Forbidden(path.Child("sni", "domainPatterns").Index(i), fmt.Sprintf("domain patterns of SNI must not match the first API server domain of the virtual cluster configuration (%s)", apiServerDomain)))
				}
			}
		}
	}

	if kubeControllerManager := virtualCluster.Kubernetes.KubeControllerManager; kubeControllerManager != nil && kubeControllerManager.KubeControllerManagerConfig != nil {
		path := fldPath.Child("kubernetes", "kubeControllerManager")

		coreKubeControllerManagerConfig := &gardencore.KubeControllerManagerConfig{}
		if err := gardenCoreScheme.Convert(kubeControllerManager.KubeControllerManagerConfig, coreKubeControllerManagerConfig, nil); err != nil {
			allErrs = append(allErrs, field.InternalError(path, err))
		}

		allErrs = append(allErrs, gardencorevalidation.ValidateKubeControllerManager(coreKubeControllerManagerConfig, nil, virtualCluster.Kubernetes.Version, true, path)...)
	}

	allErrs = append(allErrs, validateGardener(virtualCluster.Gardener, virtualCluster.Kubernetes, fldPath.Child("gardener"))...)

	for i, services := range virtualCluster.Networking.Services {
		if _, _, err := net.ParseCIDR(services); err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("networking", "services").Index(i), services, "cannot parse service network cidr: "+err.Error()))
		}
		for _, runtimePods := range runtimeCluster.Networking.Pods {
			if cidrvalidation.NetworksIntersect(runtimePods, services) {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("networking", "services").Index(i), services, "pod network of runtime cluster intersects with service network of virtual cluster"))
			}
		}
		for _, runtimeServices := range runtimeCluster.Networking.Services {
			if cidrvalidation.NetworksIntersect(runtimeServices, services) {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("networking", "services").Index(i), services, "service network of runtime cluster intersects with service network of virtual cluster"))
			}
		}
		for _, runtimeNodes := range runtimeCluster.Networking.Nodes {
			if cidrvalidation.NetworksIntersect(runtimeNodes, services) {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("networking", "services").Index(i), services, "node network of runtime cluster intersects with service network of virtual cluster"))
			}
		}
	}

	// Kubernetes standard components only allow two ranges of different IP families being specified.
	if len(virtualCluster.Networking.Services) > 2 {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("networking", "services"), fmt.Sprintf("at most two service ranges allowed: %s", virtualCluster.Networking.Services)))
	} else if len(virtualCluster.Networking.Services) == 2 {
		firstCIDR := cidrvalidation.NewCIDR(virtualCluster.Networking.Services[0], fldPath.Child("networking", "services").Index(0))
		secondCIDR := cidrvalidation.NewCIDR(virtualCluster.Networking.Services[1], fldPath.Child("networking", "services").Index(1))
		if firstCIDR.IsIPv4() && secondCIDR.IsIPv4() || firstCIDR.IsIPv6() && secondCIDR.IsIPv6() {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("networking", "services"), fmt.Sprintf("service ranges must be of different IP families: %s", virtualCluster.Networking.Services)))
		}
	}

	return allErrs
}

func validateETCDAutoscaling(autoscaling *gardencorev1beta1.ControlPlaneAutoscaling, minRequired corev1.ResourceList, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if autoscaling != nil {
		coreAutoscaling := &gardencore.ControlPlaneAutoscaling{}
		if err := gardenCoreScheme.Convert(autoscaling, coreAutoscaling, nil); err != nil {
			allErrs = append(allErrs, field.InternalError(fldPath, err))
		}

		allErrs = append(allErrs, gardencorevalidation.ValidateControlPlaneAutoscaling(coreAutoscaling, minRequired, fldPath)...)
	}

	return allErrs
}

func validateGardener(gardener operatorv1alpha1.Gardener, kubernetes operatorv1alpha1.Kubernetes, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateGardenerAPIServerConfig(gardener.APIServer, fldPath.Child("gardenerAPIServer"))...)
	allErrs = append(allErrs, validateGardenerAdmissionController(gardener.AdmissionController, fldPath.Child("gardenerAdmissionController"))...)
	allErrs = append(allErrs, validateGardenerControllerManagerConfig(gardener.ControllerManager, fldPath.Child("gardenerControllerManager"))...)
	allErrs = append(allErrs, validateGardenerSchedulerConfig(gardener.Scheduler, fldPath.Child("gardenerScheduler"))...)
	allErrs = append(allErrs, validateGardenerDashboardConfig(gardener.Dashboard, kubernetes.KubeAPIServer, fldPath.Child("gardenerDashboard"))...)

	return allErrs
}

func validateGardenerAPIServerConfig(config *operatorv1alpha1.GardenerAPIServerConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if config == nil {
		return allErrs
	}

	allErrs = append(allErrs, validateGardenerFeatureGates(config.FeatureGates, fldPath.Child("featureGates"))...)

	for i, admissionPlugin := range config.AdmissionPlugins {
		idxPath := fldPath.Child("admissionPlugins").Index(i)

		if len(admissionPlugin.Name) == 0 {
			allErrs = append(allErrs, field.Required(idxPath.Child("name"), "must provide a name"))
			return allErrs
		}

		if !slices.Contains(plugin.AllPluginNames(), admissionPlugin.Name) {
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("name"), admissionPlugin.Name, plugin.AllPluginNames()))
		}
	}

	if config.EncryptionConfig != nil {
		seenResources := sets.New[schema.GroupResource]()

		for i, resource := range config.EncryptionConfig.Resources {
			var (
				idxPath = fldPath.Child("encryptionConfig", "resources").Index(i)
				gr      = schema.ParseGroupResource(resource)
			)

			if strings.ToLower(resource) != resource {
				allErrs = append(allErrs, field.Invalid(idxPath, resource, "resource must be lower case"))
			}
			if strings.Contains(resource, "*") {
				allErrs = append(allErrs, field.Invalid(idxPath, resource, "wildcards are not supported"))
			}

			if seenResources.Has(gr) {
				allErrs = append(allErrs, field.Duplicate(idxPath, resource))
			}
			if slices.Contains(gardenerutils.DefaultGardenerGroupResourcesForEncryption(), gr) {
				allErrs = append(allErrs, field.Forbidden(idxPath, fmt.Sprintf("%q are always encrypted", resource)))
			}

			if !operator.IsServedByGardenerAPIServer(resource) {
				allErrs = append(allErrs, field.Invalid(idxPath, resource, "should be a resource served by gardener-apiserver. ie; should have any of the suffixes {authentication,core,operations,security,settings,seedmanagement}.gardener.cloud"))
			}

			seenResources.Insert(gr)
		}
	}

	if auditConfig := config.AuditConfig; auditConfig != nil {
		auditPath := fldPath.Child("auditConfig")
		if auditPolicy := auditConfig.AuditPolicy; auditPolicy != nil && auditConfig.AuditPolicy.ConfigMapRef != nil {
			allErrs = append(allErrs, gardencorevalidation.ValidateAuditPolicyConfigMapReference(auditPolicy.ConfigMapRef, auditPath.Child("auditPolicy", "configMapRef"))...)
		}
	}

	if config.WatchCacheSizes != nil {
		watchCacheSizes := &gardencore.WatchCacheSizes{}
		if err := gardenCoreScheme.Convert(config.WatchCacheSizes, watchCacheSizes, nil); err != nil {
			allErrs = append(allErrs, field.InternalError(fldPath.Child("watchCacheSizes"), err))
		}
		allErrs = append(allErrs, gardencorevalidation.ValidateWatchCacheSizes(watchCacheSizes, fldPath.Child("watchCacheSizes"))...)
	}

	if config.Logging != nil {
		logging := &gardencore.APIServerLogging{}
		if err := gardenCoreScheme.Convert(config.Logging, logging, nil); err != nil {
			allErrs = append(allErrs, field.InternalError(fldPath.Child("logging"), err))
		}
		allErrs = append(allErrs, gardencorevalidation.ValidateAPIServerLogging(logging, fldPath.Child("logging"))...)
	}

	if config.Requests != nil {
		requests := &gardencore.APIServerRequests{}
		if err := gardenCoreScheme.Convert(config.Requests, requests, nil); err != nil {
			allErrs = append(allErrs, field.InternalError(fldPath.Child("requests"), err))
		}
		allErrs = append(allErrs, gardencorevalidation.ValidateAPIServerRequests(requests, fldPath.Child("requests"))...)
	}

	if config.ShootAdminKubeconfigMaxExpiration != nil {
		if config.ShootAdminKubeconfigMaxExpiration.Duration < time.Hour || config.ShootAdminKubeconfigMaxExpiration.Duration > time.Duration(1<<32)*time.Second {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("shootAdminKubeconfigMaxExpiration"), config.ShootAdminKubeconfigMaxExpiration.Duration, "must be between 1h and 2^32 seconds"))
		}
	}

	return allErrs
}

func validateGardenerAdmissionController(config *operatorv1alpha1.GardenerAdmissionControllerConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if config == nil {
		return allErrs
	}

	if config.ResourceAdmissionConfiguration != nil {
		externalAdmissionConfiguration := operatorv1alpha1conversion.ConvertToAdmissionControllerResourceAdmissionConfiguration(config.ResourceAdmissionConfiguration)
		allErrs = append(allErrs, admissioncontrollervalidation.ValidateResourceAdmissionConfiguration(externalAdmissionConfiguration, fldPath.Child("resourceAdmissionConfiguration"))...)
	}

	return allErrs
}

func validateGardenerControllerManagerConfig(config *operatorv1alpha1.GardenerControllerManagerConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if config == nil {
		return allErrs
	}

	allErrs = append(allErrs, validateGardenerFeatureGates(config.FeatureGates, fldPath.Child("featureGates"))...)

	for i, quota := range config.DefaultProjectQuotas {
		allErrs = append(allErrs, metav1validation.ValidateLabelSelector(quota.ProjectSelector, metav1validation.LabelSelectorValidationOptions{}, fldPath.Child("defaultProjectQuotas").Index(i).Child("projectSelector"))...)
	}

	return allErrs
}

func validateGardenerSchedulerConfig(config *operatorv1alpha1.GardenerSchedulerConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if config == nil {
		return allErrs
	}

	allErrs = append(allErrs, validateGardenerFeatureGates(config.FeatureGates, fldPath.Child("featureGates"))...)

	return allErrs
}

func validateGardenerFeatureGates(featureGates map[string]bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for featureGate := range featureGates {
		spec, supported := features.AllFeatureGates[featuregate.Feature(featureGate)]
		if !supported {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child(featureGate), "not supported by Gardener"))
		} else if spec.LockToDefault && featureGates[featureGate] != spec.Default {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child(featureGate), fmt.Sprintf("cannot set feature gate to %v, feature is locked to %v", featureGates[featureGate], spec.Default)))
		}
	}

	return allErrs
}

func validateGardenerDashboardConfig(config *operatorv1alpha1.GardenerDashboardConfig, kubeAPIServerConfig *operatorv1alpha1.KubeAPIServerConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if config == nil {
		return allErrs
	}

	if !ptr.Deref(config.EnableTokenLogin, true) && config.OIDCConfig == nil {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("enableTokenLogin"), "OIDC must be configured when token login is disabled"))
	}

	if oidc := config.OIDCConfig; oidc != nil {
		oidcPath := fldPath.Child("oidcConfig")

		if kubeAPIServerConfig == nil || (kubeAPIServerConfig.OIDCConfig == nil && kubeAPIServerConfig.StructuredAuthentication == nil) {
			allErrs = append(allErrs, field.Invalid(oidcPath, config.OIDCConfig, "must set OIDC configuration in .spec.virtualCluster.kubernetes.kubeAPIServer when configuring OIDC config for dashboard"))
		}

		if oidc.IssuerURL == nil {
			if oidc.ClientIDPublic != nil {
				allErrs = append(allErrs, field.Required(oidcPath.Child("issuerURL"), "must provide Issuer URL when ClientIDPublic is set"))
			} else if kubeAPIServerConfig == nil || kubeAPIServerConfig.OIDCConfig == nil || kubeAPIServerConfig.OIDCConfig.IssuerURL == nil {
				allErrs = append(allErrs, field.Required(oidcPath.Child("issuerURL"), "must provide Issuer URL"))
			}
		}

		if oidc.ClientIDPublic == nil {
			if oidc.IssuerURL != nil {
				allErrs = append(allErrs, field.Required(oidcPath.Child("clientIDPublic"), "must provide a public client ID when Issuer URL is set"))
			} else if kubeAPIServerConfig == nil || kubeAPIServerConfig.OIDCConfig == nil || kubeAPIServerConfig.OIDCConfig.ClientID == nil {
				allErrs = append(allErrs, field.Required(oidcPath.Child("clientIDPublic"), "must provide a public client ID"))
			}
		}

		if oidc.IssuerURL != nil {
			allErrs = append(allErrs, gardencorevalidation.ValidateOIDCIssuerURL(*oidc.IssuerURL, oidcPath.Child("issuerURL"))...)
		}
	}

	return allErrs
}

func validateOperation(operation string, garden *operatorv1alpha1.Garden, fldPath *field.Path) field.ErrorList {
	var (
		allErrs       = field.ErrorList{}
		k8sLess134, _ = versionutils.CheckVersionMeetsConstraint(garden.Spec.VirtualCluster.Kubernetes.Version, "< 1.34")
	)

	if operation == "" {
		return allErrs
	}

	fldPathOp := fldPath.Key(v1beta1constants.GardenerOperation)

	if operation != "" {
		if forbiddenETCDEncryptionKeyShootOperationsWithK8s134.Has(operation) && !k8sLess134 {
			allErrs = append(allErrs, field.Forbidden(fldPathOp, fmt.Sprintf("for Kubernetes versions >= 1.34, operation '%s' is no longer supported, please use 'rotate-etcd-encryption-key' instead, which performs a complete etcd encryption key rotation", operation)))
		}
		if !operatorv1alpha1.AvailableOperationAnnotations.Has(operation) {
			allErrs = append(allErrs, field.NotSupported(fldPathOp, operation, sets.List(operatorv1alpha1.AvailableOperationAnnotations)))
		}
	}

	allErrs = append(allErrs, validateOperationContext(operation, garden, fldPathOp)...)

	return allErrs
}

func validateOperationContext(operation string, garden *operatorv1alpha1.Garden, fldPath *field.Path) field.ErrorList {
	var (
		allErrs                  = field.ErrorList{}
		encryptedResources       = sets.New[schema.GroupResource]()
		resourcesToEncrypt       = sets.New[schema.GroupResource]()
		encryptionConfig         *gardencorev1beta1.EncryptionConfig
		gardenerEncryptionConfig *gardencorev1beta1.EncryptionConfig
		k8sLess134, _            = versionutils.CheckVersionMeetsConstraint(garden.Spec.VirtualCluster.Kubernetes.Version, "< 1.34")
	)

	if garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer != nil && garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer.KubeAPIServerConfig != nil {
		encryptionConfig = garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer.EncryptionConfig
	}
	if garden.Spec.VirtualCluster.Gardener.APIServer != nil && garden.Spec.VirtualCluster.Gardener.APIServer.EncryptionConfig != nil {
		gardenerEncryptionConfig = garden.Spec.VirtualCluster.Gardener.APIServer.EncryptionConfig
	}

	for _, r := range garden.Status.EncryptedResources {
		encryptedResources.Insert(schema.ParseGroupResource(r))
	}
	resourcesToEncrypt.Insert(sharedcomponent.GetResourcesForEncryptionFromConfig(encryptionConfig)...)
	resourcesToEncrypt.Insert(sharedcomponent.GetResourcesForEncryptionFromConfig(gardenerEncryptionConfig)...)

	switch operation {
	case v1beta1constants.OperationRotateCredentialsStart:
		if garden.DeletionTimestamp != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot start rotation of all credentials if garden has deletion timestamp"))
		}
		if phase := helper.GetCARotationPhase(garden.Status.Credentials); len(phase) > 0 && phase != gardencorev1beta1.RotationCompleted {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot start rotation of all credentials if .status.credentials.rotation.certificateAuthorities.phase is not 'Completed'"))
		}
		if phase := helper.GetServiceAccountKeyRotationPhase(garden.Status.Credentials); len(phase) > 0 && phase != gardencorev1beta1.RotationCompleted {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot start rotation of all credentials if .status.credentials.rotation.serviceAccountKey.phase is not 'Completed'"))
		}
		if phase := helper.GetETCDEncryptionKeyRotationPhase(garden.Status.Credentials); len(phase) > 0 && phase != gardencorev1beta1.RotationCompleted {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot start rotation of all credentials if .status.credentials.rotation.etcdEncryptionKey.phase is not 'Completed'"))
		}
		if phase := helper.GetWorkloadIdentityKeyRotationPhase(garden.Status.Credentials); len(phase) > 0 && phase != gardencorev1beta1.RotationCompleted {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot start rotation of all credentials if .status.credentials.rotation.workloadIdentityKey.phase is not 'Completed'"))
		}
		if !resourcesToEncrypt.Equal(encryptedResources) {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot start rotation of all credentials because a previous encryption configuration change is currently being rolled out"))
		}
	case v1beta1constants.OperationRotateCredentialsComplete:
		if garden.DeletionTimestamp != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot complete rotation of all credentials if garden has deletion timestamp"))
		}
		if helper.GetCARotationPhase(garden.Status.Credentials) != gardencorev1beta1.RotationPrepared {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot complete rotation of all credentials if .status.credentials.rotation.certificateAuthorities.phase is not 'Prepared'"))
		}
		if helper.GetServiceAccountKeyRotationPhase(garden.Status.Credentials) != gardencorev1beta1.RotationPrepared {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot complete rotation of all credentials if .status.credentials.rotation.serviceAccountKey.phase is not 'Prepared'"))
		}
		if k8sLess134 && helper.GetETCDEncryptionKeyRotationPhase(garden.Status.Credentials) != gardencorev1beta1.RotationPrepared {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot complete rotation of all credentials if .status.credentials.rotation.etcdEncryptionKey.phase is not 'Prepared'"))
		}
		if helper.GetWorkloadIdentityKeyRotationPhase(garden.Status.Credentials) != gardencorev1beta1.RotationPrepared {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot complete rotation of all credentials if .status.credentials.rotation.workloadIdentityKey.phase is not 'Prepared'"))
		}

	case v1beta1constants.OperationRotateCAStart:
		if garden.DeletionTimestamp != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot start CA rotation if garden has deletion timestamp"))
		}
		if phase := helper.GetCARotationPhase(garden.Status.Credentials); len(phase) > 0 && phase != gardencorev1beta1.RotationCompleted {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot start CA rotation if .status.credentials.rotation.certificateAuthorities.phase is not 'Completed'"))
		}
	case v1beta1constants.OperationRotateCAComplete:
		if garden.DeletionTimestamp != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot complete CA rotation if garden has deletion timestamp"))
		}
		if helper.GetCARotationPhase(garden.Status.Credentials) != gardencorev1beta1.RotationPrepared {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot complete CA rotation if .status.credentials.rotation.certificateAuthorities.phase is not 'Prepared'"))
		}

	case v1beta1constants.OperationRotateServiceAccountKeyStart:
		if garden.DeletionTimestamp != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot start service account key rotation if garden has deletion timestamp"))
		}
		if phase := helper.GetServiceAccountKeyRotationPhase(garden.Status.Credentials); len(phase) > 0 && phase != gardencorev1beta1.RotationCompleted {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot start service account key rotation if .status.credentials.rotation.serviceAccountKey.phase is not 'Completed'"))
		}
	case v1beta1constants.OperationRotateServiceAccountKeyComplete:
		if garden.DeletionTimestamp != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot complete service account key rotation if garden has deletion timestamp"))
		}
		if helper.GetServiceAccountKeyRotationPhase(garden.Status.Credentials) != gardencorev1beta1.RotationPrepared {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot complete service account key rotation if .status.credentials.rotation.serviceAccountKey.phase is not 'Prepared'"))
		}

	case v1beta1constants.OperationRotateETCDEncryptionKeyStart, v1beta1constants.OperationRotateETCDEncryptionKey:
		if garden.DeletionTimestamp != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot start ETCD encryption key rotation if garden has deletion timestamp"))
		}
		if phase := helper.GetETCDEncryptionKeyRotationPhase(garden.Status.Credentials); len(phase) > 0 && phase != gardencorev1beta1.RotationCompleted {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot start ETCD encryption key rotation if .status.credentials.rotation.etcdEncryptionKey.phase is not 'Completed'"))
		}
		if !resourcesToEncrypt.Equal(encryptedResources) {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot start ETCD encryption key rotation because a previous encryption configuration change is currently being rolled out"))
		}
	case v1beta1constants.OperationRotateETCDEncryptionKeyComplete:
		if garden.DeletionTimestamp != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot complete ETCD encryption key rotation if garden has deletion timestamp"))
		}
		if helper.GetETCDEncryptionKeyRotationPhase(garden.Status.Credentials) != gardencorev1beta1.RotationPrepared {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot complete ETCD encryption key rotation if .status.credentials.rotation.etcdEncryptionKey.phase is not 'Prepared'"))
		}

	case v1beta1constants.OperationRotateObservabilityCredentials:
		if garden.DeletionTimestamp != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot start Observability credentials rotation if garden has deletion timestamp"))
		}

	case operatorv1alpha1.OperationRotateWorkloadIdentityKeyStart:
		if garden.DeletionTimestamp != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot start workload identity key rotation if garden has deletion timestamp"))
		}
		if phase := helper.GetWorkloadIdentityKeyRotationPhase(garden.Status.Credentials); len(phase) > 0 && phase != gardencorev1beta1.RotationCompleted {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot start workload identity key rotation if .status.credentials.rotation.workloadIdentityKey.phase is not 'Completed'"))
		}

	case operatorv1alpha1.OperationRotateWorkloadIdentityKeyComplete:
		if garden.DeletionTimestamp != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot complete workload identity key rotation if garden has deletion timestamp"))
		}
		if helper.GetWorkloadIdentityKeyRotationPhase(garden.Status.Credentials) != gardencorev1beta1.RotationPrepared {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot complete workload identity key rotation if .status.credentials.rotation.workloadIdentityKey.phase is not 'Prepared'"))
		}
	}

	return allErrs
}

func validateEncryptionConfigUpdate(oldGarden, newGarden *operatorv1alpha1.Garden) field.ErrorList {
	var (
		allErrs                              = field.ErrorList{}
		currentEncryptedKubernetesResources  = sets.New[schema.GroupResource]()
		currentEncryptedGardenerResources    = sets.New[schema.GroupResource]()
		oldKubeAPIServerEncryptionConfig     = &gardencore.EncryptionConfig{}
		newKubeAPIServerEncryptionConfig     = &gardencore.EncryptionConfig{}
		oldGAPIServerEncryptionConfig        = &gardencore.EncryptionConfig{}
		newGAPIServerEncryptionConfig        = &gardencore.EncryptionConfig{}
		etcdEncryptionKeyRotation            = &gardencore.ETCDEncryptionKeyRotation{}
		kubeAPIServerEncryptionConfigFldPath = field.NewPath("spec", "virtualCluster", "kubernetes", "kubeAPIServer", "encryptionConfig")
		gAPIServerEncryptionConfigFldPath    = field.NewPath("spec", "virtualCluster", "gardener", "gardenerAPIServer", "encryptionConfig")
	)

	if oldKubeAPIServer := oldGarden.Spec.VirtualCluster.Kubernetes.KubeAPIServer; oldKubeAPIServer != nil && oldKubeAPIServer.KubeAPIServerConfig != nil && oldKubeAPIServer.EncryptionConfig != nil {
		if err := gardenCoreScheme.Convert(oldKubeAPIServer.EncryptionConfig, oldKubeAPIServerEncryptionConfig, nil); err != nil {
			allErrs = append(allErrs, field.InternalError(kubeAPIServerEncryptionConfigFldPath, err))
		}
	}
	if newKubeAPIServer := newGarden.Spec.VirtualCluster.Kubernetes.KubeAPIServer; newKubeAPIServer != nil && newKubeAPIServer.KubeAPIServerConfig != nil && newKubeAPIServer.EncryptionConfig != nil {
		if err := gardenCoreScheme.Convert(newKubeAPIServer.EncryptionConfig, newKubeAPIServerEncryptionConfig, nil); err != nil {
			allErrs = append(allErrs, field.InternalError(kubeAPIServerEncryptionConfigFldPath, err))
		}
	}
	if oldGardenerAPIServer := oldGarden.Spec.VirtualCluster.Gardener.APIServer; oldGardenerAPIServer != nil && oldGardenerAPIServer.EncryptionConfig != nil {
		if err := gardenCoreScheme.Convert(oldGardenerAPIServer.EncryptionConfig, oldGAPIServerEncryptionConfig, nil); err != nil {
			allErrs = append(allErrs, field.InternalError(gAPIServerEncryptionConfigFldPath, err))
		}
	}
	if newGardenerAPIServer := newGarden.Spec.VirtualCluster.Gardener.APIServer; newGardenerAPIServer != nil && newGardenerAPIServer.EncryptionConfig != nil {
		if err := gardenCoreScheme.Convert(newGardenerAPIServer.EncryptionConfig, newGAPIServerEncryptionConfig, nil); err != nil {
			allErrs = append(allErrs, field.InternalError(gAPIServerEncryptionConfigFldPath, err))
		}
	}
	if credentials := newGarden.Status.Credentials; credentials != nil && credentials.Rotation != nil && credentials.Rotation.ETCDEncryptionKey != nil {
		if err := gardenCoreScheme.Convert(credentials.Rotation.ETCDEncryptionKey, etcdEncryptionKeyRotation, nil); err != nil {
			allErrs = append(allErrs, field.InternalError(field.NewPath("status", "credentials", "rotation", "etcdEncryptionKey"), err))
		}
	}

	for _, r := range utils.FilterEntriesByFilterFn(newGarden.Status.EncryptedResources, operator.IsServedByKubeAPIServer) {
		currentEncryptedKubernetesResources.Insert(schema.ParseGroupResource(r))
	}
	allErrs = append(allErrs, gardencorevalidation.ValidateEncryptionConfigUpdate(newKubeAPIServerEncryptionConfig, oldKubeAPIServerEncryptionConfig, currentEncryptedKubernetesResources, etcdEncryptionKeyRotation, false, kubeAPIServerEncryptionConfigFldPath)...)

	for _, r := range utils.FilterEntriesByFilterFn(newGarden.Status.EncryptedResources, operator.IsServedByGardenerAPIServer) {
		currentEncryptedGardenerResources.Insert(schema.ParseGroupResource(r))
	}
	allErrs = append(allErrs, gardencorevalidation.ValidateEncryptionConfigUpdate(newGAPIServerEncryptionConfig, oldGAPIServerEncryptionConfig, currentEncryptedGardenerResources, etcdEncryptionKeyRotation, false, gAPIServerEncryptionConfigFldPath)...)

	return allErrs
}

func hasProvider(dns *operatorv1alpha1.DNSManagement, provider string) bool {
	return slices.ContainsFunc(dns.Providers, func(p operatorv1alpha1.DNSProvider) bool {
		return p.Name == provider
	})
}

func validateExtensions(extensions []operatorv1alpha1.GardenExtension, registeredExtensions []operatorv1alpha1.Extension, fldPath *field.Path) field.ErrorList {
	var (
		allErrs                  = field.ErrorList{}
		types                    = sets.Set[string]{}
		registeredExtensionTypes = sets.Set[string]{}
	)

	for _, extension := range registeredExtensions {
		for _, resource := range extension.Spec.Resources {
			if resource.Kind != extensionsv1alpha1.ExtensionResource {
				continue
			}
			if slices.Contains(resource.ClusterCompatibility, operatorv1alpha1.ClusterTypeGarden) {
				registeredExtensionTypes.Insert(resource.Type)
			}
		}
	}

	for i, extension := range extensions {
		if !registeredExtensionTypes.Has(extension.Type) {
			allErrs = append(allErrs, field.NotSupported(fldPath.Index(i).Child("type"), extension.Type, sets.List(registeredExtensionTypes)))
		}
		if types.Has(extension.Type) {
			allErrs = append(allErrs, field.Duplicate(fldPath.Index(i).Child("type"), extension.Type))
		} else {
			types.Insert(extension.Type)
		}
	}
	return allErrs
}
