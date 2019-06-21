{{- define "global-network-policies.except-to-networks" -}}
{{- /*
TODO (mvladev): Remove this extra -to block once Gardener stops supporting
Kubernetes 1.10 for Seed clusters.
*/ -}}
{{- range $i, $network := $ }}
  - to:
    - ipBlock:
        cidr: {{ required ".network.cidr is required" $network.network }}
{{- if $network.except }}
        except:
{{ toYaml $network.except | indent 8 }}
{{- end }}
{{- end }}
{{- end -}}
