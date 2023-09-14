// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package admissionplugins

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
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
	"k8s.io/utils/pointer"

	"github.com/gardener/gardener/pkg/apis/core"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

var (
	// admissionPluginsVersionRanges contains the version ranges for all Kubernetes admission plugins.
	// Extracted from https://raw.githubusercontent.com/kubernetes/kubernetes/release-${version}/pkg/kubeapiserver/options/plugins.go
	// and https://raw.githubusercontent.com/kubernetes/kubernetes/release-${version}/staging/src/k8s.io/apiserver/pkg/server/plugins.go.
	// To maintain this list for each new Kubernetes version:
	//   - Run hack/compare-k8s-admission-plugins.sh <old-version> <new-version> (e.g. 'hack/compare-k8s-admission-plugins.sh 1.22 1.23').
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
		"ClusterTrustBundleAttest":             {AddedInVersion: "1.27"},
		"DefaultIngressClass":                  {},
		"DefaultStorageClass":                  {},
		"DefaultTolerationSeconds":             {},
		"DenyServiceExternalIPs":               {},
		"EventRateLimit":                       {},
		"ExtendedResourceToleration":           {},
		"ImagePolicyWebhook":                   {},
		"LimitPodHardAntiAffinityTopology":     {},
		"LimitRanger":                          {},
		"MutatingAdmissionWebhook":             {Required: true},
		"NamespaceAutoProvision":               {},
		"NamespaceExists":                      {},
		"NamespaceLifecycle":                   {Required: true},
		"NodeRestriction":                      {Required: true},
		"OwnerReferencesPermissionEnforcement": {},
		"PersistentVolumeClaimResize":          {},
		"PersistentVolumeLabel":                {},
		"PodNodeSelector":                      {},
		"PodSecurity":                          {Required: true},
		"PodSecurityPolicy":                    {RemovedInVersion: "1.25"},
		"PodTolerationRestriction":             {},
		"Priority":                             {Required: true},
		"ResourceQuota":                        {},
		"RuntimeClass":                         {},
		"SecurityContextDeny":                  {Forbidden: true},
		"ServiceAccount":                       {},
		"StorageObjectInUseProtection":         {Required: true},
		"TaintNodesByCondition":                {},
		"ValidatingAdmissionPolicy":            {AddedInVersion: "1.26"},
		"ValidatingAdmissionWebhook":           {Required: true},
	}

	admissionPluginsSupportingExternalKubeconfig = sets.New("ValidatingAdmissionWebhook", "MutatingAdmissionWebhook", "ImagePolicyWebhook")

	// PluginsInMigration is the list of plugins which can be specified in the Shoot spec if the constraints are satisfied. This is required to facilitate migration of
	// these plugins in some cases. For example, the "PodSecurityPolicy" plugin should be disabled in the Shoot spec for an upgrade from Kubernetes v1.24 to v1.25, but in v1.25
	// this plugin is not supported. gardener-apiserver will take care to clean this plugin from the spec. See https://github.com/gardener/gardener/pull/8212 for more details.
	PluginsInMigration = map[string]*semver.Constraints{
		"PodSecurityPolicy": versionutils.ConstraintK8sGreaterEqual125,
	}

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
	Forbidden        bool
	Required         bool
	AddedInVersion   string
	RemovedInVersion string
}

// Contains returns true if the range contains the given version, false otherwise.
// The range contains the given version only if it's greater or equal than AddedInVersion (always true if AddedInVersion is empty),
// and less than RemovedInVersion (always true if RemovedInVersion is empty).
func (r *AdmissionPluginVersionRange) Contains(version string) (bool, error) {
	var constraint string
	switch {
	case r.AddedInVersion != "" && r.RemovedInVersion == "":
		constraint = fmt.Sprintf(">= %s", r.AddedInVersion)
	case r.AddedInVersion == "" && r.RemovedInVersion != "":
		constraint = fmt.Sprintf("< %s", r.RemovedInVersion)
	case r.AddedInVersion != "" && r.RemovedInVersion != "":
		constraint = fmt.Sprintf(">= %s, < %s", r.AddedInVersion, r.RemovedInVersion)
	default:
		constraint = "*"
	}
	return versionutils.CheckVersionMeetsConstraint(version, constraint)
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

	kubernetesVersion, err := semver.NewVersion(version)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "kubernetes", "version"), version, err.Error()))
		return allErrs
	}

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
			// If the plugin is not supported, but it's disabled and it's a plugin in migration, then skip it.
			if constraint, ok := PluginsInMigration[plugin.Name]; ok && constraint.Check(kubernetesVersion) && pointer.BoolDeref(plugin.Disabled, false) {
				continue
			}
			allErrs = append(allErrs, field.Forbidden(idxPath.Child("name"), fmt.Sprintf("admission plugin %q is not supported in Kubernetes version %s", plugin.Name, version)))
		} else {
			if admissionPluginsVersionRanges[plugin.Name].Forbidden {
				allErrs = append(allErrs, field.Forbidden(idxPath.Child("name"), fmt.Sprintf("forbidden admission plugin was specified - do not use plugins from the following list: %+v", getAllForbiddenPlugins())))
			}
			if pointer.BoolDeref(plugin.Disabled, false) && admissionPluginsVersionRanges[plugin.Name].Required {
				allErrs = append(allErrs, field.Forbidden(idxPath, fmt.Sprintf("admission plugin %q cannot be disabled", plugin.Name)))
			}
			if plugin.KubeconfigSecretName != nil && !admissionPluginsSupportingExternalKubeconfig.Has(plugin.Name) {
				allErrs = append(allErrs, field.Forbidden(idxPath.Child("kubeconfigSecretName"), fmt.Sprintf("admission plugin %q does not allow specifying external kubeconfig", plugin.Name)))
			}
			if err := validateAdmissionPluginConfig(plugin, version, idxPath); err != nil {
				allErrs = append(allErrs, err)
			}
		}
	}

	return allErrs
}

func validateAdmissionPluginConfig(plugin core.AdmissionPlugin, version string, fldPath *field.Path) *field.Error {
	kubernetesVersion, err := semver.NewVersion(version)
	if err != nil {
		return field.Invalid(field.NewPath("spec", "kubernetes", "version"), version, err.Error())
	}

	switch plugin.Name {
	case "PodSecurity":
		if plugin.Config == nil {
			return nil
		}

		config, err := runtime.Decode(codec, plugin.Config.Raw)
		if err != nil {
			if runtime.IsNotRegisteredError(err) {
				return field.Invalid(fldPath.Child("config"), string(plugin.Config.Raw), "expected pod-security.admission.config.k8s.io/v1alpha1.PodSecurityConfiguration or pod-security.admission.config.k8s.io/v1beta1.PodSecurityConfiguration or pod-security.admission.config.k8s.io/v1.PodSecurityConfiguration")
			}
			return field.Invalid(fldPath.Child("config"), string(plugin.Config.Raw), fmt.Sprintf("cannot decode the given config: %s", err.Error()))
		}

		var (
			apiGroup    = config.GetObjectKind().GroupVersionKind().Group
			apiVersion  = config.GetObjectKind().GroupVersionKind().Version
			errorString = "PodSecurityConfiguration apiVersion for Kubernetes version %q should be %q but got %q"
		)

		if versionutils.ConstraintK8sLess125.Check(kubernetesVersion) &&
			apiVersion != admissionapiv1beta1.SchemeGroupVersion.Version &&
			apiVersion != admissionapiv1alpha1.SchemeGroupVersion.Version {
			return field.Invalid(fldPath.Child("config"), string(plugin.Config.Raw), fmt.Sprintf(errorString, version, "pod-security.admission.config.k8s.io/v1beta1 or pod-security.admission.config.k8s.io/v1alpha1", apiGroup+"/"+apiVersion))
		}
	}

	return nil
}
