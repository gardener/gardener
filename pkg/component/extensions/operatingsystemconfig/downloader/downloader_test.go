// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"
	"k8s.io/utils/ptr"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/downloader"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("Downloader", func() {
	Describe("#Config", func() {
		It("should properly render the expected units and files", func() {
			units, files, err := Config(ccdSecretName, apiServerURL, clusterCASecretName)

			Expect(err).ToNot(HaveOccurred())
			Expect(units).To(ConsistOf(extensionsv1alpha1.Unit{
				Name:    "cloud-config-downloader.service",
				Command: extensionsv1alpha1.UnitCommandPtr(extensionsv1alpha1.CommandStart),
				Enable:  ptr.To(true),
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
							Name:    clusterCASecretName,
							DataKey: "bundle.crt",
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
						TransmitUnencoded: ptr.To(true),
					},
				}))
		})
	})

	Describe("#GenerateRBACResourcesData", func() {
		var (
			secretName1 = "secret1"
			secretName2 = "secret2"

			roleYAML                               string
			roleBindingYAML                        string
			clusterRoleBindingNodeBootstrapperYAML string
			clusterRoleBindingNodeClientYAML       string
			clusterRoleBindingSelfNodeClientYAML   string
		)

		BeforeEach(func() {
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
  - cloud-config-downloader
  - gardener-valitail
  - gardener-node-agent
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
- kind: Group
  name: system:bootstrappers
- kind: ServiceAccount
  name: cloud-config-downloader
  namespace: kube-system
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
		})

		It("should generate the expected RBAC resources when UseGardenerNodeAgent feature gate is off", func() {
			DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.UseGardenerNodeAgent, false))

			data, err := GenerateRBACResourcesData([]string{secretName1, secretName2})
			Expect(err).NotTo(HaveOccurred())
			Expect(data).To(HaveLen(5))
			Expect(string(data["role__kube-system__cloud-config-downloader.yaml"])).To(Equal(roleYAML))
			Expect(string(data["rolebinding__kube-system__cloud-config-downloader.yaml"])).To(Equal(roleBindingYAML))
			Expect(string(data["clusterrolebinding____system_node-bootstrapper.yaml"])).To(Equal(clusterRoleBindingNodeBootstrapperYAML))
			Expect(string(data["clusterrolebinding____system_certificates.k8s.io_certificatesigningrequests_nodeclient.yaml"])).To(Equal(clusterRoleBindingNodeClientYAML))
			Expect(string(data["clusterrolebinding____system_certificates.k8s.io_certificatesigningrequests_selfnodeclient.yaml"])).To(Equal(clusterRoleBindingSelfNodeClientYAML))
		})

		It("should generate the expected RBAC resources when UseGardenerNodeAgent feature gate is on", func() {
			DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.UseGardenerNodeAgent, true))

			clusterRoleBindingNodeBootstrapperYAML = strings.ReplaceAll(clusterRoleBindingNodeBootstrapperYAML, "metadata:", `metadata:
  annotations:
    resources.gardener.cloud/mode: Ignore`)
			clusterRoleBindingNodeClientYAML = strings.ReplaceAll(clusterRoleBindingNodeClientYAML, "metadata:", `metadata:
  annotations:
    resources.gardener.cloud/mode: Ignore`)
			clusterRoleBindingSelfNodeClientYAML = strings.ReplaceAll(clusterRoleBindingSelfNodeClientYAML, "metadata:", `metadata:
  annotations:
    resources.gardener.cloud/mode: Ignore`)

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
	ccdSecretName       = "secret"
	apiServerURL        = "server"
	clusterCASecretName = "ca"

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

	ccdScript = `#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

{
SECRET_NAME="` + ccdSecretName + `"
TOKEN_SECRET_NAME="cloud-config-downloader"

PATH_CLOUDCONFIG_DOWNLOADER_CLIENT_CERT="/var/lib/cloud-config-downloader/credentials/client.crt"
PATH_CLOUDCONFIG_DOWNLOADER_CLIENT_KEY="/var/lib/cloud-config-downloader/credentials/client.key"
PATH_CLOUDCONFIG_DOWNLOADER_TOKEN="/var/lib/cloud-config-downloader/credentials/token"
PATH_BOOTSTRAP_TOKEN="/var/lib/cloud-config-downloader/credentials/bootstrap-token"
PATH_EXECUTOR_SCRIPT="/var/lib/cloud-config-downloader/downloads/execute-cloud-config.sh"
PATH_EXECUTOR_SCRIPT_CHECKSUM="/var/lib/cloud-config-downloader/downloads/execute-cloud-config-checksum"

mkdir -p "/var/lib/cloud-config-downloader/downloads"

function readSecret() {
  wget \
    -qO- \
    --ca-certificate "/var/lib/cloud-config-downloader/credentials/ca.crt" \
    "${@:2}" "$(cat "/var/lib/cloud-config-downloader/credentials/server")/api/v1/namespaces/kube-system/secrets/$1"
}

function readSecretFull() {
  readSecret "$1" "--header=Accept: application/yaml" "${@:2}"
}

function readSecretMeta() {
  readSecret "$1" "--header=Accept: application/yaml;as=PartialObjectMetadata;g=meta.k8s.io;v=v1,application/yaml;as=PartialObjectMetadata;g=meta.k8s.io;v=v1" "${@:2}"
}

function readSecretMetaWithToken() {
  readSecretMeta "$1" "--header=Authorization: Bearer $2"
}

function readSecretWithToken() {
  readSecretFull "$1" "--header=Authorization: Bearer $2"
}

function readSecretWithClientCertificate() {
  readSecretFull "$1" "--certificate=$PATH_CLOUDCONFIG_DOWNLOADER_CLIENT_CERT" "--private-key=$PATH_CLOUDCONFIG_DOWNLOADER_CLIENT_KEY"
}

function extractDataKeyFromSecret() {
  echo "$1" | sed -rn "s/  $2: (.*)/\1/p" | base64 -d
}

function extractChecksumFromSecret() {
  echo "$1" | sed -rn 's/    checksum\/data-script: (.*)/\1/p' | sed -e 's/^"//' -e 's/"$//'
}

function writeToDiskSafely() {
  local data="$1"
  local file_path="$2"

  if echo "$data" > "$file_path.tmp" && ( [[ ! -f "$file_path" ]] || ! diff "$file_path" "$file_path.tmp">/dev/null ); then
    mv "$file_path.tmp" "$file_path"
  elif [[ -f "$file_path.tmp" ]]; then
    rm -f "$file_path.tmp"
  fi
}

# download shoot access token for cloud-config-downloader
if [[ -f "$PATH_CLOUDCONFIG_DOWNLOADER_TOKEN" ]]; then
  if ! SECRET="$(readSecretWithToken "$TOKEN_SECRET_NAME" "$(cat "$PATH_CLOUDCONFIG_DOWNLOADER_TOKEN")")"; then
    echo "Could not retrieve the shoot access secret with name $TOKEN_SECRET_NAME with existing token"
    exit 1
  fi
else
  if [[ -f "$PATH_BOOTSTRAP_TOKEN" ]]; then
    if ! SECRET="$(readSecretWithToken "$TOKEN_SECRET_NAME" "$(cat "$PATH_BOOTSTRAP_TOKEN")")"; then
      echo "Could not retrieve the shoot access secret with name $TOKEN_SECRET_NAME with bootstrap token"
      exit 1
    fi
  else
    if ! SECRET="$(readSecretWithClientCertificate "$TOKEN_SECRET_NAME")"; then
      echo "Could not retrieve the shoot access secret with name $TOKEN_SECRET_NAME with client certificate"
      exit 1
    fi
  fi
fi

TOKEN="$(extractDataKeyFromSecret "$SECRET" "token")"
if [[ -z "$TOKEN" ]]; then
  echo "Token in shoot access secret $TOKEN_SECRET_NAME is empty"
  exit 1
fi
writeToDiskSafely "$TOKEN" "$PATH_CLOUDCONFIG_DOWNLOADER_TOKEN"

# download and run the cloud config execution script
if ! SECRET_META="$(readSecretMetaWithToken "$SECRET_NAME" "$TOKEN")"; then
  echo "Could not retrieve the metadata in secret with name $SECRET_NAME"
  exit 1
fi
NEW_CHECKSUM="$(extractChecksumFromSecret "$SECRET_META")"

OLD_CHECKSUM="<none>"
if [[ -f "$PATH_EXECUTOR_SCRIPT_CHECKSUM" ]]; then
  OLD_CHECKSUM="$(cat "$PATH_EXECUTOR_SCRIPT_CHECKSUM")"
fi

if [[ "$NEW_CHECKSUM" != "$OLD_CHECKSUM" ]]; then
  echo "Checksum of cloud config script has changed compared to what I had downloaded earlier (new: $NEW_CHECKSUM, old: $OLD_CHECKSUM). Fetching new script..."

  if ! SECRET="$(readSecretWithToken "$SECRET_NAME" "$TOKEN")"; then
    echo "Could not retrieve the cloud config script in secret with name $SECRET_NAME"
    exit 1
  fi

  SCRIPT="$(extractDataKeyFromSecret "$SECRET" "script")"
  if [[ -z "$SCRIPT" ]]; then
    echo "Script in cloud config secret $SECRET is empty"
    exit 1
  fi

  writeToDiskSafely "$SCRIPT" "$PATH_EXECUTOR_SCRIPT" && chmod +x "$PATH_EXECUTOR_SCRIPT"
  writeToDiskSafely "$(extractChecksumFromSecret "$SECRET")" "$PATH_EXECUTOR_SCRIPT_CHECKSUM"
fi

"$PATH_EXECUTOR_SCRIPT"
exit $?
}
`
)
