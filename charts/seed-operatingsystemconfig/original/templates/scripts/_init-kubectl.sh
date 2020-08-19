{{- define "init-kubectl-script" -}}
- path: /opt/bin/init-kubectl
  permissions: 0744
  content:
    inline:
      encoding: ""
      data: |
        #!/bin/bash -eu

        if [ -f "/opt/bin/kubectl" ]; then
            echo "kubectl binary exists, skipping."
            exit 0
        fi

        {{- if eq .Values.worker.cri.name .Values.osc.cri.names.docker }}
            {{- if semverCompare "< 1.17" .Values.kubernetes.version }}
        /usr/bin/docker run --rm -v /opt/bin:/opt/bin:rw {{ required "images.hyperkube is required" .Values.images.hyperkube }} /bin/sh -c "cp /usr/local/bin/kubectl /opt/bin"
            {{- else }}
        /usr/bin/docker run --rm -v /opt/bin:/opt/bin:rw --entrypoint /bin/sh {{ required "images.hyperkube is required" .Values.images.hyperkube }} -c "cp /usr/local/bin/kubectl /opt/bin"
            {{- end }}
        {{- else }}
        trap '/var/lib/gardener-node-bootstrap/clean-sandbox-script.sh gardener-node-bootstrap-sandbox-extract-kubectl' 0

        /usr/local/bin/crictl run /var/lib/gardener-node-bootstrap/container-config-extract-kubectl.json /var/lib/gardener-node-bootstrap/pod-sandbox-config-gardener-bootstrap-kubectl.json

        # wait until the container exited (finished copying the binary) as `crictl` returns immediately after creation
        while [ $(/usr/local/bin/crictl ps -a --name hyperkube-extract-kubectl --output json | jq -r .containers[0].state) != "CONTAINER_EXITED" ]
        do
         echo "Container still running. Waiting ..."
         sleep 1
        done
        echo "Container exited - sandbox can be terminated"
        {{- end -}}
{{- end -}}