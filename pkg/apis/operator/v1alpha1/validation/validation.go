// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
	"net"
	"slices"
	"strings"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/component-base/featuregate"
	"k8s.io/utils/ptr"

	admissioncontrollerconfig "github.com/gardener/gardener/pkg/admissioncontroller/apis/config"
	admissioncontrollerv1alpha1 "github.com/gardener/gardener/pkg/admissioncontroller/apis/config/v1alpha1"
	admissioncontrollervalidation "github.com/gardener/gardener/pkg/admissioncontroller/apis/config/validation"
	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencoreinstall "github.com/gardener/gardener/pkg/apis/core/install"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorevalidation "github.com/gardener/gardener/pkg/apis/core/validation"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	operatorv1alpha1conversion "github.com/gardener/gardener/pkg/apis/operator/v1alpha1/conversion"
	"github.com/gardener/gardener/pkg/apis/operator/v1alpha1/helper"
	sharedcomponent "github.com/gardener/gardener/pkg/component/shared"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	cidrvalidation "github.com/gardener/gardener/pkg/utils/validation/cidr"
	"github.com/gardener/gardener/pkg/utils/validation/kubernetesversion"
	plugin "github.com/gardener/gardener/plugin/pkg"
)

var gardenCoreScheme *runtime.Scheme

func init() {
	gardenCoreScheme = runtime.NewScheme()
	utilruntime.Must(gardencoreinstall.AddToScheme(gardenCoreScheme))
	utilruntime.Must(admissioncontrollerv1alpha1.AddToScheme(gardenCoreScheme))
}

// ValidateGarden contains functionality for performing extended validation of a Garden object which is not possible
// with standard CRD validation, see https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#validation-rules.
func ValidateGarden(garden *operatorv1alpha1.Garden) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateOperation(garden.Annotations[v1beta1constants.GardenerOperation], garden, field.NewPath("metadata", "annotations"))...)
	allErrs = append(allErrs, validateDNS(garden.Spec.DNS, field.NewPath("spec", "dns"))...)
	allErrs = append(allErrs, validateRuntimeCluster(garden.Spec.DNS, garden.Spec.RuntimeCluster, field.NewPath("spec", "runtimeCluster"))...)
	allErrs = append(allErrs, validateVirtualCluster(garden.Spec.DNS, garden.Spec.VirtualCluster, garden.Spec.RuntimeCluster, field.NewPath("spec", "virtualCluster"))...)

	if helper.TopologyAwareRoutingEnabled(garden.Spec.RuntimeCluster.Settings) {
		if len(garden.Spec.RuntimeCluster.Provider.Zones) <= 1 {
			allErrs = append(allErrs, field.Forbidden(field.NewPath("spec", "runtimeCluster", "settings", "topologyAwareRouting", "enabled"), "topology-aware routing can only be enabled on multi-zone garden runtime cluster (with at least two zones in spec.provider.zones)"))
		}
		if !helper.HighAvailabilityEnabled(garden) {
			allErrs = append(allErrs, field.Forbidden(field.NewPath("spec", "runtimeCluster", "settings", "topologyAwareRouting", "enabled"), "topology-aware routing can only be enabled when virtual cluster's high-availability is enabled"))
		}
	}

	return allErrs
}

// ValidateGardenUpdate contains functionality for performing extended validation of a Garden object under update which
// is not possible with standard CRD validation, see https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#validation-rules.
func ValidateGardenUpdate(oldGarden, newGarden *operatorv1alpha1.Garden) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateRuntimeClusterUpdate(oldGarden, newGarden)...)
	allErrs = append(allErrs, validateVirtualClusterUpdate(oldGarden, newGarden)...)
	allErrs = append(allErrs, ValidateGarden(newGarden)...)

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
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(oldRuntimeCluster.Ingress.Domains[0], newRuntimeCluster.Ingress.Domains[0], fldPath.Child("ingress", "domains").Index(0))...)
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
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(oldVirtualCluster.DNS.Domains[0], newVirtualCluster.DNS.Domains[0], fldPath.Child("dns", "domains").Index(0).Child("name"))...)
	}

	if oldVirtualCluster.ETCD != nil && oldVirtualCluster.ETCD.Main != nil && oldVirtualCluster.ETCD.Main.Backup != nil && oldVirtualCluster.ETCD.Main.Backup.BucketName == nil {
		if newVirtualCluster.ETCD != nil && newVirtualCluster.ETCD.Main != nil && newVirtualCluster.ETCD.Main.Backup != nil && newVirtualCluster.ETCD.Main.Backup.BucketName != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("etcd", "main", "backup", "bucketName"), "bucket name must not be set if it was not set before"))
		}
	}

	if oldVirtualCluster.ControlPlane != nil && oldVirtualCluster.ControlPlane.HighAvailability != nil &&
		(newVirtualCluster.ControlPlane == nil || newVirtualCluster.ControlPlane.HighAvailability == nil) {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(oldVirtualCluster.ControlPlane, newVirtualCluster.ControlPlane, fldPath.Child("controlPlane", "highAvailability"))...)
	}

	allErrs = append(allErrs, gardencorevalidation.ValidateKubernetesVersionUpdate(newVirtualCluster.Kubernetes.Version, oldVirtualCluster.Kubernetes.Version, false, fldPath.Child("kubernetes", "version"))...)
	allErrs = append(allErrs, validateEncryptionConfigUpdate(oldGarden, newGarden)...)

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

func validateRuntimeCluster(dns *operatorv1alpha1.DNSManagement, runtimeCluster operatorv1alpha1.RuntimeCluster, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if cidrvalidation.NetworksIntersect(runtimeCluster.Networking.Pods, runtimeCluster.Networking.Services) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("networking", "services"), runtimeCluster.Networking.Services, "pod network of runtime cluster intersects with service network of runtime cluster"))
	}
	if runtimeCluster.Networking.Nodes != nil {
		if cidrvalidation.NetworksIntersect(*runtimeCluster.Networking.Nodes, runtimeCluster.Networking.Pods) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("networking", "nodes"), *runtimeCluster.Networking.Nodes, "node network of runtime cluster intersects with pod network of runtime cluster"))
		}
		if cidrvalidation.NetworksIntersect(*runtimeCluster.Networking.Nodes, runtimeCluster.Networking.Services) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("networking", "nodes"), *runtimeCluster.Networking.Nodes, "node network of runtime cluster intersects with service network of runtime cluster"))
		}
	}

	allErrs = validateDomains(dns, runtimeCluster.Ingress.Domains, fldPath.Child("ingress", "domains"), allErrs)

	if runtimeCluster.CertManagement != nil {
		allErrs = append(allErrs, validateCertManagement(runtimeCluster.CertManagement, fldPath.Child("certManagement"))...)
	}

	return allErrs
}

func validateDomains(dns *operatorv1alpha1.DNSManagement, domains []operatorv1alpha1.DNSDomain, path *field.Path, allErrs field.ErrorList) field.ErrorList {
	names := sets.New[string]()
	for i, domain := range domains {
		allErrs = append(allErrs, gardencorevalidation.ValidateDNS1123Subdomain(domain.Name, path.Index(i).Child("name"))...)
		if names.Has(domain.Name) {
			allErrs = append(allErrs, field.Duplicate(path.Index(i).Child("name"), domain.Name))
		}
		names.Insert(domain.Name)
		if domain.Provider != nil {
			if !hasProvider(dns, *domain.Provider) {
				allErrs = append(allErrs, field.Invalid(path.Index(i).Child("provider"), *domain.Provider, "provider name not found in .spec.dns.providers"))
			}
		}
	}

	return allErrs
}

func validateCertManagement(certManagement *operatorv1alpha1.CertManagement, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if certManagement.DefaultIssuer.ACME == nil && certManagement.DefaultIssuer.CA == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("defaultIssuer"), "either ACME or CA issuer must be set"))
	} else if certManagement.DefaultIssuer.ACME != nil && certManagement.DefaultIssuer.CA != nil {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("defaultIssuer"), "ACME or CA issuers are exclusive, only one must be set"))
	}
	return allErrs
}

func validateVirtualCluster(dns *operatorv1alpha1.DNSManagement, virtualCluster operatorv1alpha1.VirtualCluster, runtimeCluster operatorv1alpha1.RuntimeCluster, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = validateDomains(dns, virtualCluster.DNS.Domains, fldPath.Child("dns", "domains"), allErrs)

	if virtualCluster.ETCD != nil && virtualCluster.ETCD.Main != nil && virtualCluster.ETCD.Main.Backup != nil {
		if virtualCluster.ETCD.Main.Backup.BucketName != nil {
			if virtualCluster.ETCD.Main.Backup.ProviderConfig != nil {
				allErrs = append(allErrs, field.Forbidden(fldPath.Child("etcd", "main", "backup", "providerConfig"), "provider must not be set when bucket name is set"))
			}
		}
	}

	if err := kubernetesversion.CheckIfSupported(virtualCluster.Kubernetes.Version); err != nil {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("kubernetes", "version"), virtualCluster.Kubernetes.Version, kubernetesversion.SupportedVersions))
	}

	if kubeAPIServer := virtualCluster.Kubernetes.KubeAPIServer; kubeAPIServer != nil && kubeAPIServer.KubeAPIServerConfig != nil {
		path := fldPath.Child("kubernetes", "kubeAPIServer")

		coreKubeAPIServerConfig := &gardencore.KubeAPIServerConfig{}
		if err := gardenCoreScheme.Convert(kubeAPIServer.KubeAPIServerConfig, coreKubeAPIServerConfig, nil); err != nil {
			allErrs = append(allErrs, field.InternalError(path, err))
		}

		allErrs = append(allErrs, gardencorevalidation.ValidateKubeAPIServer(coreKubeAPIServerConfig, virtualCluster.Kubernetes.Version, true, gardenerutils.DefaultResourcesForEncryption(), path)...)
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

	if _, _, err := net.ParseCIDR(virtualCluster.Networking.Services); err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("networking", "services"), virtualCluster.Networking.Services, "cannot parse service network cidr: "+err.Error()))
	}
	if cidrvalidation.NetworksIntersect(runtimeCluster.Networking.Pods, virtualCluster.Networking.Services) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("networking", "services"), virtualCluster.Networking.Services, "pod network of runtime cluster intersects with service network of virtual cluster"))
	}
	if cidrvalidation.NetworksIntersect(runtimeCluster.Networking.Services, virtualCluster.Networking.Services) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("networking", "services"), virtualCluster.Networking.Services, "service network of runtime cluster intersects with service network of virtual cluster"))
	}
	if runtimeCluster.Networking.Nodes != nil && cidrvalidation.NetworksIntersect(*runtimeCluster.Networking.Nodes, virtualCluster.Networking.Services) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("networking", "services"), virtualCluster.Networking.Services, "node network of runtime cluster intersects with service network of virtual cluster"))
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
		seenResources := sets.New[string]()

		for i, resource := range config.EncryptionConfig.Resources {
			idxPath := fldPath.Child("encryptionConfig", "resources").Index(i)
			if seenResources.Has(resource) {
				allErrs = append(allErrs, field.Duplicate(idxPath, resource))
			}

			if gardenerutils.DefaultGardenerResourcesForEncryption().Has(resource) {
				allErrs = append(allErrs, field.Forbidden(idxPath, fmt.Sprintf("%q are always encrypted", resource)))
			}

			if !gardenerutils.IsServedByGardenerAPIServer(resource) {
				allErrs = append(allErrs, field.Invalid(idxPath, resource, "should be a resource served by gardener-apiserver. ie; should have any of the suffixes {core,operations,settings,seedmanagement}.gardener.cloud"))
			}

			if strings.HasPrefix(resource, "*") {
				allErrs = append(allErrs, field.Invalid(idxPath, resource, "wildcards are not supported"))
			}

			seenResources.Insert(resource)
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

	return allErrs
}

func validateGardenerAdmissionController(config *operatorv1alpha1.GardenerAdmissionControllerConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if config == nil {
		return allErrs
	}

	if config.ResourceAdmissionConfiguration != nil {
		externalAdmissionConfiguration := operatorv1alpha1conversion.ConvertToAdmissionControllerResourceAdmissionConfiguration(config.ResourceAdmissionConfiguration)
		internalAdmissionConfiguration := &admissioncontrollerconfig.ResourceAdmissionConfiguration{}
		if err := gardenCoreScheme.Convert(externalAdmissionConfiguration, internalAdmissionConfiguration, nil); err != nil {
			allErrs = append(allErrs, field.InternalError(fldPath.Child("resourceAdmissionConfiguration"), err))
		}
		allErrs = append(allErrs, admissioncontrollervalidation.ValidateResourceAdmissionConfiguration(internalAdmissionConfiguration, fldPath.Child("resourceAdmissionConfiguration"))...)
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
		allErrs = append(allErrs, metav1validation.ValidateLabelSelector(quota.ProjectSelector, metav1validation.LabelSelectorValidationOptions{AllowInvalidLabelValueInSelector: true}, fldPath.Child("defaultProjectQuotas").Index(i).Child("projectSelector"))...)
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
		} else {
			if spec.LockToDefault && featureGates[featureGate] != spec.Default {
				allErrs = append(allErrs, field.Forbidden(fldPath.Child(featureGate), fmt.Sprintf("cannot set feature gate to %v, feature is locked to %v", featureGates[featureGate], spec.Default)))
			}
		}
	}

	return allErrs
}

func validateGardenerDashboardConfig(config *operatorv1alpha1.GardenerDashboardConfig, kubeAPIServerConfig *operatorv1alpha1.KubeAPIServerConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if config == nil {
		return allErrs
	}

	if !ptr.Deref(config.EnableTokenLogin, true) && config.OIDC == nil {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("enableTokenLogin"), "OIDC must be configured when token login is disabled"))
	}

	if config.OIDC != nil && (kubeAPIServerConfig == nil || kubeAPIServerConfig.OIDCConfig == nil) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("oidc"), config.OIDC, "must set .spec.virtualCluster.kubernetes.kubeAPIServer.oidcConfig when configuring OIDC config for dashboard"))
	}

	return allErrs
}

func validateOperation(operation string, garden *operatorv1alpha1.Garden, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if operation == "" {
		return allErrs
	}

	fldPathOp := fldPath.Key(v1beta1constants.GardenerOperation)

	if operation != "" && !operatorv1alpha1.AvailableOperationAnnotations.Has(operation) {
		allErrs = append(allErrs, field.NotSupported(fldPathOp, operation, sets.List(operatorv1alpha1.AvailableOperationAnnotations)))
	}
	allErrs = append(allErrs, validateOperationContext(operation, garden, fldPathOp)...)

	return allErrs
}

func validateOperationContext(operation string, garden *operatorv1alpha1.Garden, fldPath *field.Path) field.ErrorList {
	var (
		allErrs                  = field.ErrorList{}
		encryptionConfig         *gardencorev1beta1.EncryptionConfig
		gardenerEncryptionConfig *gardencorev1beta1.EncryptionConfig
	)

	if garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer != nil && garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer.KubeAPIServerConfig != nil {
		encryptionConfig = garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer.KubeAPIServerConfig.EncryptionConfig
	}
	if garden.Spec.VirtualCluster.Gardener.APIServer != nil && garden.Spec.VirtualCluster.Gardener.APIServer.EncryptionConfig != nil {
		gardenerEncryptionConfig = garden.Spec.VirtualCluster.Gardener.APIServer.EncryptionConfig
	}

	resourcesToEncrypt := append(sharedcomponent.GetResourcesForEncryptionFromConfig(encryptionConfig), sharedcomponent.GetResourcesForEncryptionFromConfig(gardenerEncryptionConfig)...)

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
		if !apiequality.Semantic.DeepEqual(resourcesToEncrypt, garden.Status.EncryptedResources) {
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
		if helper.GetETCDEncryptionKeyRotationPhase(garden.Status.Credentials) != gardencorev1beta1.RotationPrepared {
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

	case v1beta1constants.OperationRotateETCDEncryptionKeyStart:
		if garden.DeletionTimestamp != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot start ETCD encryption key rotation if garden has deletion timestamp"))
		}
		if phase := helper.GetETCDEncryptionKeyRotationPhase(garden.Status.Credentials); len(phase) > 0 && phase != gardencorev1beta1.RotationCompleted {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot start ETCD encryption key rotation if .status.credentials.rotation.etcdEncryptionKey.phase is not 'Completed'"))
		}
		if !apiequality.Semantic.DeepEqual(resourcesToEncrypt, garden.Status.EncryptedResources) {
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
		oldKubeAPIServerEncryptionConfig     = &gardencore.EncryptionConfig{}
		newKubeAPIServerEncryptionConfig     = &gardencore.EncryptionConfig{}
		oldGAPIServerEncryptionConfig        = &gardencore.EncryptionConfig{}
		newGAPIServerEncryptionConfig        = &gardencore.EncryptionConfig{}
		etcdEncryptionKeyRotation            = &gardencore.ETCDEncryptionKeyRotation{}
		kubeAPIServerEncryptionConfigFldPath = field.NewPath("spec", "virtualCluster", "kubernetes", "kubeAPIServer", "encryptionConfig")
		gAPIServerEncryptionConfigFldPath    = field.NewPath("spec", "virtualCluster", "gardener", "gardenerAPIServer", "encryptionConfig")
	)

	if oldKubeAPIServer := oldGarden.Spec.VirtualCluster.Kubernetes.KubeAPIServer; oldKubeAPIServer != nil && oldKubeAPIServer.KubeAPIServerConfig != nil && oldKubeAPIServer.KubeAPIServerConfig.EncryptionConfig != nil {
		if err := gardenCoreScheme.Convert(oldKubeAPIServer.KubeAPIServerConfig.EncryptionConfig, oldKubeAPIServerEncryptionConfig, nil); err != nil {
			allErrs = append(allErrs, field.InternalError(kubeAPIServerEncryptionConfigFldPath, err))
		}
	}
	if newKubeAPIServer := newGarden.Spec.VirtualCluster.Kubernetes.KubeAPIServer; newKubeAPIServer != nil && newKubeAPIServer.KubeAPIServerConfig != nil && newKubeAPIServer.KubeAPIServerConfig.EncryptionConfig != nil {
		if err := gardenCoreScheme.Convert(newKubeAPIServer.KubeAPIServerConfig.EncryptionConfig, newKubeAPIServerEncryptionConfig, nil); err != nil {
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

	currentEncryptedKubernetesResources := utils.FilterEntriesByFilterFn(newGarden.Status.EncryptedResources, gardenerutils.IsServedByKubeAPIServer)
	allErrs = append(allErrs, gardencorevalidation.ValidateEncryptionConfigUpdate(newKubeAPIServerEncryptionConfig, oldKubeAPIServerEncryptionConfig, sets.New(currentEncryptedKubernetesResources...), etcdEncryptionKeyRotation, false, kubeAPIServerEncryptionConfigFldPath)...)

	currentEncryptedGardenerResources := utils.FilterEntriesByFilterFn(newGarden.Status.EncryptedResources, gardenerutils.IsServedByGardenerAPIServer)
	allErrs = append(allErrs, gardencorevalidation.ValidateEncryptionConfigUpdate(newGAPIServerEncryptionConfig, oldGAPIServerEncryptionConfig, sets.New(currentEncryptedGardenerResources...), etcdEncryptionKeyRotation, false, gAPIServerEncryptionConfigFldPath)...)

	return allErrs
}

func hasProvider(dns *operatorv1alpha1.DNSManagement, provider string) bool {
	if dns == nil {
		return false
	}
	for _, p := range dns.Providers {
		if p.Name == provider {
			return true
		}
	}
	return false
}
