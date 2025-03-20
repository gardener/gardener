// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/helper"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	cidrvalidation "github.com/gardener/gardener/pkg/utils/validation/cidr"
	kubernetescorevalidation "github.com/gardener/gardener/pkg/utils/validation/kubernetes/core"
)

var (
	availableIngressKinds = sets.New(
		v1beta1constants.IngressKindNginx,
	)
	availableExternalTrafficPolicies = sets.New(
		string(corev1.ServiceExternalTrafficPolicyCluster),
		string(corev1.ServiceExternalTrafficPolicyLocal),
	)
	availableSeedOperations = sets.New(
		v1beta1constants.SeedOperationRenewGardenAccessSecrets,
		v1beta1constants.GardenerOperationReconcile,
		v1beta1constants.GardenerOperationRenewKubeconfig,
		v1beta1constants.SeedOperationRenewWorkloadIdentityTokens,
	)
)

// ValidateSeed validates a Seed object.
func ValidateSeed(seed *core.Seed) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&seed.ObjectMeta, false, apivalidation.NameIsDNSLabel, field.NewPath("metadata"))...)
	allErrs = append(allErrs, validateSeedOperation(seed.Annotations[v1beta1constants.GardenerOperation], field.NewPath("metadata", "annotations").Key(v1beta1constants.GardenerOperation))...)
	allErrs = append(allErrs, ValidateSeedSpec(&seed.Spec, field.NewPath("spec"), false)...)

	return allErrs
}

// ValidateSeedUpdate validates a Seed object before an update.
func ValidateSeedUpdate(newSeed, oldSeed *core.Seed) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&newSeed.ObjectMeta, &oldSeed.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, validateSeedOperationUpdate(newSeed.Annotations[v1beta1constants.GardenerOperation], oldSeed.Annotations[v1beta1constants.GardenerOperation], field.NewPath("metadata", "annotations").Key(v1beta1constants.GardenerOperation))...)
	allErrs = append(allErrs, ValidateSeedSpecUpdate(&newSeed.Spec, &oldSeed.Spec, field.NewPath("spec"))...)
	allErrs = append(allErrs, ValidateSeed(newSeed)...)

	return allErrs
}

// ValidateSeedTemplate validates a SeedTemplate.
func ValidateSeedTemplate(seedTemplate *core.SeedTemplate, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, metav1validation.ValidateLabels(seedTemplate.Labels, fldPath.Child("metadata", "labels"))...)
	allErrs = append(allErrs, apivalidation.ValidateAnnotations(seedTemplate.Annotations, fldPath.Child("metadata", "annotations"))...)
	allErrs = append(allErrs, ValidateSeedSpec(&seedTemplate.Spec, fldPath.Child("spec"), true)...)

	return allErrs
}

// ValidateSeedTemplateUpdate validates a SeedTemplate before an update.
func ValidateSeedTemplateUpdate(newSeedTemplate, oldSeedTemplate *core.SeedTemplate, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, ValidateSeedSpecUpdate(&newSeedTemplate.Spec, &oldSeedTemplate.Spec, fldPath.Child("spec"))...)

	return allErrs
}

// ValidateSeedSpec validates the specification of a Seed object.
func ValidateSeedSpec(seedSpec *core.SeedSpec, fldPath *field.Path, inTemplate bool) field.ErrorList {
	allErrs := field.ErrorList{}

	providerPath := fldPath.Child("provider")
	if !inTemplate && len(seedSpec.Provider.Type) == 0 {
		allErrs = append(allErrs, field.Required(providerPath.Child("type"), "must provide a provider type"))
	}
	if !inTemplate && len(seedSpec.Provider.Region) == 0 {
		allErrs = append(allErrs, field.Required(providerPath.Child("region"), "must provide a provider region"))
	}

	zones := sets.New[string]()
	for i, zone := range seedSpec.Provider.Zones {
		if zones.Has(zone) {
			allErrs = append(allErrs, field.Duplicate(providerPath.Child("zones").Index(i), zone))
			break
		}
		zones.Insert(zone)
	}

	allErrs = append(allErrs, validateSeedNetworks(seedSpec.Networks, fldPath.Child("networks"), inTemplate)...)
	allErrs = append(allErrs, validateSeedBackup(seedSpec.Backup, seedSpec.Provider.Type, fldPath.Child("backup"))...)

	var keyValues = sets.New[string]()

	for i, taint := range seedSpec.Taints {
		idxPath := fldPath.Child("taints").Index(i)

		if len(taint.Key) == 0 {
			allErrs = append(allErrs, field.Required(idxPath.Child("key"), "cannot be empty"))
		}

		id := utils.IDForKeyWithOptionalValue(taint.Key, taint.Value)
		if keyValues.Has(id) {
			allErrs = append(allErrs, field.Duplicate(idxPath, id))
		}
		keyValues.Insert(id)
	}

	if seedSpec.Volume != nil {
		if seedSpec.Volume.MinimumSize != nil {
			allErrs = append(allErrs, kubernetescorevalidation.ValidateResourceQuantityValue("minimumSize", *seedSpec.Volume.MinimumSize, fldPath.Child("volume", "minimumSize"))...)
		}

		volumeProviderPurposes := make(map[string]struct{}, len(seedSpec.Volume.Providers))
		for i, provider := range seedSpec.Volume.Providers {
			idxPath := fldPath.Child("volume", "providers").Index(i)
			if len(provider.Purpose) == 0 {
				allErrs = append(allErrs, field.Required(idxPath.Child("purpose"), "cannot be empty"))
			}
			if len(provider.Name) == 0 {
				allErrs = append(allErrs, field.Required(idxPath.Child("name"), "cannot be empty"))
			}
			if _, ok := volumeProviderPurposes[provider.Purpose]; ok {
				allErrs = append(allErrs, field.Duplicate(idxPath.Child("purpose"), provider.Purpose))
			}
			volumeProviderPurposes[provider.Purpose] = struct{}{}
		}
	}

	if seedSpec.Settings != nil {
		if seedSpec.Settings.ExcessCapacityReservation != nil {
			for i, config := range seedSpec.Settings.ExcessCapacityReservation.Configs {
				if len(config.Resources) == 0 {
					allErrs = append(allErrs, field.Required(fldPath.Child("settings", "excessCapacityReservation", "configs").Index(i).Child("resources"), "cannot be empty"))
				}
				for resource, value := range config.Resources {
					allErrs = append(allErrs, kubernetescorevalidation.ValidateResourceQuantityValue(resource.String(), value, fldPath.Child("settings", "excessCapacityReservation", "configs").Index(i).Child("resources").Child(resource.String()))...)
				}
				allErrs = append(allErrs, kubernetescorevalidation.ValidateTolerations(config.Tolerations, fldPath.Child("settings", "excessCapacityReservation", "configs").Index(i).Child("tolerations"))...)
			}
		}
		if seedSpec.Settings.LoadBalancerServices != nil {
			allErrs = append(allErrs, apivalidation.ValidateAnnotations(seedSpec.Settings.LoadBalancerServices.Annotations, fldPath.Child("settings", "loadBalancerServices", "annotations"))...)

			if policy := seedSpec.Settings.LoadBalancerServices.ExternalTrafficPolicy; policy != nil && !availableExternalTrafficPolicies.Has(string(*policy)) {
				allErrs = append(allErrs, field.NotSupported(fldPath.Child("settings", "loadBalancerServices", "externalTrafficPolicy"), *policy, sets.List(availableExternalTrafficPolicies)))
			}

			if len(seedSpec.Provider.Zones) <= 1 && len(seedSpec.Settings.LoadBalancerServices.Zones) > 0 {
				allErrs = append(allErrs, field.Forbidden(fldPath.Child("settings", "loadBalancerServices", "zones"), "zone-specific load balancer settings only allowed with at least two zones in spec.provider.zones"))
			}

			zones := sets.New(seedSpec.Provider.Zones...)
			specifiedZones := sets.New[string]()

			for i, zoneSettings := range seedSpec.Settings.LoadBalancerServices.Zones {
				if !zones.Has(zoneSettings.Name) {
					allErrs = append(allErrs, field.NotFound(fldPath.Child("settings", "loadBalancerServices", "zones").Index(i).Child("name"), zoneSettings.Name))
				}
				if specifiedZones.Has(zoneSettings.Name) {
					allErrs = append(allErrs, field.Duplicate(fldPath.Child("settings", "loadBalancerServices", "zones").Index(i).Child("name"), zoneSettings.Name))
				}
				specifiedZones.Insert(zoneSettings.Name)

				allErrs = append(allErrs, apivalidation.ValidateAnnotations(zoneSettings.Annotations, fldPath.Child("settings", "loadBalancerServices", "zones").Index(i).Child("annotations"))...)

				if policy := zoneSettings.ExternalTrafficPolicy; policy != nil && !availableExternalTrafficPolicies.Has(string(*policy)) {
					allErrs = append(allErrs, field.NotSupported(fldPath.Child("settings", "loadBalancerServices", "zones").Index(i).Child("externalTrafficPolicy"), *policy, sets.List(availableExternalTrafficPolicies)))
				}
			}
		}
		if helper.SeedSettingTopologyAwareRoutingEnabled(seedSpec.Settings) && len(seedSpec.Provider.Zones) <= 1 {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("settings", "topologyAwareRouting", "enabled"), "topology-aware routing can only be enabled on multi-zone Seed clusters (with at least two zones in spec.provider.zones)"))
		}
	}

	if !inTemplate && seedSpec.Ingress == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("ingress"), "cannot be empty"))
	}

	if seedSpec.Ingress != nil {
		if len(seedSpec.Ingress.Domain) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("ingress", "domain"), "cannot be empty"))
		} else {
			allErrs = append(allErrs, ValidateDNS1123Subdomain(seedSpec.Ingress.Domain, fldPath.Child("ingress", "domain"))...)
		}
		if !availableIngressKinds.Has(seedSpec.Ingress.Controller.Kind) {
			allErrs = append(allErrs, field.NotSupported(
				fldPath.Child("ingress", "controller", "kind"),
				seedSpec.Ingress.Controller.Kind,
				availableIngressKinds.UnsortedList()),
			)
		}
		if seedSpec.DNS.Provider == nil {
			allErrs = append(allErrs, field.Required(fldPath.Child("dns", "provider"),
				"ingress controller requires dns.provider to be set"))
		} else {
			if len(seedSpec.DNS.Provider.Type) == 0 {
				allErrs = append(allErrs, field.Required(fldPath.Child("dns", "provider", "type"),
					"DNS provider type must be set"))
			}
			if len(seedSpec.DNS.Provider.SecretRef.Name) == 0 {
				allErrs = append(allErrs, field.Required(fldPath.Child("dns", "provider", "secretRef", "name"),
					"secret reference name must be set"))
			}
			if len(seedSpec.DNS.Provider.SecretRef.Namespace) == 0 {
				allErrs = append(allErrs, field.Required(fldPath.Child("dns", "provider", "secretRef", "namespace"),
					"secret reference namespace must be set"))
			}
		}
	}

	allErrs = append(allErrs, validateExtensions(seedSpec.Extensions, fldPath.Child("extensions"))...)
	allErrs = append(allErrs, validateExtensionsForSeed(seedSpec.Extensions, fldPath.Child("extensions"))...)
	allErrs = append(allErrs, validateResources(seedSpec.Resources, fldPath.Child("resources"))...)

	return allErrs
}

func validateSeedBackup(seedBackup *core.SeedBackup, seedProviderType string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if seedBackup == nil {
		return allErrs
	}

	if len(seedBackup.Provider) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("provider"), "must provide a backup cloud provider name"))
	}

	if seedProviderType != seedBackup.Provider && (seedBackup.Region == nil || len(*seedBackup.Region) == 0) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("region"), "", "region must be specified for if backup provider is different from seed provider used in `spec.provider.type`"))
	}

	// How to achieve backward compatibility between secretRef and credentialsRef?
	// - if secretRef is set, credentialsRef must be set and refer the same secret
	// - if secretRef is not set, then credentialsRef must refer a WorkloadIdentity
	//
	// After the sync in the strategy, we can have the following cases:
	// - both secretRef and credentialsRef are unset, which we forbid here
	// - both can be set but refer to different resources, which we forbid here
	// - secretRef can be unset only when workloadIdentity is used, which we respect here

	if seedBackup.CredentialsRef == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("credentialsRef"), "must be set to refer a Secret or WorkloadIdentity"))
	} else {
		allErrs = append(allErrs, ValidateCredentialsRef(*seedBackup.CredentialsRef, fldPath.Child("credentialsRef"))...)

		if seedBackup.CredentialsRef.GroupVersionKind().String() == corev1.SchemeGroupVersion.WithKind("Secret").String() {
			if seedBackup.SecretRef.Namespace != seedBackup.CredentialsRef.Namespace || seedBackup.SecretRef.Name != seedBackup.CredentialsRef.Name {
				allErrs = append(allErrs, field.Forbidden(fldPath.Child("secretRef"), "must refer to the same secret as `spec.backup.credentialsRef`"))
			}
		} else {
			emptySecretRef := corev1.SecretReference{}
			if seedBackup.SecretRef != emptySecretRef {
				allErrs = append(allErrs, field.Forbidden(fldPath.Child("secretRef"), "must not be set when `spec.backup.credentialsRef` refer to resource other than secret"))
			}
		}

		// TODO(vpnachev): Allow WorkloadIdentities once the support in the controllers and components is fully implemented.
		if seedBackup.CredentialsRef.APIVersion == securityv1alpha1.SchemeGroupVersion.String() &&
			seedBackup.CredentialsRef.Kind == "WorkloadIdentity" {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("credentialsRef"), "support for workload identity as backup credentials is not yet fully implemented"))
		}
	}

	return allErrs
}

func validateSeedNetworks(seedNetworks core.SeedNetworks, fldPath *field.Path, inTemplate bool) field.ErrorList {
	allErrs := field.ErrorList{}

	if errs := ValidateIPFamilies(seedNetworks.IPFamilies, fldPath.Child("ipFamilies")); len(errs) > 0 {
		// further validation doesn't make any sense, because we don't know which IP family to check for in the CIDR fields
		return append(allErrs, errs...)
	}

	var (
		primaryIPFamily          = helper.DeterminePrimaryIPFamily(seedNetworks.IPFamilies)
		networks                 []cidrvalidation.CIDR
		reservedSeedServiceRange = cidrvalidation.NewCIDR(v1beta1constants.ReservedKubeApiServerMappingRange, field.NewPath(""))
	)

	if !inTemplate || len(seedNetworks.Pods) > 0 {
		networks = append(networks, cidrvalidation.NewCIDR(seedNetworks.Pods, fldPath.Child("pods")))
	}
	if !inTemplate || len(seedNetworks.Services) > 0 {
		services := cidrvalidation.NewCIDR(seedNetworks.Services, fldPath.Child("services"))
		networks = append(networks, services)
		// Service range must not be larger than /8 for ipv4
		if services.IsIPv4() {
			maxSize, _ := reservedSeedServiceRange.GetIPNet().Mask.Size()
			allErrs = append(allErrs, services.ValidateMaxSize(maxSize)...)
		}
	}
	if seedNetworks.Nodes != nil {
		networks = append(networks, cidrvalidation.NewCIDR(*seedNetworks.Nodes, fldPath.Child("nodes")))
	}
	if shootDefaults := seedNetworks.ShootDefaults; shootDefaults != nil {
		if shootDefaults.Pods != nil {
			networks = append(networks, cidrvalidation.NewCIDR(*shootDefaults.Pods, fldPath.Child("shootDefaults", "pods")))
		}
		if shootDefaults.Services != nil {
			networks = append(networks, cidrvalidation.NewCIDR(*shootDefaults.Services, fldPath.Child("shootDefaults", "services")))
		}
	}

	allErrs = append(allErrs, cidrvalidation.ValidateCIDRParse(networks...)...)
	// Don't check IP family in dualstack case.
	if len(seedNetworks.IPFamilies) != 2 {
		allErrs = append(allErrs, cidrvalidation.ValidateCIDRIPFamily(networks, string(primaryIPFamily))...)
	}
	allErrs = append(allErrs, cidrvalidation.ValidateCIDROverlap(networks, false)...)

	allErrs = append(allErrs, reservedSeedServiceRange.ValidateNotOverlap(networks...)...)
	vpnRangeV6 := cidrvalidation.NewCIDR(v1beta1constants.DefaultVPNRangeV6, field.NewPath(""))
	allErrs = append(allErrs, vpnRangeV6.ValidateNotOverlap(networks...)...)

	return allErrs
}

// ValidateSeedSpecUpdate validates the specification updates of a Seed object.
func ValidateSeedSpecUpdate(newSeedSpec, oldSeedSpec *core.SeedSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateSeedNetworksUpdate(newSeedSpec.Networks, oldSeedSpec.Networks, fldPath.Child("networks"))...)

	if oldSeedSpec.Ingress != nil && newSeedSpec.Ingress != nil {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSeedSpec.Ingress.Domain, oldSeedSpec.Ingress.Domain, fldPath.Child("ingress", "domain"))...)
	}

	if oldSeedSpec.Backup != nil {
		if newSeedSpec.Backup != nil {
			allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSeedSpec.Backup.Provider, oldSeedSpec.Backup.Provider, fldPath.Child("backup", "provider"))...)
			allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSeedSpec.Backup.Region, oldSeedSpec.Backup.Region, fldPath.Child("backup", "region"))...)
		} else {
			allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSeedSpec.Backup, oldSeedSpec.Backup, fldPath.Child("backup"))...)
		}
	}
	// If oldSeedSpec doesn't have backup configured, we allow to add it; but not the vice versa.

	return allErrs
}

func validateSeedNetworksUpdate(newSeedNetworks, oldSeedNetworks core.SeedNetworks, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSeedNetworks.IPFamilies, oldSeedNetworks.IPFamilies, fldPath.Child("ipFamilies"))...)

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSeedNetworks.Pods, oldSeedNetworks.Pods, fldPath.Child("pods"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSeedNetworks.Services, oldSeedNetworks.Services, fldPath.Child("services"))...)
	if oldSeedNetworks.Nodes != nil {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSeedNetworks.Nodes, oldSeedNetworks.Nodes, fldPath.Child("nodes"))...)
	}

	return allErrs
}

// ValidateSeedStatusUpdate validates the status field of a Seed object.
func ValidateSeedStatusUpdate(newSeed, oldSeed *core.Seed) field.ErrorList {
	var (
		allErrs   = field.ErrorList{}
		fldPath   = field.NewPath("status")
		oldStatus = oldSeed.Status
		newStatus = newSeed.Status
	)

	if oldStatus.ClusterIdentity != nil && !apiequality.Semantic.DeepEqual(oldStatus.ClusterIdentity, newStatus.ClusterIdentity) {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newStatus.ClusterIdentity, oldStatus.ClusterIdentity, fldPath.Child("clusterIdentity"))...)
	}

	return allErrs
}

func validateSeedOperation(operation string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if operation == "" {
		return allErrs
	}

	if operation != "" && !availableSeedOperations.Has(operation) {
		allErrs = append(allErrs, field.NotSupported(fldPath, operation, sets.List(availableSeedOperations)))
	}

	return allErrs
}

func validateSeedOperationUpdate(newOperation, oldOperation string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if newOperation == "" || oldOperation == "" {
		return allErrs
	}

	if newOperation != oldOperation {
		allErrs = append(allErrs, field.Forbidden(fldPath, fmt.Sprintf("must not overwrite operation %q with %q", oldOperation, newOperation)))
	}

	return allErrs
}

func validateExtensionsForSeed(extensions []core.Extension, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for i, extension := range extensions {
		if extension.Disabled != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath.Index(i).Child("disabled"), "must not be set"))
		}
	}

	return allErrs
}
