// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package admissionplugins

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	admissionapiv1 "k8s.io/pod-security-admission/admission/api/v1"
	admissionapiv1alpha1 "k8s.io/pod-security-admission/admission/api/v1alpha1"
	admissionapiv1beta1 "k8s.io/pod-security-admission/admission/api/v1beta1"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

var (
	// admissionPluginsVersionRanges contains the version ranges for all Kubernetes admission plugins.
	// Extracted from https://raw.githubusercontent.com/kubernetes/kubernetes/release-${version}/pkg/kubeapiserver/options/plugins.go
	// and https://raw.githubusercontent.com/kubernetes/kubernetes/release-${version}/staging/src/k8s.io/apiserver/pkg/server/plugins.go.
	// To maintain this list for each new Kubernetes version:
	//   - Run hack/compare-k8s-admission-plugins.sh <old-version> <new-version> (e.g. 'hack/compare-k8s-admission-plugins.sh 1.26 1.27').
	//     It will present 2 lists of admission plugins: those added and those removed in <new-version> compared to <old-version> and
	//   - Add all added admission plugins to the map with <new-version> as AddedInVersion and no RemovedInVersion.
	//   - For any removed admission plugin, add <new-version> as RemovedInVersion to the already existing admission plugin in the map.
	admissionPluginsVersionRanges = map[string]*AdmissionPluginVersionRange{
		"AlwaysAdmit":                          {},
		"AlwaysDeny":                           {},
		"AlwaysPullImages":                     {},
		"CertificateApproval":                  {},
		"CertificateSigning":                   {},
		"CertificateSubjectRestriction":        {},
		"ClusterTrustBundleAttest":             {},
		"DefaultIngressClass":                  {},
		"DefaultStorageClass":                  {},
		"DefaultTolerationSeconds":             {},
		"DenyServiceExternalIPs":               {},
		"EventRateLimit":                       {},
		"ExtendedResourceToleration":           {},
		"ImagePolicyWebhook":                   {},
		"LimitPodHardAntiAffinityTopology":     {},
		"LimitRanger":                          {},
		"MutatingAdmissionPolicy":              {VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
		"MutatingAdmissionWebhook":             {Required: true},
		"NamespaceAutoProvision":               {},
		"NamespaceExists":                      {},
		"NamespaceLifecycle":                   {Required: true},
		"NodeRestriction":                      {Required: true},
		"OwnerReferencesPermissionEnforcement": {},
		"PersistentVolumeClaimResize":          {},
		"PersistentVolumeLabel":                {VersionRange: versionutils.VersionRange{RemovedInVersion: "1.31"}},
		"PodNodeSelector":                      {},
		"PodSecurity":                          {Required: true},
		"PodTolerationRestriction":             {},
		"PodTopologyLabels":                    {VersionRange: versionutils.VersionRange{AddedInVersion: "1.33"}},
		"Priority":                             {Required: true},
		"ResourceQuota":                        {},
		"RuntimeClass":                         {},
		"SecurityContextDeny":                  {Forbidden: true, VersionRange: versionutils.VersionRange{RemovedInVersion: "1.30"}},
		"ServiceAccount":                       {},
		"StorageObjectInUseProtection":         {Required: true},
		"TaintNodesByCondition":                {},
		"ValidatingAdmissionPolicy":            {},
		"ValidatingAdmissionWebhook":           {Required: true},
	}

	admissionPluginsSupportingExternalKubeconfig = sets.New("ValidatingAdmissionWebhook", "MutatingAdmissionWebhook", "ImagePolicyWebhook")

	runtimeScheme *runtime.Scheme
	codec         runtime.Codec
)

func init() {
	runtimeScheme = runtime.NewScheme()
	utilruntime.Must(admissionapiv1alpha1.AddToScheme(runtimeScheme))
	utilruntime.Must(admissionapiv1beta1.AddToScheme(runtimeScheme))
	utilruntime.Must(admissionapiv1.AddToScheme(runtimeScheme))

	var (
		ser = json.NewSerializerWithOptions(json.DefaultMetaFactory, runtimeScheme, runtimeScheme, json.SerializerOptions{
			Yaml:   true,
			Pretty: false,
			Strict: false,
		})
		versions = schema.GroupVersions([]schema.GroupVersion{
			admissionapiv1alpha1.SchemeGroupVersion,
			admissionapiv1beta1.SchemeGroupVersion,
			admissionapiv1.SchemeGroupVersion,
		})
	)

	codec = serializer.NewCodecFactory(runtimeScheme).CodecForVersions(ser, ser, versions, versions)
}

// IsAdmissionPluginSupported returns true if the given admission plugin is supported for the given Kubernetes version.
// An admission plugin is only supported if it's a known admission plugin and its version range contains the given Kubernetes version.
func IsAdmissionPluginSupported(plugin, version string) (bool, error) {
	vr := admissionPluginsVersionRanges[plugin]
	if vr == nil {
		return false, fmt.Errorf("unknown admission plugin %q", plugin)
	}
	return vr.Contains(version)
}

// AdmissionPluginVersionRange represents a version range of type [AddedInVersion, RemovedInVersion).
type AdmissionPluginVersionRange struct {
	Forbidden bool
	Required  bool
	versionutils.VersionRange
}

func getAllForbiddenPlugins() []string {
	var allForbiddenPlugins []string
	for name, vr := range admissionPluginsVersionRanges {
		if vr.Forbidden {
			allForbiddenPlugins = append(allForbiddenPlugins, name)
		}
	}
	return allForbiddenPlugins
}

// ValidateAdmissionPlugins validates the given Kubernetes admission plugins against the given Kubernetes version.
func ValidateAdmissionPlugins(admissionPlugins []core.AdmissionPlugin, version string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for i, plugin := range admissionPlugins {
		idxPath := fldPath.Index(i)

		if len(plugin.Name) == 0 {
			allErrs = append(allErrs, field.Required(idxPath.Child("name"), "must provide a name"))
			return allErrs
		}

		supported, err := IsAdmissionPluginSupported(plugin.Name, version)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(idxPath.Child("name"), plugin.Name, err.Error()))
		} else if !supported {
			allErrs = append(allErrs, field.Forbidden(idxPath.Child("name"), fmt.Sprintf("admission plugin %q is not supported in Kubernetes version %s", plugin.Name, version)))
		} else {
			if admissionPluginsVersionRanges[plugin.Name].Forbidden {
				allErrs = append(allErrs, field.Forbidden(idxPath.Child("name"), fmt.Sprintf("forbidden admission plugin was specified - do not use plugins from the following list: %+v", getAllForbiddenPlugins())))
			}
			if ptr.Deref(plugin.Disabled, false) && admissionPluginsVersionRanges[plugin.Name].Required {
				allErrs = append(allErrs, field.Forbidden(idxPath, fmt.Sprintf("admission plugin %q cannot be disabled", plugin.Name)))
			}
			if plugin.KubeconfigSecretName != nil && !admissionPluginsSupportingExternalKubeconfig.Has(plugin.Name) {
				allErrs = append(allErrs, field.Forbidden(idxPath.Child("kubeconfigSecretName"), fmt.Sprintf("admission plugin %q does not allow specifying external kubeconfig", plugin.Name)))
			}
			if err := validateAdmissionPluginConfig(plugin, idxPath); err != nil {
				allErrs = append(allErrs, err)
			}
		}
	}

	return allErrs
}

func validateAdmissionPluginConfig(plugin core.AdmissionPlugin, fldPath *field.Path) *field.Error {
	switch plugin.Name {
	case "PodSecurity":
		if plugin.Config == nil {
			return nil
		}

		_, err := runtime.Decode(codec, plugin.Config.Raw)
		if err != nil {
			if runtime.IsNotRegisteredError(err) {
				return field.Invalid(fldPath.Child("config"), string(plugin.Config.Raw), "expected pod-security.admission.config.k8s.io/v1.PodSecurityConfiguration")
			}
			return field.Invalid(fldPath.Child("config"), string(plugin.Config.Raw), "cannot decode the given config: "+err.Error())
		}
	}

	return nil
}
