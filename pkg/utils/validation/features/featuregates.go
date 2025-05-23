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
// Extracted from:
// - https://kubernetes.io/docs/reference/command-line-tools-reference/feature-gates/
// - https://kubernetes.io/docs/reference/command-line-tools-reference/feature-gates-removed/
// To maintain this list for each new Kubernetes version:
// Alpha & Beta Feature Gates
// 1. Open: https://kubernetes.io/docs/reference/command-line-tools-reference/feature-gates/#feature-gates-for-alpha-or-beta-features
// 2. Search the page for the new Kubernetes version, e.g. "1.32".
// 3. Add new alpha feature gates that have been added "Since" the new Kubernetes version.
// 4. Change the `Default` for Beta feature gates that have been promoted "Since" the new Kubernetes version.
//
// Graduated & Deprecated Feature Gates
// 1. Open: https://kubernetes.io/docs/reference/command-line-tools-reference/feature-gates/#feature-gates-for-graduated-or-deprecated-features
// 2. Search the page for the new Kubernetes version, e.g. "1.32".
// 3. Change `LockedToDefaultInVersion` for GA and Deprecated feature gates that have been graduated/deprecated "Since" the new Kubernetes version.
//
// Removed Feature Gates
// 1. Open: https://kubernetes.io/docs/reference/command-line-tools-reference/feature-gates-removed/#feature-gates-that-are-removed
// 2. Search the page for the _current_ Kubernetes version, e.g. if the new version is "1.32", search for "1.31".
// 3. Set `RemovedInVersion` to the _new_ Kubernetes version for feature gates that have been removed after the _current_ Kubernetes version according to the "To" column.
// TODO(marc1404): Reference the `compare-k8s-feature-gates.sh` script once it has been fixed (https://github.com/gardener/gardener/issues/11198).
var featureGateVersionRanges = map[string]*FeatureGateVersionRange{
	// These are special feature gates to toggle all alpha or beta feature gates on and off.
	// They were introduced in version 1.17 (although they are absent from the corresponding kube_features.go file).
	"AllAlpha": {VersionRange: versionutils.VersionRange{AddedInVersion: "1.17"}},
	"AllBeta":  {VersionRange: versionutils.VersionRange{AddedInVersion: "1.17"}},

	"AdmissionWebhookMatchConditions":                {Default: true, LockedToDefaultInVersion: "1.30"},
	"AllowDNSOnlyNodeCSR":                            {VersionRange: versionutils.VersionRange{AddedInVersion: "1.31"}},
	"AllowInsecureKubeletCertificateSigningRequests": {VersionRange: versionutils.VersionRange{AddedInVersion: "1.31"}},
	"AllowOverwriteTerminationGracePeriodSeconds":    {VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
	"AllowServiceLBStatusOnNonLB":                    {Default: false, LockedToDefaultInVersion: "1.32", VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"AllowUnsafeMalformedObjectDeletion":             {VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
	"APIListChunking":                                {Default: true, LockedToDefaultInVersion: "1.29"},
	"APIPriorityAndFairness":                         {Default: true, LockedToDefaultInVersion: "1.29", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.31"}},
	"APIResponseCompression":                         {},
	"APISelfSubjectReview":                           {Default: true, LockedToDefaultInVersion: "1.28", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.30"}},
	"APIServerIdentity":                              {},
	"APIServerTracing":                               {},
	"APIServingWithRoutine":                          {VersionRange: versionutils.VersionRange{AddedInVersion: "1.30"}},
	"AdvancedAuditing":                               {Default: true, LockedToDefaultInVersion: "1.27", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.28"}},
	"AggregatedDiscoveryEndpoint":                    {Default: true, LockedToDefaultInVersion: "1.30"},
	"AnonymousAuthConfigurableEndpoints":             {Default: true, VersionRange: versionutils.VersionRange{AddedInVersion: "1.31"}},
	"AnyVolumeDataSource":                            {},
	"AppArmor":                                       {Default: true, LockedToDefaultInVersion: "1.31"},
	"AppArmorFields":                                 {Default: true, LockedToDefaultInVersion: "1.31", VersionRange: versionutils.VersionRange{AddedInVersion: "1.30"}},
	"AuthorizeNodeWithSelectors":                     {Default: true, VersionRange: versionutils.VersionRange{AddedInVersion: "1.31"}},
	"AuthorizeWithSelectors":                         {Default: true, VersionRange: versionutils.VersionRange{AddedInVersion: "1.31"}},
	"BtreeWatchCache":                                {Default: true, VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
	"CBORServingAndStorage":                          {VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
	"CloudControllerManagerWebhook":                  {},
	"CloudDualStackNodeIPs":                          {Default: true, LockedToDefaultInVersion: "1.30", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.32"}},
	"ClusterTrustBundle":                             {},
	"ClusterTrustBundleProjection":                   {VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"ComponentFlagz":                                 {VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
	"ComponentSLIs":                                  {},
	"ComponentStatusz":                               {VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
	"ConcurrentWatchObjectDecode":                    {VersionRange: versionutils.VersionRange{AddedInVersion: "1.31"}},
	"ContainerCheckpoint":                            {},
	"ContextualLogging":                              {Default: true, LockedToDefaultInVersion: "1.30"},
	"ConsistentHTTPGetHandlers":                      {Default: true, LockedToDefaultInVersion: "1.30", VersionRange: versionutils.VersionRange{AddedInVersion: "1.26", RemovedInVersion: "1.31"}},
	"ConsistentListFromCache":                        {VersionRange: versionutils.VersionRange{AddedInVersion: "1.28"}},
	"CoordinatedLeaderElection":                      {VersionRange: versionutils.VersionRange{AddedInVersion: "1.31"}},
	"CPUManager":                                     {Default: true, LockedToDefaultInVersion: "1.26"},
	"CPUManagerPolicyAlphaOptions":                   {},
	"CPUManagerPolicyBetaOptions":                    {},
	"CPUManagerPolicyOptions":                        {},
	"CRDValidationRatcheting":                        {VersionRange: versionutils.VersionRange{AddedInVersion: "1.28"}},
	"CronJobsScheduledAnnotation":                    {Default: true, LockedToDefaultInVersion: "1.32", VersionRange: versionutils.VersionRange{AddedInVersion: "1.28"}},
	"CronJobTimeZone":                                {Default: true, LockedToDefaultInVersion: "1.27", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.29"}},
	"CrossNamespaceVolumeDataSource":                 {},
	"CSIMigrationAzureFile":                          {Default: true, LockedToDefaultInVersion: "1.27", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.30"}},
	"CSIMigrationGCE":                                {Default: true, LockedToDefaultInVersion: "1.25", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.28"}},
	"CSIMigrationPortworx":                           {},
	"CSIMigrationRBD":                                {VersionRange: versionutils.VersionRange{RemovedInVersion: "1.31"}},
	"CSIMigrationvSphere":                            {Default: true, LockedToDefaultInVersion: "1.27", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.29"}},
	"CSINodeExpandSecret":                            {Default: true, LockedToDefaultInVersion: "1.29", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.31"}},
	"CSIStorageCapacity":                             {Default: true, LockedToDefaultInVersion: "1.24", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.28"}},
	"CSIVolumeHealth":                                {},
	"CustomCPUCFSQuotaPeriod":                        {},
	"CustomResourceFieldSelectors":                   {Default: true, LockedToDefaultInVersion: "1.32", VersionRange: versionutils.VersionRange{AddedInVersion: "1.30"}},
	"CustomResourceValidationExpressions":            {Default: true, LockedToDefaultInVersion: "1.29", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.31"}},
	"DefaultHostNetworkHostPortsInPodTemplates":      {VersionRange: versionutils.VersionRange{AddedInVersion: "1.28", RemovedInVersion: "1.31"}},
	"DelegateFSGroupToCSIDriver":                     {Default: true, LockedToDefaultInVersion: "1.26", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.28"}},
	"DevicePluginCDIDevices":                         {Default: true, LockedToDefaultInVersion: "1.31", VersionRange: versionutils.VersionRange{AddedInVersion: "1.28"}},
	"DevicePlugins":                                  {Default: true, LockedToDefaultInVersion: "1.26", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.28"}},
	"DisableAcceleratorUsageMetrics":                 {Default: true, LockedToDefaultInVersion: "1.25", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.28"}},
	"DisableAllocatorDualWrite":                      {VersionRange: versionutils.VersionRange{AddedInVersion: "1.31"}},
	"DisableCloudProviders":                          {Default: true, LockedToDefaultInVersion: "1.31"},
	"DisableKubeletCloudCredentialProviders":         {Default: true, LockedToDefaultInVersion: "1.31"},
	"DisableNodeKubeProxyVersion":                    {VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"DownwardAPIHugePages":                           {Default: true, LockedToDefaultInVersion: "1.27", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.29"}},
	"DRAAdminAccess":                                 {VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
	"DRAControlPlaneController":                      {VersionRange: versionutils.VersionRange{AddedInVersion: "1.26", RemovedInVersion: "1.32"}},
	"DRAResourceClaimDeviceStatus":                   {VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
	"DryRun":                                         {Default: true, LockedToDefaultInVersion: "1.26", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.28"}},
	"DynamicResourceAllocation":                      {},
	"EfficientWatchResumption":                       {Default: true, LockedToDefaultInVersion: "1.24"},
	"ElasticIndexedJob":                              {Default: true, LockedToDefaultInVersion: "1.31"},
	"EndpointSliceTerminatingCondition":              {Default: true, LockedToDefaultInVersion: "1.26", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.28"}},
	"EventedPLEG":                                    {},
	"ExecProbeTimeout":                               {},
	"ExpandedDNSConfig":                              {Default: true, LockedToDefaultInVersion: "1.28", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.30"}},
	"ExperimentalHostUserNamespaceDefaulting":        {Default: false, LockedToDefaultInVersion: "1.28", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.30"}},
	"ExternalServiceAccountTokenSigner":              {VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
	"GRPCContainerProbe":                             {Default: true, LockedToDefaultInVersion: "1.27", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.29"}},
	"GracefulNodeShutdown":                           {},
	"GracefulNodeShutdownBasedOnPodPriority":         {},
	"HonorPVReclaimPolicy":                           {},
	"HPAContainerMetrics":                            {Default: true, LockedToDefaultInVersion: "1.30", VersionRange: versionutils.VersionRange{AddedInVersion: "1.20", RemovedInVersion: "1.32"}},
	"HPAScaleToZero":                                 {},
	"IPTablesOwnershipCleanup":                       {Default: true, LockedToDefaultInVersion: "1.28", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.30"}},
	"ImageMaximumGCAge":                              {VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"ImageVolume":                                    {VersionRange: versionutils.VersionRange{AddedInVersion: "1.31"}},
	"InPlacePodVerticalScaling":                      {},
	"InPlacePodVerticalScalingAllocatedStatus":       {VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
	"InPlacePodVerticalScalingExclusiveCPUs":         {VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
	"InTreePluginAWSUnregister":                      {VersionRange: versionutils.VersionRange{RemovedInVersion: "1.31"}},
	"InTreePluginAzureDiskUnregister":                {VersionRange: versionutils.VersionRange{RemovedInVersion: "1.31"}},
	"InTreePluginAzureFileUnregister":                {VersionRange: versionutils.VersionRange{RemovedInVersion: "1.31"}},
	"InTreePluginGCEUnregister":                      {VersionRange: versionutils.VersionRange{RemovedInVersion: "1.31"}},
	"InTreePluginOpenStackUnregister":                {VersionRange: versionutils.VersionRange{RemovedInVersion: "1.31"}},
	"InTreePluginPortworxUnregister":                 {},
	"InTreePluginRBDUnregister":                      {VersionRange: versionutils.VersionRange{RemovedInVersion: "1.31"}},
	"InTreePluginvSphereUnregister":                  {VersionRange: versionutils.VersionRange{RemovedInVersion: "1.31"}},
	"InformerResourceVersion":                        {VersionRange: versionutils.VersionRange{AddedInVersion: "1.30"}},
	"JobBackoffLimitPerIndex":                        {VersionRange: versionutils.VersionRange{AddedInVersion: "1.28"}},
	"JobManagedBy":                                   {Default: true, VersionRange: versionutils.VersionRange{AddedInVersion: "1.30"}},
	"JobMutableNodeSchedulingDirectives":             {Default: true, LockedToDefaultInVersion: "1.27", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.29"}},
	"JobPodFailurePolicy":                            {Default: true, LockedToDefaultInVersion: "1.31"},
	"JobPodReplacementPolicy":                        {VersionRange: versionutils.VersionRange{AddedInVersion: "1.28"}},
	"JobReadyPods":                                   {Default: true, LockedToDefaultInVersion: "1.29", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.31"}},
	"JobSuccessPolicy":                               {VersionRange: versionutils.VersionRange{AddedInVersion: "1.30"}},
	"JobTrackingWithFinalizers":                      {Default: true, LockedToDefaultInVersion: "1.26", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.29"}},
	"KMSv1":                                          {VersionRange: versionutils.VersionRange{AddedInVersion: "1.28"}},
	"KMSv2":                                          {Default: true, LockedToDefaultInVersion: "1.29", VersionRange: versionutils.VersionRange{AddedInVersion: "1.25", RemovedInVersion: "1.32"}},
	"KMSv2KDF":                                       {Default: true, LockedToDefaultInVersion: "1.29", VersionRange: versionutils.VersionRange{AddedInVersion: "1.28", RemovedInVersion: "1.32"}},
	"KubeletCgroupDriverFromCRI":                     {VersionRange: versionutils.VersionRange{AddedInVersion: "1.28"}},
	"KubeletCrashLoopBackOffMax":                     {VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
	"KubeletCredentialProviders":                     {Default: true, LockedToDefaultInVersion: "1.26", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.28"}},
	"KubeletFineGrainedAuthz":                        {VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
	"KubeletInUserNamespace":                         {},
	"KubeletPodResources":                            {Default: true, LockedToDefaultInVersion: "1.28", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.30"}},
	"KubeletPodResourcesDynamicResources":            {},
	"KubeletPodResourcesGet":                         {},
	"KubeletPodResourcesGetAllocatable":              {Default: true, LockedToDefaultInVersion: "1.28", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.30"}},
	"KubeletRegistrationGetOnExistsOnly":             {VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
	"KubeletSeparateDiskGC":                          {VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"KubeletTracing":                                 {},
	"KubeProxyDrainingTerminatingNodes":              {Default: true, LockedToDefaultInVersion: "1.31", VersionRange: versionutils.VersionRange{AddedInVersion: "1.28"}},
	"LegacyServiceAccountTokenCleanUp":               {Default: true, LockedToDefaultInVersion: "1.30", VersionRange: versionutils.VersionRange{AddedInVersion: "1.28", RemovedInVersion: "1.32"}},
	"LegacyServiceAccountTokenNoAutoGeneration":      {Default: true, LockedToDefaultInVersion: "1.27", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.29"}},
	"LegacyServiceAccountTokenTracking":              {Default: true, LockedToDefaultInVersion: "1.28", VersionRange: versionutils.VersionRange{AddedInVersion: "1.26", RemovedInVersion: "1.30"}},
	"LoadBalancerIPMode":                             {Default: true, LockedToDefaultInVersion: "1.32", VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"LocalStorageCapacityIsolationFSQuotaMonitoring": {},
	"LogarithmicScaleDown":                           {Default: true, LockedToDefaultInVersion: "1.31"},
	"MatchLabelKeysInPodAffinity":                    {VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"MatchLabelKeysInPodTopologySpread":              {},
	"MaxUnavailableStatefulSet":                      {},
	"MemoryManager":                                  {Default: true, LockedToDefaultInVersion: "1.32", VersionRange: versionutils.VersionRange{AddedInVersion: "1.21"}},
	"MemoryQoS":                                      {},
	"MinDomainsInPodTopologySpread":                  {Default: true, LockedToDefaultInVersion: "1.30", VersionRange: versionutils.VersionRange{AddedInVersion: "1.24", RemovedInVersion: "1.32"}},
	"MinimizeIPTablesRestore":                        {Default: true, LockedToDefaultInVersion: "1.28", VersionRange: versionutils.VersionRange{AddedInVersion: "1.26", RemovedInVersion: "1.30"}},
	"MixedProtocolLBService":                         {Default: true, LockedToDefaultInVersion: "1.26", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.28"}},
	"MultiCIDRRangeAllocator":                        {VersionRange: versionutils.VersionRange{RemovedInVersion: "1.29"}},
	"MultiCIDRServiceAllocator":                      {},
	"MutatingAdmissionPolicy":                        {VersionRange: versionutils.VersionRange{AddedInVersion: "1.30"}},
	"NetworkPolicyStatus":                            {VersionRange: versionutils.VersionRange{RemovedInVersion: "1.28"}},
	"NewVolumeManagerReconstruction":                 {Default: true, LockedToDefaultInVersion: "1.30", VersionRange: versionutils.VersionRange{AddedInVersion: "1.25", RemovedInVersion: "1.32"}},
	"NFTablesProxyMode":                              {VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"NodeInclusionPolicyInPodTopologySpread":         {},
	"NodeLogQuery":                                   {},
	"NodeOutOfServiceVolumeDetach":                   {Default: true, LockedToDefaultInVersion: "1.28", VersionRange: versionutils.VersionRange{AddedInVersion: "1.24", RemovedInVersion: "1.32"}},
	"NodeSwap":                                       {},
	"OpenAPIEnums":                                   {},
	"OpenAPIV3":                                      {Default: true, LockedToDefaultInVersion: "1.27", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.29"}},
	"PDBUnhealthyPodEvictionPolicy":                  {Default: true, LockedToDefaultInVersion: "1.31"},
	"PersistentVolumeLastPhaseTransitionTime":        {Default: true, LockedToDefaultInVersion: "1.31", VersionRange: versionutils.VersionRange{AddedInVersion: "1.28"}},
	"PodAndContainerStatsFromCRI":                    {},
	"PodDeletionCost":                                {},
	"PodDisruptionConditions":                        {Default: true, LockedToDefaultInVersion: "1.31"},
	"PodHasNetworkCondition":                         {VersionRange: versionutils.VersionRange{RemovedInVersion: "1.28"}},
	"PodHostIPs":                                     {Default: true, LockedToDefaultInVersion: "1.30", VersionRange: versionutils.VersionRange{AddedInVersion: "1.28", RemovedInVersion: "1.32"}},
	"PodIndexLabel":                                  {Default: true, LockedToDefaultInVersion: "1.32", VersionRange: versionutils.VersionRange{AddedInVersion: "1.28"}},
	"PodLevelResources":                              {VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
	"PodLifecycleSleepAction":                        {VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"PodLifecycleSleepActionAllowZero":               {VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
	"PodLogsQuerySplitStreams":                       {VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
	"PodReadyToStartContainersCondition":             {VersionRange: versionutils.VersionRange{AddedInVersion: "1.28"}},
	"PodSchedulingReadiness":                         {Default: true, LockedToDefaultInVersion: "1.30"},
	"PodSecurity":                                    {Default: true, LockedToDefaultInVersion: "1.25", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.28"}},
	"PortForwardWebsockets":                          {VersionRange: versionutils.VersionRange{AddedInVersion: "1.30"}},
	"PreferAlignCpusByUncoreCache":                   {VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
	"ProbeTerminationGracePeriod":                    {Default: true, LockedToDefaultInVersion: "1.28", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.29"}},
	"ProcMountType":                                  {},
	"ProxyTerminatingEndpoints":                      {Default: true, LockedToDefaultInVersion: "1.28", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.30"}},
	"QOSReserved":                                    {},
	"ReadWriteOncePod":                               {Default: true, LockedToDefaultInVersion: "1.29", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.31"}},
	"RecoverVolumeExpansionFailure":                  {Default: true, VersionRange: versionutils.VersionRange{AddedInVersion: "1.28"}},
	"RecursiveReadOnlyMounts":                        {VersionRange: versionutils.VersionRange{AddedInVersion: "1.30"}},
	"RelaxedDNSSearchValidation":                     {VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
	"RelaxedEnvironmentVariableValidation":           {Default: true, VersionRange: versionutils.VersionRange{AddedInVersion: "1.30"}},
	"ReloadKubeletServerCertificateFile":             {VersionRange: versionutils.VersionRange{AddedInVersion: "1.31"}},
	"RemainingItemCount":                             {Default: true, LockedToDefaultInVersion: "1.29"},
	"RemoteRequestHeaderUID":                         {VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
	"RemoveSelfLink":                                 {Default: true, LockedToDefaultInVersion: "1.24", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.30"}},
	"ResilientWatchCacheInitialization":              {VersionRange: versionutils.VersionRange{AddedInVersion: "1.31"}},
	"ResourceHealthStatus":                           {VersionRange: versionutils.VersionRange{AddedInVersion: "1.31"}},
	"RetroactiveDefaultStorageClass":                 {Default: true, LockedToDefaultInVersion: "1.28", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.29"}},
	"RetryGenerateName":                              {Default: true, LockedToDefaultInVersion: "1.32", VersionRange: versionutils.VersionRange{AddedInVersion: "1.30"}},
	"RotateKubeletServerCertificate":                 {},
	"RuntimeClassInImageCriApi":                      {VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"SchedulerAsyncPreemption":                       {VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
	"SchedulerQueueingHints":                         {Default: true, VersionRange: versionutils.VersionRange{AddedInVersion: "1.28"}},
	"SeccompDefault":                                 {Default: true, LockedToDefaultInVersion: "1.27", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.29"}},
	"SecurityContextDeny":                            {VersionRange: versionutils.VersionRange{RemovedInVersion: "1.30"}},
	"SELinuxChangePolicy":                            {VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
	"SELinuxMount":                                   {VersionRange: versionutils.VersionRange{AddedInVersion: "1.30"}},
	"SELinuxMountReadWriteOncePod":                   {Default: true, LockedToDefaultInVersion: "1.29"},
	"SeparateCacheWatchRPC":                          {VersionRange: versionutils.VersionRange{AddedInVersion: "1.28"}},
	"SeparateTaintEvictionController":                {VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"ServerSideApply":                                {Default: true, LockedToDefaultInVersion: "1.22", VersionRange: versionutils.VersionRange{AddedInVersion: "1.14", RemovedInVersion: "1.32"}},
	"ServerSideFieldValidation":                      {Default: true, LockedToDefaultInVersion: "1.27", VersionRange: versionutils.VersionRange{AddedInVersion: "1.23", RemovedInVersion: "1.32"}},
	"ServiceAccountNodeAudienceRestriction":          {Default: true, VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
	"ServiceAccountTokenJTI":                         {Default: true, LockedToDefaultInVersion: "1.32", VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"ServiceAccountTokenNodeBinding":                 {VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"ServiceAccountTokenNodeBindingValidation":       {Default: true, LockedToDefaultInVersion: "1.32", VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"ServiceAccountTokenPodNodeInfo":                 {Default: true, LockedToDefaultInVersion: "1.32", VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"ServiceInternalTrafficPolicy":                   {Default: true, LockedToDefaultInVersion: "1.26", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.28"}},
	"ServiceIPStaticSubrange":                        {Default: true, LockedToDefaultInVersion: "1.26", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.28"}},
	"ServiceNodePortStaticSubrange":                  {Default: true, LockedToDefaultInVersion: "1.29", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.31"}},
	"ServiceTrafficDistribution":                     {VersionRange: versionutils.VersionRange{AddedInVersion: "1.30"}},
	"SidecarContainers":                              {VersionRange: versionutils.VersionRange{AddedInVersion: "1.28"}},
	"SizeMemoryBackedVolumes":                        {Default: true, LockedToDefaultInVersion: "1.32", VersionRange: versionutils.VersionRange{AddedInVersion: "1.20"}},
	"SkipReadOnlyValidationGCE":                      {VersionRange: versionutils.VersionRange{AddedInVersion: "1.28", RemovedInVersion: "1.31"}},
	"StableLoadBalancerNodeSet":                      {Default: true, LockedToDefaultInVersion: "1.30", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.32"}},
	"StatefulSetAutoDeletePVC":                       {Default: true, LockedToDefaultInVersion: "1.32", VersionRange: versionutils.VersionRange{AddedInVersion: "1.23"}},
	"StatefulSetStartOrdinal":                        {Default: true, LockedToDefaultInVersion: "1.31"},
	"StorageNamespaceIndex":                          {VersionRange: versionutils.VersionRange{AddedInVersion: "1.30"}},
	"StorageVersionAPI":                              {},
	"StorageVersionHash":                             {},
	"StorageVersionMigrator":                         {VersionRange: versionutils.VersionRange{AddedInVersion: "1.30"}},
	"StrictCostEnforcementForVAP":                    {Default: true, LockedToDefaultInVersion: "1.32", VersionRange: versionutils.VersionRange{AddedInVersion: "1.30"}},
	"StrictCostEnforcementForWebhooks":               {Default: true, LockedToDefaultInVersion: "1.32", VersionRange: versionutils.VersionRange{AddedInVersion: "1.30"}},
	"StructuredAuthenticationConfiguration":          {VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"StructuredAuthorizationConfiguration":           {Default: true, LockedToDefaultInVersion: "1.32", VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"SupplementalGroupsPolicy":                       {VersionRange: versionutils.VersionRange{AddedInVersion: "1.31"}},
	"SystemdWatchdog":                                {Default: true, VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
	"TopologyAwareHints":                             {},
	"TopologyManager":                                {Default: true, LockedToDefaultInVersion: "1.27", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.29"}},
	"TopologyManagerPolicyAlphaOptions":              {},
	"TopologyManagerPolicyBetaOptions":               {},
	"TopologyManagerPolicyOptions":                   {Default: true, LockedToDefaultInVersion: "1.32"},
	"TranslateStreamCloseWebsocketRequests":          {VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"UnauthenticatedHTTP2DOSMitigation":              {},
	"UnknownVersionInteroperabilityProxy":            {VersionRange: versionutils.VersionRange{AddedInVersion: "1.28"}},
	"UserNamespacesPodSecurityStandards":             {VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"UserNamespacesStatelessPodsSupport":             {VersionRange: versionutils.VersionRange{RemovedInVersion: "1.28"}},
	"UserNamespacesSupport":                          {VersionRange: versionutils.VersionRange{AddedInVersion: "1.28"}},
	"ValidatingAdmissionPolicy":                      {Default: true, LockedToDefaultInVersion: "1.30"},
	"VolumeAttributesClass":                          {VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	"VolumeCapacityPriority":                         {},
	"WatchBookmark":                                  {Default: true, LockedToDefaultInVersion: "1.17"},
	"WatchCacheInitializationPostStartHook":          {VersionRange: versionutils.VersionRange{AddedInVersion: "1.31"}},
	"WatchFromStorageWithoutResourceVersion":         {},
	"WatchList":                                      {Default: true},
	"WatchListClient":                                {VersionRange: versionutils.VersionRange{AddedInVersion: "1.30"}},
	"WinDSR":                                         {},
	"WinOverlay":                                     {},
	"WindowsCPUAndMemoryAffinity":                    {VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
	"WindowsGracefulNodeShutdown":                    {VersionRange: versionutils.VersionRange{AddedInVersion: "1.32"}},
	"WindowsHostNetwork":                             {},
	"WindowsHostProcessContainers":                   {Default: true, LockedToDefaultInVersion: "1.26", VersionRange: versionutils.VersionRange{RemovedInVersion: "1.28"}},
	"ZeroLimitedNominalConcurrencyShares":            {Default: true, LockedToDefaultInVersion: "1.30", VersionRange: versionutils.VersionRange{AddedInVersion: "1.29", RemovedInVersion: "1.32"}},
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
			} else if isLockedToDefault && featureGates[featureGate] != featureGateVersionRanges[featureGate].Default {
				allErrs = append(allErrs, field.Forbidden(fldPath.Child(featureGate), fmt.Sprintf("cannot set feature gate to %v, feature is locked to %v", featureGates[featureGate], featureGateVersionRanges[featureGate].Default)))
			}
		}
	}

	return allErrs
}
