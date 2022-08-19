// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	utilsversion "github.com/gardener/gardener/pkg/utils/version"
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

	"APIListChunking":                                {},
	"APIPriorityAndFairness":                         {AddedInVersion: "1.17"},
	"APIResponseCompression":                         {},
	"APIServerIdentity":                              {AddedInVersion: "1.20"},
	"APIServerTracing":                               {AddedInVersion: "1.22"},
	"AdvancedAuditing":                               {},
	"AllowInsecureBackendProxy":                      {AddedInVersion: "1.17", RemovedInVersion: "1.23"},
	"AnyVolumeDataSource":                            {AddedInVersion: "1.18"},
	"AppArmor":                                       {},
	"AttachVolumeLimit":                              {RemovedInVersion: "1.21"},
	"BalanceAttachedNodeVolumes":                     {RemovedInVersion: "1.22"},
	"BlockVolume":                                    {RemovedInVersion: "1.21"},
	"BoundServiceAccountTokenVolume":                 {RemovedInVersion: "1.23"},
	"CPUManager":                                     {},
	"CPUManagerPolicyAlphaOptions":                   {AddedInVersion: "1.23"},
	"CPUManagerPolicyBetaOptions":                    {AddedInVersion: "1.23"},
	"CPUManagerPolicyOptions":                        {AddedInVersion: "1.22"},
	"CRIContainerLogRotation":                        {RemovedInVersion: "1.22"},
	"CronJobTimeZone":                                {AddedInVersion: "1.24"},
	"CSIBlockVolume":                                 {RemovedInVersion: "1.21"},
	"CSIDriverRegistry":                              {RemovedInVersion: "1.21"},
	"CSIInlineVolume":                                {},
	"CSIMigration":                                   {},
	"CSIMigrationAWS":                                {},
	"CSIMigrationAWSComplete":                        {AddedInVersion: "1.17", RemovedInVersion: "1.21"},
	"CSIMigrationAzureDisk":                          {},
	"CSIMigrationAzureDiskComplete":                  {AddedInVersion: "1.17", RemovedInVersion: "1.21"},
	"CSIMigrationAzureFile":                          {},
	"CSIMigrationAzureFileComplete":                  {AddedInVersion: "1.17", RemovedInVersion: "1.21"},
	"CSIMigrationGCE":                                {},
	"CSIMigrationGCEComplete":                        {AddedInVersion: "1.17", RemovedInVersion: "1.21"},
	"CSIMigrationOpenStack":                          {Default: true, AddedInVersion: "1.14", LockedToDefaultInVersion: "1.24"},
	"CSIMigrationOpenStackComplete":                  {AddedInVersion: "1.17", RemovedInVersion: "1.21"},
	"CSIMigrationPortworx":                           {AddedInVersion: "1.23"},
	"CSIMigrationRBD":                                {AddedInVersion: "1.24"},
	"CSIMigrationvSphere":                            {AddedInVersion: "1.19"},
	"CSIMigrationvSphereComplete":                    {AddedInVersion: "1.19", RemovedInVersion: "1.22"},
	"CSINodeInfo":                                    {RemovedInVersion: "1.21"},
	"CSIPersistentVolume":                            {RemovedInVersion: "1.16"},
	"CSIServiceAccountToken":                         {Default: true, AddedInVersion: "1.20", LockedToDefaultInVersion: "1.22"},
	"CSIStorageCapacity":                             {Default: true, AddedInVersion: "1.19", LockedToDefaultInVersion: "1.24"},
	"CSIVolumeFSGroupPolicy":                         {Default: true, AddedInVersion: "1.19", LockedToDefaultInVersion: "1.23"},
	"CSIVolumeHealth":                                {AddedInVersion: "1.21"},
	"CSRDuration":                                    {Default: true, AddedInVersion: "1.22", LockedToDefaultInVersion: "1.24"},
	"ConfigurableFSGroupPolicy":                      {Default: true, AddedInVersion: "1.18", LockedToDefaultInVersion: "1.23"},
	"ControllerManagerLeaderMigration":               {Default: true, AddedInVersion: "1.21", LockedToDefaultInVersion: "1.24"}, // Missing from docu?
	"CronJobControllerV2":                            {AddedInVersion: "1.20", RemovedInVersion: "1.23"},
	"CustomCPUCFSQuotaPeriod":                        {},
	"CustomPodDNS":                                   {RemovedInVersion: "1.16"},
	"CustomResourceDefaulting":                       {RemovedInVersion: "1.18"},
	"CustomResourcePublishOpenAPI":                   {RemovedInVersion: "1.18"},
	"CustomResourceSubresources":                     {RemovedInVersion: "1.18"},
	"CustomResourceValidation":                       {RemovedInVersion: "1.18"},
	"CustomResourceValidationExpressions":            {AddedInVersion: "1.23"},
	"CustomResourceWebhookConversion":                {RemovedInVersion: "1.18"},
	"DaemonSetUpdateSurge":                           {AddedInVersion: "1.21"},                           // Missing from docu?
	"DebugContainers":                                {RemovedInVersion: "1.16"},                         // Missing from docu?
	"DefaultIngressClass":                            {AddedInVersion: "1.18", RemovedInVersion: "1.20"}, // Missing from docu?
	"DefaultPodTopologySpread":                       {Default: true, AddedInVersion: "1.19", LockedToDefaultInVersion: "1.24"},
	"DelegateFSGroupToCSIDriver":                     {AddedInVersion: "1.22"},
	"DevicePlugins":                                  {},
	"DisableAcceleratorUsageMetrics":                 {AddedInVersion: "1.19"},
	"DisableCloudProviders":                          {AddedInVersion: "1.22"},
	"DisableKubeletCloudCredentialProviders":         {AddedInVersion: "1.23"},
	"DownwardAPIHugePages":                           {AddedInVersion: "1.20"},
	"DryRun":                                         {},
	"DynamicAuditing":                                {RemovedInVersion: "1.19"},
	"DynamicKubeletConfig":                           {},
	"EfficientWatchResumption":                       {Default: true, AddedInVersion: "1.20", LockedToDefaultInVersion: "1.24"},
	"EnableAggregatedDiscoveryTimeout":               {AddedInVersion: "1.16", RemovedInVersion: "1.17"},
	"EndpointSlice":                                  {Default: true, AddedInVersion: "1.16", LockedToDefaultInVersion: "1.21"},
	"EndpointSliceNodeName":                          {Default: true, AddedInVersion: "1.20", LockedToDefaultInVersion: "1.21"},
	"EndpointSliceProxying":                          {Default: true, AddedInVersion: "1.18", LockedToDefaultInVersion: "1.22"},
	"EndpointSliceTerminatingCondition":              {AddedInVersion: "1.20"},
	"EphemeralContainers":                            {AddedInVersion: "1.16"},
	"EvenPodsSpread":                                 {AddedInVersion: "1.16", RemovedInVersion: "1.21"},
	"ExecProbeTimeout":                               {AddedInVersion: "1.20"},
	"ExpandCSIVolumes":                               {},
	"ExpandedDNSConfig":                              {AddedInVersion: "1.22"},
	"ExpandInUsePersistentVolumes":                   {},
	"ExpandPersistentVolumes":                        {},
	"ExperimentalCriticalPodAnnotation":              {RemovedInVersion: "1.16"},
	"ExperimentalHostUserNamespaceDefaulting":        {},
	"ExternalPolicyForExternalIP":                    {AddedInVersion: "1.18", RemovedInVersion: "1.22"}, // Missing from docu?
	"GCERegionalPersistentDisk":                      {RemovedInVersion: "1.17"},
	"GRPCContainerProbe":                             {AddedInVersion: "1.23"},
	"GenericEphemeralVolume":                         {Default: true, AddedInVersion: "1.19", LockedToDefaultInVersion: "1.23"},
	"GracefulNodeShutdown":                           {AddedInVersion: "1.20"},
	"GracefulNodeShutdownBasedOnPodPriority":         {AddedInVersion: "1.23"},
	"HonorPVReclaimPolicy":                           {AddedInVersion: "1.23"},
	"HPAContainerMetrics":                            {AddedInVersion: "1.20"},
	"HPAScaleToZero":                                 {AddedInVersion: "1.16"},
	"HugePageStorageMediumSize":                      {Default: true, AddedInVersion: "1.18", LockedToDefaultInVersion: "1.22", RemovedInVersion: "1.24"},
	"HugePages":                                      {RemovedInVersion: "1.16"},
	"HyperVContainer":                                {RemovedInVersion: "1.21"},
	"IPv6DualStack":                                  {Default: true, AddedInVersion: "1.16", LockedToDefaultInVersion: "1.23"},
	"IdentifyPodOS":                                  {AddedInVersion: "1.23"},
	"ImmutableEphemeralVolumes":                      {Default: true, AddedInVersion: "1.18", LockedToDefaultInVersion: "1.21", RemovedInVersion: "1.24"},
	"InTreePluginAWSUnregister":                      {AddedInVersion: "1.21"}, // Missing from docu?
	"InTreePluginAzureDiskUnregister":                {AddedInVersion: "1.21"}, // Missing from docu?
	"InTreePluginAzureFileUnregister":                {AddedInVersion: "1.21"}, // Missing from docu?
	"InTreePluginGCEUnregister":                      {AddedInVersion: "1.21"}, // Missing from docu?
	"InTreePluginOpenStackUnregister":                {AddedInVersion: "1.21"}, // Missing from docu?
	"InTreePluginPortworxUnregister":                 {AddedInVersion: "1.23"},
	"InTreePluginRBDUnregister":                      {AddedInVersion: "1.23"},
	"InTreePluginvSphereUnregister":                  {AddedInVersion: "1.21"}, // Missing from docu?
	"IndexedJob":                                     {Default: true, AddedInVersion: "1.21", LockedToDefaultInVersion: "1.24"},
	"IngressClassNamespacedParams":                   {Default: true, AddedInVersion: "1.21", LockedToDefaultInVersion: "1.23"},
	"JobMutableNodeSchedulingDirectives":             {AddedInVersion: "1.23"},
	"JobReadyPods":                                   {AddedInVersion: "1.23"},
	"JobTrackingWithFinalizers":                      {AddedInVersion: "1.22"},
	"KubeletCredentialProviders":                     {AddedInVersion: "1.20"},
	"KubeletInUserNamespace":                         {AddedInVersion: "1.22"},
	"KubeletPluginsWatcher":                          {RemovedInVersion: "1.16"},
	"KubeletPodResources":                            {},
	"KubeletPodResourcesGetAllocatable":              {AddedInVersion: "1.21"},
	"LegacyNodeRoleBehavior":                         {AddedInVersion: "1.16", RemovedInVersion: "1.22"},
	"LegacyServiceAccountTokenNoAutoGeneration":      {AddedInVersion: "1.24"},
	"LocalStorageCapacityIsolation":                  {},
	"LocalStorageCapacityIsolationFSQuotaMonitoring": {},
	"LogarithmicScaleDown":                           {AddedInVersion: "1.21"},
	"MaxUnavailableStatefulSet":                      {AddedInVersion: "1.24"},
	"MemoryManager":                                  {AddedInVersion: "1.21"}, // Missing from docu?
	"MemoryQoS":                                      {AddedInVersion: "1.22"},
	"MigrationRBD":                                   {AddedInVersion: "1.23", RemovedInVersion: "1.24"},
	"MinDomainsInPodTopologySpread":                  {AddedInVersion: "1.24"},
	"MixedProtocolLBService":                         {AddedInVersion: "1.20"},
	"MountContainers":                                {RemovedInVersion: "1.17"},
	"NamespaceDefaultLabelName":                      {Default: true, AddedInVersion: "1.21", LockedToDefaultInVersion: "1.22", RemovedInVersion: "1.24"},
	"NetworkPolicyEndPort":                           {AddedInVersion: "1.21"},
	"NetworkPolicyStatus":                            {AddedInVersion: "1.24"},
	"NodeDisruptionExclusion":                        {AddedInVersion: "1.16", RemovedInVersion: "1.22"},
	"NodeLease":                                      {RemovedInVersion: "1.23"},
	"NodeOutOfServiceVolumeDetach":                   {AddedInVersion: "1.24"},
	"NonPreemptingPriority":                          {Default: true, LockedToDefaultInVersion: "1.24"},
	"NodeSwap":                                       {AddedInVersion: "1.22"},
	"OpenAPIEnums":                                   {AddedInVersion: "1.23"},
	"OpenAPIV3":                                      {AddedInVersion: "1.23"},
	"PersistentLocalVolumes":                         {RemovedInVersion: "1.17"},
	"PodAndContainerStatsFromCRI":                    {AddedInVersion: "1.23"},
	"PodAffinityNamespaceSelector":                   {Default: true, AddedInVersion: "1.21", LockedToDefaultInVersion: "1.24"},
	"PodDeletionCost":                                {AddedInVersion: "1.21"},
	"PodDisruptionBudget":                            {Default: true, AddedInVersion: "1.17", LockedToDefaultInVersion: "1.21"}, // Docu says 1.3?
	"PodOverhead":                                    {Default: true, AddedInVersion: "1.16", LockedToDefaultInVersion: "1.24"},
	"PodPriority":                                    {RemovedInVersion: "1.18"},
	"PodReadinessGates":                              {RemovedInVersion: "1.16"},
	"PodSecurity":                                    {AddedInVersion: "1.22"},
	"PodShareProcessNamespace":                       {RemovedInVersion: "1.19"},
	"PreferNominatedNode":                            {Default: true, AddedInVersion: "1.21", LockedToDefaultInVersion: "1.24"}, // Missing from docu?
	"ProbeTerminationGracePeriod":                    {AddedInVersion: "1.21"},
	"ProcMountType":                                  {},
	"ProxyTerminatingEndpoints":                      {AddedInVersion: "1.22"},
	"QOSReserved":                                    {},
	"ReadWriteOncePod":                               {AddedInVersion: "1.22"},
	"RecoverVolumeExpansionFailure":                  {AddedInVersion: "1.23"},
	"RemainingItemCount":                             {},
	"RemoveSelfLink":                                 {Default: true, AddedInVersion: "1.16", LockedToDefaultInVersion: "1.24"},
	"RequestManagement":                              {RemovedInVersion: "1.17"},
	"ResourceLimitsPriorityFunction":                 {RemovedInVersion: "1.19"},
	"ResourceQuotaScopeSelectors":                    {RemovedInVersion: "1.18"},
	"RootCAConfigMap":                                {AddedInVersion: "1.20", RemovedInVersion: "1.22"}, // Docu says 1.13?
	"RotateKubeletClientCertificate":                 {RemovedInVersion: "1.21"},
	"RotateKubeletServerCertificate":                 {},
	"RunAsGroup":                                     {RemovedInVersion: "1.22"},
	"RuntimeClass":                                   {Default: true, LockedToDefaultInVersion: "1.20", RemovedInVersion: "1.24"},
	"SCTPSupport":                                    {RemovedInVersion: "1.22"},
	"ScheduleDaemonSetPods":                          {RemovedInVersion: "1.18"},
	"SeccompDefault":                                 {AddedInVersion: "1.22"},
	"SelectorIndex":                                  {Default: true, AddedInVersion: "1.18", LockedToDefaultInVersion: "1.20"}, // Missing from docu?
	"ServerSideApply":                                {},
	"ServerSideFieldValidation":                      {AddedInVersion: "1.23"},
	"ServiceAccountIssuerDiscovery":                  {AddedInVersion: "1.18", RemovedInVersion: "1.23"},
	"ServiceAppProtocol":                             {AddedInVersion: "1.18", RemovedInVersion: "1.22"},
	"ServiceInternalTrafficPolicy":                   {AddedInVersion: "1.21"},
	"ServiceIPStaticSubrange":                        {AddedInVersion: "1.24"},
	"ServiceLBNodePortControl":                       {Default: true, AddedInVersion: "1.20", LockedToDefaultInVersion: "1.24"},
	"ServiceLoadBalancerClass":                       {Default: true, AddedInVersion: "1.21", LockedToDefaultInVersion: "1.24"},
	"ServiceLoadBalancerFinalizer":                   {RemovedInVersion: "1.20"},
	"ServiceNodeExclusion":                           {RemovedInVersion: "1.22"},
	"ServiceTopology":                                {AddedInVersion: "1.17", RemovedInVersion: "1.22"},
	"SetHostnameAsFQDN":                              {Default: true, AddedInVersion: "1.19", LockedToDefaultInVersion: "1.22", RemovedInVersion: "1.24"},
	"SizeMemoryBackedVolumes":                        {AddedInVersion: "1.20"},
	"StartupProbe":                                   {AddedInVersion: "1.16", RemovedInVersion: "1.23"},
	"StatefulSetAutoDeletePVC":                       {AddedInVersion: "1.23"},
	"StatefulSetMinReadySeconds":                     {AddedInVersion: "1.22"},
	"StorageObjectInUseProtection":                   {Default: true, LockedToDefaultInVersion: "1.23"},
	"StorageVersionAPI":                              {AddedInVersion: "1.20"},
	"StorageVersionHash":                             {},
	"StreamingProxyRedirects":                        {RemovedInVersion: "1.24"},
	"SupportIPVSProxyMode":                           {RemovedInVersion: "1.20"},
	"SupportNodePidsLimit":                           {RemovedInVersion: "1.23"},
	"SupportPodPidsLimit":                            {RemovedInVersion: "1.23"},
	"SuspendJob":                                     {Default: true, AddedInVersion: "1.21", LockedToDefaultInVersion: "1.24"},
	"Sysctls":                                        {RemovedInVersion: "1.23"},
	"TTLAfterFinished":                               {Default: true, LockedToDefaultInVersion: "1.23"},
	"TaintBasedEvictions":                            {RemovedInVersion: "1.20"},
	"TaintNodesByCondition":                          {RemovedInVersion: "1.18"},
	"TokenRequest":                                   {RemovedInVersion: "1.21"},
	"TokenRequestProjection":                         {RemovedInVersion: "1.21"},
	"TopologyAwareHints":                             {AddedInVersion: "1.21"},
	"TopologyManager":                                {AddedInVersion: "1.16"},
	"ValidateProxyRedirects":                         {RemovedInVersion: "1.24"},
	"VolumeCapacityPriority":                         {AddedInVersion: "1.21"},
	"VolumePVCDataSource":                            {RemovedInVersion: "1.21"},
	"VolumeScheduling":                               {RemovedInVersion: "1.16"},
	"VolumeSnapshotDataSource":                       {RemovedInVersion: "1.22"},
	"VolumeSubpath":                                  {},
	"VolumeSubpathEnvExpansion":                      {RemovedInVersion: "1.19"},
	"WarningHeaders":                                 {Default: true, AddedInVersion: "1.19", LockedToDefaultInVersion: "1.22", RemovedInVersion: "1.24"},
	"WatchBookmark":                                  {Default: true, LockedToDefaultInVersion: "1.17"},
	"WinDSR":                                         {},
	"WinOverlay":                                     {},
	"WindowsEndpointSliceProxying":                   {Default: true, AddedInVersion: "1.19", LockedToDefaultInVersion: "1.22"},
	"WindowsGMSA":                                    {RemovedInVersion: "1.21"},
	"WindowsHostProcessContainers":                   {AddedInVersion: "1.22"},
	"WindowsRunAsUserName":                           {AddedInVersion: "1.16", RemovedInVersion: "1.21"},
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
	return utilsversion.CheckVersionMeetsConstraint(version, constraint)
}

func isFeatureLockedToDefault(featureGate, version string) (bool, error) {
	var constraint string
	vr := featureGateVersionRanges[featureGate]
	if vr.LockedToDefaultInVersion != "" {
		constraint = fmt.Sprintf(">= %s", vr.LockedToDefaultInVersion)
		return utilsversion.CheckVersionMeetsConstraint(version, constraint)
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
