{{/*
This file should only be symlinked! This text should appear to be
modified only for a file in charts/_versions.tpl
*/}}

{{- define "kubeletcomponentconfigversion" -}}
kubelet.config.k8s.io/v1beta1
{{- end -}}

{{- define "schedulercomponentconfigversion" -}}
componentconfig/v1alpha1
{{- end -}}

{{- define "proxycomponentconfigversion" -}}
{{- if semverCompare ">= 1.9" .Capabilities.KubeVersion.GitVersion -}}
kubeproxy.config.k8s.io/v1alpha1
{{- else -}}
componentconfig/v1alpha1
{{- end -}}
{{- end -}}

{{- define "rbacversion" -}}
rbac.authorization.k8s.io/v1
{{- end -}}

{{- define "deploymentversion" -}}
{{- if semverCompare ">= 1.9" .Capabilities.KubeVersion.GitVersion -}}
apps/v1
{{- else -}}
apps/v1beta2
{{- end -}}
{{- end -}}

{{- define "daemonsetversion" -}}
{{- if semverCompare ">= 1.9" .Capabilities.KubeVersion.GitVersion -}}
apps/v1
{{- else -}}
apps/v1beta2
{{- end -}}
{{- end -}}

{{- define "statefulsetversion" -}}
{{- if semverCompare ">= 1.9" .Capabilities.KubeVersion.GitVersion -}}
apps/v1
{{- else -}}
apps/v1beta2
{{- end -}}
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
