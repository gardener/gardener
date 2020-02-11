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

package validation

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/helper"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	cidrvalidation "github.com/gardener/gardener/pkg/utils/validation/cidr"
	versionutils "github.com/gardener/gardener/pkg/utils/version"

	"github.com/Masterminds/semver"
	"github.com/robfig/cron"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/resource"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

var (
	availableProxyMode = sets.NewString(
		string(core.ProxyModeIPTables),
		string(core.ProxyModeIPVS),
	)
	availableKubernetesDashboardAuthenticationModes = sets.NewString(
		core.KubernetesDashboardAuthModeBasic,
		core.KubernetesDashboardAuthModeToken,
	)
	availableNginxIngressExternalTrafficPolicies = sets.NewString(
		string(corev1.ServiceExternalTrafficPolicyTypeCluster),
		string(corev1.ServiceExternalTrafficPolicyTypeLocal),
	)
	availableShootPurposes = sets.NewString(
		string(core.ShootPurposeEvaluation),
		string(core.ShootPurposeTesting),
		string(core.ShootPurposeDevelopment),
		string(core.ShootPurposeProduction),
	)
)

// ValidateShoot validates a Shoot object.
func ValidateShoot(shoot *core.Shoot) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&shoot.ObjectMeta, true, apivalidation.NameIsDNSLabel, field.NewPath("metadata"))...)
	allErrs = append(allErrs, validateNameConsecutiveHyphens(shoot.Name, field.NewPath("metadata", "name"))...)
	allErrs = append(allErrs, ValidateShootSpec(shoot.ObjectMeta, &shoot.Spec, field.NewPath("spec"))...)

	return allErrs
}

// ValidateShootUpdate validates a Shoot object before an update.
func ValidateShootUpdate(newShoot, oldShoot *core.Shoot) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&newShoot.ObjectMeta, &oldShoot.ObjectMeta, field.NewPath("metadata"))...)

	// TODO: Just a temporary solution. Remove this in a future version once Kyma is moved out again.
	if metav1.HasAnnotation(oldShoot.ObjectMeta, common.ShootExperimentalAddonKyma) && !metav1.HasAnnotation(newShoot.ObjectMeta, common.ShootExperimentalAddonKyma) {
		allErrs = append(allErrs, field.Forbidden(field.NewPath("metadata", "annotations", common.ShootExperimentalAddonKyma), "experimental kyma addon can not be removed/uninstalled - please delete your cluster if you want to get rid of it"))
	}

	allErrs = append(allErrs, ValidateShootSpecUpdate(&newShoot.Spec, &oldShoot.Spec, newShoot.DeletionTimestamp != nil, field.NewPath("spec"))...)
	allErrs = append(allErrs, ValidateShoot(newShoot)...)

	return allErrs
}

// ValidateShootSpec validates the specification of a Shoot object.
func ValidateShootSpec(meta metav1.ObjectMeta, spec *core.ShootSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateAddons(spec.Addons, spec.Kubernetes.KubeAPIServer, fldPath.Child("addons"))...)
	allErrs = append(allErrs, validateDNS(spec.DNS, fldPath.Child("dns"))...)
	allErrs = append(allErrs, validateExtensions(spec.Extensions, fldPath.Child("extensions"))...)
	allErrs = append(allErrs, validateKubernetes(spec.Kubernetes, fldPath.Child("kubernetes"))...)
	allErrs = append(allErrs, validateNetworking(spec.Networking, fldPath.Child("networking"))...)
	allErrs = append(allErrs, validateMaintenance(spec.Maintenance, fldPath.Child("maintenance"))...)
	allErrs = append(allErrs, validateMonitoring(spec.Monitoring, fldPath.Child("monitoring"))...)
	allErrs = append(allErrs, ValidateHibernation(spec.Hibernation, fldPath.Child("hibernation"))...)
	allErrs = append(allErrs, validateProvider(spec.Provider, spec.Kubernetes, fldPath.Child("provider"))...)

	if len(spec.Region) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("region"), "must specify a region"))
	}
	if len(spec.CloudProfileName) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("cloudProfileName"), "must specify a cloud profile"))
	}
	if len(spec.SecretBindingName) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("secretBindingName"), "must specify a name"))
	}
	if spec.SeedName != nil && len(*spec.SeedName) == 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("seedName"), spec.SeedName, "seed name must not be empty when providing the key"))
	}
	if purpose := spec.Purpose; purpose != nil {
		allowedShootPurposes := availableShootPurposes
		if meta.Namespace == v1beta1constants.GardenNamespace {
			allowedShootPurposes = sets.NewString(append(availableShootPurposes.List(), string(core.ShootPurposeInfrastructure))...)
		}

		if !allowedShootPurposes.Has(string(*purpose)) {
			allErrs = append(allErrs, field.NotSupported(fldPath.Child("purpose"), *purpose, allowedShootPurposes.List()))
		}
	}

	// TODO: Just a temporary solution. Remove this in a future version once Kyma is moved out again.
	if metav1.HasAnnotation(meta, common.ShootExperimentalAddonKyma) {
		kubernetesGeq114Less116, err := versionutils.CheckVersionMeetsConstraint(spec.Kubernetes.Version, ">= 1.14, < 1.16")
		if err != nil {
			kubernetesGeq114Less116 = false
		}
		if !kubernetesGeq114Less116 {
			allErrs = append(allErrs, field.Forbidden(field.NewPath("kubernetes", "version"), "experimental kyma addon needs Kubernetes >= 1.14 and < 1.16"))
		}
	}

	return allErrs
}

// ValidateShootSpecUpdate validates the specification of a Shoot object.
func ValidateShootSpecUpdate(newSpec, oldSpec *core.ShootSpec, deletionTimestampSet bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if deletionTimestampSet && !apiequality.Semantic.DeepEqual(newSpec, oldSpec) {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec, oldSpec, fldPath)...)
		return allErrs
	}

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Region, oldSpec.Region, fldPath.Child("region"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.CloudProfileName, oldSpec.CloudProfileName, fldPath.Child("cloudProfileName"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.SecretBindingName, oldSpec.SecretBindingName, fldPath.Child("secretBindingName"))...)
	if oldSpec.SeedName != nil {
		// allow initial seed assignment
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.SeedName, oldSpec.SeedName, fldPath.Child("seedName"))...)
	}

	seedGotAssigned := oldSpec.SeedName == nil && newSpec.SeedName != nil

	allErrs = append(allErrs, validateDNSUpdate(newSpec.DNS, oldSpec.DNS, seedGotAssigned, fldPath.Child("dns"))...)
	allErrs = append(allErrs, validateKubernetesVersionUpdate(newSpec.Kubernetes.Version, oldSpec.Kubernetes.Version, fldPath.Child("kubernetes", "version"))...)
	allErrs = append(allErrs, validateKubeProxyModeUpdate(newSpec.Kubernetes.KubeProxy, oldSpec.Kubernetes.KubeProxy, newSpec.Kubernetes.Version, fldPath.Child("kubernetes", "kubeProxy"))...)
	allErrs = append(allErrs, validateKubeControllerManagerConfiguration(newSpec.Kubernetes.KubeControllerManager, oldSpec.Kubernetes.KubeControllerManager, fldPath.Child("kubernetes", "kubeControllerManager"))...)

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Provider.Type, oldSpec.Provider.Type, fldPath.Child("provider", "type"))...)
	for i, worker := range newSpec.Provider.Workers {
		if ShouldEnforceImmutability(worker.Zones, oldSpec.Provider.Workers[i].Zones) {
			allErrs = append(allErrs, apivalidation.ValidateImmutableField(worker.Zones, oldSpec.Provider.Workers[i].Zones, fldPath.Child("provider", "workers").Index(i).Child("zones"))...)
		}
	}

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Networking.Type, oldSpec.Networking.Type, fldPath.Child("networking", "type"))...)
	if oldSpec.Networking.Pods != nil {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Networking.Pods, oldSpec.Networking.Pods, fldPath.Child("networking", "pods"))...)
	}
	if oldSpec.Networking.Services != nil {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Networking.Services, oldSpec.Networking.Services, fldPath.Child("networking", "services"))...)
	}
	if oldSpec.Networking.Nodes != nil {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Networking.Nodes, oldSpec.Networking.Nodes, fldPath.Child("networking", "nodes"))...)
	}

	return allErrs
}

// ValidateShootStatusUpdate validates the status field of a Shoot object.
func ValidateShootStatusUpdate(newStatus, oldStatus core.ShootStatus) field.ErrorList {
	var (
		allErrs = field.ErrorList{}
		fldPath = field.NewPath("status")
	)

	if len(oldStatus.UID) > 0 {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newStatus.UID, oldStatus.UID, fldPath.Child("uid"))...)
	}
	if len(oldStatus.TechnicalID) > 0 {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newStatus.TechnicalID, oldStatus.TechnicalID, fldPath.Child("technicalID"))...)
	}

	return allErrs
}

func validateAddons(addons *core.Addons, kubeAPIServerConfig *core.KubeAPIServerConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if addons == nil {
		return allErrs
	}

	if addons.NginxIngress != nil && addons.NginxIngress.Enabled {
		if policy := addons.NginxIngress.ExternalTrafficPolicy; policy != nil {
			if !availableNginxIngressExternalTrafficPolicies.Has(string(*policy)) {
				allErrs = append(allErrs, field.NotSupported(fldPath.Child("nginx-ingress", "externalTrafficPolicy"), *policy, availableNginxIngressExternalTrafficPolicies.List()))
			}
		}
	}

	if addons.KubernetesDashboard != nil && addons.KubernetesDashboard.Enabled {
		if authMode := addons.KubernetesDashboard.AuthenticationMode; authMode != nil {
			if !availableKubernetesDashboardAuthenticationModes.Has(*authMode) {
				allErrs = append(allErrs, field.NotSupported(fldPath.Child("kubernetes-dashboard", "authenticationMode"), *authMode, availableKubernetesDashboardAuthenticationModes.List()))
			}

			if *authMode == core.KubernetesDashboardAuthModeBasic && !helper.ShootWantsBasicAuthentication(kubeAPIServerConfig) {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("kubernetes-dashboard", "authenticationMode"), *authMode, "cannot use basic auth mode when basic auth is not enabled in kube-apiserver configuration"))
			}
		}
	}

	return allErrs
}

// ValidateNodeCIDRMaskWithMaxPod validates if the Pod Network has enough ip addresses (configured via the NodeCIDRMask on the kube controller manager) to support the highest max pod setting on the shoot
func ValidateNodeCIDRMaskWithMaxPod(maxPod int32, nodeCIDRMaskSize int32) field.ErrorList {
	allErrs := field.ErrorList{}

	free := float64(32 - nodeCIDRMaskSize)
	// first and last ips are reserved
	ipAdressesAvailable := int32(math.Pow(2, free) - 2)

	if ipAdressesAvailable < maxPod {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("kubernetes").Child("kubeControllerManager").Child("nodeCIDRMaskSize"), nodeCIDRMaskSize, fmt.Sprintf("kubelet or kube-controller configuration incorrect. Please adjust the NodeCIDRMaskSize of the kube-controller to support the highest maxPod on any worker pool. The NodeCIDRMaskSize of '%d (default: 24)' of the kube-controller only supports '%d' ip adresses. Highest maxPod setting on kubelet is '%d (default: 110)'. Please choose a NodeCIDRMaskSize that at least supports %d ip adresses", nodeCIDRMaskSize, ipAdressesAvailable, maxPod, maxPod)))
	}

	return allErrs
}

func validateKubeControllerManagerConfiguration(newConfig, oldConfig *core.KubeControllerManagerConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	var (
		newSize *int32
		oldSize *int32
	)

	if newConfig != nil {
		newSize = newConfig.NodeCIDRMaskSize
	}
	if oldConfig != nil {
		oldSize = oldConfig.NodeCIDRMaskSize
	}

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSize, oldSize, fldPath.Child("nodeCIDRMaskSize"))...)

	return allErrs
}

func validateKubeProxyModeUpdate(newConfig, oldConfig *core.KubeProxyConfig, version string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	newMode := core.ProxyModeIPTables
	oldMode := core.ProxyModeIPTables
	if newConfig != nil {
		newMode = *newConfig.Mode
	}
	if oldConfig != nil {
		oldMode = *oldConfig.Mode
	}
	if ok, _ := versionutils.CheckVersionMeetsConstraint(version, ">= 1.14.1"); ok {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newMode, oldMode, fldPath.Child("mode"))...)
	}
	return allErrs
}

func validateDNSUpdate(new, old *core.DNS, seedGotAssigned bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if old != nil && new == nil {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(new, old, fldPath)...)
	}

	if new != nil && old != nil {
		if old.Domain != nil && new.Domain != old.Domain {
			allErrs = append(allErrs, apivalidation.ValidateImmutableField(new.Domain, old.Domain, fldPath.Child("domain"))...)
		}

		// allow to finalize DNS configuration during seed assignment. this is required because
		// some decisions about the DNS setup can only be taken once the target seed is clarified.
		if !seedGotAssigned {
			providersNew := len(new.Providers)
			providersOld := len(old.Providers)

			if providersNew != providersOld {
				allErrs = append(allErrs, field.Forbidden(fldPath.Child("providers"), "adding or removing providers is not yet allowed"))
				return allErrs
			}

			for i, provider := range new.Providers {
				if provider.Type != old.Providers[i].Type {
					allErrs = append(allErrs, apivalidation.ValidateImmutableField(provider.Type, old.Providers[i].Type, fldPath.Child("providers").Index(i).Child("type"))...)
				}
			}
		}
	}

	return allErrs
}

func validateKubernetesVersionUpdate(new, old string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(new) == 0 {
		allErrs = append(allErrs, field.Invalid(fldPath, new, "cannot validate kubernetes version upgrade because it is unset"))
		return allErrs
	}

	// Forbid Kubernetes version downgrade
	downgrade, err := versionutils.CompareVersions(new, "<", old)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, new, err.Error()))
	}
	if downgrade {
		allErrs = append(allErrs, field.Forbidden(fldPath, "kubernetes version downgrade is not supported"))
	}

	// Forbid Kubernetes version upgrade which skips a minor version
	oldVersion, err := semver.NewVersion(old)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, old, err.Error()))
	}
	nextMinorVersion := oldVersion.IncMinor().IncMinor()

	skippingMinorVersion, err := versionutils.CompareVersions(new, ">=", nextMinorVersion.String())
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, new, err.Error()))
	}
	if skippingMinorVersion {
		allErrs = append(allErrs, field.Forbidden(fldPath, "kubernetes version upgrade cannot skip a minor version"))
	}

	return allErrs
}

func validateDNS(dns *core.DNS, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if dns == nil {
		return allErrs
	}

	if dns.Domain != nil {
		allErrs = append(allErrs, validateDNS1123Subdomain(*dns.Domain, fldPath.Child("domain"))...)
	}

	for i, provider := range dns.Providers {
		idxPath := fldPath.Child("providers").Index(i)
		if providerType := provider.Type; providerType != nil {
			if *providerType == core.DNSUnmanaged && provider.SecretName != nil {
				allErrs = append(allErrs, field.Invalid(idxPath.Child("secretName"), provider.SecretName, fmt.Sprintf("secretName must not be set when type is %q", core.DNSUnmanaged)))
			}

			if *providerType != core.DNSUnmanaged && dns.Domain == nil {
				allErrs = append(allErrs, field.Required(idxPath.Child("domain"), fmt.Sprintf("domain must be set when type is not set to %q", core.DNSUnmanaged)))
			}
		}

		if provider.SecretName != nil && provider.Type == nil {
			allErrs = append(allErrs, field.Required(idxPath.Child("type"), "type must be set when secretName is set"))
		}
	}

	return allErrs
}

func validateExtensions(extensions []core.Extension, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	for i, extension := range extensions {
		if extension.Type == "" {
			allErrs = append(allErrs, field.Required(fldPath.Index(i).Child("type"), "field must not be empty"))
		}
	}
	return allErrs
}

func validateKubernetes(kubernetes core.Kubernetes, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(kubernetes.Version) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("version"), "kubernetes version must not be empty"))
		return allErrs
	}

	if kubeAPIServer := kubernetes.KubeAPIServer; kubeAPIServer != nil {
		if oidc := kubeAPIServer.OIDCConfig; oidc != nil {
			oidcPath := fldPath.Child("kubeAPIServer", "oidcConfig")

			geqKubernetes111, err := versionutils.CheckVersionMeetsConstraint(kubernetes.Version, ">= 1.11")
			if err != nil {
				geqKubernetes111 = false
			}

			if oidc.CABundle != nil {
				if _, err := utils.DecodeCertificate([]byte(*oidc.CABundle)); err != nil {
					allErrs = append(allErrs, field.Invalid(oidcPath.Child("caBundle"), *oidc.CABundle, "caBundle is not a valid PEM-encoded certificate"))
				}
			}
			if oidc.ClientID != nil && len(*oidc.ClientID) == 0 {
				allErrs = append(allErrs, field.Invalid(oidcPath.Child("clientID"), *oidc.ClientID, "client id cannot be empty when key is provided"))
			}
			if oidc.GroupsClaim != nil && len(*oidc.GroupsClaim) == 0 {
				allErrs = append(allErrs, field.Invalid(oidcPath.Child("groupsClaim"), *oidc.GroupsClaim, "groups claim cannot be empty when key is provided"))
			}
			if oidc.GroupsPrefix != nil && len(*oidc.GroupsPrefix) == 0 {
				allErrs = append(allErrs, field.Invalid(oidcPath.Child("groupsPrefix"), *oidc.GroupsPrefix, "groups prefix cannot be empty when key is provided"))
			}
			if oidc.IssuerURL != nil && len(*oidc.IssuerURL) == 0 {
				allErrs = append(allErrs, field.Invalid(oidcPath.Child("issuerURL"), *oidc.IssuerURL, "issuer url cannot be empty when key is provided"))
			}
			if oidc.SigningAlgs != nil && len(oidc.SigningAlgs) == 0 {
				allErrs = append(allErrs, field.Invalid(oidcPath.Child("signingAlgs"), oidc.SigningAlgs, "signings algs cannot be empty when key is provided"))
			}
			if !geqKubernetes111 && oidc.RequiredClaims != nil {
				allErrs = append(allErrs, field.Forbidden(oidcPath.Child("requiredClaims"), "required claims cannot be provided when version is not greater or equal 1.11"))
			}
			if oidc.UsernameClaim != nil && len(*oidc.UsernameClaim) == 0 {
				allErrs = append(allErrs, field.Invalid(oidcPath.Child("usernameClaim"), *oidc.UsernameClaim, "username claim cannot be empty when key is provided"))
			}
			if oidc.UsernamePrefix != nil && len(*oidc.UsernamePrefix) == 0 {
				allErrs = append(allErrs, field.Invalid(oidcPath.Child("usernamePrefix"), *oidc.UsernamePrefix, "username prefix cannot be empty when key is provided"))
			}
		}

		admissionPluginsPath := fldPath.Child("kubeAPIServer", "admissionPlugins")
		for i, plugin := range kubeAPIServer.AdmissionPlugins {
			idxPath := admissionPluginsPath.Index(i)

			if len(plugin.Name) == 0 {
				allErrs = append(allErrs, field.Required(idxPath.Child("name"), "must provide a name"))
			}
		}

		if auditConfig := kubeAPIServer.AuditConfig; auditConfig != nil {
			auditPath := fldPath.Child("kubeAPIServer", "auditConfig")
			if auditPolicy := auditConfig.AuditPolicy; auditPolicy != nil && auditConfig.AuditPolicy.ConfigMapRef != nil {
				allErrs = append(allErrs, validateAuditPolicyConfigMapReference(auditPolicy.ConfigMapRef, auditPath.Child("auditPolicy", "configMapRef"))...)
			}
		}
	}

	allErrs = append(allErrs, validateKubeControllerManager(kubernetes.Version, kubernetes.KubeControllerManager, fldPath.Child("kubeControllerManager"))...)
	allErrs = append(allErrs, validateKubeProxy(kubernetes.KubeProxy, fldPath.Child("kubeProxy"))...)
	if clusterAutoscaler := kubernetes.ClusterAutoscaler; clusterAutoscaler != nil {
		allErrs = append(allErrs, ValidateClusterAutoscaler(*clusterAutoscaler, fldPath.Child("clusterAutoscaler"))...)
	}

	return allErrs
}

func validateNetworking(networking core.Networking, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(networking.Type) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("type"), "networking type must be provided"))
	}

	if networking.Nodes != nil {
		path := fldPath.Child("nodes")
		cidr := cidrvalidation.NewCIDR(*networking.Nodes, path)

		allErrs = append(allErrs, cidr.ValidateParse()...)
		allErrs = append(allErrs, cidrvalidation.ValidateCIDRIsCanonical(path, cidr.GetCIDR())...)
	}

	if networking.Pods != nil {
		path := fldPath.Child("pods")
		cidr := cidrvalidation.NewCIDR(*networking.Pods, path)

		allErrs = append(allErrs, cidr.ValidateParse()...)
		allErrs = append(allErrs, cidrvalidation.ValidateCIDRIsCanonical(path, cidr.GetCIDR())...)
	}

	if networking.Services != nil {
		path := fldPath.Child("services")
		cidr := cidrvalidation.NewCIDR(*networking.Services, path)

		allErrs = append(allErrs, cidr.ValidateParse()...)
		allErrs = append(allErrs, cidrvalidation.ValidateCIDRIsCanonical(path, cidr.GetCIDR())...)
	}

	return allErrs
}

// ValidateClusterAutoscaler validates the given ClusterAutoscaler fields.
func ValidateClusterAutoscaler(autoScaler core.ClusterAutoscaler, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if threshold := autoScaler.ScaleDownUtilizationThreshold; threshold != nil {
		if *threshold < 0.0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("scaleDownUtilizationThreshold"), *threshold, "can not be negative"))
		}
		if *threshold > 1.0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("scaleDownUtilizationThreshold"), *threshold, "can not be greater than 1.0"))
		}
	}
	return allErrs
}

func validateKubeControllerManager(kubernetesVersion string, kcm *core.KubeControllerManagerConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	k8sVersionLessThan112, err := versionutils.CompareVersions(kubernetesVersion, "<", "1.12")
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, kubernetesVersion, err.Error()))
	}
	if kcm != nil {
		if maskSize := kcm.NodeCIDRMaskSize; maskSize != nil {
			if *maskSize < 16 || *maskSize > 28 {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("nodeCIDRMaskSize"), *maskSize, "nodeCIDRMaskSize must be between 16 and 28"))
			}
		}
		if hpa := kcm.HorizontalPodAutoscalerConfig; hpa != nil {
			fldPath = fldPath.Child("horizontalPodAutoscaler")

			if hpa.SyncPeriod != nil && hpa.SyncPeriod.Duration < 1*time.Second {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("syncPeriod"), *hpa.SyncPeriod, "syncPeriod must not be less than a second"))
			}
			if hpa.Tolerance != nil && *hpa.Tolerance <= 0 {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("tolerance"), *hpa.Tolerance, "tolerance of must be greater than 0"))
			}

			if k8sVersionLessThan112 {
				if hpa.DownscaleDelay != nil && hpa.DownscaleDelay.Duration < 0 {
					allErrs = append(allErrs, field.Invalid(fldPath.Child("downscaleDelay"), *hpa.DownscaleDelay, "downscale delay must not be negative"))
				}
				if hpa.UpscaleDelay != nil && hpa.UpscaleDelay.Duration < 0 {
					allErrs = append(allErrs, field.Invalid(fldPath.Child("upscaleDelay"), *hpa.UpscaleDelay, "upscale delay must not be negative"))
				}
				if hpa.DownscaleStabilization != nil {
					allErrs = append(allErrs, field.Forbidden(fldPath.Child("downscaleStabilization"), "downscale stabilization is not supported in k8s versions < 1.12"))
				}
				if hpa.InitialReadinessDelay != nil {
					allErrs = append(allErrs, field.Forbidden(fldPath.Child("initialReadinessDelay"), "initial readiness delay is not supported in k8s versions < 1.12"))
				}
				if hpa.CPUInitializationPeriod != nil {
					allErrs = append(allErrs, field.Forbidden(fldPath.Child("cpuInitializationPeriod"), "cpu initialization period is not supported in k8s versions < 1.12"))
				}
			} else {
				if hpa.DownscaleDelay != nil {
					allErrs = append(allErrs, field.Forbidden(fldPath.Child("downscaleDelay"), "downscale delay is not supported in k8s versions >= 1.12"))
				}
				if hpa.UpscaleDelay != nil {
					allErrs = append(allErrs, field.Forbidden(fldPath.Child("upscaleDelay"), "upscale delay is not supported in k8s versions >= 1.12"))
				}
				if hpa.DownscaleStabilization != nil && hpa.DownscaleStabilization.Duration < 1*time.Second {
					allErrs = append(allErrs, field.Invalid(fldPath.Child("downscaleStabilization"), *hpa.DownscaleStabilization, "downScale stabilization must not be less than a second"))
				}
				if hpa.InitialReadinessDelay != nil && hpa.InitialReadinessDelay.Duration <= 0 {
					allErrs = append(allErrs, field.Invalid(fldPath.Child("initialReadinessDelay"), *hpa.InitialReadinessDelay, "initial readiness delay must be greater than 0"))
				}
				if hpa.CPUInitializationPeriod != nil && hpa.CPUInitializationPeriod.Duration < 1*time.Second {
					allErrs = append(allErrs, field.Invalid(fldPath.Child("cpuInitializationPeriod"), *hpa.CPUInitializationPeriod, "cpu initialization period must not be less than a second"))
				}
			}
		}
	}
	return allErrs
}

func validateKubeProxy(kp *core.KubeProxyConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if kp != nil {
		if kp.Mode == nil {
			allErrs = append(allErrs, field.Required(fldPath.Child("mode"), "must be set when .spec.kubernetes.kubeProxy is set"))
		} else if mode := *kp.Mode; !availableProxyMode.Has(string(mode)) {
			allErrs = append(allErrs, field.NotSupported(fldPath.Child("mode"), mode, availableProxyMode.List()))
		}
	}
	return allErrs
}

func validateMonitoring(monitoring *core.Monitoring, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if monitoring != nil && monitoring.Alerting != nil {
		allErrs = append(allErrs, validateAlerting(monitoring.Alerting, fldPath.Child("alerting"))...)
	}
	return allErrs
}

func validateAlerting(alerting *core.Alerting, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	emails := make(map[string]struct{})
	for i, email := range alerting.EmailReceivers {
		if !utils.TestEmail(email) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("emailReceivers").Index(i), email, "must provide a valid email"))
		}

		if _, duplicate := emails[email]; duplicate {
			allErrs = append(allErrs, field.Duplicate(fldPath.Child("emailReceivers").Index(i), email))
		} else {
			emails[email] = struct{}{}
		}
	}
	return allErrs
}

func validateMaintenance(maintenance *core.Maintenance, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if maintenance == nil {
		allErrs = append(allErrs, field.Required(fldPath, "maintenance information is required"))
		return allErrs
	}

	if maintenance.AutoUpdate == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("autoUpdate"), "auto update information is required"))
	}

	if maintenance.TimeWindow == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("timeWindow"), "time window information is required"))
	} else {
		maintenanceTimeWindow, err := utils.ParseMaintenanceTimeWindow(maintenance.TimeWindow.Begin, maintenance.TimeWindow.End)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("timeWindow", "begin/end"), maintenance.TimeWindow, err.Error()))
		}

		if err == nil {
			duration := maintenanceTimeWindow.Duration()
			if duration > 6*time.Hour {
				allErrs = append(allErrs, field.Forbidden(fldPath.Child("timeWindow"), "time window must not be greater than 6 hours"))
				return allErrs
			}
			if duration < 30*time.Minute {
				allErrs = append(allErrs, field.Forbidden(fldPath.Child("timeWindow"), "time window must not be smaller than 30 minutes"))
				return allErrs
			}
		}
	}

	return allErrs
}

func validateProvider(provider core.Provider, kubernetes core.Kubernetes, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(provider.Type) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("type"), "must specify a provider type"))
	}

	var maxPod int32
	if kubernetes.Kubelet != nil && kubernetes.Kubelet.MaxPods != nil {
		maxPod = *kubernetes.Kubelet.MaxPods
	}

	for i, worker := range provider.Workers {
		allErrs = append(allErrs, ValidateWorker(worker, fldPath.Child("workers").Index(i))...)

		if worker.Kubernetes != nil && worker.Kubernetes.Kubelet != nil && worker.Kubernetes.Kubelet.MaxPods != nil && *worker.Kubernetes.Kubelet.MaxPods > maxPod {
			maxPod = *worker.Kubernetes.Kubelet.MaxPods
		}
	}

	allErrs = append(allErrs, ValidateWorkers(provider.Workers, fldPath.Child("workers"))...)

	if kubernetes.KubeControllerManager != nil && kubernetes.KubeControllerManager.NodeCIDRMaskSize != nil {
		if maxPod == 0 {
			// default maxPod setting on kubelet
			maxPod = 110
		}
		allErrs = append(allErrs, ValidateNodeCIDRMaskWithMaxPod(maxPod, *kubernetes.KubeControllerManager.NodeCIDRMaskSize)...)
	}

	return allErrs
}

// ValidateWorker validates the worker object.
func ValidateWorker(worker core.Worker, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateDNS1123Label(worker.Name, fldPath.Child("name"))...)
	maxWorkerNameLength := 15
	if len(worker.Name) > maxWorkerNameLength {
		allErrs = append(allErrs, field.TooLong(fldPath.Child("name"), worker.Name, maxWorkerNameLength))
	}
	if len(worker.Machine.Type) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("machine", "type"), "must specify a machine type"))
	}
	if worker.Minimum < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("minimum"), worker.Minimum, "minimum value must not be negative"))
	}
	if worker.Maximum < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("maximum"), worker.Maximum, "maximum value must not be negative"))
	}
	if worker.Maximum < worker.Minimum {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("maximum"), "maximum value must not be less or equal than minimum value"))
	}
	if worker.Maximum != 0 && worker.Minimum == 0 {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("minimum"), "minimum value must be >= 1 if maximum value > 0 (cluster-autoscaler cannot handle min=0)"))
	}

	allErrs = append(allErrs, ValidatePositiveIntOrPercent(worker.MaxSurge, fldPath.Child("maxSurge"))...)
	allErrs = append(allErrs, ValidatePositiveIntOrPercent(worker.MaxUnavailable, fldPath.Child("maxUnavailable"))...)
	allErrs = append(allErrs, IsNotMoreThan100Percent(worker.MaxUnavailable, fldPath.Child("maxUnavailable"))...)

	if (worker.MaxUnavailable == nil && worker.MaxSurge == nil) || (getIntOrPercentValue(*worker.MaxUnavailable) == 0 && getIntOrPercentValue(*worker.MaxSurge) == 0) {
		// Both MaxSurge and MaxUnavailable cannot be zero.
		allErrs = append(allErrs, field.Invalid(fldPath.Child("maxUnavailable"), worker.MaxUnavailable, "may not be 0 when `maxSurge` is 0"))
	}

	allErrs = append(allErrs, metav1validation.ValidateLabels(worker.Labels, fldPath.Child("labels"))...)
	allErrs = append(allErrs, apivalidation.ValidateAnnotations(worker.Annotations, fldPath.Child("annotations"))...)
	if len(worker.Taints) > 0 {
		allErrs = append(allErrs, validateTaints(worker.Taints, fldPath.Child("taints"))...)
	}
	if worker.Kubernetes != nil && worker.Kubernetes.Kubelet != nil {
		allErrs = append(allErrs, ValidateKubeletConfig(*worker.Kubernetes.Kubelet, fldPath.Child("kubernetes", "kubelet"))...)
	}

	if worker.CABundle != nil {
		if _, err := utils.DecodeCertificate([]byte(*worker.CABundle)); err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("caBundle"), *(worker.CABundle), "caBundle is not a valid PEM-encoded certificate"))
		}
	}

	volumeSizeRegex, _ := regexp.Compile(`^(\d)+Gi$`)

	if worker.Volume != nil {
		if !volumeSizeRegex.MatchString(worker.Volume.Size) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("volume", "size"), worker.Volume.Size, fmt.Sprintf("volume size must match the regex %s", volumeSizeRegex)))
		}
	}

	if worker.DataVolumes != nil {
		var volumeNames = make(map[string]int)
		if len(worker.DataVolumes) > 0 && worker.Volume == nil {
			allErrs = append(allErrs, field.Required(fldPath.Child("volume"), fmt.Sprintf("a worker volume must be defined if data volumes are defined")))
		}
		for idx, volume := range worker.DataVolumes {
			idxPath := fldPath.Child("dataVolumes").Index(idx)
			if volume.Name == nil {
				allErrs = append(allErrs, field.Required(idxPath.Child("name"), fmt.Sprintf("data volume name is required")))
			} else {
				volName := *volume.Name
				allErrs = append(allErrs, validateDNS1123Label(volName, idxPath.Child("name"))...)
				maxVolumeNameLength := 15
				if len(volName) > maxVolumeNameLength {
					allErrs = append(allErrs, field.TooLong(idxPath.Child("name"), volName, maxVolumeNameLength))
				}
				if _, keyExist := volumeNames[volName]; keyExist {
					volumeNames[volName]++
					allErrs = append(allErrs, field.Duplicate(idxPath.Child("name"), volName))
				} else {
					volumeNames[volName] = 1
				}
				if !volumeSizeRegex.MatchString(volume.Size) {
					allErrs = append(allErrs, field.Invalid(idxPath.Child("size"), volume.Size, fmt.Sprintf("data volume size must match the regex %s", volumeSizeRegex)))
				}
			}
		}

	}

	if worker.KubeletDataVolumeName != nil {
		found := false
		for _, volume := range worker.DataVolumes {
			if *volume.Name == *worker.KubeletDataVolumeName {
				found = true
			}
		}
		if !found {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("kubeletDataVolumeName"), worker.KubeletDataVolumeName, fmt.Sprintf("KubeletDataVolumeName refers to unrecognized data volume %s", *worker.KubeletDataVolumeName)))
		}
	}

	return allErrs
}

// PodPIDsLimitMinimum is a constant for the minimum value for the podPIDsLimit field.
const PodPIDsLimitMinimum int64 = 100

// ValidateKubeletConfig validates the KubeletConfig object.
func ValidateKubeletConfig(kubeletConfig core.KubeletConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if kubeletConfig.MaxPods != nil {
		allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(*kubeletConfig.MaxPods), fldPath.Child("maxPods"))...)
	}
	if value := kubeletConfig.PodPIDsLimit; value != nil {
		if *value < PodPIDsLimitMinimum {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("podPIDsLimit"), *value, fmt.Sprintf("podPIDsLimit value must be at least %d", PodPIDsLimitMinimum)))
		}
	}
	if kubeletConfig.EvictionPressureTransitionPeriod != nil {
		allErrs = append(allErrs, ValidatePositiveDuration(kubeletConfig.EvictionPressureTransitionPeriod, fldPath.Child("evictionPressureTransitionPeriod"))...)
	}
	if kubeletConfig.EvictionMaxPodGracePeriod != nil {
		allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(*kubeletConfig.EvictionMaxPodGracePeriod), fldPath.Child("evictionMaxPodGracePeriod"))...)
	}
	if kubeletConfig.EvictionHard != nil {
		allErrs = append(allErrs, validateKubeletConfigEviction(kubeletConfig.EvictionHard, fldPath.Child("evictionHard"))...)
	}
	if kubeletConfig.EvictionSoft != nil {
		allErrs = append(allErrs, validateKubeletConfigEviction(kubeletConfig.EvictionSoft, fldPath.Child("evictionSoft"))...)
	}
	if kubeletConfig.EvictionMinimumReclaim != nil {
		allErrs = append(allErrs, validateKubeletConfigEvictionMinimumReclaim(kubeletConfig.EvictionMinimumReclaim, fldPath.Child("evictionMinimumReclaim"))...)
	}
	if kubeletConfig.EvictionSoftGracePeriod != nil {
		allErrs = append(allErrs, validateKubeletConfigEvictionSoftGracePeriod(kubeletConfig.EvictionSoftGracePeriod, fldPath.Child("evictionSoftGracePeriod"))...)
	}
	return allErrs
}

func validateKubeletConfigEviction(eviction *core.KubeletConfigEviction, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, ValidateResourceQuantityOrPercent(eviction.MemoryAvailable, fldPath, "memoryAvailable")...)
	allErrs = append(allErrs, ValidateResourceQuantityOrPercent(eviction.ImageFSAvailable, fldPath, "imagefsAvailable")...)
	allErrs = append(allErrs, ValidateResourceQuantityOrPercent(eviction.ImageFSInodesFree, fldPath, "imagefsInodesFree")...)
	allErrs = append(allErrs, ValidateResourceQuantityOrPercent(eviction.NodeFSAvailable, fldPath, "nodefsAvailable")...)
	allErrs = append(allErrs, ValidateResourceQuantityOrPercent(eviction.ImageFSInodesFree, fldPath, "imagefsInodesFree")...)
	return allErrs
}

func validateKubeletConfigEvictionMinimumReclaim(eviction *core.KubeletConfigEvictionMinimumReclaim, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if eviction.MemoryAvailable != nil {
		allErrs = append(allErrs, validateResourceQuantityValue("memoryAvailable", *eviction.MemoryAvailable, fldPath.Child("memoryAvailable"))...)
	}
	if eviction.ImageFSAvailable != nil {
		allErrs = append(allErrs, validateResourceQuantityValue("imagefsAvailable", *eviction.ImageFSAvailable, fldPath.Child("imagefsAvailable"))...)
	}
	if eviction.ImageFSInodesFree != nil {
		allErrs = append(allErrs, validateResourceQuantityValue("imagefsInodesFree", *eviction.ImageFSInodesFree, fldPath.Child("imagefsInodesFree"))...)
	}
	if eviction.NodeFSAvailable != nil {
		allErrs = append(allErrs, validateResourceQuantityValue("nodefsAvailable", *eviction.NodeFSAvailable, fldPath.Child("nodefsAvailable"))...)
	}
	if eviction.ImageFSInodesFree != nil {
		allErrs = append(allErrs, validateResourceQuantityValue("imagefsInodesFree", *eviction.ImageFSInodesFree, fldPath.Child("imagefsInodesFree"))...)
	}
	return allErrs
}

func validateKubeletConfigEvictionSoftGracePeriod(eviction *core.KubeletConfigEvictionSoftGracePeriod, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, ValidatePositiveDuration(eviction.MemoryAvailable, fldPath.Child("memoryAvailable"))...)
	allErrs = append(allErrs, ValidatePositiveDuration(eviction.ImageFSAvailable, fldPath.Child("imagefsAvailable"))...)
	allErrs = append(allErrs, ValidatePositiveDuration(eviction.ImageFSInodesFree, fldPath.Child("imagefsInodesFree"))...)
	allErrs = append(allErrs, ValidatePositiveDuration(eviction.NodeFSAvailable, fldPath.Child("nodefsAvailable"))...)
	allErrs = append(allErrs, ValidatePositiveDuration(eviction.ImageFSInodesFree, fldPath.Child("imagefsInodesFree"))...)
	return allErrs
}

// https://github.com/kubernetes/kubernetes/blob/ee9079f8ec39914ff8975b5390749771b9303ea4/pkg/apis/core/validation/validation.go#L4057-L4089
func validateTaints(taints []corev1.Taint, fldPath *field.Path) field.ErrorList {
	allErrors := field.ErrorList{}

	uniqueTaints := map[corev1.TaintEffect]sets.String{}

	for i, taint := range taints {
		idxPath := fldPath.Index(i)
		// validate the taint key
		allErrors = append(allErrors, metav1validation.ValidateLabelName(taint.Key, idxPath.Child("key"))...)
		// validate the taint value
		if errs := validation.IsValidLabelValue(taint.Value); len(errs) != 0 {
			allErrors = append(allErrors, field.Invalid(idxPath.Child("value"), taint.Value, strings.Join(errs, ";")))
		}
		// validate the taint effect
		allErrors = append(allErrors, validateTaintEffect(&taint.Effect, false, idxPath.Child("effect"))...)

		// validate if taint is unique by <key, effect>
		if len(uniqueTaints[taint.Effect]) > 0 && uniqueTaints[taint.Effect].Has(taint.Key) {
			duplicatedError := field.Duplicate(idxPath, taint)
			duplicatedError.Detail = "taints must be unique by key and effect pair"
			allErrors = append(allErrors, duplicatedError)
			continue
		}

		// add taint to existingTaints for uniqueness check
		if len(uniqueTaints[taint.Effect]) == 0 {
			uniqueTaints[taint.Effect] = sets.String{}
		}
		uniqueTaints[taint.Effect].Insert(taint.Key)
	}
	return allErrors
}

// https://github.com/kubernetes/kubernetes/blob/ee9079f8ec39914ff8975b5390749771b9303ea4/pkg/apis/core/validation/validation.go#L2774-L2795
func validateTaintEffect(effect *corev1.TaintEffect, allowEmpty bool, fldPath *field.Path) field.ErrorList {
	if !allowEmpty && len(*effect) == 0 {
		return field.ErrorList{field.Required(fldPath, "")}
	}

	allErrors := field.ErrorList{}
	switch *effect {
	case corev1.TaintEffectNoSchedule, corev1.TaintEffectPreferNoSchedule, corev1.TaintEffectNoExecute:
	default:
		validValues := []string{
			string(corev1.TaintEffectNoSchedule),
			string(corev1.TaintEffectPreferNoSchedule),
			string(corev1.TaintEffectNoExecute),
		}
		allErrors = append(allErrors, field.NotSupported(fldPath, *effect, validValues))
	}
	return allErrors
}

// ValidateWorkers validates worker objects.
func ValidateWorkers(workers []core.Worker, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	workerNames := make(map[string]bool)
	atLeastOneActivePool := false

	for i, worker := range workers {
		if worker.Minimum != 0 && worker.Maximum != 0 {
			atLeastOneActivePool = true
		}

		if workerNames[worker.Name] {
			allErrs = append(allErrs, field.Duplicate(fldPath.Index(i).Child("name"), worker.Name))
		}
		workerNames[worker.Name] = true
	}

	if !atLeastOneActivePool {
		allErrs = append(allErrs, field.Forbidden(fldPath, "at least one worker pool with min>0 and max> 0 needed"))
	}

	return allErrs
}

// ValidateHibernation validates a Hibernation object.
func ValidateHibernation(hibernation *core.Hibernation, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if hibernation == nil {
		return allErrs
	}

	allErrs = append(allErrs, ValidateHibernationSchedules(hibernation.Schedules, fldPath.Child("schedules"))...)

	return allErrs
}

// ValidateHibernationSchedules validates a list of hibernation schedules.
func ValidateHibernationSchedules(schedules []core.HibernationSchedule, fldPath *field.Path) field.ErrorList {
	var (
		allErrs = field.ErrorList{}
		seen    = sets.NewString()
	)

	for i, schedule := range schedules {
		allErrs = append(allErrs, ValidateHibernationSchedule(seen, &schedule, fldPath.Index(i))...)
	}

	return allErrs
}

// ValidateHibernationCronSpec validates a cron specification of a hibernation schedule.
func ValidateHibernationCronSpec(seenSpecs sets.String, spec string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	_, err := cron.ParseStandard(spec)
	switch {
	case err != nil:
		allErrs = append(allErrs, field.Invalid(fldPath, spec, fmt.Sprintf("not a valid cron spec: %v", err)))
	case seenSpecs.Has(spec):
		allErrs = append(allErrs, field.Duplicate(fldPath, spec))
	default:
		seenSpecs.Insert(spec)
	}

	return allErrs
}

// ValidateHibernationScheduleLocation validates that the location of a HibernationSchedule is correct.
func ValidateHibernationScheduleLocation(location string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if _, err := time.LoadLocation(location); err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, location, fmt.Sprintf("not a valid location: %v", err)))
	}

	return allErrs
}

// ValidateHibernationSchedule validates the correctness of a HibernationSchedule.
// It checks whether the set start and end time are valid cron specs.
func ValidateHibernationSchedule(seenSpecs sets.String, schedule *core.HibernationSchedule, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if schedule.Start == nil && schedule.End == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("start/end"), "either start or end has to be provided"))
	}
	if schedule.Start != nil {
		allErrs = append(allErrs, ValidateHibernationCronSpec(seenSpecs, *schedule.Start, fldPath.Child("start"))...)
	}
	if schedule.End != nil {
		allErrs = append(allErrs, ValidateHibernationCronSpec(seenSpecs, *schedule.End, fldPath.Child("end"))...)
	}
	if schedule.Location != nil {
		allErrs = append(allErrs, ValidateHibernationScheduleLocation(*schedule.Location, fldPath.Child("location"))...)
	}

	return allErrs
}

// ValidatePositiveDuration validates that a duration is positive.
func ValidatePositiveDuration(duration *metav1.Duration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if duration == nil {
		return allErrs
	}
	if duration.Seconds() < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath, duration.Duration.String(), "must be non-negative"))
	}
	return allErrs
}

// ValidateResourceQuantityOrPercent checks if a value can be parsed to either a resource.quantity, a positive int or percent.
func ValidateResourceQuantityOrPercent(valuePtr *string, fldPath *field.Path, key string) field.ErrorList {
	allErrs := field.ErrorList{}

	if valuePtr == nil {
		return allErrs
	}
	value := *valuePtr
	// check for resource quantity
	if quantity, err := resource.ParseQuantity(value); err == nil {
		if len(validateResourceQuantityValue(key, quantity, fldPath)) == 0 {
			return allErrs
		}
	}

	if validation.IsValidPercent(value) != nil {
		allErrs = append(allErrs, field.Invalid(fldPath.Child(key), value, "field must be either a valid resource quantity (e.g 200Mi) or a percentage (e.g '5%')"))
		return allErrs
	}

	percentValue, _ := strconv.Atoi(value[:len(value)-1])
	if percentValue > 100 || percentValue < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child(key), value, "must not be greater than 100% and not smaller than 0%"))
	}
	return allErrs
}

// ValidatePositiveIntOrPercent validates a int or string object and ensures it is positive.
func ValidatePositiveIntOrPercent(intOrPercent *intstr.IntOrString, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if intOrPercent == nil {
		return allErrs
	}

	if intOrPercent.Type == intstr.String {
		if validation.IsValidPercent(intOrPercent.StrVal) != nil {
			allErrs = append(allErrs, field.Invalid(fldPath, intOrPercent, "must be an integer or percentage (e.g '5%')"))
		}
	} else if intOrPercent.Type == intstr.Int {
		allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(intOrPercent.IntValue()), fldPath)...)
	}

	return allErrs
}

// IsNotMoreThan100Percent validates an int or string object and ensures it is not more than 100%.
func IsNotMoreThan100Percent(intOrStringValue *intstr.IntOrString, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if intOrStringValue == nil {
		return allErrs
	}

	value, isPercent := getPercentValue(*intOrStringValue)
	if !isPercent || value <= 100 {
		return nil
	}
	allErrs = append(allErrs, field.Invalid(fldPath, intOrStringValue, "must not be greater than 100%"))

	return allErrs
}
