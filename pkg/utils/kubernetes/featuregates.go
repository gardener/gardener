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

package kubernetes

import (
	"fmt"

	utilsversion "github.com/gardener/gardener/pkg/utils/version"
)

// featureGateVersionRanges contains the version ranges for all Kubernetes feature gates.
// Extracted from https://raw.githubusercontent.com/kubernetes/kubernetes/release-${version}/pkg/features/kube_features.go.
// To maintain this list for each new Kubernetes version:
// * Run hack/compare-k8s-feature-gates.sh <old-version> <new-version>.
//   It will present 2 lists of feature gates: those added and those removed in <new-version> compared to <old-version>.
// * Add all added feature gates to the map with <new-version> as AddedInVersion and no RemovedInVersion.
// * For any removed feature gates, add <new-version> as RemovedInVersion to the already existing feature gate in the map.
var featureGateVersionRanges = map[string]*FeatureGateVersionRange{
	// These are special feature gates to toggle all alpha or beta feature gates on and off.
	// They were introduced in version 1.17 (although they are absent from the corresponding kube_features.go file).
	"AllAlpha": {AddedInVersion: "1.17"},
	"AllBeta":  {AddedInVersion: "1.17"},

	"APIListChunking":                                {},
	"APIPriorityAndFairness":                         {AddedInVersion: "1.17"},
	"APIResponseCompression":                         {},
	"APIServerIdentity":                              {AddedInVersion: "1.20"},
	"AdvancedAuditing":                               {},
	"AllowInsecureBackendProxy":                      {AddedInVersion: "1.17"},
	"AnyVolumeDataSource":                            {AddedInVersion: "1.18"},
	"AppArmor":                                       {},
	"AttachVolumeLimit":                              {RemovedInVersion: "1.21"},
	"BalanceAttachedNodeVolumes":                     {},
	"BlockVolume":                                    {RemovedInVersion: "1.21"},
	"BoundServiceAccountTokenVolume":                 {},
	"CPUManager":                                     {},
	"CRIContainerLogRotation":                        {},
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
	"CSIMigrationOpenStack":                          {},
	"CSIMigrationOpenStackComplete":                  {AddedInVersion: "1.17", RemovedInVersion: "1.21"},
	"CSIMigrationvSphere":                            {AddedInVersion: "1.19"},
	"CSIMigrationvSphereComplete":                    {AddedInVersion: "1.19"},
	"CSINodeInfo":                                    {RemovedInVersion: "1.21"},
	"CSIPersistentVolume":                            {RemovedInVersion: "1.16"},
	"CSIServiceAccountToken":                         {AddedInVersion: "1.20"},
	"CSIStorageCapacity":                             {AddedInVersion: "1.19"},
	"CSIVolumeFSGroupPolicy":                         {AddedInVersion: "1.19"},
	"CSIVolumeHealth":                                {AddedInVersion: "1.21"},
	"ConfigurableFSGroupPolicy":                      {AddedInVersion: "1.18"},
	"ControllerManagerLeaderMigration":               {AddedInVersion: "1.21"}, // Missing from docu?
	"CronJobControllerV2":                            {AddedInVersion: "1.20"},
	"CustomCPUCFSQuotaPeriod":                        {},
	"CustomPodDNS":                                   {RemovedInVersion: "1.16"},
	"CustomResourceDefaulting":                       {RemovedInVersion: "1.18"},
	"CustomResourcePublishOpenAPI":                   {RemovedInVersion: "1.18"},
	"CustomResourceSubresources":                     {RemovedInVersion: "1.18"},
	"CustomResourceValidation":                       {RemovedInVersion: "1.18"},
	"CustomResourceWebhookConversion":                {RemovedInVersion: "1.18"},
	"DaemonSetUpdateSurge":                           {AddedInVersion: "1.21"},                           // Missing from docu?
	"DebugContainers":                                {RemovedInVersion: "1.16"},                         // Missing from docu?
	"DefaultIngressClass":                            {AddedInVersion: "1.18", RemovedInVersion: "1.20"}, // Missing from docu?
	"DefaultPodTopologySpread":                       {AddedInVersion: "1.19"},
	"DevicePlugins":                                  {},
	"DisableAcceleratorUsageMetrics":                 {AddedInVersion: "1.19"},
	"DownwardAPIHugePages":                           {AddedInVersion: "1.20"},
	"DryRun":                                         {},
	"DynamicAuditing":                                {RemovedInVersion: "1.19"},
	"DynamicKubeletConfig":                           {},
	"EfficientWatchResumption":                       {AddedInVersion: "1.20"},
	"EnableAggregatedDiscoveryTimeout":               {AddedInVersion: "1.16", RemovedInVersion: "1.17"},
	"EndpointSlice":                                  {AddedInVersion: "1.16"},
	"EndpointSliceNodeName":                          {AddedInVersion: "1.20"},
	"EndpointSliceProxying":                          {AddedInVersion: "1.18"},
	"EndpointSliceTerminatingCondition":              {AddedInVersion: "1.20"},
	"EphemeralContainers":                            {AddedInVersion: "1.16"},
	"EvenPodsSpread":                                 {AddedInVersion: "1.16", RemovedInVersion: "1.21"},
	"ExecProbeTimeout":                               {AddedInVersion: "1.20"},
	"ExpandCSIVolumes":                               {},
	"ExpandInUsePersistentVolumes":                   {},
	"ExpandPersistentVolumes":                        {},
	"ExperimentalCriticalPodAnnotation":              {RemovedInVersion: "1.16"},
	"ExperimentalHostUserNamespaceDefaulting":        {},
	"ExternalPolicyForExternalIP":                    {AddedInVersion: "1.18"}, // Missing from docu?
	"GCERegionalPersistentDisk":                      {RemovedInVersion: "1.17"},
	"GenericEphemeralVolume":                         {AddedInVersion: "1.19"},
	"GracefulNodeShutdown":                           {AddedInVersion: "1.20"},
	"HPAContainerMetrics":                            {AddedInVersion: "1.20"},
	"HPAScaleToZero":                                 {AddedInVersion: "1.16"},
	"HugePageStorageMediumSize":                      {AddedInVersion: "1.18"},
	"HugePages":                                      {RemovedInVersion: "1.16"},
	"HyperVContainer":                                {RemovedInVersion: "1.21"},
	"IPv6DualStack":                                  {AddedInVersion: "1.16"},
	"ImmutableEphemeralVolumes":                      {AddedInVersion: "1.18"},
	"InTreePluginAWSUnregister":                      {AddedInVersion: "1.21"}, // Missing from docu?
	"InTreePluginAzureDiskUnregister":                {AddedInVersion: "1.21"}, // Missing from docu?
	"InTreePluginAzureFileUnregister":                {AddedInVersion: "1.21"}, // Missing from docu?
	"InTreePluginGCEUnregister":                      {AddedInVersion: "1.21"}, // Missing from docu?
	"InTreePluginOpenStackUnregister":                {AddedInVersion: "1.21"}, // Missing from docu?
	"InTreePluginvSphereUnregister":                  {AddedInVersion: "1.21"}, // Missing from docu?
	"IndexedJob":                                     {AddedInVersion: "1.21"},
	"IngressClassNamespacedParams":                   {AddedInVersion: "1.21"},
	"KubeletCredentialProviders":                     {AddedInVersion: "1.20"},
	"KubeletPluginsWatcher":                          {RemovedInVersion: "1.16"},
	"KubeletPodResources":                            {},
	"KubeletPodResourcesGetAllocatable":              {AddedInVersion: "1.21"},
	"LegacyNodeRoleBehavior":                         {AddedInVersion: "1.16"},
	"LocalStorageCapacityIsolation":                  {},
	"LocalStorageCapacityIsolationFSQuotaMonitoring": {},
	"LogarithmicScaleDown":                           {AddedInVersion: "1.21"},
	"MemoryManager":                                  {AddedInVersion: "1.21"}, // Missing from docu?
	"MixedProtocolLBService":                         {AddedInVersion: "1.20"},
	"MountContainers":                                {RemovedInVersion: "1.17"},
	"NamespaceDefaultLabelName":                      {AddedInVersion: "1.21"},
	"NetworkPolicyEndPort":                           {AddedInVersion: "1.21"},
	"NodeDisruptionExclusion":                        {AddedInVersion: "1.16"},
	"NodeLease":                                      {},
	"NonPreemptingPriority":                          {},
	"PersistentLocalVolumes":                         {RemovedInVersion: "1.17"},
	"PodAffinityNamespaceSelector":                   {AddedInVersion: "1.21"},
	"PodDeletionCost":                                {AddedInVersion: "1.21"},
	"PodDisruptionBudget":                            {AddedInVersion: "1.17"}, // Docu says 1.3?
	"PodOverhead":                                    {AddedInVersion: "1.16"},
	"PodPriority":                                    {RemovedInVersion: "1.18"},
	"PodReadinessGates":                              {RemovedInVersion: "1.16"},
	"PodShareProcessNamespace":                       {RemovedInVersion: "1.19"},
	"PreferNominatedNode":                            {AddedInVersion: "1.21"}, // Missing from docu?
	"ProbeTerminationGracePeriod":                    {AddedInVersion: "1.21"},
	"ProcMountType":                                  {},
	"QOSReserved":                                    {},
	"RemainingItemCount":                             {},
	"RemoveSelfLink":                                 {AddedInVersion: "1.16"},
	"RequestManagement":                              {RemovedInVersion: "1.17"},
	"ResourceLimitsPriorityFunction":                 {RemovedInVersion: "1.19"},
	"ResourceQuotaScopeSelectors":                    {RemovedInVersion: "1.18"},
	"RootCAConfigMap":                                {AddedInVersion: "1.20"}, // Docu says 1.13?
	"RotateKubeletClientCertificate":                 {RemovedInVersion: "1.21"},
	"RotateKubeletServerCertificate":                 {},
	"RunAsGroup":                                     {},
	"RuntimeClass":                                   {},
	"SCTPSupport":                                    {},
	"ScheduleDaemonSetPods":                          {RemovedInVersion: "1.18"},
	"SelectorIndex":                                  {AddedInVersion: "1.18"}, // Missing from docu?
	"ServerSideApply":                                {},
	"ServiceAccountIssuerDiscovery":                  {AddedInVersion: "1.18"},
	"ServiceAppProtocol":                             {AddedInVersion: "1.18"},
	"ServiceInternalTrafficPolicy":                   {AddedInVersion: "1.21"},
	"ServiceLBNodePortControl":                       {AddedInVersion: "1.20"},
	"ServiceLoadBalancerClass":                       {AddedInVersion: "1.21"},
	"ServiceLoadBalancerFinalizer":                   {RemovedInVersion: "1.20"},
	"ServiceNodeExclusion":                           {},
	"ServiceTopology":                                {AddedInVersion: "1.17"},
	"SetHostnameAsFQDN":                              {AddedInVersion: "1.19"},
	"SizeMemoryBackedVolumes":                        {AddedInVersion: "1.20"},
	"StartupProbe":                                   {AddedInVersion: "1.16"},
	"StorageObjectInUseProtection":                   {},
	"StorageVersionAPI":                              {AddedInVersion: "1.20"},
	"StorageVersionHash":                             {},
	"StreamingProxyRedirects":                        {},
	"SupportIPVSProxyMode":                           {RemovedInVersion: "1.20"},
	"SupportNodePidsLimit":                           {},
	"SupportPodPidsLimit":                            {},
	"SuspendJob":                                     {AddedInVersion: "1.21"},
	"Sysctls":                                        {},
	"TTLAfterFinished":                               {},
	"TaintBasedEvictions":                            {RemovedInVersion: "1.20"},
	"TaintNodesByCondition":                          {RemovedInVersion: "1.18"},
	"TokenRequest":                                   {RemovedInVersion: "1.21"},
	"TokenRequestProjection":                         {RemovedInVersion: "1.21"},
	"TopologyAwareHints":                             {AddedInVersion: "1.21"},
	"TopologyManager":                                {AddedInVersion: "1.16"},
	"ValidateProxyRedirects":                         {},
	"VolumeCapacityPriority":                         {AddedInVersion: "1.21"},
	"VolumePVCDataSource":                            {RemovedInVersion: "1.21"},
	"VolumeScheduling":                               {RemovedInVersion: "1.16"},
	"VolumeSnapshotDataSource":                       {},
	"VolumeSubpath":                                  {},
	"VolumeSubpathEnvExpansion":                      {RemovedInVersion: "1.19"},
	"WarningHeaders":                                 {AddedInVersion: "1.19"},
	"WatchBookmark":                                  {},
	"WinDSR":                                         {},
	"WinOverlay":                                     {},
	"WindowsEndpointSliceProxying":                   {AddedInVersion: "1.19"},
	"WindowsGMSA":                                    {RemovedInVersion: "1.21"},
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
	AddedInVersion   string
	RemovedInVersion string
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
