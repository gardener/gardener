// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apigroups

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation/field"

	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

var (
	// apiGroupVersionRanges contains the version ranges for all Kubernetes API GroupVersions.
	// Extracted from https://raw.githubusercontent.com/kubernetes/kubernetes/release-${version}/staging/src/k8s.io/client-go/informers/generic.go
	// To maintain this list for each new Kubernetes version, refer https://github.com/gardener/gardener/blob/master/docs/development/new-kubernetes-version.md#adapting-gardener
	// Keep the list ordered alphabetically.
	apiGroupVersionRanges = map[string]*APIVersionRange{
		"admissionregistration.k8s.io/v1":       {Required: true},
		"admissionregistration.k8s.io/v1alpha1": {VersionRange: versionutils.VersionRange{AddedInVersion: "1.26"}},
		"admissionregistration.k8s.io/v1beta1":  {},
		"apps/v1":                               {Required: true},
		"apps/v1beta1":                          {},
		"apps/v1beta2":                          {},
		"autoscaling/v1":                        {},
		"autoscaling/v2":                        {},
		"autoscaling/v2beta1":                   {},
		"autoscaling/v2beta2":                   {},
		"batch/v1":                              {},
		"batch/v1beta1":                         {},
		"certificates.k8s.io/v1":                {},
		"certificates.k8s.io/v1alpha1":          {VersionRange: versionutils.VersionRange{AddedInVersion: "1.27"}},
		"certificates.k8s.io/v1beta1":           {},
		"coordination.k8s.io/v1":                {},
		"coordination.k8s.io/v1alpha1":          {VersionRange: versionutils.VersionRange{AddedInVersion: "1.31", RemovedInVersion: "1.32"}},
		"coordination.k8s.io/v1alpha2":          {VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
		"coordination.k8s.io/v1beta1":           {},
		"discovery.k8s.io/v1":                   {},
		"discovery.k8s.io/v1beta1":              {},
		"events.k8s.io/v1":                      {},
		"events.k8s.io/v1beta1":                 {},
		"extensions/v1beta1":                    {},
		"flowcontrol.apiserver.k8s.io/v1":       {VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
		"flowcontrol.apiserver.k8s.io/v1alpha1": {VersionRange: versionutils.VersionRange{RemovedInVersion: "1.29"}},
		"flowcontrol.apiserver.k8s.io/v1beta1":  {},
		"flowcontrol.apiserver.k8s.io/v1beta2":  {},
		"flowcontrol.apiserver.k8s.io/v1beta3":  {VersionRange: versionutils.VersionRange{AddedInVersion: "1.26"}},
		"internal.apiserver.k8s.io/v1alpha1":    {},
		"networking.k8s.io/v1":                  {Required: true, RequiredForWorkerless: true},
		"networking.k8s.io/v1alpha1":            {},
		"networking.k8s.io/v1beta1":             {},
		"node.k8s.io/v1":                        {},
		"node.k8s.io/v1alpha1":                  {},
		"node.k8s.io/v1beta1":                   {},
		"policy/v1":                             {},
		"policy/v1beta1":                        {},
		"rbac.authorization.k8s.io/v1":          {Required: true, RequiredForWorkerless: true},
		"rbac.authorization.k8s.io/v1alpha1":    {},
		"rbac.authorization.k8s.io/v1beta1":     {},
		"resource.k8s.io/v1alpha1":              {VersionRange: versionutils.VersionRange{AddedInVersion: "1.26", RemovedInVersion: "1.27"}},
		"resource.k8s.io/v1alpha2":              {VersionRange: versionutils.VersionRange{AddedInVersion: "1.27", RemovedInVersion: "1.31"}},
		"resource.k8s.io/v1alpha3":              {VersionRange: versionutils.VersionRange{AddedInVersion: "1.31"}},
		"resource.k8s.io/v1beta1":               {VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
		"scheduling.k8s.io/v1":                  {},
		"scheduling.k8s.io/v1alpha1":            {},
		"scheduling.k8s.io/v1beta1":             {},
		"storage.k8s.io/v1":                     {Required: true},
		"storage.k8s.io/v1alpha1":               {},
		"storage.k8s.io/v1beta1":                {},
		"storagemigration.k8s.io/v1alpha1":      {VersionRange: versionutils.VersionRange{AddedInVersion: "1.30"}},
		"v1":                                    {Required: true, RequiredForWorkerless: true},
	}

	// Keep the list ordered alphabetically.
	apiGVRVersionRanges = map[string]*APIVersionRange{
		"admissionregistration.k8s.io/v1/mutatingwebhookconfigurations":           {Required: true},
		"admissionregistration.k8s.io/v1/validatingadmissionpolicies":             {VersionRange: versionutils.VersionRange{AddedInVersion: "1.30"}},
		"admissionregistration.k8s.io/v1/validatingadmissionpolicybindings":       {VersionRange: versionutils.VersionRange{AddedInVersion: "1.30"}},
		"admissionregistration.k8s.io/v1/validatingwebhookconfigurations":         {Required: true},
		"admissionregistration.k8s.io/v1alpha1/mutatingadmissionpolicies":         {VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
		"admissionregistration.k8s.io/v1alpha1/mutatingadmissionpolicybindings":   {VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
		"admissionregistration.k8s.io/v1alpha1/validatingadmissionpolicies":       {VersionRange: versionutils.VersionRange{AddedInVersion: "1.26"}},
		"admissionregistration.k8s.io/v1alpha1/validatingadmissionpolicybindings": {VersionRange: versionutils.VersionRange{AddedInVersion: "1.26"}},
		"admissionregistration.k8s.io/v1beta1/mutatingwebhookconfigurations":      {},
		"admissionregistration.k8s.io/v1beta1/validatingadmissionpolicies":        {VersionRange: versionutils.VersionRange{AddedInVersion: "1.28"}},
		"admissionregistration.k8s.io/v1beta1/validatingadmissionpolicybindings":  {VersionRange: versionutils.VersionRange{AddedInVersion: "1.28"}},
		"admissionregistration.k8s.io/v1beta1/validatingwebhookconfigurations":    {},
		"apps/v1/controllerrevisions":                                             {},
		"apps/v1/daemonsets":                                                      {},
		"apps/v1/deployments":                                                     {},
		"apps/v1/replicasets":                                                     {},
		"apps/v1/statefulsets":                                                    {},
		"apps/v1beta1/controllerrevisions":                                        {},
		"apps/v1beta1/deployments":                                                {},
		"apps/v1beta1/statefulsets":                                               {},
		"apps/v1beta2/controllerrevisions":                                        {},
		"apps/v1beta2/daemonsets":                                                 {},
		"apps/v1beta2/deployments":                                                {},
		"apps/v1beta2/replicasets":                                                {},
		"apps/v1beta2/statefulsets":                                               {},
		"autoscaling/v1/horizontalpodautoscalers":                                 {},
		"autoscaling/v2/horizontalpodautoscalers":                                 {Required: true},
		"autoscaling/v2beta1/horizontalpodautoscalers":                            {},
		"autoscaling/v2beta2/horizontalpodautoscalers":                            {},
		"batch/v1/cronjobs":                                                       {},
		"batch/v1/jobs":                                                           {},
		"batch/v1beta1/cronjobs":                                                  {},
		"certificates.k8s.io/v1/certificatesigningrequests":                       {Required: true},
		"certificates.k8s.io/v1alpha1/clustertrustbundles":                        {VersionRange: versionutils.VersionRange{AddedInVersion: "1.27"}},
		"certificates.k8s.io/v1beta1/certificatesigningrequests":                  {},
		"coordination.k8s.io/v1/leases":                                           {Required: true},
		"coordination.k8s.io/v1alpha1/leasecandidates":                            {VersionRange: versionutils.VersionRange{AddedInVersion: "1.31", RemovedInVersion: "1.32"}},
		"coordination.k8s.io/v1alpha2/leasecandidates":                            {VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
		"coordination.k8s.io/v1beta1/leases":                                      {},
		"discovery.k8s.io/v1/endpointslices":                                      {Required: true, RequiredForWorkerless: true},
		"discovery.k8s.io/v1beta1/endpointslices":                                 {},
		"events.k8s.io/v1/events":                                                 {Required: true, RequiredForWorkerless: true},
		"events.k8s.io/v1beta1/events":                                            {},
		"extensions/v1beta1/daemonsets":                                           {},
		"extensions/v1beta1/deployments":                                          {},
		"extensions/v1beta1/ingresses":                                            {},
		"extensions/v1beta1/networkpolicies":                                      {},
		"extensions/v1beta1/podsecuritypolicies":                                  {VersionRange: versionutils.VersionRange{RemovedInVersion: "1.27"}},
		"extensions/v1beta1/replicasets":                                          {},
		"flowcontrol.apiserver.k8s.io/v1/flowschemas":                             {VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
		"flowcontrol.apiserver.k8s.io/v1/prioritylevelconfigurations":             {VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
		"flowcontrol.apiserver.k8s.io/v1alpha1/flowschemas":                       {VersionRange: versionutils.VersionRange{RemovedInVersion: "1.29"}},
		"flowcontrol.apiserver.k8s.io/v1alpha1/prioritylevelconfigurations":       {VersionRange: versionutils.VersionRange{RemovedInVersion: "1.29"}},
		"flowcontrol.apiserver.k8s.io/v1beta1/flowschemas":                        {},
		"flowcontrol.apiserver.k8s.io/v1beta1/prioritylevelconfigurations":        {},
		"flowcontrol.apiserver.k8s.io/v1beta2/flowschemas":                        {},
		"flowcontrol.apiserver.k8s.io/v1beta2/prioritylevelconfigurations":        {},
		"flowcontrol.apiserver.k8s.io/v1beta3/flowschemas":                        {VersionRange: versionutils.VersionRange{AddedInVersion: "1.26"}},
		"flowcontrol.apiserver.k8s.io/v1beta3/prioritylevelconfigurations":        {VersionRange: versionutils.VersionRange{AddedInVersion: "1.26"}},
		"internal.apiserver.k8s.io/v1alpha1/storageversions":                      {},
		"networking.k8s.io/v1/ingressclasses":                                     {},
		"networking.k8s.io/v1/ingresses":                                          {},
		"networking.k8s.io/v1/networkpolicies":                                    {},
		"networking.k8s.io/v1alpha1/clustercidrs":                                 {VersionRange: versionutils.VersionRange{RemovedInVersion: "1.29"}},
		"networking.k8s.io/v1alpha1/ipaddresses":                                  {VersionRange: versionutils.VersionRange{AddedInVersion: "1.27"}},
		"networking.k8s.io/v1alpha1/servicecidrs":                                 {VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
		"networking.k8s.io/v1beta1/ingressclasses":                                {},
		"networking.k8s.io/v1beta1/ingresses":                                     {},
		"networking.k8s.io/v1beta1/ipaddresses":                                   {VersionRange: versionutils.VersionRange{RemovedInVersion: "1.31"}},
		"networking.k8s.io/v1beta1/servicecidrs":                                  {VersionRange: versionutils.VersionRange{RemovedInVersion: "1.31"}},
		"node.k8s.io/v1/runtimeclasses":                                           {Required: true},
		"node.k8s.io/v1alpha1/runtimeclasses":                                     {},
		"node.k8s.io/v1beta1/runtimeclasses":                                      {},
		"policy/v1/poddisruptionbudgets":                                          {Required: true},
		"policy/v1beta1/poddisruptionbudgets":                                     {},
		"policy/v1beta1/podsecuritypolicies":                                      {VersionRange: versionutils.VersionRange{RemovedInVersion: "1.29"}},
		"rbac.authorization.k8s.io/v1/clusterrolebindings":                        {},
		"rbac.authorization.k8s.io/v1/clusterroles":                               {},
		"rbac.authorization.k8s.io/v1/rolebindings":                               {},
		"rbac.authorization.k8s.io/v1/roles":                                      {},
		"rbac.authorization.k8s.io/v1alpha1/clusterrolebindings":                  {},
		"rbac.authorization.k8s.io/v1alpha1/clusterroles":                         {},
		"rbac.authorization.k8s.io/v1alpha1/rolebindings":                         {},
		"rbac.authorization.k8s.io/v1alpha1/roles":                                {},
		"rbac.authorization.k8s.io/v1beta1/clusterrolebindings":                   {},
		"rbac.authorization.k8s.io/v1beta1/clusterroles":                          {},
		"rbac.authorization.k8s.io/v1beta1/rolebindings":                          {},
		"rbac.authorization.k8s.io/v1beta1/roles":                                 {},
		"resource.k8s.io/v1alpha1/podschedulings":                                 {VersionRange: versionutils.VersionRange{AddedInVersion: "1.26", RemovedInVersion: "1.27"}},
		"resource.k8s.io/v1alpha1/resourceclaims":                                 {VersionRange: versionutils.VersionRange{AddedInVersion: "1.26", RemovedInVersion: "1.27"}},
		"resource.k8s.io/v1alpha1/resourceclaimtemplates":                         {VersionRange: versionutils.VersionRange{AddedInVersion: "1.26", RemovedInVersion: "1.27"}},
		"resource.k8s.io/v1alpha1/resourceclasses":                                {VersionRange: versionutils.VersionRange{AddedInVersion: "1.26", RemovedInVersion: "1.27"}},
		"resource.k8s.io/v1alpha2/podschedulingcontexts":                          {VersionRange: versionutils.VersionRange{AddedInVersion: "1.27", RemovedInVersion: "1.31"}},
		"resource.k8s.io/v1alpha2/resourceclaimparameters":                        {VersionRange: versionutils.VersionRange{AddedInVersion: "1.30", RemovedInVersion: "1.31"}},
		"resource.k8s.io/v1alpha2/resourceclaims":                                 {VersionRange: versionutils.VersionRange{AddedInVersion: "1.27", RemovedInVersion: "1.31"}},
		"resource.k8s.io/v1alpha2/resourceclaimtemplates":                         {VersionRange: versionutils.VersionRange{AddedInVersion: "1.27", RemovedInVersion: "1.31"}},
		"resource.k8s.io/v1alpha2/resourceclasses":                                {VersionRange: versionutils.VersionRange{AddedInVersion: "1.27", RemovedInVersion: "1.31"}},
		"resource.k8s.io/v1alpha2/resourceclassparameters":                        {VersionRange: versionutils.VersionRange{AddedInVersion: "1.30", RemovedInVersion: "1.31"}},
		"resource.k8s.io/v1alpha2/resourceslices":                                 {VersionRange: versionutils.VersionRange{AddedInVersion: "1.30", RemovedInVersion: "1.31"}},
		"resource.k8s.io/v1alpha3/deviceclasses":                                  {VersionRange: versionutils.VersionRange{AddedInVersion: "1.31"}},
		"resource.k8s.io/v1alpha3/podschedulingcontexts":                          {VersionRange: versionutils.VersionRange{AddedInVersion: "1.31", RemovedInVersion: "1.32"}},
		"resource.k8s.io/v1alpha3/resourceclaims":                                 {VersionRange: versionutils.VersionRange{AddedInVersion: "1.31"}},
		"resource.k8s.io/v1alpha3/resourceclaimtemplates":                         {VersionRange: versionutils.VersionRange{AddedInVersion: "1.31"}},
		"resource.k8s.io/v1alpha3/resourceslices":                                 {VersionRange: versionutils.VersionRange{AddedInVersion: "1.31"}},
		"resource.k8s.io/v1beta1/deviceclasses":                                   {VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
		"resource.k8s.io/v1beta1/resourceclaims":                                  {VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
		"resource.k8s.io/v1beta1/resourceclaimtemplates":                          {VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
		"resource.k8s.io/v1beta1/resourceslices":                                  {VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
		"scheduling.k8s.io/v1/priorityclasses":                                    {Required: true},
		"scheduling.k8s.io/v1alpha1/priorityclasses":                              {},
		"scheduling.k8s.io/v1beta1/priorityclasses":                               {},
		"storage.k8s.io/v1/csidrivers":                                            {},
		"storage.k8s.io/v1/csinodes":                                              {},
		"storage.k8s.io/v1/csistoragecapacities":                                  {},
		"storage.k8s.io/v1/storageclasses":                                        {},
		"storage.k8s.io/v1/volumeattachments":                                     {},
		"storage.k8s.io/v1alpha1/csistoragecapacities":                            {},
		"storage.k8s.io/v1alpha1/volumeattachments":                               {},
		"storage.k8s.io/v1alpha1/volumeattributesclasses":                         {VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
		"storage.k8s.io/v1beta1/csidrivers":                                       {},
		"storage.k8s.io/v1beta1/csinodes":                                         {},
		"storage.k8s.io/v1beta1/csistoragecapacities":                             {},
		"storage.k8s.io/v1beta1/storageclasses":                                   {},
		"storage.k8s.io/v1beta1/volumeattachments":                                {},
		"storage.k8s.io/v1beta1/volumeattributesclasses":                          {VersionRange: versionutils.VersionRange{AddedInVersion: "1.31"}},
		"storagemigration.k8s.io/v1alpha1/storageversionmigrations":               {VersionRange: versionutils.VersionRange{AddedInVersion: "1.30"}},
		"v1/componentstatuses":                                                    {},
		"v1/configmaps":                                                           {},
		"v1/endpoints":                                                            {},
		"v1/events":                                                               {},
		"v1/limitranges":                                                          {},
		"v1/namespaces":                                                           {},
		"v1/nodes":                                                                {},
		"v1/persistentvolumeclaims":                                               {},
		"v1/persistentvolumes":                                                    {},
		"v1/pods":                                                                 {},
		"v1/podtemplates":                                                         {},
		"v1/replicationcontrollers":                                               {},
		"v1/resourcequotas":                                                       {},
		"v1/secrets":                                                              {},
		"v1/serviceaccounts":                                                      {},
		"v1/services":                                                             {},
	}
)

// IsAPISupported returns true if the given API is supported for the given Kubernetes version.
// An API is only supported if it's a known API and its version range contains the given Kubernetes version.
func IsAPISupported(api, version string) (bool, string, error) {
	var versionRange *APIVersionRange

	apiGroupVersion, apiGVR, err := SplitAPI(api)
	if err != nil {
		return false, "", err
	}

	if apiGVR != "" {
		vr, ok := apiGVRVersionRanges[apiGVR]
		if !ok {
			return false, "", fmt.Errorf("unknown API group version resource %q", apiGVR)
		}

		versionRange = vr
	} else {
		vr, ok := apiGroupVersionRanges[apiGroupVersion]
		if !ok {
			return false, "", fmt.Errorf("unknown API group version %q", apiGroupVersion)
		}

		versionRange = vr
	}

	contains, err := versionRange.Contains(version)
	return contains, versionRange.SupportedVersionRange(), err
}

// APIVersionRange represents a version range of type [AddedInVersion, RemovedInVersion).
// Required defines whether this APIVersion is required for a Shoot with workers.
// RequiredForWorkerless defines whether this APIVersion is required for Workerless Shoots.
// If an API is required for both Shoot types, then both booleans need to be set to true.
type APIVersionRange struct {
	Required              bool
	RequiredForWorkerless bool
	versionutils.VersionRange
}

// SupportedVersionRange returns the supported version range for the given API.
func (r *APIVersionRange) SupportedVersionRange() string {
	switch {
	case r.AddedInVersion != "" && r.RemovedInVersion == "":
		return "versions >= " + r.AddedInVersion
	case r.AddedInVersion == "" && r.RemovedInVersion != "":
		return "versions < " + r.RemovedInVersion
	case r.AddedInVersion != "" && r.RemovedInVersion != "":
		return fmt.Sprintf("versions >= %s, < %s", r.AddedInVersion, r.RemovedInVersion)
	default:
		return "all kubernetes versions"
	}
}

// ValidateAPIGroupVersions validates the given Kubernetes APIs against the given Kubernetes version.
func ValidateAPIGroupVersions(runtimeConfig map[string]bool, version string, workerless bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for api, enabled := range runtimeConfig {
		supported, supportedVersionRange, err := IsAPISupported(api, version)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Key(api), api, err.Error()))
		} else if !supported {
			allErrs = append(allErrs, field.Forbidden(fldPath.Key(api), fmt.Sprintf("api %q is not supported in Kubernetes version %s, only supported in %s", api, version, supportedVersionRange)))
		} else if !enabled {
			apiGroupVersion, apiGVR, _ := SplitAPI(api)

			if apiGVR != "" {
				if err := checkIfRequired(apiGVRVersionRanges[apiGVR], apiGVR, workerless, fldPath.Key(api)); err != nil {
					allErrs = append(allErrs, err)
				} else {
					// Check if the whole API group version is marked as required
					if err := checkIfRequired(apiGroupVersionRanges[apiGroupVersion], apiGroupVersion, workerless, fldPath.Key(api)); err != nil {
						allErrs = append(allErrs, err)
					}
				}
			} else {
				if err := checkIfRequired(apiGroupVersionRanges[apiGroupVersion], apiGroupVersion, workerless, fldPath.Key(api)); err != nil {
					allErrs = append(allErrs, err)
				} else {
					// Check if any of the resources in the API group version are required
					for a, vr := range apiGVRVersionRanges {
						if strings.HasPrefix(a, apiGroupVersion+"/") {
							if err := checkIfRequired(vr, a, workerless, fldPath.Key(api)); err != nil {
								allErrs = append(allErrs, err)
							}
						}
					}
				}
			}
		}
	}

	return allErrs
}

func checkIfRequired(vr *APIVersionRange, api string, workerless bool, fldPath *field.Path) *field.Error {
	if workerless {
		if vr.RequiredForWorkerless {
			return field.Forbidden(fldPath, fmt.Sprintf("api %q cannot be disabled for workerless clusters", api))
		}
	} else if vr.Required {
		return field.Forbidden(fldPath, fmt.Sprintf("api %q cannot be disabled", api))
	}
	return nil
}

// SplitAPI splits the given api into API GroupVersion and API GroupVersionResource.
func SplitAPI(api string) (string, string, error) {
	var (
		apiGroupVersion string
		apiGVR          string
		apis            = strings.Split(api, "/")
	)

	switch len(apis) {
	case 1:
		apiGroupVersion = apis[0]
	case 2:
		apiGroupVersion = strings.Join(apis[:2], "/")
		if apis[0] == "v1" {
			apiGroupVersion = "v1"
			apiGVR = "v1/" + apis[1]
		}
	case 3:
		apiGroupVersion = strings.Join(apis[:2], "/")
		apiGVR = api
	default:
		return "", "", fmt.Errorf("invalid API Group format %q", api)
	}

	return apiGroupVersion, apiGVR, nil
}
