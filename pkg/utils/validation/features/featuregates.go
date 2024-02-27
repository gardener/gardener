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
//   - Run hack/compare-k8s-feature-gates.sh <old-version> <new-version> (e.g. 'hack/compare-k8s-feature-gates.sh 1.26 1.27').
//     It will present 3 lists of feature gates: those added and those removed in <new-version> compared to <old-version> and
//     feature gates that got locked to default in `<new-version>`.
//   - Add all added feature gates to the map with <new-version> as AddedInVersion and no RemovedInVersion.
//   - For any removed feature gates, add <new-version> as RemovedInVersion to the already existing feature gate in the map.
//   - For feature gates locked to default, add `<new-version>` as LockedToDefaultInVersion to the already existing feature gate in the map.
var featureGateVersionRanges = map[string]*FeatureGateVersionRange{
	// These are special feature gates to toggle all alpha or beta feature gates on and off.
	// They were introduced in version 1.17 (although they are absent from the corresponding kube_features.go file).
	"AllAlpha": {VersionRange: versionutils.VersionRange{AddedInVersion: "1.17"}},
	"AllBeta":  {VersionRange: versionutils.VersionRange{AddedInVersion: "1.17"}},

	"AdmissionWebhookMatchConditions":                {VersionRange: versionutils.VersionRange{AddedInVersion: "1.27"}},
	"AllowServiceLBStatusOnNonLB":                    {VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"APIListChunking":                                {Default: true, LockedToDefaultInVersion: "1.29"},
	"APIPriorityAndFairness":                         {Default: true, LockedToDefaultInVersion: "1.29"},
	"APIResponseCompression":                         {},
	"APISelfSubjectReview":                           {Default: true, VersionRange: versionutils.VersionRange{AddedInVersion: "1.26"}, LockedToDefaultInVersion: "1.28"},
	"APIServerIdentity":                              {},
	"APIServerTracing":                               {},
	"AdvancedAuditing":                               {Default: true, LockedToDefaultInVersion: "1.27", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.28"}},
	"AggregatedDiscoveryEndpoint":                    {VersionRange: versionutils.VersionRange{AddedInVersion: "1.26"}},
	"AnyVolumeDataSource":                            {},
	"AppArmor":                                       {},
	"CloudControllerManagerWebhook":                  {VersionRange: versionutils.VersionRange{AddedInVersion: "1.27"}},
	"CloudDualStackNodeIPs":                          {VersionRange: versionutils.VersionRange{AddedInVersion: "1.27"}},
	"ClusterTrustBundle":                             {VersionRange: versionutils.VersionRange{AddedInVersion: "1.27"}},
	"ClusterTrustBundleProjection":                   {VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"ConsistentListFromCache":                        {VersionRange: versionutils.VersionRange{AddedInVersion: "1.28"}},
	"CPUManager":                                     {Default: true, LockedToDefaultInVersion: "1.26"},
	"CPUManagerPolicyAlphaOptions":                   {},
	"CPUManagerPolicyBetaOptions":                    {},
	"CPUManagerPolicyOptions":                        {},
	"CRDValidationRatcheting":                        {VersionRange: versionutils.VersionRange{AddedInVersion: "1.28"}},
	"CronJobsScheduledAnnotation":                    {VersionRange: versionutils.VersionRange{AddedInVersion: "1.28"}},
	"CronJobTimeZone":                                {Default: true, LockedToDefaultInVersion: "1.27", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.29"}},
	"CSIInlineVolume":                                {Default: true, LockedToDefaultInVersion: "1.25", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.27"}},
	"CSIMigration":                                   {Default: true, LockedToDefaultInVersion: "1.25", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.27"}},
	"CSIMigrationAWS":                                {Default: true, LockedToDefaultInVersion: "1.25", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.27"}},
	"CSIMigrationAzureDisk":                          {Default: true, LockedToDefaultInVersion: "1.25", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.27"}},
	"CSIMigrationAzureFile":                          {Default: true, LockedToDefaultInVersion: "1.27"},
	"CSIMigrationGCE":                                {Default: true, LockedToDefaultInVersion: "1.25", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.28"}},
	"CSIMigrationOpenStack":                          {Default: true, LockedToDefaultInVersion: "1.24", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.26"}},
	"CSIMigrationPortworx":                           {},
	"CSIMigrationRBD":                                {},
	"CSIMigrationvSphere":                            {Default: true, LockedToDefaultInVersion: "1.27", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.29"}},
	"CSINodeExpandSecret":                            {Default: true, LockedToDefaultInVersion: "1.29"},
	"CSIStorageCapacity":                             {Default: true, LockedToDefaultInVersion: "1.24", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.28"}},
	"CSIVolumeHealth":                                {},
	"CSRDuration":                                    {Default: true, LockedToDefaultInVersion: "1.24", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.26"}},
	"ConsistentHTTPGetHandlers":                      {VersionRange: versionutils.VersionRange{AddedInVersion: "1.26"}},
	"ContainerCheckpoint":                            {},
	"ControllerManagerLeaderMigration":               {Default: true, LockedToDefaultInVersion: "1.24", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.27"}}, // Missing from docu?
	"CrossNamespaceVolumeDataSource":                 {VersionRange: versionutils.VersionRange{AddedInVersion: "1.26"}},
	"CustomCPUCFSQuotaPeriod":                        {},
	"CustomResourceValidationExpressions":            {Default: true, LockedToDefaultInVersion: "1.29"},
	"DaemonSetUpdateSurge":                           {Default: true, LockedToDefaultInVersion: "1.25", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.27"}}, // Missing from docu?
	"DefaultHostNetworkHostPortsInPodTemplates":      {VersionRange: versionutils.VersionRange{AddedInVersion: "1.28"}},
	"DefaultPodTopologySpread":                       {Default: true, LockedToDefaultInVersion: "1.24", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.26"}},
	"DelegateFSGroupToCSIDriver":                     {Default: true, LockedToDefaultInVersion: "1.26", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.28"}},
	"DevicePluginCDIDevices":                         {VersionRange: versionutils.VersionRange{AddedInVersion: "1.28"}},
	"DevicePlugins":                                  {Default: true, LockedToDefaultInVersion: "1.26", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.28"}},
	"DisableAcceleratorUsageMetrics":                 {Default: true, LockedToDefaultInVersion: "1.25", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.28"}},
	"DisableCloudProviders":                          {},
	"DisableKubeletCloudCredentialProviders":         {},
	"DisableNodeKubeProxyVersion":                    {VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"DownwardAPIHugePages":                           {Default: true, LockedToDefaultInVersion: "1.27", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.29"}},
	"DryRun":                                         {Default: true, LockedToDefaultInVersion: "1.26", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.28"}},
	"DynamicKubeletConfig":                           {VersionRange: versionutils.VersionRange{RemovedInVersion: "1.26"}},
	"DynamicResourceAllocation":                      {VersionRange: versionutils.VersionRange{AddedInVersion: "1.26"}},
	"EfficientWatchResumption":                       {Default: true, LockedToDefaultInVersion: "1.24"},
	"ElasticIndexedJob":                              {VersionRange: versionutils.VersionRange{AddedInVersion: "1.27"}},
	"EndpointSliceTerminatingCondition":              {Default: true, LockedToDefaultInVersion: "1.26", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.28"}},
	"EphemeralContainers":                            {Default: true, LockedToDefaultInVersion: "1.25", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.27"}},
	"EventedPLEG":                                    {VersionRange: versionutils.VersionRange{AddedInVersion: "1.26"}},
	"ExecProbeTimeout":                               {},
	"ExpandCSIVolumes":                               {Default: true, VersionRange: versionutils.VersionRange{RemovedInVersion: "1.27"}},
	"ExpandedDNSConfig":                              {Default: true, LockedToDefaultInVersion: "1.28"},
	"ExpandInUsePersistentVolumes":                   {Default: true, VersionRange: versionutils.VersionRange{RemovedInVersion: "1.27"}},
	"ExpandPersistentVolumes":                        {Default: true, VersionRange: versionutils.VersionRange{RemovedInVersion: "1.27"}},
	"ExperimentalHostUserNamespaceDefaulting":        {Default: false, LockedToDefaultInVersion: "1.28"},
	"GRPCContainerProbe":                             {Default: true, LockedToDefaultInVersion: "1.27", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.29"}},
	"GracefulNodeShutdown":                           {},
	"GracefulNodeShutdownBasedOnPodPriority":         {},
	"HonorPVReclaimPolicy":                           {},
	"HPAContainerMetrics":                            {},
	"HPAScaleToZero":                                 {},
	"IPv6DualStack":                                  {Default: true, LockedToDefaultInVersion: "1.23", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.27"}},
	"IPTablesOwnershipCleanup":                       {Default: true, LockedToDefaultInVersion: "1.28"},
	"IdentifyPodOS":                                  {Default: true, LockedToDefaultInVersion: "1.25", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.27"}},
	"ImageMaximumGCAge":                              {VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"InPlacePodVerticalScaling":                      {VersionRange: versionutils.VersionRange{AddedInVersion: "1.27"}},
	"InTreePluginAWSUnregister":                      {}, // Missing from docu?
	"InTreePluginAzureDiskUnregister":                {}, // Missing from docu?
	"InTreePluginAzureFileUnregister":                {}, // Missing from docu?
	"InTreePluginGCEUnregister":                      {}, // Missing from docu?
	"InTreePluginOpenStackUnregister":                {}, // Missing from docu?
	"InTreePluginPortworxUnregister":                 {},
	"InTreePluginRBDUnregister":                      {},
	"InTreePluginvSphereUnregister":                  {}, // Missing from docu?
	"IndexedJob":                                     {Default: true, LockedToDefaultInVersion: "1.24", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.26"}},
	"JobBackoffLimitPerIndex":                        {VersionRange: versionutils.VersionRange{AddedInVersion: "1.28"}},
	"JobMutableNodeSchedulingDirectives":             {Default: true, LockedToDefaultInVersion: "1.27", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.29"}},
	"JobPodFailurePolicy":                            {},
	"JobPodReplacementPolicy":                        {VersionRange: versionutils.VersionRange{AddedInVersion: "1.28"}},
	"JobReadyPods":                                   {Default: true, LockedToDefaultInVersion: "1.29"},
	"JobTrackingWithFinalizers":                      {Default: true, LockedToDefaultInVersion: "1.26", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.29"}},
	"KMSv1":                                          {VersionRange: versionutils.VersionRange{AddedInVersion: "1.28"}},
	"KMSv2":                                          {Default: true, LockedToDefaultInVersion: "1.29"},
	"KMSv2KDF":                                       {Default: true, LockedToDefaultInVersion: "1.29", VersionRange: versionutils.VersionRange{AddedInVersion: "1.28"}},
	"KubeletCgroupDriverFromCRI":                     {VersionRange: versionutils.VersionRange{AddedInVersion: "1.28"}},
	"KubeletCredentialProviders":                     {Default: true, LockedToDefaultInVersion: "1.26", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.28"}},
	"KubeletInUserNamespace":                         {},
	"KubeletPodResources":                            {Default: true, LockedToDefaultInVersion: "1.28"},
	"KubeletPodResourcesDynamicResources":            {VersionRange: versionutils.VersionRange{AddedInVersion: "1.27"}},
	"KubeletPodResourcesGet":                         {VersionRange: versionutils.VersionRange{AddedInVersion: "1.27"}},
	"KubeletPodResourcesGetAllocatable":              {Default: true, LockedToDefaultInVersion: "1.28"},
	"KubeletSeparateDiskGC":                          {VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"KubeletTracing":                                 {},
	"KubeProxyDrainingTerminatingNodes":              {VersionRange: versionutils.VersionRange{AddedInVersion: "1.28"}},
	"LegacyServiceAccountTokenCleanUp":               {VersionRange: versionutils.VersionRange{AddedInVersion: "1.28"}},
	"LegacyServiceAccountTokenNoAutoGeneration":      {Default: true, LockedToDefaultInVersion: "1.27", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.29"}},
	"LegacyServiceAccountTokenTracking":              {Default: true, VersionRange: versionutils.VersionRange{AddedInVersion: "1.26"}, LockedToDefaultInVersion: "1.28"},
	"LoadBalancerIPMode":                             {VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"LocalStorageCapacityIsolation":                  {Default: true, LockedToDefaultInVersion: "1.25", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.27"}},
	"LocalStorageCapacityIsolationFSQuotaMonitoring": {},
	"LogarithmicScaleDown":                           {},
	"MatchLabelKeysInPodAffinity":                    {VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"MatchLabelKeysInPodTopologySpread":              {},
	"MaxUnavailableStatefulSet":                      {},
	"MemoryManager":                                  {}, // Missing from docu?
	"MemoryQoS":                                      {},
	"MinDomainsInPodTopologySpread":                  {},
	"MinimizeIPTablesRestore":                        {Default: true, VersionRange: versionutils.VersionRange{AddedInVersion: "1.26"}, LockedToDefaultInVersion: "1.28"},
	"MixedProtocolLBService":                         {Default: true, LockedToDefaultInVersion: "1.26", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.28"}},
	"MultiCIDRRangeAllocator":                        {VersionRange: versionutils.VersionRange{RemovedInVersion: "1.29"}},
	"MultiCIDRServiceAllocator":                      {VersionRange: versionutils.VersionRange{AddedInVersion: "1.27"}},
	"NetworkPolicyEndPort":                           {Default: true, LockedToDefaultInVersion: "1.25", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.27"}},
	"NetworkPolicyStatus":                            {VersionRange: versionutils.VersionRange{RemovedInVersion: "1.28"}},
	"NewVolumeManagerReconstruction":                 {VersionRange: versionutils.VersionRange{AddedInVersion: "1.27"}},
	"NFTablesProxyMode":                              {VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"NodeInclusionPolicyInPodTopologySpread":         {},
	"NodeLogQuery":                                   {VersionRange: versionutils.VersionRange{AddedInVersion: "1.27"}},
	"NodeOutOfServiceVolumeDetach":                   {Default: true, LockedToDefaultInVersion: "1.28"},
	"NonPreemptingPriority":                          {Default: true, LockedToDefaultInVersion: "1.24", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.26"}},
	"NodeSwap":                                       {},
	"OpenAPIEnums":                                   {},
	"OpenAPIV3":                                      {Default: true, LockedToDefaultInVersion: "1.27", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.29"}},
	"PersistentVolumeLastPhaseTransitionTime":        {VersionRange: versionutils.VersionRange{AddedInVersion: "1.28"}},
	"PDBUnhealthyPodEvictionPolicy":                  {VersionRange: versionutils.VersionRange{AddedInVersion: "1.26"}},
	"PodAndContainerStatsFromCRI":                    {},
	"PodAffinityNamespaceSelector":                   {Default: true, LockedToDefaultInVersion: "1.24", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.26"}},
	"PodDeletionCost":                                {},
	"PodDisruptionConditions":                        {},
	"PodHasNetworkCondition":                         {VersionRange: versionutils.VersionRange{RemovedInVersion: "1.28"}},
	"PodHostIPs":                                     {VersionRange: versionutils.VersionRange{AddedInVersion: "1.28"}},
	"PodIndexLabel":                                  {VersionRange: versionutils.VersionRange{AddedInVersion: "1.28"}},
	"PodLifecycleSleepAction":                        {VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"PodOverhead":                                    {Default: true, LockedToDefaultInVersion: "1.24", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.26"}},
	"PodReadyToStartContainersCondition":             {VersionRange: versionutils.VersionRange{AddedInVersion: "1.28"}},
	"PodSchedulingReadiness":                         {VersionRange: versionutils.VersionRange{AddedInVersion: "1.26"}},
	"PodSecurity":                                    {Default: true, LockedToDefaultInVersion: "1.25", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.28"}},
	"PreferNominatedNode":                            {Default: true, LockedToDefaultInVersion: "1.24", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.26"}}, // Missing from docu?
	"ProbeTerminationGracePeriod":                    {Default: true, LockedToDefaultInVersion: "1.28", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.29"}},
	"ProcMountType":                                  {},
	"ProxyTerminatingEndpoints":                      {Default: true, LockedToDefaultInVersion: "1.28"},
	"QOSReserved":                                    {},
	"ReadWriteOncePod":                               {Default: true, LockedToDefaultInVersion: "1.29"},
	"RecoverVolumeExpansionFailure":                  {},
	"RemainingItemCount":                             {Default: true, LockedToDefaultInVersion: "1.29"},
	"RemoveSelfLink":                                 {Default: true, LockedToDefaultInVersion: "1.24"},
	"RetroactiveDefaultStorageClass":                 {Default: true, VersionRange: versionutils.VersionRange{RemovedInVersion: "1.29"}, LockedToDefaultInVersion: "1.28"},
	"RotateKubeletServerCertificate":                 {},
	"RuntimeClassInImageCriApi":                      {VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"SchedulerQueueingHints":                         {VersionRange: versionutils.VersionRange{AddedInVersion: "1.28"}},
	"SeccompDefault":                                 {Default: true, LockedToDefaultInVersion: "1.27", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.29"}},
	"SecurityContextDeny":                            {VersionRange: versionutils.VersionRange{AddedInVersion: "1.27"}},
	"SeparateTaintEvictionController":                {VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"SELinuxMountReadWriteOncePod":                   {Default: true, LockedToDefaultInVersion: "1.29"},
	"ServerSideApply":                                {Default: true, LockedToDefaultInVersion: "1.26"},
	"ServerSideFieldValidation":                      {Default: true, LockedToDefaultInVersion: "1.27"},
	"ServiceAccountTokenJTI":                         {VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"ServiceAccountTokenNodeBinding":                 {VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"ServiceAccountTokenNodeBindingValidation":       {VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"ServiceAccountTokenPodNodeInfo":                 {VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"ServiceInternalTrafficPolicy":                   {Default: true, LockedToDefaultInVersion: "1.26", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.28"}},
	"ServiceIPStaticSubrange":                        {Default: true, LockedToDefaultInVersion: "1.26", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.28"}},
	"ServiceLBNodePortControl":                       {Default: true, LockedToDefaultInVersion: "1.24", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.26"}},
	"ServiceLoadBalancerClass":                       {Default: true, LockedToDefaultInVersion: "1.24", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.26"}},
	"ServiceNodePortStaticSubrange":                  {Default: true, LockedToDefaultInVersion: "1.29", VersionRange: versionutils.VersionRange{AddedInVersion: "1.27"}},
	"SidecarContainers":                              {VersionRange: versionutils.VersionRange{AddedInVersion: "1.28"}},
	"SizeMemoryBackedVolumes":                        {},
	"SkipReadOnlyValidationGCE":                      {VersionRange: versionutils.VersionRange{AddedInVersion: "1.28"}},
	"StableLoadBalancerNodeSet":                      {VersionRange: versionutils.VersionRange{AddedInVersion: "1.27"}},
	"StatefulSetAutoDeletePVC":                       {},
	"StatefulSetMinReadySeconds":                     {Default: true, LockedToDefaultInVersion: "1.25", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.27"}},
	"StatefulSetStartOrdinal":                        {VersionRange: versionutils.VersionRange{AddedInVersion: "1.26"}},
	"StorageVersionAPI":                              {},
	"StorageVersionHash":                             {},
	"StructuredAuthenticationConfiguration":          {VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"StructuredAuthorizationConfiguration":           {VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"SuspendJob":                                     {Default: true, LockedToDefaultInVersion: "1.24", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.26"}},
	"TopologyAwareHints":                             {},
	"TopologyManager":                                {Default: true, LockedToDefaultInVersion: "1.27", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.29"}},
	"TopologyManagerPolicyAlphaOptions":              {VersionRange: versionutils.VersionRange{AddedInVersion: "1.26"}},
	"TopologyManagerPolicyBetaOptions":               {VersionRange: versionutils.VersionRange{AddedInVersion: "1.26"}},
	"TopologyManagerPolicyOptions":                   {VersionRange: versionutils.VersionRange{AddedInVersion: "1.26"}},
	"TranslateStreamCloseWebsocketRequests":          {VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"UnauthenticatedHTTP2DOSMitigation":              {},
	"UnknownVersionInteroperabilityProxy":            {VersionRange: versionutils.VersionRange{AddedInVersion: "1.28"}},
	"UserNamespacesPodSecurityStandards":             {VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"UserNamespacesStatelessPodsSupport":             {VersionRange: versionutils.VersionRange{RemovedInVersion: "1.28"}},
	"UserNamespacesSupport":                          {VersionRange: versionutils.VersionRange{AddedInVersion: "1.28"}},
	"ValidatingAdmissionPolicy":                      {VersionRange: versionutils.VersionRange{AddedInVersion: "1.26"}},
	"VolumeAttributesClass":                          {VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"VolumeCapacityPriority":                         {},
	"WatchBookmark":                                  {Default: true, LockedToDefaultInVersion: "1.17"},
	"WatchList":                                      {VersionRange: versionutils.VersionRange{AddedInVersion: "1.27"}},
	"WinDSR":                                         {},
	"WinOverlay":                                     {},
	"WindowsHostNetwork":                             {VersionRange: versionutils.VersionRange{AddedInVersion: "1.26"}},
	"WindowsHostProcessContainers":                   {Default: true, LockedToDefaultInVersion: "1.26", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.28"}},
	"ZeroLimitedNominalConcurrencyShares":            {VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
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
	LockedToDefaultInVersion string
	versionutils.VersionRange
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
