{{- define "kube-proxy.name" -}}
{{- if eq .name "" -}}
kube-proxy
{{- else -}}
kube-proxy-{{ .name }}-v{{ .kubernetesVersion }}
{{- end -}}
{{- end -}}

{{- define "kube-proxy.cleanup-script.data" -}}
cleanup.sh: |
  #!/bin/sh -e
  OLD_KUBE_PROXY_MODE="$(cat "$1")"
  if [ -z "${OLD_KUBE_PROXY_MODE}" ] || [ "${OLD_KUBE_PROXY_MODE}" = "${KUBE_PROXY_MODE}" ]; then
    echo "${KUBE_PROXY_MODE}" >"$1"
    echo "Nothing to cleanup - the mode didn't change."
    exit 0
  fi

  /usr/local/bin/kube-proxy --v=2 --cleanup --config=/var/lib/kube-proxy-config/config.yaml --proxy-mode="${OLD_KUBE_PROXY_MODE}"
  echo "${KUBE_PROXY_MODE}" >"$1"
{{- end -}}

{{- define "kube-proxy.cleanup-script.name" -}}
{{ include "kube-proxy.name" . }}-cleanup-script-{{ include "kube-proxy.cleanup-script.data" . | sha256sum | trunc 8 }}
{{- end }}

