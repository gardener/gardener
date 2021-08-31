{{- define "proxycomponentconfigversion" -}}
kubeproxy.config.k8s.io/v1alpha1
{{- end -}}

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
batch/v1beta1
{{- end -}}

{{- define "hpaversion" -}}
autoscaling/v2beta1
{{- end -}}

{{- define "webhookadmissionregistration" -}}
{{- if semverCompare "<= 1.15.x" .Capabilities.KubeVersion.GitVersion -}}
admissionregistration.k8s.io/v1beta1
{{- else -}}
admissionregistration.k8s.io/v1
{{- end -}}
{{- end -}}

{{- define "poddisruptionbudgetversion" -}}
policy/v1beta1
{{- end -}}

{{- define "podsecuritypolicyversion" -}}
policy/v1beta1
{{- end -}}

{{- define "ingressversion" -}}
{{- if semverCompare ">= 1.19-0" .Capabilities.KubeVersion.GitVersion -}}
networking.k8s.io/v1
{{- else -}}
networking.k8s.io/v1beta1
{{- end -}}
{{- end -}}
