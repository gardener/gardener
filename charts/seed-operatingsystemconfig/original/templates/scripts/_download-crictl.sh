{{- define "download-crictl-script" -}}
- path: /opt/bin/download-crictl
  permissions: 0744
  content:
    inline:
      encoding: ""
      data: |
        #!/bin/bash -eu

        if [ ! -f "/usr/local/bin/crictl" ]; then
            echo "Crictl not found in `/usr/local/bin/crictl` - downloading!"

            # the version of the CRI is independent of the kubelet version - so always use latest crictl version available
            VERSION="v1.18.0"
            wget https://github.com/kubernetes-sigs/cri-tools/releases/download/$VERSION/crictl-$VERSION-linux-amd64.tar.gz
            sudo tar zxvf crictl-$VERSION-linux-amd64.tar.gz -C /usr/local/bin
            rm -f crictl-$VERSION-linux-amd64.tar.gz
        fi
{{- end -}}