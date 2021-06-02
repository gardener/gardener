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

	"github.com/gardener/gardener/pkg/utils/version"
)

// featureGateVersionRanges contains the version ranges for all Kubernetes feature gates.
// Extracted from https://raw.githubusercontent.com/kubernetes/kubernetes/release-${version}/pkg/features/kube_features.go.
// To maintain this list for each new Kubernetes version:
// * Run hack/compare-k8s-feature-gates.sh <old-version> <new-version>.
//   It will present 2 lists of feature gates: those added and those removed in <new-version> compared to <old-version>.
// * Add all added feature gates to the map with <new-version> as MinVersion and no MaxVersion.
// * For any removed feature gates, add <new-version> as MaxVersion to the already existing feature gate in the map.
var featureGateVersionRanges = map[string]*version.Range{
	"APIListChunking":                                {},
	"APIPriorityAndFairness":                         {MinVersion: "1.17"},
	"APIResponseCompression":                         {},
	"APIServerIdentity":                              {MinVersion: "1.20"},
	"AdvancedAuditing":                               {},
	"AllowInsecureBackendProxy":                      {MinVersion: "1.17"},
	"AnyVolumeDataSource":                            {MinVersion: "1.18"},
	"AppArmor":                                       {},
	"AttachVolumeLimit":                              {MaxVersion: "1.21"},
	"BalanceAttachedNodeVolumes":                     {},
	"BlockVolume":                                    {MaxVersion: "1.21"},
	"BoundServiceAccountTokenVolume":                 {},
	"CPUManager":                                     {},
	"CRIContainerLogRotation":                        {},
	"CSIBlockVolume":                                 {MaxVersion: "1.21"},
	"CSIDriverRegistry":                              {MaxVersion: "1.21"},
	"CSIInlineVolume":                                {},
	"CSIMigration":                                   {},
	"CSIMigrationAWS":                                {},
	"CSIMigrationAWSComplete":                        {MinVersion: "1.17", MaxVersion: "1.21"},
	"CSIMigrationAzureDisk":                          {},
	"CSIMigrationAzureDiskComplete":                  {MinVersion: "1.17", MaxVersion: "1.21"},
	"CSIMigrationAzureFile":                          {},
	"CSIMigrationAzureFileComplete":                  {MinVersion: "1.17", MaxVersion: "1.21"},
	"CSIMigrationGCE":                                {},
	"CSIMigrationGCEComplete":                        {MinVersion: "1.17", MaxVersion: "1.21"},
	"CSIMigrationOpenStack":                          {},
	"CSIMigrationOpenStackComplete":                  {MinVersion: "1.17", MaxVersion: "1.21"},
	"CSIMigrationvSphere":                            {MinVersion: "1.19"},
	"CSIMigrationvSphereComplete":                    {MinVersion: "1.19"},
	"CSINodeInfo":                                    {MaxVersion: "1.21"},
	"CSIPersistentVolume":                            {MaxVersion: "1.16"},
	"CSIServiceAccountToken":                         {MinVersion: "1.20"},
	"CSIStorageCapacity":                             {MinVersion: "1.19"},
	"CSIVolumeFSGroupPolicy":                         {MinVersion: "1.19"},
	"CSIVolumeHealth":                                {MinVersion: "1.21"},
	"ConfigurableFSGroupPolicy":                      {MinVersion: "1.18"},
	"ControllerManagerLeaderMigration":               {MinVersion: "1.21"}, // Missing from docu?
	"CronJobControllerV2":                            {MinVersion: "1.20"},
	"CustomCPUCFSQuotaPeriod":                        {},
	"CustomPodDNS":                                   {MaxVersion: "1.16"},
	"CustomResourceDefaulting":                       {MaxVersion: "1.18"},
	"CustomResourcePublishOpenAPI":                   {MaxVersion: "1.18"},
	"CustomResourceSubresources":                     {MaxVersion: "1.18"},
	"CustomResourceValidation":                       {MaxVersion: "1.18"},
	"CustomResourceWebhookConversion":                {MaxVersion: "1.18"},
	"DaemonSetUpdateSurge":                           {MinVersion: "1.21"},                     // Missing from docu?
	"DebugContainers":                                {MaxVersion: "1.16"},                     // Missing from docu?
	"DefaultIngressClass":                            {MinVersion: "1.18", MaxVersion: "1.20"}, // Missing from docu?
	"DefaultPodTopologySpread":                       {MinVersion: "1.19"},
	"DevicePlugins":                                  {},
	"DisableAcceleratorUsageMetrics":                 {MinVersion: "1.19"},
	"DownwardAPIHugePages":                           {MinVersion: "1.20"},
	"DryRun":                                         {},
	"DynamicAuditing":                                {MaxVersion: "1.19"},
	"DynamicKubeletConfig":                           {},
	"EfficientWatchResumption":                       {MinVersion: "1.20"},
	"EnableAggregatedDiscoveryTimeout":               {MinVersion: "1.16", MaxVersion: "1.17"},
	"EndpointSlice":                                  {MinVersion: "1.16"},
	"EndpointSliceNodeName":                          {MinVersion: "1.20"},
	"EndpointSliceProxying":                          {MinVersion: "1.18"},
	"EndpointSliceTerminatingCondition":              {MinVersion: "1.20"},
	"EphemeralContainers":                            {MinVersion: "1.16"},
	"EvenPodsSpread":                                 {MinVersion: "1.16", MaxVersion: "1.21"},
	"ExecProbeTimeout":                               {MinVersion: "1.20"},
	"ExpandCSIVolumes":                               {},
	"ExpandInUsePersistentVolumes":                   {},
	"ExpandPersistentVolumes":                        {},
	"ExperimentalCriticalPodAnnotation":              {MaxVersion: "1.16"},
	"ExperimentalHostUserNamespaceDefaulting":        {},
	"ExternalPolicyForExternalIP":                    {MinVersion: "1.18"}, // Missing from docu?
	"GCERegionalPersistentDisk":                      {MaxVersion: "1.17"},
	"GenericEphemeralVolume":                         {MinVersion: "1.19"},
	"GracefulNodeShutdown":                           {MinVersion: "1.20"},
	"HPAContainerMetrics":                            {MinVersion: "1.20"},
	"HPAScaleToZero":                                 {MinVersion: "1.16"},
	"HugePageStorageMediumSize":                      {MinVersion: "1.18"},
	"HugePages":                                      {MaxVersion: "1.16"},
	"HyperVContainer":                                {MaxVersion: "1.21"},
	"IPv6DualStack":                                  {MinVersion: "1.16"},
	"ImmutableEphemeralVolumes":                      {MinVersion: "1.18"},
	"InTreePluginAWSUnregister":                      {MinVersion: "1.21"}, // Missing from docu?
	"InTreePluginAzureDiskUnregister":                {MinVersion: "1.21"}, // Missing from docu?
	"InTreePluginAzureFileUnregister":                {MinVersion: "1.21"}, // Missing from docu?
	"InTreePluginGCEUnregister":                      {MinVersion: "1.21"}, // Missing from docu?
	"InTreePluginOpenStackUnregister":                {MinVersion: "1.21"}, // Missing from docu?
	"InTreePluginvSphereUnregister":                  {MinVersion: "1.21"}, // Missing from docu?
	"IndexedJob":                                     {MinVersion: "1.21"},
	"IngressClassNamespacedParams":                   {MinVersion: "1.21"},
	"KubeletCredentialProviders":                     {MinVersion: "1.20"},
	"KubeletPluginsWatcher":                          {MaxVersion: "1.16"},
	"KubeletPodResources":                            {},
	"KubeletPodResourcesGetAllocatable":              {MinVersion: "1.21"},
	"LegacyNodeRoleBehavior":                         {MinVersion: "1.16"},
	"LocalStorageCapacityIsolation":                  {},
	"LocalStorageCapacityIsolationFSQuotaMonitoring": {},
	"LogarithmicScaleDown":                           {MinVersion: "1.21"},
	"MemoryManager":                                  {MinVersion: "1.21"}, // Missing from docu?
	"MixedProtocolLBService":                         {MinVersion: "1.20"},
	"MountContainers":                                {MaxVersion: "1.17"},
	"NamespaceDefaultLabelName":                      {MinVersion: "1.21"},
	"NetworkPolicyEndPort":                           {MinVersion: "1.21"},
	"NodeDisruptionExclusion":                        {MinVersion: "1.16"},
	"NodeLease":                                      {},
	"NonPreemptingPriority":                          {},
	"PersistentLocalVolumes":                         {MaxVersion: "1.17"},
	"PodAffinityNamespaceSelector":                   {MinVersion: "1.21"},
	"PodDeletionCost":                                {MinVersion: "1.21"},
	"PodDisruptionBudget":                            {MinVersion: "1.17"}, // Docu says 1.3?
	"PodOverhead":                                    {MinVersion: "1.16"},
	"PodPriority":                                    {MaxVersion: "1.18"},
	"PodReadinessGates":                              {MaxVersion: "1.16"},
	"PodShareProcessNamespace":                       {MaxVersion: "1.19"},
	"PreferNominatedNode":                            {MinVersion: "1.21"}, // Missing from docu?
	"ProbeTerminationGracePeriod":                    {MinVersion: "1.21"},
	"ProcMountType":                                  {},
	"QOSReserved":                                    {},
	"RemainingItemCount":                             {},
	"RemoveSelfLink":                                 {MinVersion: "1.16"},
	"RequestManagement":                              {MaxVersion: "1.17"},
	"ResourceLimitsPriorityFunction":                 {MaxVersion: "1.19"},
	"ResourceQuotaScopeSelectors":                    {MaxVersion: "1.18"},
	"RootCAConfigMap":                                {MinVersion: "1.20"}, // Docu says 1.13?
	"RotateKubeletClientCertificate":                 {MaxVersion: "1.21"},
	"RotateKubeletServerCertificate":                 {},
	"RunAsGroup":                                     {},
	"RuntimeClass":                                   {},
	"SCTPSupport":                                    {},
	"ScheduleDaemonSetPods":                          {MaxVersion: "1.18"},
	"SelectorIndex":                                  {MinVersion: "1.18"}, // Missing from docu?
	"ServerSideApply":                                {},
	"ServiceAccountIssuerDiscovery":                  {MinVersion: "1.18"},
	"ServiceAppProtocol":                             {MinVersion: "1.18"},
	"ServiceInternalTrafficPolicy":                   {MinVersion: "1.21"},
	"ServiceLBNodePortControl":                       {MinVersion: "1.20"},
	"ServiceLoadBalancerClass":                       {MinVersion: "1.21"},
	"ServiceLoadBalancerFinalizer":                   {MaxVersion: "1.20"},
	"ServiceNodeExclusion":                           {},
	"ServiceTopology":                                {MinVersion: "1.17"},
	"SetHostnameAsFQDN":                              {MinVersion: "1.19"},
	"SizeMemoryBackedVolumes":                        {MinVersion: "1.20"},
	"StartupProbe":                                   {MinVersion: "1.16"},
	"StorageObjectInUseProtection":                   {},
	"StorageVersionAPI":                              {MinVersion: "1.20"},
	"StorageVersionHash":                             {},
	"StreamingProxyRedirects":                        {},
	"SupportIPVSProxyMode":                           {MaxVersion: "1.20"},
	"SupportNodePidsLimit":                           {},
	"SupportPodPidsLimit":                            {},
	"SuspendJob":                                     {MinVersion: "1.21"},
	"Sysctls":                                        {},
	"TTLAfterFinished":                               {},
	"TaintBasedEvictions":                            {MaxVersion: "1.20"},
	"TaintNodesByCondition":                          {MaxVersion: "1.18"},
	"TokenRequest":                                   {MaxVersion: "1.21"},
	"TokenRequestProjection":                         {MaxVersion: "1.21"},
	"TopologyAwareHints":                             {MinVersion: "1.21"},
	"TopologyManager":                                {MinVersion: "1.16"},
	"ValidateProxyRedirects":                         {},
	"VolumeCapacityPriority":                         {MinVersion: "1.21"},
	"VolumePVCDataSource":                            {MaxVersion: "1.21"},
	"VolumeScheduling":                               {MaxVersion: "1.16"},
	"VolumeSnapshotDataSource":                       {},
	"VolumeSubpath":                                  {},
	"VolumeSubpathEnvExpansion":                      {MaxVersion: "1.19"},
	"WarningHeaders":                                 {MinVersion: "1.19"},
	"WatchBookmark":                                  {},
	"WinDSR":                                         {},
	"WinOverlay":                                     {},
	"WindowsEndpointSliceProxying":                   {MinVersion: "1.19"},
	"WindowsGMSA":                                    {MaxVersion: "1.21"},
	"WindowsRunAsUserName":                           {MinVersion: "1.16", MaxVersion: "1.21"},
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
