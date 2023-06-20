// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package features

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/validation/field"

	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

// featureGateVersionRanges contains the version ranges for all Kubernetes feature gates.
// Extracted from https://raw.githubusercontent.com/kubernetes/kubernetes/release-${version}/pkg/features/kube_features.go.
// To maintain this list for each new Kubernetes version:
//   - Run hack/compare-k8s-feature-gates.sh <old-version> <new-version> (e.g. 'hack/compare-k8s-feature-gates.sh 1.22 1.23').
//     It will present 3 lists of feature gates: those added and those removed in <new-version> compared to <old-version> and
//     feature gates that got locked to default in `<new-version>`.
//   - Add all added feature gates to the map with <new-version> as AddedInVersion and no RemovedInVersion.
//   - For any removed feature gates, add <new-version> as RemovedInVersion to the already existing feature gate in the map.
//   - For feature gates locked to default, add `<new-version>` as LockedToDefaultInVersion to the already existing feature gate in the map.
var featureGateVersionRanges = map[string]*FeatureGateVersionRange{
	// These are special feature gates to toggle all alpha or beta feature gates on and off.
	// They were introduced in version 1.17 (although they are absent from the corresponding kube_features.go file).
	"AllAlpha": {AddedInVersion: "1.17"},
	"AllBeta":  {AddedInVersion: "1.17"},

	"AdmissionWebhookMatchConditions":                {AddedInVersion: "1.27"},
	"APIListChunking":                                {},
	"APIPriorityAndFairness":                         {},
	"APIResponseCompression":                         {},
	"APISelfSubjectReview":                           {AddedInVersion: "1.26"},
	"APIServerIdentity":                              {},
	"APIServerTracing":                               {},
	"AdvancedAuditing":                               {LockedToDefaultInVersion: "1.27"},
	"AggregatedDiscoveryEndpoint":                    {AddedInVersion: "1.26"},
	"AllowInsecureBackendProxy":                      {RemovedInVersion: "1.23"},
	"AnyVolumeDataSource":                            {},
	"AppArmor":                                       {},
	"BoundServiceAccountTokenVolume":                 {RemovedInVersion: "1.23"},
	"CloudControllerManagerWebhook":                  {AddedInVersion: "1.27"},
	"CloudDualStackNodeIPs":                          {AddedInVersion: "1.27"},
	"ClusterTrustBundle":                             {AddedInVersion: "1.27"},
	"CPUManager":                                     {LockedToDefaultInVersion: "1.26"},
	"CPUManagerPolicyAlphaOptions":                   {AddedInVersion: "1.23"},
	"CPUManagerPolicyBetaOptions":                    {AddedInVersion: "1.23"},
	"CPUManagerPolicyOptions":                        {},
	"CronJobTimeZone":                                {AddedInVersion: "1.24", LockedToDefaultInVersion: "1.27"},
	"CSIInlineVolume":                                {Default: true, LockedToDefaultInVersion: "1.25", RemovedInVersion: "1.27"},
	"CSIMigration":                                   {Default: true, LockedToDefaultInVersion: "1.25", RemovedInVersion: "1.27"},
	"CSIMigrationAWS":                                {Default: true, LockedToDefaultInVersion: "1.25", RemovedInVersion: "1.27"},
	"CSIMigrationAzureDisk":                          {Default: true, LockedToDefaultInVersion: "1.25", RemovedInVersion: "1.27"},
	"CSIMigrationAzureFile":                          {LockedToDefaultInVersion: "1.27"},
	"CSIMigrationGCE":                                {Default: true, LockedToDefaultInVersion: "1.25"},
	"CSIMigrationOpenStack":                          {Default: true, LockedToDefaultInVersion: "1.24", RemovedInVersion: "1.26"},
	"CSIMigrationPortworx":                           {AddedInVersion: "1.23"},
	"CSIMigrationRBD":                                {AddedInVersion: "1.24"},
	"CSIMigrationvSphere":                            {LockedToDefaultInVersion: "1.27"},
	"CSINodeExpandSecret":                            {AddedInVersion: "1.25"},
	"CSIServiceAccountToken":                         {Default: true, LockedToDefaultInVersion: "1.22", RemovedInVersion: "1.25"},
	"CSIStorageCapacity":                             {Default: true, LockedToDefaultInVersion: "1.24"},
	"CSIVolumeFSGroupPolicy":                         {Default: true, LockedToDefaultInVersion: "1.23", RemovedInVersion: "1.25"},
	"CSIVolumeHealth":                                {},
	"CSRDuration":                                    {Default: true, LockedToDefaultInVersion: "1.24", RemovedInVersion: "1.26"},
	"ConfigurableFSGroupPolicy":                      {Default: true, LockedToDefaultInVersion: "1.23", RemovedInVersion: "1.25"},
	"ConsistentHTTPGetHandlers":                      {AddedInVersion: "1.26"},
	"ContainerCheckpoint":                            {AddedInVersion: "1.25"},
	"ControllerManagerLeaderMigration":               {Default: true, LockedToDefaultInVersion: "1.24", RemovedInVersion: "1.27"}, // Missing from docu?
	"CronJobControllerV2":                            {RemovedInVersion: "1.23"},
	"CrossNamespaceVolumeDataSource":                 {AddedInVersion: "1.26"},
	"CustomCPUCFSQuotaPeriod":                        {},
	"CustomResourceValidationExpressions":            {AddedInVersion: "1.23"},
	"DaemonSetUpdateSurge":                           {Default: true, LockedToDefaultInVersion: "1.25", RemovedInVersion: "1.27"}, // Missing from docu?
	"DefaultPodTopologySpread":                       {Default: true, LockedToDefaultInVersion: "1.24", RemovedInVersion: "1.26"},
	"DelegateFSGroupToCSIDriver":                     {LockedToDefaultInVersion: "1.26"},
	"DevicePlugins":                                  {LockedToDefaultInVersion: "1.26"},
	"DisableAcceleratorUsageMetrics":                 {Default: true, LockedToDefaultInVersion: "1.25"},
	"DisableCloudProviders":                          {},
	"DisableKubeletCloudCredentialProviders":         {AddedInVersion: "1.23"},
	"DownwardAPIHugePages":                           {LockedToDefaultInVersion: "1.27"},
	"DryRun":                                         {LockedToDefaultInVersion: "1.26"},
	"DynamicKubeletConfig":                           {RemovedInVersion: "1.26"},
	"DynamicResourceAllocation":                      {AddedInVersion: "1.26"},
	"EfficientWatchResumption":                       {Default: true, LockedToDefaultInVersion: "1.24"},
	"ElasticIndexedJob":                              {AddedInVersion: "1.27"},
	"EndpointSlice":                                  {Default: true, LockedToDefaultInVersion: "1.21", RemovedInVersion: "1.25"},
	"EndpointSliceNodeName":                          {Default: true, LockedToDefaultInVersion: "1.21", RemovedInVersion: "1.25"},
	"EndpointSliceProxying":                          {Default: true, LockedToDefaultInVersion: "1.22", RemovedInVersion: "1.25"},
	"EndpointSliceTerminatingCondition":              {LockedToDefaultInVersion: "1.26"},
	"EphemeralContainers":                            {Default: true, LockedToDefaultInVersion: "1.25", RemovedInVersion: "1.27"},
	"EventedPLEG":                                    {AddedInVersion: "1.26"},
	"ExecProbeTimeout":                               {},
	"ExpandCSIVolumes":                               {RemovedInVersion: "1.27"},
	"ExpandedDNSConfig":                              {},
	"ExpandInUsePersistentVolumes":                   {RemovedInVersion: "1.27"},
	"ExpandPersistentVolumes":                        {RemovedInVersion: "1.27"},
	"ExperimentalHostUserNamespaceDefaulting":        {},
	"GRPCContainerProbe":                             {AddedInVersion: "1.23", LockedToDefaultInVersion: "1.27"},
	"GenericEphemeralVolume":                         {Default: true, LockedToDefaultInVersion: "1.23", RemovedInVersion: "1.25"},
	"GracefulNodeShutdown":                           {},
	"GracefulNodeShutdownBasedOnPodPriority":         {AddedInVersion: "1.23"},
	"HonorPVReclaimPolicy":                           {AddedInVersion: "1.23"},
	"HPAContainerMetrics":                            {},
	"HPAScaleToZero":                                 {},
	"HugePageStorageMediumSize":                      {Default: true, LockedToDefaultInVersion: "1.22", RemovedInVersion: "1.24"},
	"IPv6DualStack":                                  {Default: true, LockedToDefaultInVersion: "1.23", RemovedInVersion: "1.27"},
	"IPTablesOwnershipCleanup":                       {AddedInVersion: "1.25"},
	"IdentifyPodOS":                                  {Default: true, AddedInVersion: "1.23", LockedToDefaultInVersion: "1.25", RemovedInVersion: "1.27"},
	"ImmutableEphemeralVolumes":                      {Default: true, LockedToDefaultInVersion: "1.21", RemovedInVersion: "1.24"},
	"InPlacePodVerticalScaling":                      {AddedInVersion: "1.27"},
	"InTreePluginAWSUnregister":                      {}, // Missing from docu?
	"InTreePluginAzureDiskUnregister":                {}, // Missing from docu?
	"InTreePluginAzureFileUnregister":                {}, // Missing from docu?
	"InTreePluginGCEUnregister":                      {}, // Missing from docu?
	"InTreePluginOpenStackUnregister":                {}, // Missing from docu?
	"InTreePluginPortworxUnregister":                 {AddedInVersion: "1.23"},
	"InTreePluginRBDUnregister":                      {AddedInVersion: "1.23"},
	"InTreePluginvSphereUnregister":                  {}, // Missing from docu?
	"IndexedJob":                                     {Default: true, LockedToDefaultInVersion: "1.24", RemovedInVersion: "1.26"},
	"IngressClassNamespacedParams":                   {Default: true, LockedToDefaultInVersion: "1.23", RemovedInVersion: "1.25"},
	"JobMutableNodeSchedulingDirectives":             {AddedInVersion: "1.23", LockedToDefaultInVersion: "1.27"},
	"JobPodFailurePolicy":                            {AddedInVersion: "1.25"},
	"JobReadyPods":                                   {AddedInVersion: "1.23"},
	"JobTrackingWithFinalizers":                      {LockedToDefaultInVersion: "1.26"},
	"KMSv2":                                          {AddedInVersion: "1.25"},
	"KubeletCredentialProviders":                     {LockedToDefaultInVersion: "1.26"},
	"KubeletInUserNamespace":                         {},
	"KubeletPodResources":                            {},
	"KubeletPodResourcesDynamicResources":            {AddedInVersion: "1.27"},
	"KubeletPodResourcesGet":                         {AddedInVersion: "1.27"},
	"KubeletPodResourcesGetAllocatable":              {},
	"KubeletTracing":                                 {AddedInVersion: "1.25"},
	"LegacyServiceAccountTokenNoAutoGeneration":      {AddedInVersion: "1.24", LockedToDefaultInVersion: "1.27"},
	"LegacyServiceAccountTokenTracking":              {AddedInVersion: "1.26"},
	"LocalStorageCapacityIsolation":                  {Default: true, LockedToDefaultInVersion: "1.25", RemovedInVersion: "1.27"},
	"LocalStorageCapacityIsolationFSQuotaMonitoring": {},
	"LogarithmicScaleDown":                           {},
	"MatchLabelKeysInPodTopologySpread":              {AddedInVersion: "1.25"},
	"MaxUnavailableStatefulSet":                      {AddedInVersion: "1.24"},
	"MemoryManager":                                  {}, // Missing from docu?
	"MemoryQoS":                                      {},
	"MigrationRBD":                                   {AddedInVersion: "1.23", RemovedInVersion: "1.24"},
	"MinDomainsInPodTopologySpread":                  {AddedInVersion: "1.24"},
	"MinimizeIPTablesRestore":                        {AddedInVersion: "1.26"},
	"MixedProtocolLBService":                         {LockedToDefaultInVersion: "1.26"},
	"MultiCIDRRangeAllocator":                        {AddedInVersion: "1.25"},
	"MultiCIDRServiceAllocator":                      {AddedInVersion: "1.27"},
	"NamespaceDefaultLabelName":                      {Default: true, LockedToDefaultInVersion: "1.22", RemovedInVersion: "1.24"},
	"NetworkPolicyEndPort":                           {Default: true, LockedToDefaultInVersion: "1.25", RemovedInVersion: "1.27"},
	"NetworkPolicyStatus":                            {AddedInVersion: "1.24"},
	"NewVolumeManagerReconstruction":                 {AddedInVersion: "1.27"},
	"NodeInclusionPolicyInPodTopologySpread":         {AddedInVersion: "1.25"},
	"NodeLogQuery":                                   {AddedInVersion: "1.27"},
	"NodeLease":                                      {RemovedInVersion: "1.23"},
	"NodeOutOfServiceVolumeDetach":                   {AddedInVersion: "1.24"},
	"NonPreemptingPriority":                          {Default: true, LockedToDefaultInVersion: "1.24", RemovedInVersion: "1.26"},
	"NodeSwap":                                       {},
	"OpenAPIEnums":                                   {AddedInVersion: "1.23"},
	"OpenAPIV3":                                      {AddedInVersion: "1.23", LockedToDefaultInVersion: "1.27"},
	"PDBUnhealthyPodEvictionPolicy":                  {AddedInVersion: "1.26"},
	"PodAndContainerStatsFromCRI":                    {AddedInVersion: "1.23"},
	"PodAffinityNamespaceSelector":                   {Default: true, LockedToDefaultInVersion: "1.24", RemovedInVersion: "1.26"},
	"PodDeletionCost":                                {},
	"PodDisruptionBudget":                            {Default: true, LockedToDefaultInVersion: "1.21", RemovedInVersion: "1.25"}, // Docu says 1.3?
	"PodDisruptionConditions":                        {AddedInVersion: "1.25"},
	"PodHasNetworkCondition":                         {AddedInVersion: "1.25"},
	"PodOverhead":                                    {Default: true, LockedToDefaultInVersion: "1.24", RemovedInVersion: "1.26"},
	"PodSchedulingReadiness":                         {AddedInVersion: "1.26"},
	"PodSecurity":                                    {Default: true, LockedToDefaultInVersion: "1.25"},
	"PreferNominatedNode":                            {Default: true, LockedToDefaultInVersion: "1.24", RemovedInVersion: "1.26"}, // Missing from docu?
	"ProbeTerminationGracePeriod":                    {},
	"ProcMountType":                                  {},
	"ProxyTerminatingEndpoints":                      {},
	"QOSReserved":                                    {},
	"ReadWriteOncePod":                               {},
	"RecoverVolumeExpansionFailure":                  {AddedInVersion: "1.23"},
	"RemainingItemCount":                             {},
	"RemoveSelfLink":                                 {Default: true, LockedToDefaultInVersion: "1.24"},
	"RetroactiveDefaultStorageClass":                 {AddedInVersion: "1.25"},
	"RotateKubeletServerCertificate":                 {},
	"RuntimeClass":                                   {Default: true, LockedToDefaultInVersion: "1.20", RemovedInVersion: "1.24"},
	"SeccompDefault":                                 {LockedToDefaultInVersion: "1.27"},
	"SecurityContextDeny":                            {AddedInVersion: "1.27"},
	"SelectorIndex":                                  {Default: true, LockedToDefaultInVersion: "1.20", RemovedInVersion: "1.25"}, // Missing from docu?
	"SELinuxMountReadWriteOncePod":                   {AddedInVersion: "1.25"},
	"ServerSideApply":                                {LockedToDefaultInVersion: "1.26"},
	"ServerSideFieldValidation":                      {AddedInVersion: "1.23", LockedToDefaultInVersion: "1.27"},
	"ServiceAccountIssuerDiscovery":                  {RemovedInVersion: "1.23"},
	"ServiceInternalTrafficPolicy":                   {LockedToDefaultInVersion: "1.26"},
	"ServiceIPStaticSubrange":                        {AddedInVersion: "1.24", LockedToDefaultInVersion: "1.26"},
	"ServiceLBNodePortControl":                       {Default: true, LockedToDefaultInVersion: "1.24", RemovedInVersion: "1.26"},
	"ServiceLoadBalancerClass":                       {Default: true, LockedToDefaultInVersion: "1.24", RemovedInVersion: "1.26"},
	"ServiceNodePortStaticSubrange":                  {AddedInVersion: "1.27"},
	"SetHostnameAsFQDN":                              {Default: true, LockedToDefaultInVersion: "1.22", RemovedInVersion: "1.24"},
	"SizeMemoryBackedVolumes":                        {},
	"StableLoadBalancerNodeSet":                      {AddedInVersion: "1.27"},
	"StartupProbe":                                   {RemovedInVersion: "1.23"},
	"StatefulSetAutoDeletePVC":                       {AddedInVersion: "1.23"},
	"StatefulSetMinReadySeconds":                     {Default: true, LockedToDefaultInVersion: "1.25", RemovedInVersion: "1.27"},
	"StatefulSetStartOrdinal":                        {AddedInVersion: "1.26"},
	"StorageObjectInUseProtection":                   {Default: true, LockedToDefaultInVersion: "1.23", RemovedInVersion: "1.25"},
	"StorageVersionAPI":                              {},
	"StorageVersionHash":                             {},
	"StreamingProxyRedirects":                        {RemovedInVersion: "1.24"},
	"SupportNodePidsLimit":                           {RemovedInVersion: "1.23"},
	"SupportPodPidsLimit":                            {RemovedInVersion: "1.23"},
	"SuspendJob":                                     {Default: true, LockedToDefaultInVersion: "1.24", RemovedInVersion: "1.26"},
	"Sysctls":                                        {RemovedInVersion: "1.23"},
	"TTLAfterFinished":                               {Default: true, LockedToDefaultInVersion: "1.23", RemovedInVersion: "1.25"},
	"TopologyAwareHints":                             {},
	"TopologyManager":                                {LockedToDefaultInVersion: "1.27"},
	"TopologyManagerPolicyAlphaOptions":              {AddedInVersion: "1.26"},
	"TopologyManagerPolicyBetaOptions":               {AddedInVersion: "1.26"},
	"TopologyManagerPolicyOptions":                   {AddedInVersion: "1.26"},
	"UserNamespacesStatelessPodsSupport":             {AddedInVersion: "1.25"},
	"ValidateProxyRedirects":                         {RemovedInVersion: "1.24"},
	"ValidatingAdmissionPolicy":                      {AddedInVersion: "1.26"},
	"VolumeCapacityPriority":                         {},
	"VolumeSubpath":                                  {RemovedInVersion: "1.25"},
	"WarningHeaders":                                 {Default: true, LockedToDefaultInVersion: "1.22", RemovedInVersion: "1.24"},
	"WatchBookmark":                                  {Default: true, LockedToDefaultInVersion: "1.17"},
	"WatchList":                                      {AddedInVersion: "1.27"},
	"WinDSR":                                         {},
	"WinOverlay":                                     {},
	"WindowsEndpointSliceProxying":                   {Default: true, LockedToDefaultInVersion: "1.22", RemovedInVersion: "1.25"},
	"WindowsHostNetwork":                             {AddedInVersion: "1.26"},
	"WindowsHostProcessContainers":                   {LockedToDefaultInVersion: "1.26"},
}

// IsFeatureGateSupported returns true if the given feature gate is supported for the given Kubernetes version.
// A feature gate is only supported if it's a known feature gate and its version range contains the given Kubernetes version.
func IsFeatureGateSupported(featureGate, version string) (bool, error) {
	vr := featureGateVersionRanges[featureGate]
	if vr == nil {
		return false, fmt.Errorf("unknown feature gate %s", featureGate)
	}

	return vr.Contains(version)
}

// FeatureGateVersionRange represents a version range of type [AddedInVersion, RemovedInVersion).
type FeatureGateVersionRange struct {
	Default                  bool
	AddedInVersion           string
	LockedToDefaultInVersion string
	RemovedInVersion         string
}

// Contains returns true if the range contains the given version, false otherwise.
// The range contains the given version only if it's greater or equal than AddedInVersion (always true if AddedInVersion is empty),
// and less than RemovedInVersion (always true if RemovedInVersion is empty).
func (r *FeatureGateVersionRange) Contains(version string) (bool, error) {
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

func isFeatureLockedToDefault(featureGate, version string) (bool, error) {
	var constraint string
	vr := featureGateVersionRanges[featureGate]
	if vr.LockedToDefaultInVersion != "" {
		constraint = fmt.Sprintf(">= %s", vr.LockedToDefaultInVersion)
		return versionutils.CheckVersionMeetsConstraint(version, constraint)
	}

	return false, nil
}

// ValidateFeatureGates validates the given Kubernetes feature gates against the given Kubernetes version.
func ValidateFeatureGates(featureGates map[string]bool, version string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for featureGate := range featureGates {
		supported, err := IsFeatureGateSupported(featureGate, version)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child(featureGate), featureGate, err.Error()))
		} else if !supported {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child(featureGate), fmt.Sprintf("not supported in Kubernetes version %s", version)))
		} else {
			isLockedToDefault, err := isFeatureLockedToDefault(featureGate, version)
			if err != nil {
				allErrs = append(allErrs, field.Invalid(fldPath.Child(featureGate), featureGate, err.Error()))
			} else if isLockedToDefault && featureGates[featureGate] != featureGateVersionRanges[featureGate].Default {
				allErrs = append(allErrs, field.Forbidden(fldPath.Child(featureGate), fmt.Sprintf("cannot set feature gate to %v, feature is locked to %v", featureGates[featureGate], featureGateVersionRanges[featureGate].Default)))
			}
		}
	}

	return allErrs
}
