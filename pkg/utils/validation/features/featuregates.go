// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package features

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/validation/field"

	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

// featureGateVersionRanges contains the version ranges for all Kubernetes feature gates.
// Extracted from https://raw.githubusercontent.com/kubernetes/kubernetes/release-${version}/test/compatibility_lifecycle/reference/versioned_feature_list.yaml.
// To maintain this list for each new Kubernetes version:
//   - Run `hack/compare-k8s-feature-gates.sh <old-version> <new-version>` (e.g. `hack/compare-k8s-feature-gates.sh v1.33 v1.34`).
//     It will present 3 lists of feature gates: those added and those removed in `<new-version>` compared to `<old-version>` and
//     feature gates that got locked to default in `<new-version>`.
//   - Add all added feature gates to the map with `<new-version>` as `AddedInVersion` and no `RemovedInVersion`.
//   - For any removed feature gates, add `<new-version>` as `RemovedInVersion` to the already existing feature gate in the map.
//   - For feature gates locked to default, add `<new-version>` as `LockedToDefaultInVersion` and the corresponding `LockedValue` to the already existing feature gate in the map.
var featureGateVersionRanges = map[string]*FeatureGateVersionRange{
	// These are special feature gates to toggle all alpha or beta feature gates on and off.
	// They were introduced in version 1.17 (although they are absent from the corresponding kube_features.go file).
	"AllAlpha": {VersionRange: versionutils.VersionRange{AddedInVersion: "1.17"}},
	"AllBeta":  {VersionRange: versionutils.VersionRange{AddedInVersion: "1.17"}},

	"AdmissionWebhookMatchConditions":                     {LockedValue: true, LockedToDefaultInVersion: "1.30", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.33"}},
	"AggregatedDiscoveryRemoveBetaType":                   {VersionRange: versionutils.VersionRange{AddedInVersion: "1.33"}},
	"AllowDNSOnlyNodeCSR":                                 {VersionRange: versionutils.VersionRange{AddedInVersion: "1.31"}},
	"AllowInsecureKubeletCertificateSigningRequests":      {VersionRange: versionutils.VersionRange{AddedInVersion: "1.31"}},
	"AllowOverwriteTerminationGracePeriodSeconds":         {VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
	"AllowServiceLBStatusOnNonLB":                         {LockedValue: false, LockedToDefaultInVersion: "1.32", VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"AllowUnsafeMalformedObjectDeletion":                  {VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
	"AllowParsingUserUIDFromCertAuth":                     {VersionRange: versionutils.VersionRange{AddedInVersion: "1.33"}},
	"APIListChunking":                                     {LockedValue: true, LockedToDefaultInVersion: "1.29", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.33"}},
	"APIPriorityAndFairness":                              {LockedValue: true, LockedToDefaultInVersion: "1.29", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.31"}},
	"APIResponseCompression":                              {},
	"APISelfSubjectReview":                                {LockedValue: true, LockedToDefaultInVersion: "1.28", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.30"}},
	"APIServerIdentity":                                   {},
	"APIServerTracing":                                    {LockedValue: true, LockedToDefaultInVersion: "1.34"},
	"APIServingWithRoutine":                               {VersionRange: versionutils.VersionRange{AddedInVersion: "1.30"}},
	"AggregatedDiscoveryEndpoint":                         {LockedValue: true, LockedToDefaultInVersion: "1.30", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.33"}},
	"AnonymousAuthConfigurableEndpoints":                  {LockedValue: true, LockedToDefaultInVersion: "1.34", VersionRange: versionutils.VersionRange{AddedInVersion: "1.31"}},
	"AnyVolumeDataSource":                                 {LockedValue: true, LockedToDefaultInVersion: "1.33"},
	"AppArmor":                                            {LockedValue: true, LockedToDefaultInVersion: "1.31", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.33"}},
	"AppArmorFields":                                      {LockedValue: true, LockedToDefaultInVersion: "1.31", VersionRange: versionutils.VersionRange{AddedInVersion: "1.30", RemovedInVersion: "1.33"}},
	"AuthorizeNodeWithSelectors":                          {LockedValue: true, LockedToDefaultInVersion: "1.34", VersionRange: versionutils.VersionRange{AddedInVersion: "1.31"}},
	"AuthorizeWithSelectors":                              {LockedValue: true, LockedToDefaultInVersion: "1.34", VersionRange: versionutils.VersionRange{AddedInVersion: "1.31"}},
	"BtreeWatchCache":                                     {LockedValue: true, LockedToDefaultInVersion: "1.33", VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
	"CBORServingAndStorage":                               {VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
	"ClearingNominatedNodeNameAfterBinding":               {VersionRange: versionutils.VersionRange{AddedInVersion: "1.34"}},
	"CloudControllerManagerWebhook":                       {},
	"CloudDualStackNodeIPs":                               {LockedValue: true, LockedToDefaultInVersion: "1.30", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.32"}},
	"ClusterTrustBundle":                                  {},
	"ClusterTrustBundleProjection":                        {VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"ComponentFlagz":                                      {VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
	"ComponentSLIs":                                       {},
	"ComponentStatusz":                                    {VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
	"ConcurrentWatchObjectDecode":                         {VersionRange: versionutils.VersionRange{AddedInVersion: "1.31"}},
	"ContainerCheckpoint":                                 {},
	"ContainerRestartRules":                               {VersionRange: versionutils.VersionRange{AddedInVersion: "1.34"}},
	"ContainerStopSignals":                                {VersionRange: versionutils.VersionRange{AddedInVersion: "1.33"}},
	"ContextualLogging":                                   {LockedValue: true, LockedToDefaultInVersion: "1.30"},
	"ConsistentHTTPGetHandlers":                           {LockedValue: true, LockedToDefaultInVersion: "1.30", VersionRange: versionutils.VersionRange{AddedInVersion: "1.26", RemovedInVersion: "1.31"}},
	"ConsistentListFromCache":                             {LockedValue: true, LockedToDefaultInVersion: "1.34", VersionRange: versionutils.VersionRange{AddedInVersion: "1.28"}},
	"CoordinatedLeaderElection":                           {VersionRange: versionutils.VersionRange{AddedInVersion: "1.31"}},
	"CPUManager":                                          {LockedValue: true, LockedToDefaultInVersion: "1.26", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.33"}},
	"CPUManagerPolicyAlphaOptions":                        {},
	"CPUManagerPolicyBetaOptions":                         {},
	"CPUManagerPolicyOptions":                             {LockedValue: true, LockedToDefaultInVersion: "1.33"},
	"CRDValidationRatcheting":                             {LockedValue: true, LockedToDefaultInVersion: "1.33", VersionRange: versionutils.VersionRange{AddedInVersion: "1.28"}},
	"CronJobsScheduledAnnotation":                         {LockedValue: true, LockedToDefaultInVersion: "1.32", VersionRange: versionutils.VersionRange{AddedInVersion: "1.28"}},
	"CrossNamespaceVolumeDataSource":                      {},
	"CSIMigrationAzureFile":                               {LockedValue: true, LockedToDefaultInVersion: "1.27", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.30"}},
	"CSIMigrationPortworx":                                {LockedValue: true, LockedToDefaultInVersion: "1.33"},
	"CSIMigrationRBD":                                     {VersionRange: versionutils.VersionRange{RemovedInVersion: "1.31"}},
	"CSINodeExpandSecret":                                 {LockedValue: true, LockedToDefaultInVersion: "1.29", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.31"}},
	"CSIVolumeHealth":                                     {},
	"CustomCPUCFSQuotaPeriod":                             {},
	"CustomResourceFieldSelectors":                        {LockedValue: true, LockedToDefaultInVersion: "1.32", VersionRange: versionutils.VersionRange{AddedInVersion: "1.30"}},
	"CustomResourceValidationExpressions":                 {LockedValue: true, LockedToDefaultInVersion: "1.29", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.31"}},
	"DeclarativeValidation":                               {VersionRange: versionutils.VersionRange{AddedInVersion: "1.33"}},
	"DeclarativeValidationTakeover":                       {VersionRange: versionutils.VersionRange{AddedInVersion: "1.33"}},
	"DefaultHostNetworkHostPortsInPodTemplates":           {VersionRange: versionutils.VersionRange{AddedInVersion: "1.28", RemovedInVersion: "1.31"}},
	"DeploymentReplicaSetTerminatingReplicas":             {VersionRange: versionutils.VersionRange{AddedInVersion: "1.33"}},
	"DetectCacheInconsistency":                            {VersionRange: versionutils.VersionRange{AddedInVersion: "1.34"}},
	"DevicePluginCDIDevices":                              {LockedValue: true, LockedToDefaultInVersion: "1.31", VersionRange: versionutils.VersionRange{AddedInVersion: "1.28", RemovedInVersion: "1.34"}},
	"DisableAllocatorDualWrite":                           {VersionRange: versionutils.VersionRange{AddedInVersion: "1.31"}},
	"DisableCloudProviders":                               {LockedValue: true, LockedToDefaultInVersion: "1.31", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.33"}},
	"DisableCPUQuotaWithExclusiveCPUs":                    {VersionRange: versionutils.VersionRange{AddedInVersion: "1.33"}},
	"DisableKubeletCloudCredentialProviders":              {LockedValue: true, LockedToDefaultInVersion: "1.31", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.33"}},
	"DisableNodeKubeProxyVersion":                         {VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"DRAAdminAccess":                                      {VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
	"DRAConsumableCapacity":                               {VersionRange: versionutils.VersionRange{AddedInVersion: "1.34"}},
	"DRAControlPlaneController":                           {VersionRange: versionutils.VersionRange{AddedInVersion: "1.26", RemovedInVersion: "1.32"}},
	"DRADeviceBindingConditions":                          {VersionRange: versionutils.VersionRange{AddedInVersion: "1.34"}},
	"DRADeviceTaints":                                     {VersionRange: versionutils.VersionRange{AddedInVersion: "1.33"}},
	"DRAExtendedResource":                                 {VersionRange: versionutils.VersionRange{AddedInVersion: "1.34"}},
	"DRAPartitionableDevices":                             {VersionRange: versionutils.VersionRange{AddedInVersion: "1.33"}},
	"DRAPrioritizedList":                                  {VersionRange: versionutils.VersionRange{AddedInVersion: "1.33"}},
	"DRAResourceClaimDeviceStatus":                        {VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
	"DRASchedulerFilterTimeout":                           {VersionRange: versionutils.VersionRange{AddedInVersion: "1.34"}},
	"DynamicResourceAllocation":                           {},
	"EfficientWatchResumption":                            {LockedValue: true, LockedToDefaultInVersion: "1.24", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.33"}},
	"ElasticIndexedJob":                                   {LockedValue: true, LockedToDefaultInVersion: "1.31", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.34"}},
	"EnvFiles":                                            {VersionRange: versionutils.VersionRange{AddedInVersion: "1.34"}},
	"EventedPLEG":                                         {},
	"ExecProbeTimeout":                                    {},
	"ExpandedDNSConfig":                                   {LockedValue: true, LockedToDefaultInVersion: "1.28", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.30"}},
	"ExperimentalHostUserNamespaceDefaulting":             {LockedValue: false, LockedToDefaultInVersion: "1.28", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.30"}},
	"ExternalServiceAccountTokenSigner":                   {VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
	"GitRepoVolumeDriver":                                 {VersionRange: versionutils.VersionRange{AddedInVersion: "1.33"}},
	"GracefulNodeShutdown":                                {},
	"GracefulNodeShutdownBasedOnPodPriority":              {},
	"HonorPVReclaimPolicy":                                {LockedValue: true, LockedToDefaultInVersion: "1.33"},
	"HostnameOverride":                                    {VersionRange: versionutils.VersionRange{AddedInVersion: "1.34"}},
	"HPAConfigurableTolerance":                            {VersionRange: versionutils.VersionRange{AddedInVersion: "1.33"}},
	"HPAContainerMetrics":                                 {LockedValue: true, LockedToDefaultInVersion: "1.30", VersionRange: versionutils.VersionRange{AddedInVersion: "1.20", RemovedInVersion: "1.32"}},
	"HPAScaleToZero":                                      {},
	"IPTablesOwnershipCleanup":                            {LockedValue: true, LockedToDefaultInVersion: "1.28", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.30"}},
	"ImageMaximumGCAge":                                   {VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"ImageVolume":                                         {VersionRange: versionutils.VersionRange{AddedInVersion: "1.31"}},
	"InPlacePodVerticalScaling":                           {VersionRange: versionutils.VersionRange{AddedInVersion: "1.27"}},
	"InPlacePodVerticalScalingAllocatedStatus":            {VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
	"InPlacePodVerticalScalingExclusiveCPUs":              {VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
	"InPlacePodVerticalScalingExclusiveMemory":            {VersionRange: versionutils.VersionRange{AddedInVersion: "1.34"}},
	"InTreePluginAWSUnregister":                           {VersionRange: versionutils.VersionRange{RemovedInVersion: "1.31"}},
	"InTreePluginAzureDiskUnregister":                     {VersionRange: versionutils.VersionRange{RemovedInVersion: "1.31"}},
	"InTreePluginAzureFileUnregister":                     {VersionRange: versionutils.VersionRange{RemovedInVersion: "1.31"}},
	"InTreePluginGCEUnregister":                           {VersionRange: versionutils.VersionRange{RemovedInVersion: "1.31"}},
	"InTreePluginOpenStackUnregister":                     {VersionRange: versionutils.VersionRange{RemovedInVersion: "1.31"}},
	"InTreePluginPortworxUnregister":                      {},
	"InTreePluginRBDUnregister":                           {VersionRange: versionutils.VersionRange{RemovedInVersion: "1.31"}},
	"InTreePluginvSphereUnregister":                       {VersionRange: versionutils.VersionRange{RemovedInVersion: "1.31"}},
	"InformerResourceVersion":                             {VersionRange: versionutils.VersionRange{AddedInVersion: "1.30"}},
	"JobBackoffLimitPerIndex":                             {LockedValue: true, LockedToDefaultInVersion: "1.33", VersionRange: versionutils.VersionRange{AddedInVersion: "1.28"}},
	"JobManagedBy":                                        {VersionRange: versionutils.VersionRange{AddedInVersion: "1.30"}},
	"JobPodFailurePolicy":                                 {LockedValue: true, LockedToDefaultInVersion: "1.31", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.33"}},
	"JobPodReplacementPolicy":                             {LockedValue: true, LockedToDefaultInVersion: "1.34", VersionRange: versionutils.VersionRange{AddedInVersion: "1.28"}},
	"JobReadyPods":                                        {LockedValue: true, LockedToDefaultInVersion: "1.29", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.31"}},
	"JobSuccessPolicy":                                    {LockedValue: true, LockedToDefaultInVersion: "1.33", VersionRange: versionutils.VersionRange{AddedInVersion: "1.30"}},
	"KMSv1":                                               {VersionRange: versionutils.VersionRange{AddedInVersion: "1.28"}},
	"KMSv2":                                               {LockedValue: true, LockedToDefaultInVersion: "1.29", VersionRange: versionutils.VersionRange{AddedInVersion: "1.25", RemovedInVersion: "1.32"}},
	"KMSv2KDF":                                            {LockedValue: true, LockedToDefaultInVersion: "1.29", VersionRange: versionutils.VersionRange{AddedInVersion: "1.28", RemovedInVersion: "1.32"}},
	"KubeletEnsureSecretPulledImages":                     {VersionRange: versionutils.VersionRange{AddedInVersion: "1.33"}},
	"KubeletCgroupDriverFromCRI":                          {LockedValue: true, LockedToDefaultInVersion: "1.34", VersionRange: versionutils.VersionRange{AddedInVersion: "1.28"}},
	"KubeletCrashLoopBackOffMax":                          {VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
	"KubeletFineGrainedAuthz":                             {VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
	"KubeletInUserNamespace":                              {},
	"KubeletPodResources":                                 {LockedValue: true, LockedToDefaultInVersion: "1.28", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.30"}},
	"KubeletPodResourcesDynamicResources":                 {},
	"KubeletPodResourcesListUseActivePods":                {VersionRange: versionutils.VersionRange{AddedInVersion: "1.34"}},
	"KubeletPodResourcesGet":                              {},
	"KubeletPodResourcesGetAllocatable":                   {LockedValue: true, LockedToDefaultInVersion: "1.28", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.30"}},
	"KubeletPSI":                                          {VersionRange: versionutils.VersionRange{AddedInVersion: "1.33"}},
	"KubeletRegistrationGetOnExistsOnly":                  {VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
	"KubeletSeparateDiskGC":                               {VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"KubeletServiceAccountTokenForCredentialProviders":    {VersionRange: versionutils.VersionRange{AddedInVersion: "1.33"}},
	"KubeletTracing":                                      {LockedValue: true, LockedToDefaultInVersion: "1.34"},
	"KubeProxyDrainingTerminatingNodes":                   {LockedValue: true, LockedToDefaultInVersion: "1.31", VersionRange: versionutils.VersionRange{AddedInVersion: "1.28", RemovedInVersion: "1.33"}},
	"LegacyServiceAccountTokenCleanUp":                    {LockedValue: true, LockedToDefaultInVersion: "1.30", VersionRange: versionutils.VersionRange{AddedInVersion: "1.28", RemovedInVersion: "1.32"}},
	"LegacyServiceAccountTokenTracking":                   {LockedValue: true, LockedToDefaultInVersion: "1.28", VersionRange: versionutils.VersionRange{AddedInVersion: "1.26", RemovedInVersion: "1.30"}},
	"LegacySidecarContainers":                             {VersionRange: versionutils.VersionRange{AddedInVersion: "1.33", RemovedInVersion: "1.34"}},
	"LoadBalancerIPMode":                                  {LockedValue: true, LockedToDefaultInVersion: "1.32", VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"ListFromCacheSnapshot":                               {VersionRange: versionutils.VersionRange{AddedInVersion: "1.33"}},
	"LocalStorageCapacityIsolationFSQuotaMonitoring":      {},
	"LogarithmicScaleDown":                                {LockedValue: true, LockedToDefaultInVersion: "1.31"},
	"MatchLabelKeysInPodAffinity":                         {LockedValue: true, LockedToDefaultInVersion: "1.33", VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"MatchLabelKeysInPodTopologySpread":                   {},
	"MatchLabelKeysInPodTopologySpreadSelectorMerge":      {VersionRange: versionutils.VersionRange{AddedInVersion: "1.34"}},
	"MaxUnavailableStatefulSet":                           {},
	"MemoryManager":                                       {LockedValue: true, LockedToDefaultInVersion: "1.32", VersionRange: versionutils.VersionRange{AddedInVersion: "1.21"}},
	"MemoryQoS":                                           {},
	"MinDomainsInPodTopologySpread":                       {LockedValue: true, LockedToDefaultInVersion: "1.30", VersionRange: versionutils.VersionRange{AddedInVersion: "1.24", RemovedInVersion: "1.32"}},
	"MinimizeIPTablesRestore":                             {LockedValue: true, LockedToDefaultInVersion: "1.28", VersionRange: versionutils.VersionRange{AddedInVersion: "1.26", RemovedInVersion: "1.30"}},
	"MultiCIDRServiceAllocator":                           {LockedValue: true, LockedToDefaultInVersion: "1.34"},
	"MutableCSINodeAllocatableCount":                      {VersionRange: versionutils.VersionRange{AddedInVersion: "1.33"}},
	"MutatingAdmissionPolicy":                             {VersionRange: versionutils.VersionRange{AddedInVersion: "1.30"}},
	"NewVolumeManagerReconstruction":                      {LockedValue: true, LockedToDefaultInVersion: "1.30", VersionRange: versionutils.VersionRange{AddedInVersion: "1.25", RemovedInVersion: "1.32"}},
	"NFTablesProxyMode":                                   {LockedValue: true, LockedToDefaultInVersion: "1.33", VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"NodeInclusionPolicyInPodTopologySpread":              {LockedValue: true, LockedToDefaultInVersion: "1.33"},
	"NodeLogQuery":                                        {},
	"NodeOutOfServiceVolumeDetach":                        {LockedValue: true, LockedToDefaultInVersion: "1.28", VersionRange: versionutils.VersionRange{AddedInVersion: "1.24", RemovedInVersion: "1.32"}},
	"NodeSwap":                                            {LockedValue: true, LockedToDefaultInVersion: "1.34"},
	"NominatedNodeNameForExpectation":                     {VersionRange: versionutils.VersionRange{AddedInVersion: "1.34"}},
	"OpenAPIEnums":                                        {},
	"OrderedNamespaceDeletion":                            {LockedValue: true, LockedToDefaultInVersion: "1.34", VersionRange: versionutils.VersionRange{AddedInVersion: "1.33"}},
	"PDBUnhealthyPodEvictionPolicy":                       {LockedValue: true, LockedToDefaultInVersion: "1.31", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.33"}},
	"PersistentVolumeLastPhaseTransitionTime":             {LockedValue: true, LockedToDefaultInVersion: "1.31", VersionRange: versionutils.VersionRange{AddedInVersion: "1.28", RemovedInVersion: "1.33"}},
	"PodAndContainerStatsFromCRI":                         {},
	"PodCertificateRequest":                               {VersionRange: versionutils.VersionRange{AddedInVersion: "1.34"}},
	"PodDeletionCost":                                     {},
	"PodDisruptionConditions":                             {LockedValue: true, LockedToDefaultInVersion: "1.31", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.34"}},
	"PodHostIPs":                                          {LockedValue: true, LockedToDefaultInVersion: "1.30", VersionRange: versionutils.VersionRange{AddedInVersion: "1.28", RemovedInVersion: "1.32"}},
	"PodIndexLabel":                                       {LockedValue: true, LockedToDefaultInVersion: "1.32", VersionRange: versionutils.VersionRange{AddedInVersion: "1.28"}},
	"PodLevelResources":                                   {VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
	"PodLifecycleSleepAction":                             {LockedValue: true, LockedToDefaultInVersion: "1.34", VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"PodLifecycleSleepActionAllowZero":                    {LockedValue: true, LockedToDefaultInVersion: "1.34", VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
	"PodLogsQuerySplitStreams":                            {VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
	"PodObservedGenerationTracking":                       {VersionRange: versionutils.VersionRange{AddedInVersion: "1.33"}},
	"PodReadyToStartContainersCondition":                  {VersionRange: versionutils.VersionRange{AddedInVersion: "1.28"}},
	"PodSchedulingReadiness":                              {LockedValue: true, LockedToDefaultInVersion: "1.30"},
	"PodTopologyLabelsAdmission":                          {VersionRange: versionutils.VersionRange{AddedInVersion: "1.33"}},
	"PortForwardWebsockets":                               {VersionRange: versionutils.VersionRange{AddedInVersion: "1.30"}},
	"PreferAlignCpusByUncoreCache":                        {VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
	"PreferSameTrafficDistribution":                       {VersionRange: versionutils.VersionRange{AddedInVersion: "1.33"}},
	"PreventStaticPodAPIReferences":                       {VersionRange: versionutils.VersionRange{AddedInVersion: "1.34"}},
	"ProbeHostPodSecurityStandards":                       {LockedValue: true, LockedToDefaultInVersion: "1.34", VersionRange: versionutils.VersionRange{AddedInVersion: "1.34"}},
	"ProcMountType":                                       {},
	"ProxyTerminatingEndpoints":                           {LockedValue: true, LockedToDefaultInVersion: "1.28", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.30"}},
	"QOSReserved":                                         {},
	"ReadWriteOncePod":                                    {LockedValue: true, LockedToDefaultInVersion: "1.29", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.31"}},
	"RecoverVolumeExpansionFailure":                       {LockedValue: true, LockedToDefaultInVersion: "1.34", VersionRange: versionutils.VersionRange{AddedInVersion: "1.28"}},
	"RecursiveReadOnlyMounts":                             {LockedValue: true, LockedToDefaultInVersion: "1.33", VersionRange: versionutils.VersionRange{AddedInVersion: "1.30"}},
	"ReduceDefaultCrashLoopBackOffDecay":                  {VersionRange: versionutils.VersionRange{AddedInVersion: "1.33"}},
	"RelaxedDNSSearchValidation":                          {LockedValue: true, LockedToDefaultInVersion: "1.34", VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
	"RelaxedEnvironmentVariableValidation":                {LockedValue: true, LockedToDefaultInVersion: "1.34", VersionRange: versionutils.VersionRange{AddedInVersion: "1.30"}},
	"RelaxedServiceNameValidation":                        {VersionRange: versionutils.VersionRange{AddedInVersion: "1.34"}},
	"ReloadKubeletServerCertificateFile":                  {VersionRange: versionutils.VersionRange{AddedInVersion: "1.31"}},
	"RemainingItemCount":                                  {LockedValue: true, LockedToDefaultInVersion: "1.29", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.33"}},
	"RemoteRequestHeaderUID":                              {VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
	"RemoveSelfLink":                                      {LockedValue: true, LockedToDefaultInVersion: "1.24", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.30"}},
	"ResilientWatchCacheInitialization":                   {LockedValue: true, LockedToDefaultInVersion: "1.34", VersionRange: versionutils.VersionRange{AddedInVersion: "1.31"}},
	"ResourceHealthStatus":                                {VersionRange: versionutils.VersionRange{AddedInVersion: "1.31"}},
	"RetryGenerateName":                                   {LockedValue: true, LockedToDefaultInVersion: "1.32", VersionRange: versionutils.VersionRange{AddedInVersion: "1.30"}},
	"RotateKubeletServerCertificate":                      {},
	"RuntimeClassInImageCriApi":                           {VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"SchedulerAsyncAPICalls":                              {VersionRange: versionutils.VersionRange{AddedInVersion: "1.34"}},
	"SchedulerAsyncPreemption":                            {VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
	"SchedulerPopFromBackoffQ":                            {VersionRange: versionutils.VersionRange{AddedInVersion: "1.33"}},
	"SchedulerQueueingHints":                              {LockedValue: true, LockedToDefaultInVersion: "1.34", VersionRange: versionutils.VersionRange{AddedInVersion: "1.28"}},
	"SecurityContextDeny":                                 {VersionRange: versionutils.VersionRange{RemovedInVersion: "1.30"}},
	"SELinuxChangePolicy":                                 {VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
	"SELinuxMount":                                        {VersionRange: versionutils.VersionRange{AddedInVersion: "1.30"}},
	"SELinuxMountReadWriteOncePod":                        {LockedValue: true, LockedToDefaultInVersion: "1.29"},
	"SeparateCacheWatchRPC":                               {VersionRange: versionutils.VersionRange{AddedInVersion: "1.28"}},
	"SeparateTaintEvictionController":                     {LockedValue: true, LockedToDefaultInVersion: "1.34", VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"ServerSideApply":                                     {LockedValue: true, LockedToDefaultInVersion: "1.22", VersionRange: versionutils.VersionRange{AddedInVersion: "1.14", RemovedInVersion: "1.32"}},
	"ServerSideFieldValidation":                           {LockedValue: true, LockedToDefaultInVersion: "1.27", VersionRange: versionutils.VersionRange{AddedInVersion: "1.23", RemovedInVersion: "1.32"}},
	"ServiceAccountNodeAudienceRestriction":               {VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
	"ServiceAccountTokenJTI":                              {LockedValue: true, LockedToDefaultInVersion: "1.32", VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"ServiceAccountTokenNodeBinding":                      {LockedValue: true, LockedToDefaultInVersion: "1.33", VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"ServiceAccountTokenNodeBindingValidation":            {LockedValue: true, LockedToDefaultInVersion: "1.32", VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"ServiceAccountTokenPodNodeInfo":                      {LockedValue: true, LockedToDefaultInVersion: "1.32", VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"ServiceNodePortStaticSubrange":                       {LockedValue: true, LockedToDefaultInVersion: "1.29", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.31"}},
	"ServiceTrafficDistribution":                          {LockedValue: true, LockedToDefaultInVersion: "1.33", VersionRange: versionutils.VersionRange{AddedInVersion: "1.30"}},
	"SidecarContainers":                                   {LockedValue: true, LockedToDefaultInVersion: "1.33", VersionRange: versionutils.VersionRange{AddedInVersion: "1.28"}},
	"SizeBasedListCostEstimate":                           {VersionRange: versionutils.VersionRange{AddedInVersion: "1.34"}},
	"SizeMemoryBackedVolumes":                             {LockedValue: true, LockedToDefaultInVersion: "1.32", VersionRange: versionutils.VersionRange{AddedInVersion: "1.20"}},
	"SkipReadOnlyValidationGCE":                           {VersionRange: versionutils.VersionRange{AddedInVersion: "1.28", RemovedInVersion: "1.31"}},
	"StableLoadBalancerNodeSet":                           {LockedValue: true, LockedToDefaultInVersion: "1.30", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.32"}},
	"StatefulSetAutoDeletePVC":                            {LockedValue: true, LockedToDefaultInVersion: "1.32", VersionRange: versionutils.VersionRange{AddedInVersion: "1.23"}},
	"StatefulSetStartOrdinal":                             {LockedValue: true, LockedToDefaultInVersion: "1.31", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.34"}},
	"StorageCapacityScoring":                              {VersionRange: versionutils.VersionRange{AddedInVersion: "1.33"}},
	"StorageNamespaceIndex":                               {VersionRange: versionutils.VersionRange{AddedInVersion: "1.30"}},
	"StorageVersionAPI":                                   {},
	"StorageVersionHash":                                  {},
	"StorageVersionMigrator":                              {VersionRange: versionutils.VersionRange{AddedInVersion: "1.30"}},
	"StreamingCollectionEncodingToJSON":                   {LockedValue: true, LockedToDefaultInVersion: "1.34", VersionRange: versionutils.VersionRange{AddedInVersion: "1.33"}},
	"StreamingCollectionEncodingToProtobuf":               {LockedValue: true, LockedToDefaultInVersion: "1.34", VersionRange: versionutils.VersionRange{AddedInVersion: "1.33"}},
	"StrictCostEnforcementForVAP":                         {LockedValue: true, LockedToDefaultInVersion: "1.32", VersionRange: versionutils.VersionRange{AddedInVersion: "1.30"}},
	"StrictCostEnforcementForWebhooks":                    {LockedValue: true, LockedToDefaultInVersion: "1.32", VersionRange: versionutils.VersionRange{AddedInVersion: "1.30"}},
	"StrictIPCIDRValidation":                              {VersionRange: versionutils.VersionRange{AddedInVersion: "1.33"}},
	"StructuredAuthenticationConfiguration":               {LockedValue: true, LockedToDefaultInVersion: "1.34", VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"StructuredAuthenticationConfigurationEgressSelector": {VersionRange: versionutils.VersionRange{AddedInVersion: "1.34"}},
	"StructuredAuthorizationConfiguration":                {LockedValue: true, LockedToDefaultInVersion: "1.32", VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"SupplementalGroupsPolicy":                            {VersionRange: versionutils.VersionRange{AddedInVersion: "1.31"}},
	"SystemdWatchdog":                                     {VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
	"TokenRequestServiceAccountUIDValidation":             {VersionRange: versionutils.VersionRange{AddedInVersion: "1.34"}},
	"TopologyAwareHints":                                  {LockedValue: true, LockedToDefaultInVersion: "1.33"},
	"TopologyManagerPolicyAlphaOptions":                   {},
	"TopologyManagerPolicyBetaOptions":                    {},
	"TopologyManagerPolicyOptions":                        {LockedValue: true, LockedToDefaultInVersion: "1.32"},
	"TranslateStreamCloseWebsocketRequests":               {VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"UnauthenticatedHTTP2DOSMitigation":                   {},
	"UnknownVersionInteroperabilityProxy":                 {VersionRange: versionutils.VersionRange{AddedInVersion: "1.28"}},
	"UserNamespacesPodSecurityStandards":                  {VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"UserNamespacesSupport":                               {VersionRange: versionutils.VersionRange{AddedInVersion: "1.28"}},
	"ValidatingAdmissionPolicy":                           {LockedValue: true, LockedToDefaultInVersion: "1.30", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.32"}},
	"VolumeAttributesClass":                               {VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"VolumeCapacityPriority":                              {VersionRange: versionutils.VersionRange{RemovedInVersion: "1.33"}},
	"WatchBookmark":                                       {LockedValue: true, LockedToDefaultInVersion: "1.17", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.33"}},
	"WatchCacheInitializationPostStartHook":               {VersionRange: versionutils.VersionRange{AddedInVersion: "1.31"}},
	"WatchFromStorageWithoutResourceVersion":              {LockedValue: false, LockedToDefaultInVersion: "1.33"},
	"WatchList":                                           {},
	"WatchListClient":                                     {VersionRange: versionutils.VersionRange{AddedInVersion: "1.30"}},
	"WinDSR":                                              {LockedValue: true, LockedToDefaultInVersion: "1.34"},
	"WinOverlay":                                          {LockedValue: true, LockedToDefaultInVersion: "1.34"},
	"WindowsCPUAndMemoryAffinity":                         {VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
	"WindowsGracefulNodeShutdown":                         {VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
	"WindowsHostNetwork":                                  {},
	"ZeroLimitedNominalConcurrencyShares":                 {LockedValue: true, LockedToDefaultInVersion: "1.30", VersionRange: versionutils.VersionRange{AddedInVersion: "1.29", RemovedInVersion: "1.32"}},
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
	versionutils.VersionRange

	LockedValue              bool
	LockedToDefaultInVersion string
}

func isFeatureLockedToDefault(featureGate, version string) (bool, error) {
	var constraint string

	vr := featureGateVersionRanges[featureGate]
	if vr.LockedToDefaultInVersion != "" {
		constraint = ">= " + vr.LockedToDefaultInVersion
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
			allErrs = append(allErrs, field.Forbidden(fldPath.Child(featureGate), "not supported in Kubernetes version "+version))
		} else {
			isLockedToDefault, err := isFeatureLockedToDefault(featureGate, version)
			if err != nil {
				allErrs = append(allErrs, field.Invalid(fldPath.Child(featureGate), featureGate, err.Error()))
			} else if isLockedToDefault && featureGates[featureGate] != featureGateVersionRanges[featureGate].LockedValue {
				allErrs = append(allErrs, field.Forbidden(fldPath.Child(featureGate), fmt.Sprintf("cannot set feature gate to %v, feature is locked to %v", featureGates[featureGate], featureGateVersionRanges[featureGate].LockedValue)))
			}
		}
	}

	return allErrs
}
