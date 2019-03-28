{{- define "kubeletcomponentconfigversion" -}}
kubelet.config.k8s.io/v1beta1
{{- end -}}

{{- define "schedulercomponentconfigversion" -}}
{{- if semverCompare ">= 1.12-0" .Capabilities.KubeVersion.GitVersion -}}
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

{{- define "auditkubernetesversion" -}}
{{- if semverCompare ">= 1.12-0" .Capabilities.KubeVersion.GitVersion -}}
audit.k8s.io/v1
{{- else -}}
audit.k8s.io/v1beta1
{{- end -}}
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
{{- if semverCompare ">= 1.14-0" .Capabilities.KubeVersion.GitVersion -}}
scheduling.k8s.io/v1
{{- else if semverCompare ">= 1.11-0" .Capabilities.KubeVersion.GitVersion -}}
scheduling.k8s.io/v1beta1
{{- else -}}
scheduling.k8s.io/v1alpha1
{{- end -}}
{{- end -}}

{{- define "cronjobversion" -}}
batch/v1beta1
{{- end -}}

{{- define "hpaversion" -}}
autoscaling/v2beta1
{{- end -}}

{{- define "webhookadmissionregistration" -}}
admissionregistration.k8s.io/v1beta1
{{- end -}}

{{- define "poddisruptionbudgetversion" -}}
policy/v1beta1
{{- end -}}

{{- define "podsecuritypolicyversion" -}}
policy/v1beta1
{{- end -}}

{{- define "ingressversion" -}}
{{- if semverCompare ">= 1.14-0" .Capabilities.KubeVersion.GitVersion -}}
networking.k8s.io/v1beta1
{{- else -}}
extensions/v1beta1
{{- end -}}
{{- end -}}

{{- define "storageclassversion" -}}
{{- if semverCompare ">= 1.13-0" .Capabilities.KubeVersion.GitVersion -}}
storage.k8s.io/v1
{{- else -}}
storage.k8s.io/v1beta1
{{- end -}}
{{- end -}}
