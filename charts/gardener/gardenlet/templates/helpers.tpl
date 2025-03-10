{{- define "image" -}}
{{- if $.ref -}}
{{ $.ref }}
{{- else -}}
{{- if hasPrefix "sha256:" (required "$.tag is required" $.tag) -}}
{{ required "$.repository is required" $.repository }}@{{ required "$.tag is required" $.tag }}
{{- else -}}
{{ required "$.repository is required" $.repository }}:{{ required "$.tag is required" $.tag }}
{{- end -}}
{{- end -}}
{{- end -}}

{{- define "gardenlet.kubeconfig-garden.data" -}}
kubeconfig: {{ .Values.config.gardenClientConnection.kubeconfig | b64enc }}
{{- end -}}

{{- define "gardenlet.kubeconfig-garden.name" -}}
gardenlet-kubeconfig-garden-{{ include "gardenlet.kubeconfig-garden.data" . | sha256sum | trunc 8 }}
{{- end -}}

{{- define "gardenlet.kubeconfig-seed.data" -}}
kubeconfig: {{ .Values.config.seedClientConnection.kubeconfig | b64enc }}
{{- end -}}

{{- define "gardenlet.kubeconfig-seed.name" -}}
gardenlet-kubeconfig-seed-{{ include "gardenlet.kubeconfig-seed.data" . | sha256sum | trunc 8 }}
{{- end -}}

{{- define "gardenlet.imagevector-overwrite.data" -}}
images_overwrite.yaml: |
{{ .Values.imageVectorOverwrite | indent 2 }}
{{- end -}}

{{- define "gardenlet.imagevector-overwrite.name" -}}
gardenlet-imagevector-overwrite-{{ include "gardenlet.imagevector-overwrite.data" . | sha256sum | trunc 8 }}
{{- end -}}

{{- define "gardenlet.imagevector-overwrite-components.data" -}}
components.yaml: |
{{ .Values.componentImageVectorOverwrites | indent 2 }}
{{- end -}}

{{- define "gardenlet.imagevector-overwrite-components.name" -}}
gardenlet-imagevector-overwrite-components-{{ include "gardenlet.imagevector-overwrite-components.data" . | sha256sum | trunc 8 }}
{{- end -}}

{{- define "gardenlet.cert.name" -}}
gardenlet-cert-{{ include "gardenlet.cert.data" . | sha256sum | trunc 8 }}
{{- end -}}

{{- define "gardenlet.seed.numberOfZones" -}}
{{- if .Values.config.seedConfig.spec -}}
{{- if .Values.config.seedConfig.spec.provider -}}
{{- if .Values.config.seedConfig.spec.provider.zones -}}
{{ len .Values.config.seedConfig.spec.provider.zones }}
{{- else -}}
1
{{- end -}}
{{- else -}}
1
{{- end -}}
{{- else -}}
1
{{- end -}}
{{- end -}}

{{- define "gardenlet.deployment.topologySpreadConstraints" -}}
{{- if gt (int .Values.replicaCount) 1 -}}
topologySpreadConstraints:
- maxSkew: 1
  topologyKey: kubernetes.io/hostname
  whenUnsatisfiable: ScheduleAnyway
  labelSelector:
    matchLabels:
{{ include "gardenlet.deployment.matchLabels" . | indent 6 }}
  matchLabelKeys:
  - "pod-template-hash"
{{- if gt (int (include "gardenlet.seed.numberOfZones" .)) 1 }}
- maxSkew: 1
  minDomains: {{ include "gardenlet.deployment.minDomains" . }}
  topologyKey: topology.kubernetes.io/zone
  whenUnsatisfiable: DoNotSchedule
  labelSelector:
    matchLabels:
{{ include "gardenlet.deployment.matchLabels" . | indent 6 }}
  matchLabelKeys:
  - "pod-template-hash"
{{- end }}
{{- end }}
{{- end -}}

{{- define "gardenlet.deployment.minDomains" }}
{{- if gt (int .Values.replicaCount) (int (include "gardenlet.seed.numberOfZones" .)) }}
{{- include "gardenlet.seed.numberOfZones" . }}
{{- else }}
{{- .Values.replicaCount }}
{{- end }}
{{- end -}}

{{- define "gardenlet.config.data" -}}
config.yaml: |
{{ include "gardenlet.config" . | indent 2 }}
{{- end -}}

{{- define "gardenlet.config" -}}
apiVersion: gardenlet.config.gardener.cloud/v1alpha1
kind: GardenletConfiguration
gardenClientConnection:
  {{- with .Values.config.gardenClientConnection.acceptContentTypes }}
  acceptContentTypes: {{ . | quote }}
  {{- end }}
  {{- with .Values.config.gardenClientConnection.contentType }}
  contentType: {{ . | quote }}
  {{- end }}
  qps: {{ required ".Values.config.gardenClientConnection.qps is required" .Values.config.gardenClientConnection.qps }}
  burst: {{ required ".Values.config.gardenClientConnection.burst is required" .Values.config.gardenClientConnection.burst }}
  {{- if .Values.config.gardenClientConnection.gardenClusterAddress }}
  gardenClusterAddress: {{ .Values.config.gardenClientConnection.gardenClusterAddress }}
  {{- end }}
  {{- if .Values.config.gardenClientConnection.gardenClusterCACert }}
  gardenClusterCACert: {{ .Values.config.gardenClientConnection.gardenClusterCACert }}
  {{- end }}
  {{- if .Values.config.gardenClientConnection.bootstrapKubeconfig }}
  bootstrapKubeconfig:
    {{- if .Values.config.gardenClientConnection.bootstrapKubeconfig.secretRef }}
    name: {{ required ".Values.config.gardenClientConnection.bootstrapKubeconfig.secretRef.name is required" .Values.config.gardenClientConnection.bootstrapKubeconfig.secretRef.name }}
    namespace: {{ required ".Values.config.gardenClientConnection.bootstrapKubeconfig.secretRef.namespace is required" .Values.config.gardenClientConnection.bootstrapKubeconfig.secretRef.namespace }}
    {{- else }}
    name: {{ required ".Values.config.gardenClientConnection.bootstrapKubeconfig.name is required" .Values.config.gardenClientConnection.bootstrapKubeconfig.name }}
    namespace: {{ required ".Values.config.gardenClientConnection.bootstrapKubeconfig.namespace is required" .Values.config.gardenClientConnection.bootstrapKubeconfig.namespace }}
    {{- end }}
  {{- end }}
  {{- if .Values.config.gardenClientConnection.kubeconfigSecret }}
  kubeconfigSecret:
    name: {{ required ".Values.config.gardenClientConnection.kubeconfigSecret.name is required" .Values.config.gardenClientConnection.kubeconfigSecret.name }}
    namespace: {{ required ".Values.config.gardenClientConnection.kubeconfigSecret.namespace is required" .Values.config.gardenClientConnection.kubeconfigSecret.namespace }}
  {{- end }}
{{- if .Values.config.gardenClientConnection.kubeconfigValidity }}
  kubeconfigValidity:
{{ toYaml .Values.config.gardenClientConnection.kubeconfigValidity | indent 4 }}
  {{- end }}
  {{- if .Values.config.gardenClientConnection.kubeconfig }}
  kubeconfig: /etc/gardenlet/kubeconfig-garden/kubeconfig
  {{- end }}
seedClientConnection:
  {{- with .Values.config.seedClientConnection.acceptContentTypes }}
  acceptContentTypes: {{ . | quote }}
  {{- end }}
  {{- with .Values.config.seedClientConnection.contentType }}
  contentType: {{ . | quote }}
  {{- end }}
  qps: {{ required ".Values.config.seedClientConnection.qps is required" .Values.config.seedClientConnection.qps }}
  burst: {{ required ".Values.config.seedClientConnection.burst is required" .Values.config.seedClientConnection.burst }}
  {{- if .Values.config.seedClientConnection.kubeconfig }}
  kubeconfig: /etc/gardenlet/kubeconfig-seed/kubeconfig
  {{- end }}
shootClientConnection:
  {{- with .Values.config.shootClientConnection.acceptContentTypes }}
  acceptContentTypes: {{ . | quote }}
  {{- end }}
  {{- with .Values.config.shootClientConnection.contentType }}
  contentType: {{ . | quote }}
  {{- end }}
  qps: {{ required ".Values.config.shootClientConnection.qps is required" .Values.config.shootClientConnection.qps }}
  burst: {{ required ".Values.config.shootClientConnection.burst is required" .Values.config.shootClientConnection.burst }}
controllers:
  backupBucket:
    concurrentSyncs: {{ required ".Values.config.controllers.backupBucket.concurrentSyncs is required" .Values.config.controllers.backupBucket.concurrentSyncs }}
  backupEntry:
    concurrentSyncs: {{ required ".Values.config.controllers.backupEntry.concurrentSyncs is required" .Values.config.controllers.backupEntry.concurrentSyncs }}
    {{- if .Values.config.controllers.backupEntry.deletionGracePeriodHours }}
    deletionGracePeriodHours: {{ .Values.config.controllers.backupEntry.deletionGracePeriodHours }}
    {{- end }}
    {{- if .Values.config.controllers.backupEntry.deletionGracePeriodShootPurposes }}
    deletionGracePeriodShootPurposes:
{{ toYaml .Values.config.controllers.backupEntry.deletionGracePeriodShootPurposes | indent 4 }}
    {{- end }}
  bastion:
    concurrentSyncs: {{ required ".Values.config.controllers.bastion.concurrentSyncs is required" .Values.config.controllers.bastion.concurrentSyncs }}
  {{- if .Values.config.controllers.controllerInstallation }}
  controllerInstallation:
    concurrentSyncs: {{ required ".Values.config.controllers.controllerInstallation.concurrentSyncs is required" .Values.config.controllers.controllerInstallation.concurrentSyncs }}
  {{- end }}
  {{- if .Values.config.controllers.controllerInstallationCare }}
  controllerInstallationCare:
    concurrentSyncs: {{ required ".Values.config.controllers.controllerInstallationCare.concurrentSyncs is required" .Values.config.controllers.controllerInstallationCare.concurrentSyncs }}
    syncPeriod: {{ required ".Values.config.controllers.controllerInstallationCare.syncPeriod is required" .Values.config.controllers.controllerInstallationCare.syncPeriod }}
  {{- end }}
  {{- if .Values.config.controllers.controllerInstallationRequired }}
  controllerInstallationRequired:
    concurrentSyncs: {{ required ".Values.config.controllers.controllerInstallationRequired.concurrentSyncs is required" .Values.config.controllers.controllerInstallationRequired.concurrentSyncs }}
  {{- end }}
  {{- if .Values.config.controllers.gardenlet }}
  gardenlet:
    syncPeriod: {{ required ".Values.config.controllers.gardenlet.syncPeriod is required" .Values.config.controllers.gardenlet.syncPeriod }}
  {{- end }}
  {{- if .Values.config.controllers.seed }}
  seed:
    syncPeriod: {{ required ".Values.config.controllers.seed.syncPeriod is required" .Values.config.controllers.seed.syncPeriod }}
    {{- if .Values.config.controllers.seed.leaseResyncSeconds }}
    leaseResyncSeconds: {{ .Values.config.controllers.seed.leaseResyncSeconds }}
    {{- end }}
    {{- if .Values.config.controllers.seed.leaseResyncMissThreshold }}
    leaseResyncMissThreshold: {{ .Values.config.controllers.seed.leaseResyncMissThreshold }}
    {{- end }}
  {{- end }}
  shoot:
    concurrentSyncs: {{ required ".Values.config.controllers.shoot.concurrentSyncs is required" .Values.config.controllers.shoot.concurrentSyncs }}
    {{- if .Values.config.controllers.shoot.progressReportPeriod }}
    progressReportPeriod: {{ .Values.config.controllers.shoot.progressReportPeriod }}
    {{- end }}
    {{- if .Values.config.controllers.shoot.respectSyncPeriodOverwrite }}
    respectSyncPeriodOverwrite: {{ .Values.config.controllers.shoot.respectSyncPeriodOverwrite }}
    {{- end }}
    {{- if .Values.config.controllers.shoot.reconcileInMaintenanceOnly }}
    reconcileInMaintenanceOnly: {{ .Values.config.controllers.shoot.reconcileInMaintenanceOnly }}
    {{- end }}
    syncPeriod: {{ required ".Values.config.controllers.shoot.syncPeriod is required" .Values.config.controllers.shoot.syncPeriod }}
    retryDuration: {{ required ".Values.config.controllers.shoot.retryDuration is required" .Values.config.controllers.shoot.retryDuration }}
    {{- if .Values.config.controllers.shoot.dnsEntryTTLSeconds }}
    dnsEntryTTLSeconds: {{ .Values.config.controllers.shoot.dnsEntryTTLSeconds }}
    {{- end }}
  shootCare:
    concurrentSyncs: {{ required ".Values.config.controllers.shootCare.concurrentSyncs is required" .Values.config.controllers.shootCare.concurrentSyncs }}
    syncPeriod: {{ required ".Values.config.controllers.shootCare.syncPeriod is required" .Values.config.controllers.shootCare.syncPeriod }}
    {{- if .Values.config.controllers.shootCare.staleExtensionHealthChecks }}
    staleExtensionHealthChecks:
      enabled: {{ required ".Values.config.controllers.shootCare.staleExtensionHealthChecks.enabled is required" .Values.config.controllers.shootCare.staleExtensionHealthChecks.enabled }}
      {{- if .Values.config.controllers.shootCare.staleExtensionHealthChecks.threshold }}
      threshold: {{ .Values.config.controllers.shootCare.staleExtensionHealthChecks.threshold }}
      {{- end }}
    {{- end }}
    {{- if .Values.config.controllers.shootCare.managedResourceProgressingThreshold }}
    managedResourceProgressingThreshold: {{ .Values.config.controllers.shootCare.managedResourceProgressingThreshold }}
    {{- end }}
    conditionThresholds:
    {{- if .Values.config.controllers.shootCare.conditionThresholds }}
{{ toYaml .Values.config.controllers.shootCare.conditionThresholds | indent 4 }}
    {{- end }}
    webhookRemediatorEnabled: {{ required ".Values.config.controllers.shootCare.webhookRemediatorEnabled is required" .Values.config.controllers.shootCare.webhookRemediatorEnabled }}
  seedCare:
    syncPeriod: {{ required ".Values.config.controllers.seedCare.syncPeriod is required" .Values.config.controllers.seedCare.syncPeriod }}
    conditionThresholds:
    {{- if .Values.config.controllers.seedCare.conditionThresholds }}
{{ toYaml .Values.config.controllers.seedCare.conditionThresholds | indent 4 }}
    {{- end }}
  {{- if .Values.config.controllers.shootState }}
  shootState:
    concurrentSyncs: {{ required ".Values.config.controllers.shootState.concurrentSyncs is required" .Values.config.controllers.shootState.concurrentSyncs }}
    syncPeriod: {{ required ".Values.config.controllers.shootState.syncPeriod is required" .Values.config.controllers.shootState.syncPeriod }}
  {{- end }}
  {{- if .Values.config.controllers.managedSeed }}
  managedSeed:
    concurrentSyncs: {{ required ".Values.config.controllers.managedSeed.concurrentSyncs is required" .Values.config.controllers.managedSeed.concurrentSyncs }}
    syncPeriod: {{ required ".Values.config.controllers.managedSeed.syncPeriod is required" .Values.config.controllers.managedSeed.syncPeriod }}
    waitSyncPeriod: {{ required ".Values.config.controllers.managedSeed.waitSyncPeriod is required" .Values.config.controllers.managedSeed.waitSyncPeriod }}
    {{- if .Values.config.controllers.managedSeed.syncJitterPeriod }}
    syncJitterPeriod: {{ .Values.config.controllers.managedSeed.syncJitterPeriod }}
    {{- end }}
    {{- if .Values.config.controllers.managedSeed.jitterUpdates }}
    jitterUpdates: {{ .Values.config.controllers.managedSeed.jitterUpdates }}
    {{- end }}
  {{- end }}
  {{- if .Values.config.controllers.networkPolicy }}
  networkPolicy:
    {{- if .Values.config.controllers.networkPolicy.concurrentSyncs }}
    concurrentSyncs: {{ .Values.config.controllers.networkPolicy.concurrentSyncs }}
    {{- end }}
    {{- if .Values.config.controllers.networkPolicy.additionalNamespaceSelectors }}
    additionalNamespaceSelectors:
{{ toYaml .Values.config.controllers.networkPolicy.additionalNamespaceSelectors | indent 4 }}
    {{- end }}
  {{- end }}
  tokenRequestor:
    concurrentSyncs: {{ required ".Values.config.controllers.tokenRequestor.concurrentSyncs is required" .Values.config.controllers.tokenRequestor.concurrentSyncs }}
  tokenRequestorWorkloadIdentity:
    concurrentSyncs: {{ required ".Values.config.controllers.tokenRequestorWorkloadIdentity.concurrentSyncs is required" .Values.config.controllers.tokenRequestorWorkloadIdentity.concurrentSyncs }}
  {{- if .Values.config.controllers.vpaEvictionRequirements }}
  vpaEvictionRequirements:
    {{- if .Values.config.controllers.vpaEvictionRequirements.concurrentSyncs }}
    concurrentSyncs: {{ .Values.config.controllers.vpaEvictionRequirements.concurrentSyncs }}
    {{- end }}
  {{- end }}
resources:
  capacity:
    shoots: {{ required ".Values.config.resources.capacity.shoots is required" .Values.config.resources.capacity.shoots }}
leaderElection:
  leaderElect: {{ required ".Values.config.leaderElection.leaderElect is required" .Values.config.leaderElection.leaderElect }}
  leaseDuration: {{ required ".Values.config.leaderElection.leaseDuration is required" .Values.config.leaderElection.leaseDuration }}
  renewDeadline: {{ required ".Values.config.leaderElection.renewDeadline is required" .Values.config.leaderElection.renewDeadline }}
  retryPeriod: {{ required ".Values.config.leaderElection.retryPeriod is required" .Values.config.leaderElection.retryPeriod }}
  resourceLock: {{ required ".Values.config.leaderElection.resourceLock is required" .Values.config.leaderElection.resourceLock }}
  {{- if .Values.config.leaderElection.resourceName }}
  resourceName: {{ .Values.config.leaderElection.resourceName }}
  {{- end }}
  {{- if .Values.config.leaderElection.resourceNamespace }}
  resourceNamespace: {{ .Values.config.leaderElection.resourceNamespace }}
  {{- end }}
logLevel: {{ .Values.config.logLevel }}
logFormat: {{ .Values.config.logFormat }}
server:
  healthProbes:
    {{- if .Values.config.server.healthProbes.bindAddress }}
    bindAddress: {{ .Values.config.server.healthProbes.bindAddress }}
    {{- end }}
    port: {{ required ".Values.config.server.healthProbes.port is required" .Values.config.server.healthProbes.port }}
  {{- if .Values.config.server.metrics }}
  metrics:
    {{- if .Values.config.server.metrics.bindAddress }}
    bindAddress: {{ .Values.config.server.metrics.bindAddress }}
    {{- end }}
    port: {{ required ".Values.config.server.metrics.port is required" .Values.config.server.metrics.port }}
  {{- end }}
{{- if .Values.config.debugging }}
debugging:
  enableProfiling: {{ .Values.config.debugging.enableProfiling | default false }}
  enableContentionProfiling: {{ .Values.config.debugging.enableContentionProfiling | default false }}
{{- end }}
{{- if .Values.config.featureGates }}
featureGates:
{{ toYaml .Values.config.featureGates | indent 2 }}
{{- end }}
{{- if .Values.config.seedConfig }}
seedConfig:
{{ toYaml .Values.config.seedConfig | indent 2 }}
{{- end }}
{{- if .Values.config.logging }}
logging:
{{ toYaml .Values.config.logging | indent 2 }}
{{- end }}
{{- if .Values.config.monitoring }}
monitoring:
{{ toYaml .Values.config.monitoring | indent 2 }}
{{- end }}
{{- if .Values.config.sni }}
sni:
{{ toYaml .Values.config.sni | trim | indent 2 }}
{{- end }}
{{- if .Values.config.etcdConfig }}
etcdConfig:
{{ toYaml .Values.config.etcdConfig | indent 2 }}
{{- end}}
{{- if .Values.config.exposureClassHandlers }}
exposureClassHandlers:
{{ toYaml .Values.config.exposureClassHandlers }}
{{- end }}
{{- if .Values.nodeToleration }}
nodeToleration:
{{ toYaml .Values.nodeToleration | indent 2 }}
{{- end}}
{{- end -}}

{{- define "gardenlet.config.name" -}}
gardenlet-configmap-{{ include "gardenlet.config.data" . | sha256sum | trunc 8 }}
{{- end -}}

