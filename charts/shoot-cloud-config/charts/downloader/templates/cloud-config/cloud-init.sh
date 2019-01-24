{{- define "shoot-cloud-config.cloud-init-alicloud" -}}
#!/bin/bash

#Disable locksmithd
systemctl disable locksmithd
systemctl stop locksmithd

#Fix mis-configuration of dockerd
mkdir -p /etc/docker
echo '{ "storage-driver": "devicemapper" }' > /etc/docker/daemon.json
sed -i '/Environment=DOCKER_SELINUX=--selinux-enabled=true/s/^/#/g' /run/systemd/system/docker.service


#Cloud Config Downloader

#ServiceUnit
cat <<EOF > "/etc/systemd/system/cloud-config-downloader.service"
{{ include  "shoot-cloud-config.cloud-config-downloader-svc" . }}
EOF

DOWNLOAD_MAIN_PATH=/var/lib/cloud-config-downloader
mkdir -p $DOWNLOAD_MAIN_PATH

echo {{ include "shoot-cloud-config.cloud-config-downloader" . | b64enc }} | base64 -d > $DOWNLOAD_MAIN_PATH/download-cloud-config.sh
chmod 0744  $DOWNLOAD_MAIN_PATH/download-cloud-config.sh

echo {{ ( required "kubeconfig is required" .Values.kubeconfig ) | b64enc }} | base64 -d > $DOWNLOAD_MAIN_PATH/kubeconfig
chmod 0644  $DOWNLOAD_MAIN_PATH/kubeconfig

META_EP=http://100.100.100.200/latest/meta-data
PROVIDER_ID=`curl -s $META_EP/region-id`.`curl -s $META_EP/instance-id`
echo PROVIDER_ID=$PROVIDER_ID > $DOWNLOAD_MAIN_PATH/provider-id
echo PROVIDER_ID=$PROVIDER_ID >> /etc/environment

systemctl daemon-reload && systemctl restart docker && systemctl enable cloud-config-downloader && systemctl start cloud-config-downloader 
{{- end }}
