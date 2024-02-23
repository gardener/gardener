{{- define "image" -}}
{{- if hasPrefix "sha256:" (required "$.tag is required" $.tag) -}}
{{ required "$.repository is required" $.repository }}@{{ required "$.tag is required" $.tag }}
{{- else -}}
{{ required "$.repository is required" $.repository }}:{{ required "$.tag is required" $.tag }}
{{- end -}}
{{- end -}}

{{- define "operator.kubeconfig.data" -}}
kubeconfig: {{ .Values.config.runtimeClientConnection.kubeconfig | b64enc }}
{{- end -}}

{{- define "operator.kubeconfig.name" -}}
gardener-operator-kubeconfig-{{ include "operator.kubeconfig.data" . | sha256sum | trunc 8 }}
{{- end -}}

{{- define "operator.imagevector-overwrite.data" -}}
images_overwrite.yaml: |
{{ .Values.imageVectorOverwrite | indent 2 }}
{{- end -}}

{{- define "operator.imagevector-overwrite.name" -}}
gardener-operator-imagevector-overwrite-{{ include "operator.imagevector-overwrite.data" . | sha256sum | trunc 8 }}
{{- end -}}

{{- define "operator.imagevector-overwrite-components.data" -}}
components.yaml: |
{{ .Values.componentImageVectorOverwrites | indent 2 }}
{{- end -}}

{{- define "operator.imagevector-overwrite-components.name" -}}
gardener-operator-imagevector-overwrite-components-{{ include "operator.imagevector-overwrite-components.data" . | sha256sum | trunc 8 }}
{{- end -}}

{{- define "operator.config.data" -}}
config.yaml: |
  ---
  apiVersion: operator.config.gardener.cloud/v1alpha1
  kind: OperatorConfiguration
  runtimeClientConnection:
    qps: {{ .Values.config.runtimeClientConnection.qps }}
    burst: {{ .Values.config.runtimeClientConnection.burst }}
  {{- if .Values.config.runtimeClientConnection.kubeconfig }}
    kubeconfig: /etc/gardener-operator/kubeconfig/kubeconfig
  {{- end }}
  virtualClientConnection:
    qps: {{ .Values.config.virtualClientConnection.qps }}
    burst: {{ .Values.config.virtualClientConnection.burst }}
  leaderElection:
    leaderElect: {{ .Values.config.leaderElection.leaderElect }}
    leaseDuration: {{ .Values.config.leaderElection.leaseDuration }}
    renewDeadline: {{ .Values.config.leaderElection.renewDeadline }}
    retryPeriod: {{ .Values.config.leaderElection.retryPeriod }}
    resourceLock: {{ .Values.config.leaderElection.resourceLock }}
    resourceName: {{ .Values.config.leaderElection.resourceName }}
    resourceNamespace: {{ .Release.Namespace }}
  logLevel: {{ .Values.config.logLevel | default "info" }}
  logFormat: {{ .Values.config.logFormat | default "json" }}
  server:
    webhooks:
      bindAddress: {{ .Values.config.server.webhooks.bindAddress }}
      port: {{ .Values.config.server.webhooks.port }}
    healthProbes:
      bindAddress: {{ .Values.config.server.healthProbes.bindAddress }}
      port: {{ .Values.config.server.healthProbes.port }}
    metrics:
      bindAddress: {{ .Values.config.server.metrics.bindAddress }}
      port: {{ .Values.config.server.metrics.port }}
  {{- if .Values.config.debugging }}
  debugging:
    enableProfiling: {{ .Values.config.debugging.enableProfiling }}
    enableContentionProfiling: {{ .Values.config.debugging.enableContentionProfiling }}
  {{- end }}
  featureGates:
{{ toYaml .Values.config.featureGates | indent 4 }}
  controllers:
    garden:
      {{- if .Values.config.controllers.garden.concurrentSyncs }}
      concurrentSyncs: {{ .Values.config.controllers.garden.concurrentSyncs }}
      {{- end }}
      {{- if .Values.config.controllers.garden.syncPeriod }}
      syncPeriod: {{ .Values.config.controllers.garden.syncPeriod }}
      {{- end }}
      {{- if .Values.config.controllers.garden.etcdConfig }}
      etcdConfig:
{{ toYaml .Values.config.controllers.garden.etcdConfig | indent 8 }}
      {{- end }}
    {{- if .Values.config.controllers.gardenCare }}
    gardenCare:
      {{- if .Values.config.controllers.gardenCare.syncPeriod }}
      syncPeriod: {{ .Values.config.controllers.gardenCare.syncPeriod }}
      {{- end }}
      {{- if .Values.config.controllers.gardenCare.conditionThresholds }}
      conditionThresholds:
{{ toYaml .Values.config.controllers.gardenCare.conditionThresholds | indent 6 }}
      {{- end }}
    {{- end }}
    {{- if .Values.config.controllers.networkPolicy }}
    networkPolicy:
      {{- if .Values.config.controllers.networkPolicy.concurrentSyncs }}
      concurrentSyncs: {{ .Values.config.controllers.networkPolicy.concurrentSyncs }}
      {{- end }}
      {{- if .Values.config.controllers.networkPolicy.additionalNamespaceSelectors }}
      additionalNamespaceSelectors:
{{ toYaml .Values.config.controllers.networkPolicy.additionalNamespaceSelectors | indent 6 }}
      {{- end }}
    {{- end }}
    {{- if .Values.config.controllers.vpaEvictionRequirements }}
    vpaEvictionRequirements:
      {{- if .Values.config.controllers.vpaEvictionRequirements.concurrentSyncs }}
      concurrentSyncs: {{ .Values.config.controllers.vpaEvictionRequirements.concurrentSyncs }}
      {{- end }}
    {{- end }}
  {{- if .Values.nodeToleration }}
  nodeToleration:
{{ toYaml .Values.nodeToleration | indent 4 }}
  {{- end }}
{{- end -}}

{{- define "operator.config.name" -}}
gardener-operator-configmap-{{ include "operator.config.data" . | sha256sum | trunc 8 }}
{{- end -}}

