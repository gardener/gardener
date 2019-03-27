{{- define "prune.whitelist" -}}
{{ include "webhookadmissionregistration" . }}/MutatingWebhookConfiguration
{{ include "webhookadmissionregistration" . }}/ValidatingWebhookConfiguration
{{ include "apiserviceversion" . }}/APIService
{{ include "hpaversion" . }}/HorizontalPodAutoscaler
core/v1/LimitRange
core/v1/ResourceQuota
core/v1/ServiceAccount
{{ include "ingressversion" . }}/Ingress
{{ include "podsecuritypolicyversion" .}}/PodSecurityPolicy
{{ include "networkpolicyversion" . }}/NetworkPolicy
{{ include "poddisruptionbudgetversion" . }}/PodDisruptionBudget
{{ include "rbacversion" . }}/ClusterRole
{{ include "rbacversion" . }}/ClusterRoleBinding
{{ include "rbacversion" . }}/Role
{{ include "rbacversion" . }}/RoleBinding
storage.k8s.io/v1/StorageClass
{{- end -}}
