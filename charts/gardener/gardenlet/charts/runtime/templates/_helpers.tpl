{{- define "gardenlet.kubeconfig-garden.data" -}}
kubeconfig: {{ .Values.global.gardenlet.config.gardenClientConnection.kubeconfig | b64enc }}
{{- end -}}

{{- define "gardenlet.kubeconfig-garden.name" -}}
gardenlet-kubeconfig-garden-{{ include "gardenlet.kubeconfig-garden.data" . | sha256sum | trunc 8 }}
{{- end -}}

{{- define "gardenlet.kubeconfig-seed.data" -}}
kubeconfig: {{ .Values.global.gardenlet.config.seedClientConnection.kubeconfig | b64enc }}
{{- end -}}

{{- define "gardenlet.kubeconfig-seed.name" -}}
gardenlet-kubeconfig-seed-{{ include "gardenlet.kubeconfig-seed.data" . | sha256sum | trunc 8 }}
{{- end -}}

{{- define "gardenlet.imagevector-overwrite.data" -}}
images_overwrite.yaml: |
{{ .Values.global.gardenlet.imageVectorOverwrite | indent 2 }}
{{- end -}}

{{- define "gardenlet.imagevector-overwrite.name" -}}
gardenlet-imagevector-overwrite-{{ include "gardenlet.imagevector-overwrite.data" . | sha256sum | trunc 8 }}
{{- end -}}

{{- define "gardenlet.imagevector-overwrite-components.data" -}}
components.yaml: |
{{ .Values.global.gardenlet.componentImageVectorOverwrites | indent 2 }}
{{- end -}}

{{- define "gardenlet.imagevector-overwrite-components.name" -}}
gardenlet-imagevector-overwrite-components-{{ include "gardenlet.imagevector-overwrite-components.data" . | sha256sum | trunc 8 }}
{{- end -}}


{{- define "gardenlet.cert.data" -}}
gardenlet.crt: {{ required ".Values.global.gardenlet.config.server.https.tls.crt is required" (b64enc .Values.global.gardenlet.config.server.https.tls.crt) }}
gardenlet.key: {{ required ".Values.global.gardenlet.config.server.https.tls.key is required" (b64enc .Values.global.gardenlet.config.server.https.tls.key) }}
{{- end -}}

{{- define "gardenlet.cert.name" -}}
gardenlet-cert-{{ include "gardenlet.cert.data" . | sha256sum | trunc 8 }}
{{- end -}}

{{- define "gardenlet.config.data" -}}
config.yaml: |
  ---
  apiVersion: gardenlet.config.gardener.cloud/v1alpha1
  kind: GardenletConfiguration
  gardenClientConnection:
    {{- with .Values.global.gardenlet.config.gardenClientConnection.acceptContentTypes }}
    acceptContentTypes: {{ . | quote }}
    {{- end }}
    {{- with .Values.global.gardenlet.config.gardenClientConnection.contentType }}
    contentType: {{ . | quote }}
    {{- end }}
    qps: {{ required ".Values.global.gardenlet.config.gardenClientConnection.qps is required" .Values.global.gardenlet.config.gardenClientConnection.qps }}
    burst: {{ required ".Values.global.gardenlet.config.gardenClientConnection.burst is required" .Values.global.gardenlet.config.gardenClientConnection.burst }}
    {{- if .Values.global.gardenlet.config.gardenClientConnection.gardenClusterAddress }}
    gardenClusterAddress: {{ .Values.global.gardenlet.config.gardenClientConnection.gardenClusterAddress }}
    {{- end }}
    {{- if .Values.global.gardenlet.config.gardenClientConnection.gardenClusterCACert }}
    gardenClusterCACert: {{ .Values.global.gardenlet.config.gardenClientConnection.gardenClusterCACert }}
    {{- end }}
    {{- if .Values.global.gardenlet.config.gardenClientConnection.bootstrapKubeconfig }}
    bootstrapKubeconfig:
      {{- if .Values.global.gardenlet.config.gardenClientConnection.bootstrapKubeconfig.secretRef }}
      name: {{ required ".Values.global.gardenlet.config.gardenClientConnection.bootstrapKubeconfig.secretRef.name is required" .Values.global.gardenlet.config.gardenClientConnection.bootstrapKubeconfig.secretRef.name }}
      namespace: {{ required ".Values.global.gardenlet.config.gardenClientConnection.bootstrapKubeconfig.secretRef.namespace is required" .Values.global.gardenlet.config.gardenClientConnection.bootstrapKubeconfig.secretRef.namespace }}
      {{- else }}
      name: {{ required ".Values.global.gardenlet.config.gardenClientConnection.bootstrapKubeconfig.name is required" .Values.global.gardenlet.config.gardenClientConnection.bootstrapKubeconfig.name }}
      namespace: {{ required ".Values.global.gardenlet.config.gardenClientConnection.bootstrapKubeconfig.namespace is required" .Values.global.gardenlet.config.gardenClientConnection.bootstrapKubeconfig.namespace }}
      {{- end }}
    {{- end }}
    {{- if .Values.global.gardenlet.config.gardenClientConnection.kubeconfigSecret }}
    kubeconfigSecret:
      name: {{ required ".Values.global.gardenlet.config.gardenClientConnection.kubeconfigSecret.name is required" .Values.global.gardenlet.config.gardenClientConnection.kubeconfigSecret.name }}
      namespace: {{ required ".Values.global.gardenlet.config.gardenClientConnection.kubeconfigSecret.namespace is required" .Values.global.gardenlet.config.gardenClientConnection.kubeconfigSecret.namespace }}
    {{- end }}
    {{- if .Values.global.gardenlet.config.gardenClientConnection.kubeconfig }}
    kubeconfig: /etc/gardenlet/kubeconfig-garden/kubeconfig
    {{- end }}
  seedClientConnection:
    {{- with .Values.global.gardenlet.config.seedClientConnection.acceptContentTypes }}
    acceptContentTypes: {{ . | quote }}
    {{- end }}
    {{- with .Values.global.gardenlet.config.seedClientConnection.contentType }}
    contentType: {{ . | quote }}
    {{- end }}
    qps: {{ required ".Values.global.gardenlet.config.seedClientConnection.qps is required" .Values.global.gardenlet.config.seedClientConnection.qps }}
    burst: {{ required ".Values.global.gardenlet.config.seedClientConnection.burst is required" .Values.global.gardenlet.config.seedClientConnection.burst }}
    {{- if .Values.global.gardenlet.config.seedClientConnection.kubeconfig }}
    kubeconfig: /etc/gardenlet/kubeconfig-seed/kubeconfig
    {{- end }}
  shootClientConnection:
    {{- with .Values.global.gardenlet.config.shootClientConnection.acceptContentTypes }}
    acceptContentTypes: {{ . | quote }}
    {{- end }}
    {{- with .Values.global.gardenlet.config.shootClientConnection.contentType }}
    contentType: {{ . | quote }}
    {{- end }}
    qps: {{ required ".Values.global.gardenlet.config.shootClientConnection.qps is required" .Values.global.gardenlet.config.shootClientConnection.qps }}
    burst: {{ required ".Values.global.gardenlet.config.shootClientConnection.burst is required" .Values.global.gardenlet.config.shootClientConnection.burst }}
  controllers:
    backupBucket:
      concurrentSyncs: {{ required ".Values.global.gardenlet.config.controllers.backupBucket.concurrentSyncs is required" .Values.global.gardenlet.config.controllers.backupBucket.concurrentSyncs }}
    backupEntry:
      concurrentSyncs: {{ required ".Values.global.gardenlet.config.controllers.backupEntry.concurrentSyncs is required" .Values.global.gardenlet.config.controllers.backupEntry.concurrentSyncs }}
      {{- if .Values.global.gardenlet.config.controllers.backupEntry.deletionGracePeriodHours }}
      deletionGracePeriodHours: {{ .Values.global.gardenlet.config.controllers.backupEntry.deletionGracePeriodHours }}
      {{- end }}
      {{- if .Values.global.gardenlet.config.controllers.backupEntry.deletionGracePeriodShootPurposes }}
      deletionGracePeriodShootPurposes:
{{ toYaml .Values.global.gardenlet.config.controllers.backupEntry.deletionGracePeriodShootPurposes | indent 6 }}
      {{- end }}
    bastion:
      concurrentSyncs: {{ required ".Values.global.gardenlet.config.controllers.bastion.concurrentSyncs is required" .Values.global.gardenlet.config.controllers.bastion.concurrentSyncs }}
    {{- if .Values.global.gardenlet.config.controllers.controllerInstallation }}
    controllerInstallation:
      concurrentSyncs: {{ required ".Values.global.gardenlet.config.controllers.controllerInstallation.concurrentSyncs is required" .Values.global.gardenlet.config.controllers.controllerInstallation.concurrentSyncs }}
    {{- end }}
    {{- if .Values.global.gardenlet.config.controllers.controllerInstallationCare }}
    controllerInstallationCare:
      concurrentSyncs: {{ required ".Values.global.gardenlet.config.controllers.controllerInstallationCare.concurrentSyncs is required" .Values.global.gardenlet.config.controllers.controllerInstallationCare.concurrentSyncs }}
      syncPeriod: {{ required ".Values.global.gardenlet.config.controllers.controllerInstallationCare.syncPeriod is required" .Values.global.gardenlet.config.controllers.controllerInstallationCare.syncPeriod }}
    {{- end }}
    {{- if .Values.global.gardenlet.config.controllers.controllerInstallationRequired }}
    controllerInstallationRequired:
      concurrentSyncs: {{ required ".Values.global.gardenlet.config.controllers.controllerInstallationRequired.concurrentSyncs is required" .Values.global.gardenlet.config.controllers.controllerInstallationRequired.concurrentSyncs }}
    {{- end }}
    {{- if .Values.global.gardenlet.config.controllers.seed }}
    seed:
      concurrentSyncs: {{ required ".Values.global.gardenlet.config.controllers.seed.concurrentSyncs is required" .Values.global.gardenlet.config.controllers.seed.concurrentSyncs }}
      syncPeriod: {{ required ".Values.global.gardenlet.config.controllers.seed.syncPeriod is required" .Values.global.gardenlet.config.controllers.seed.syncPeriod }}
      {{- if .Values.global.gardenlet.config.controllers.seed.leaseResyncSeconds }}
      leaseResyncSeconds: {{ .Values.global.gardenlet.config.controllers.seed.leaseResyncSeconds }}
      {{- end }}
      {{- if .Values.global.gardenlet.config.controllers.seed.leaseResyncMissThreshold }}
      leaseResyncMissThreshold: {{ .Values.global.gardenlet.config.controllers.seed.leaseResyncMissThreshold }}
      {{- end }}
    {{- end }}
    shoot:
      concurrentSyncs: {{ required ".Values.global.gardenlet.config.controllers.shoot.concurrentSyncs is required" .Values.global.gardenlet.config.controllers.shoot.concurrentSyncs }}
      {{- if .Values.global.gardenlet.config.controllers.shoot.progressReportPeriod }}
      progressReportPeriod: {{ .Values.global.gardenlet.config.controllers.shoot.progressReportPeriod }}
      {{- end }}
      {{- if .Values.global.gardenlet.config.controllers.shoot.respectSyncPeriodOverwrite }}
      respectSyncPeriodOverwrite: {{ .Values.global.gardenlet.config.controllers.shoot.respectSyncPeriodOverwrite }}
      {{- end }}
      {{- if .Values.global.gardenlet.config.controllers.shoot.reconcileInMaintenanceOnly }}
      reconcileInMaintenanceOnly: {{ .Values.global.gardenlet.config.controllers.shoot.reconcileInMaintenanceOnly }}
      {{- end }}
      syncPeriod: {{ required ".Values.global.gardenlet.config.controllers.shoot.syncPeriod is required" .Values.global.gardenlet.config.controllers.shoot.syncPeriod }}
      retryDuration: {{ required ".Values.global.gardenlet.config.controllers.shoot.retryDuration is required" .Values.global.gardenlet.config.controllers.shoot.retryDuration }}
      {{- if .Values.global.gardenlet.config.controllers.shoot.dnsEntryTTLSeconds }}
      dnsEntryTTLSeconds: {{ .Values.global.gardenlet.config.controllers.shoot.dnsEntryTTLSeconds }}
      {{- end }}
    shootCare:
      concurrentSyncs: {{ required ".Values.global.gardenlet.config.controllers.shootCare.concurrentSyncs is required" .Values.global.gardenlet.config.controllers.shootCare.concurrentSyncs }}
      syncPeriod: {{ required ".Values.global.gardenlet.config.controllers.shootCare.syncPeriod is required" .Values.global.gardenlet.config.controllers.shootCare.syncPeriod }}
      {{- if .Values.global.gardenlet.config.controllers.shootCare.staleExtensionHealthChecks }}
      staleExtensionHealthChecks:
        enabled: {{ required ".Values.global.gardenlet.config.controllers.shootCare.staleExtensionHealthChecks.enabled is required" .Values.global.gardenlet.config.controllers.shootCare.staleExtensionHealthChecks.enabled }}
        {{- if .Values.global.gardenlet.config.controllers.shootCare.staleExtensionHealthChecks.threshold }}
        threshold: {{ .Values.global.gardenlet.config.controllers.shootCare.staleExtensionHealthChecks.threshold }}
        {{- end }}
      {{- end }}
      conditionThresholds:
      {{- if .Values.global.gardenlet.config.controllers.shootCare.conditionThresholds }}
{{ toYaml .Values.global.gardenlet.config.controllers.shootCare.conditionThresholds | indent 6 }}
      {{- end }}
    seedCare:
      syncPeriod: {{ required ".Values.global.gardenlet.config.controllers.seedCare.syncPeriod is required" .Values.global.gardenlet.config.controllers.seedCare.syncPeriod }}
      conditionThresholds:
      {{- if .Values.global.gardenlet.config.controllers.seedCare.conditionThresholds }}
{{ toYaml .Values.global.gardenlet.config.controllers.seedCare.conditionThresholds | indent 6 }}
      {{- end }}
    {{- if .Values.global.gardenlet.config.controllers.shootSecret }}
    shootSecret:
      concurrentSyncs: {{ required ".Values.global.gardenlet.config.controllers.shootSecret.concurrentSyncs is required" .Values.global.gardenlet.config.controllers.shootSecret.concurrentSyncs }}
    {{- end }}
    shootStateSync:
      concurrentSyncs: {{ required ".Values.global.gardenlet.config.controllers.shootStateSync.concurrentSyncs is required" .Values.global.gardenlet.config.controllers.shootStateSync.concurrentSyncs }}
      syncPeriod: {{ required ".Values.global.gardenlet.config.controllers.shootStateSync.syncPeriod is required" .Values.global.gardenlet.config.controllers.shootStateSync.syncPeriod }}
    {{- if .Values.global.gardenlet.config.controllers.managedSeed }}
    managedSeed:
      concurrentSyncs: {{ required ".Values.global.gardenlet.config.controllers.managedSeed.concurrentSyncs is required" .Values.global.gardenlet.config.controllers.managedSeed.concurrentSyncs }}
      syncPeriod: {{ required ".Values.global.gardenlet.config.controllers.managedSeed.syncPeriod is required" .Values.global.gardenlet.config.controllers.managedSeed.syncPeriod }}
      waitSyncPeriod: {{ required ".Values.global.gardenlet.config.controllers.managedSeed.waitSyncPeriod is required" .Values.global.gardenlet.config.controllers.managedSeed.waitSyncPeriod }}
      {{- if .Values.global.gardenlet.config.controllers.managedSeed.syncJitterPeriod }}
      syncJitterPeriod: {{ .Values.global.gardenlet.config.controllers.managedSeed.syncJitterPeriod }}
      {{- end }}
      {{- if .Values.global.gardenlet.config.controllers.managedSeed.jitterUpdates }}
      jitterUpdates: {{ .Values.global.gardenlet.config.controllers.managedSeed.jitterUpdates }}
      {{- end }}
    {{- end }}
    shootMigration:
      concurrentSyncs: {{ required ".Values.global.gardenlet.config.controllers.shootMigration.concurrentSyncs is required" .Values.global.gardenlet.config.controllers.shootMigration.concurrentSyncs }}
      syncPeriod: {{ required ".Values.global.gardenlet.config.controllers.shootMigration.syncPeriod is required" .Values.global.gardenlet.config.controllers.shootMigration.syncPeriod }}
      gracePeriod: {{ required ".Values.global.gardenlet.config.controllers.shootMigration.gracePeriod is required" .Values.global.gardenlet.config.controllers.shootMigration.gracePeriod }}
      lastOperationStaleDuration: {{ required ".Values.global.gardenlet.config.controllers.shootMigration.lastOperationStaleDuration is required" .Values.global.gardenlet.config.controllers.shootMigration.lastOperationStaleDuration }}
    backupEntryMigration:
      concurrentSyncs: {{ required ".Values.global.gardenlet.config.controllers.backupEntryMigration.concurrentSyncs is required" .Values.global.gardenlet.config.controllers.backupEntryMigration.concurrentSyncs }}
      syncPeriod: {{ required ".Values.global.gardenlet.config.controllers.backupEntryMigration.syncPeriod is required" .Values.global.gardenlet.config.controllers.backupEntryMigration.syncPeriod }}
      gracePeriod: {{ required ".Values.global.gardenlet.config.controllers.backupEntryMigration.gracePeriod is required" .Values.global.gardenlet.config.controllers.backupEntryMigration.gracePeriod }}
      lastOperationStaleDuration: {{ required ".Values.global.gardenlet.config.controllers.backupEntryMigration.lastOperationStaleDuration is required" .Values.global.gardenlet.config.controllers.backupEntryMigration.lastOperationStaleDuration }}
  resources:
    capacity:
      shoots: {{ required ".Values.global.gardenlet.config.resources.capacity.shoots is required" .Values.global.gardenlet.config.resources.capacity.shoots }}
  leaderElection:
    leaderElect: {{ required ".Values.global.gardenlet.config.leaderElection.leaderElect is required" .Values.global.gardenlet.config.leaderElection.leaderElect }}
    leaseDuration: {{ required ".Values.global.gardenlet.config.leaderElection.leaseDuration is required" .Values.global.gardenlet.config.leaderElection.leaseDuration }}
    renewDeadline: {{ required ".Values.global.gardenlet.config.leaderElection.renewDeadline is required" .Values.global.gardenlet.config.leaderElection.renewDeadline }}
    retryPeriod: {{ required ".Values.global.gardenlet.config.leaderElection.retryPeriod is required" .Values.global.gardenlet.config.leaderElection.retryPeriod }}
    resourceLock: {{ required ".Values.global.gardenlet.config.leaderElection.resourceLock is required" .Values.global.gardenlet.config.leaderElection.resourceLock }}
    {{- if .Values.global.gardenlet.config.leaderElection.resourceName }}
    resourceName: {{ .Values.global.gardenlet.config.leaderElection.resourceName }}
    {{- end }}
    {{- if .Values.global.gardenlet.config.leaderElection.resourceNamespace }}
    resourceNamespace: {{ .Values.global.gardenlet.config.leaderElection.resourceNamespace }}
    {{- end }}
  logLevel: {{ required ".Values.global.gardenlet.config.logLevel is required" .Values.global.gardenlet.config.logLevel }}
  kubernetesLogLevel: {{ required ".Values.global.gardenlet.config.kubernetesLogLevel is required" .Values.global.gardenlet.config.kubernetesLogLevel }}
  server:
    https:
      bindAddress: {{ required ".Values.global.gardenlet.config.server.https.bindAddress is required" .Values.global.gardenlet.config.server.https.bindAddress }}
      port: {{ required ".Values.global.gardenlet.config.server.https.port is required" .Values.global.gardenlet.config.server.https.port }}
      {{- if .Values.global.gardenlet.config.server.https.tls }}
      tls:
        serverCertPath: /etc/gardenlet/srv/gardenlet.crt
        serverKeyPath: /etc/gardenlet/srv/gardenlet.key
      {{- end }}
  {{- if .Values.global.gardenlet.config.debugging }}
  debugging:
    enableProfiling: {{ .Values.global.gardenlet.config.debugging.enableProfiling | default false }}
    enableContentionProfiling: {{ .Values.global.gardenlet.config.debugging.enableContentionProfiling | default false }}
  {{- end }}
  {{- if .Values.global.gardenlet.config.featureGates }}
  featureGates:
{{ toYaml .Values.global.gardenlet.config.featureGates | indent 4 }}
  {{- end }}
  {{- if .Values.global.gardenlet.config.seedConfig }}
  seedConfig:
{{ toYaml .Values.global.gardenlet.config.seedConfig | indent 4 }}
  {{- end }}
  {{- if .Values.global.gardenlet.config.logging }}
  logging:
{{ toYaml .Values.global.gardenlet.config.logging | indent 4 }}
  {{- end }}
  {{- if .Values.global.gardenlet.config.monitoring }}
  monitoring:
{{ toYaml .Values.global.gardenlet.config.monitoring | indent 4 }}
  {{- end }}
  {{- if .Values.global.gardenlet.config.sni }}
  sni:
{{ toYaml .Values.global.gardenlet.config.sni | trim | indent 4 }}
  {{- end }}
  {{- if .Values.global.gardenlet.config.etcdConfig }}
  etcdConfig:
{{ toYaml .Values.global.gardenlet.config.etcdConfig | indent 4}}
  {{- end}}
  {{- if .Values.global.gardenlet.config.exposureClassHandlers }}
  exposureClassHandlers:
{{ toYaml .Values.global.gardenlet.config.exposureClassHandlers | indent 2 }}
  {{- end }}

{{- end -}}

{{- define "gardenlet.config.name" -}}
gardenlet-configmap-{{ include "gardenlet.config.data" . | sha256sum | trunc 8 }}
{{- end -}}

