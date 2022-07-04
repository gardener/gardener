{{- define "apiserverversion" -}}
apiserver.k8s.io/v1alpha1
{{- end -}}

{{- define "auditkubernetesversion" -}}
audit.k8s.io/v1
{{- end -}}

{{- define "rbacversion" -}}
rbac.authorization.k8s.io/v1
{{- end -}}

{{- define "deploymentversion" -}}
apps/v1
{{- end -}}

{{- define "daemonsetversion" -}}
apps/v1
{{- end -}}

{{- define "statefulsetversion" -}}
apps/v1
{{- end -}}

{{- define "apiserviceversion" -}}
apiregistration.k8s.io/v1
{{- end -}}

{{- define "networkpolicyversion" -}}
networking.k8s.io/v1
{{- end -}}

{{- define "priorityclassversion" -}}
scheduling.k8s.io/v1
{{- end -}}

{{- define "cronjobversion" -}}
{{- if semverCompare ">= 1.21-0" .Capabilities.KubeVersion.GitVersion -}}
batch/v1
{{- else -}}
batch/v1beta1
{{- end -}}
{{- end -}}

{{- define "hpaversion" -}}
{{- if semverCompare ">= 1.23-0" .Capabilities.KubeVersion.GitVersion -}}
autoscaling/v2
{{- else -}}
autoscaling/v2beta1
{{- end -}}
{{- end -}}

{{- define "webhookadmissionregistration" -}}
admissionregistration.k8s.io/v1
{{- end -}}

{{- define "poddisruptionbudgetversion" -}}
policy/v1beta1
{{- end -}}

{{- define "podsecuritypolicyversion" -}}
policy/v1beta1
{{- end -}}

{{- define "ingressversion" -}}
networking.k8s.io/v1
{{- end -}}
