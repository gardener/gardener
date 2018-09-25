{{- define "prune.whitelist" -}}
{{ include "initializeradmissionregistrationversion" . }}/InitializerConfiguration
{{ include "webhookadmissionregistration" . }}/MutatingWebhookConfiguration
{{ include "webhookadmissionregistration" . }}/ValidatingWebhookConfiguration
{{ include "apiserviceversion" . }}/APIService
{{ include "hpaversion" . }}/HorizontalPodAutoscaler
core/v1/LimitRange
core/v1/ResourceQuota
core/v1/ServiceAccount
extensions/v1beta1/Ingress
extensions/v1beta1/PodSecurityPolicy
{{ include "networkpolicyversion" . }}/NetworkPolicy
policy/v1beta1/PodDisruptionBudget
{{ include "rbacversion" . }}/ClusterRole
{{ include "rbacversion" . }}/ClusterRoleBinding
{{ include "rbacversion" . }}/Role
{{ include "rbacversion" . }}/RoleBinding
storage.k8s.io/v1/StorageClass
{{- end -}}