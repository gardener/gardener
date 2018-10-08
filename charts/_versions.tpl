{{/*
This file should only be symlinked! This text should appear to be
modified only for a file in charts/_versions.tpl
*/}}

{{- define "kubeletcomponentconfigversion" -}}
kubelet.config.k8s.io/v1beta1
{{- end -}}

{{- define "schedulercomponentconfigversion" -}}
{{- if semverCompare ">= 1.12" .Capabilities.KubeVersion.GitVersion -}}
kubescheduler.config.k8s.io/v1alpha1
{{- else -}}
componentconfig/v1alpha1
{{- end -}}
{{- end -}}

{{- define "proxycomponentconfigversion" -}}
kubeproxy.config.k8s.io/v1alpha1
{{- end -}}

{{- define "apiserverversion" -}}
apiserver.k8s.io/v1alpha1
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
{{- if semverCompare ">= 1.10" .Capabilities.KubeVersion.GitVersion -}}
apiregistration.k8s.io/v1
{{- else -}}
apiregistration.k8s.io/v1beta1
{{- end -}}
{{- end -}}

{{- define "networkpolicyversion" -}}
networking.k8s.io/v1
{{- end -}}

{{- define "priorityclassversion" -}}
{{- if semverCompare ">= 1.11" .Capabilities.KubeVersion.GitVersion -}}
scheduling.k8s.io/v1beta1
{{- else -}}
scheduling.k8s.io/v1alpha1
{{- end -}}
{{- end -}}

{{- define "hpaversion" -}}
autoscaling/v2beta1
{{- end -}}

{{- define "webhookadmissionregistration" -}}
admissionregistration.k8s.io/v1beta1
{{- end -}}

{{- define "initializeradmissionregistrationversion" -}}
admissionregistration.k8s.io/v1alpha1
{{- end -}}
