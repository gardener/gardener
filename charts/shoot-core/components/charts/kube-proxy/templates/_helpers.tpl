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

{{- define "kube-proxy.conntrack-fix-script.data" -}}
conntrack_fix.sh: |
  #!/bin/sh -e
  trap "kill -s INT 1" TERM
  apk add conntrack-tools
  sleep 120 & wait
  date
  # conntrack example:
  # tcp      6 113 SYN_SENT src=21.73.193.93 dst=21.71.0.65 sport=1413 dport=443 \
  #   [UNREPLIED] src=21.71.0.65 dst=21.73.193.93 sport=443 dport=1413 mark=0 use=1
  eval "$(
    conntrack -L -p tcp --state SYN_SENT \
    | sed 's/=/ /g'                      \
    | awk '$6 !~ /^10\./ &&
           $8 !~ /^10\./ &&
           $6  == $17    &&
           $8  == $15    &&
           $10 == $21    &&
           $12 == $19 {
             printf "conntrack -D -p tcp -s %s --sport %s -d %s --dport %s;\n",
                                            $6,        $10,  $8,        $12}'
  )"
  while true; do
    date
    sleep 3600 & wait
  done
{{- end -}}

{{- define "kube-proxy.conntrack-fix-script.name" -}}
kube-proxy-conntrack-fix-script-{{ include "kube-proxy.conntrack-fix-script.data" . | sha256sum | trunc 8 }}
{{- end }}
