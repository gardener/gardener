// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package downloader_test

import (
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/downloader"
	"github.com/gardener/gardener/pkg/utils"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"
)

var _ = Describe("Downloader", func() {
	Describe("#Config", func() {
		It("should properly render the expected units and files", func() {
			units, files, err := Config(ccdSecretName, apiServerURL)

			Expect(err).ToNot(HaveOccurred())
			Expect(units).To(ConsistOf(extensionsv1alpha1.Unit{
				Name:    "cloud-config-downloader.service",
				Command: pointer.String("start"),
				Enable:  pointer.Bool(true),
				Content: pointer.String(unitContent),
			}))
			Expect(files).To(ConsistOf(
				extensionsv1alpha1.File{
					Path:        "/var/lib/cloud-config-downloader/credentials/server",
					Permissions: pointer.Int32(0644),
					Content: extensionsv1alpha1.FileContent{
						Inline: &extensionsv1alpha1.FileContentInline{
							Encoding: "b64",
							Data:     utils.EncodeBase64([]byte(apiServerURL)),
						},
					},
				},
				extensionsv1alpha1.File{
					Path:        "/var/lib/cloud-config-downloader/credentials/ca.crt",
					Permissions: pointer.Int32(0644),
					Content: extensionsv1alpha1.FileContent{
						SecretRef: &extensionsv1alpha1.FileContentSecretRef{
							Name:    "cloud-config-downloader",
							DataKey: "ca.crt",
						},
					},
				},
				extensionsv1alpha1.File{
					Path:        "/var/lib/cloud-config-downloader/credentials/client.crt",
					Permissions: pointer.Int32(0644),
					Content: extensionsv1alpha1.FileContent{
						SecretRef: &extensionsv1alpha1.FileContentSecretRef{
							Name:    "cloud-config-downloader",
							DataKey: "cloud-config-downloader.crt",
						},
					},
				},
				extensionsv1alpha1.File{
					Path:        "/var/lib/cloud-config-downloader/credentials/client.key",
					Permissions: pointer.Int32(0644),
					Content: extensionsv1alpha1.FileContent{
						SecretRef: &extensionsv1alpha1.FileContentSecretRef{
							Name:    "cloud-config-downloader",
							DataKey: "cloud-config-downloader.key",
						},
					},
				},
				extensionsv1alpha1.File{
					Path:        "/var/lib/cloud-config-downloader/download-cloud-config.sh",
					Permissions: pointer.Int32(0744),
					Content: extensionsv1alpha1.FileContent{
						Inline: &extensionsv1alpha1.FileContentInline{
							Encoding: "b64",
							Data:     utils.EncodeBase64([]byte(ccdScript)),
						},
					},
				},
				extensionsv1alpha1.File{
					Path:        "/var/lib/cloud-config-downloader/credentials/bootstrap-token",
					Permissions: pointer.Int32(0644),
					Content: extensionsv1alpha1.FileContent{
						Inline: &extensionsv1alpha1.FileContentInline{
							Data: "<<BOOTSTRAP_TOKEN>>",
						},
						TransmitUnencoded: pointer.Bool(true),
					},
				}))
		})
	})

	Describe("#GenerateRBACResourcesData", func() {
		var (
			secretName1 = "secret1"
			secretName2 = "secret2"

			roleYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  creationTimestamp: null
  name: cloud-config-downloader
  namespace: kube-system
rules:
- apiGroups:
  - ""
  resourceNames:
  - ` + secretName1 + `
  - ` + secretName2 + `
  resources:
  - secrets
  verbs:
  - get
`

			roleBindingYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  creationTimestamp: null
  name: cloud-config-downloader
  namespace: kube-system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: cloud-config-downloader
subjects:
- kind: User
  name: cloud-config-downloader
`

			clusterRoleBindingNodeBootstrapperYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  creationTimestamp: null
  name: system:node-bootstrapper
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:node-bootstrapper
subjects:
- apiGroup: rbac.authorization.k8s.io
  kind: Group
  name: system:bootstrappers
`

			clusterRoleBindingNodeClientYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  creationTimestamp: null
  name: system:certificates.k8s.io:certificatesigningrequests:nodeclient
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:certificates.k8s.io:certificatesigningrequests:nodeclient
subjects:
- apiGroup: rbac.authorization.k8s.io
  kind: Group
  name: system:bootstrappers
`
			clusterRoleBindingSelfNodeClientYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  creationTimestamp: null
  name: system:certificates.k8s.io:certificatesigningrequests:selfnodeclient
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:certificates.k8s.io:certificatesigningrequests:selfnodeclient
subjects:
- apiGroup: rbac.authorization.k8s.io
  kind: Group
  name: system:nodes
`
		)

		It("should generate the expected RBAC resources", func() {
			data, err := GenerateRBACResourcesData([]string{secretName1, secretName2})
			Expect(err).NotTo(HaveOccurred())
			Expect(data).To(HaveLen(5))
			Expect(string(data["role__kube-system__cloud-config-downloader.yaml"])).To(Equal(roleYAML))
			Expect(string(data["rolebinding__kube-system__cloud-config-downloader.yaml"])).To(Equal(roleBindingYAML))
			Expect(string(data["clusterrolebinding____system_node-bootstrapper.yaml"])).To(Equal(clusterRoleBindingNodeBootstrapperYAML))
			Expect(string(data["clusterrolebinding____system_certificates.k8s.io_certificatesigningrequests_nodeclient.yaml"])).To(Equal(clusterRoleBindingNodeClientYAML))
			Expect(string(data["clusterrolebinding____system_certificates.k8s.io_certificatesigningrequests_selfnodeclient.yaml"])).To(Equal(clusterRoleBindingSelfNodeClientYAML))
		})
	})
})

const (
	ccdSecretName = "secret"
	apiServerURL  = "server"

	unitContent = `[Unit]
Description=Downloads the actual cloud config from the Shoot API server and executes it
After=docker.service docker.socket
Wants=docker.socket
[Service]
Restart=always
RestartSec=30
RuntimeMaxSec=1200
EnvironmentFile=/etc/environment
ExecStart=/var/lib/cloud-config-downloader/download-cloud-config.sh
[Install]
WantedBy=multi-user.target`

	ccdScript = `#!/bin/bash -eu

SECRET_NAME="` + ccdSecretName + `"

PATH_CLOUDCONFIG_DOWNLOADER_SERVER="/var/lib/cloud-config-downloader/credentials/server"
PATH_CLOUDCONFIG_DOWNLOADER_CA_CERT="/var/lib/cloud-config-downloader/credentials/ca.crt"
PATH_CLOUDCONFIG_DOWNLOADER_CLIENT_CERT="/var/lib/cloud-config-downloader/credentials/client.crt"
PATH_CLOUDCONFIG_DOWNLOADER_CLIENT_KEY="/var/lib/cloud-config-downloader/credentials/client.key"
PATH_CLOUDCONFIG_CHECKSUM="/var/lib/cloud-config-downloader/downloaded_checksum"

if ! SECRET="$(wget \
  -qO- \
  --header         "Accept: application/yaml" \
  --ca-certificate "$PATH_CLOUDCONFIG_DOWNLOADER_CA_CERT" \
  --certificate    "$PATH_CLOUDCONFIG_DOWNLOADER_CLIENT_CERT" \
  --private-key    "$PATH_CLOUDCONFIG_DOWNLOADER_CLIENT_KEY" \
  "$(cat "$PATH_CLOUDCONFIG_DOWNLOADER_SERVER")/api/v1/namespaces/kube-system/secrets/$SECRET_NAME")"; then

  echo "Could not retrieve the cloud config script in secret with name $SECRET_NAME"
  exit 1
fi

CHECKSUM="$(echo "$SECRET" | sed -rn 's/    checksum\/data-script: (.*)/\1/p' | sed -e 's/^"//' -e 's/"$//')"
echo "$CHECKSUM" > "$PATH_CLOUDCONFIG_CHECKSUM"

SCRIPT="$(echo "$SECRET" | sed -rn 's/  script: (.*)/\1/p')"
echo "$SCRIPT" | base64 -d | bash
`
)
