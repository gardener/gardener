// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package nodeagent

import (
	"fmt"
	"time"

	"github.com/Masterminds/semver/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/imagevector"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	extensionsv1alpha1helper "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/valitail"
	valiconstants "github.com/gardener/gardener/pkg/component/observability/logging/vali/constants"
	nodeagentv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
)

// PathBinary is a constant for the path to the gardener-node-agent binary file on the VMs.
const PathBinary = v1beta1constants.OperatingSystemConfigFilePathBinaries + "/gardener-node-agent"

var codec runtime.Codec

func init() {
	scheme := runtime.NewScheme()
	utilruntime.Must(nodeagentv1alpha1.AddToScheme(scheme))
	utilruntime.Must(extensionsv1alpha1.AddToScheme(scheme))

	ser := json.NewSerializerWithOptions(json.DefaultMetaFactory, scheme, scheme, json.SerializerOptions{Yaml: true, Pretty: false, Strict: false})
	versions := schema.GroupVersions([]schema.GroupVersion{nodeagentv1alpha1.SchemeGroupVersion, extensionsv1alpha1.SchemeGroupVersion})
	codec = serializer.NewCodecFactory(scheme).CodecForVersions(ser, ser, versions, versions)
}

type component struct{}

// New returns a new Gardener user component.
func New() *component {
	return &component{}
}

func (component) Name() string {
	return "gardener-node-agent"
}

func (component) Config(ctx components.Context) ([]extensionsv1alpha1.Unit, []extensionsv1alpha1.File, error) {
	var caBundle []byte
	if ctx.CABundle != nil {
		caBundle = []byte(*ctx.CABundle)
	}

	var additionalTokenSyncConfigs []nodeagentv1alpha1.TokenSecretSyncConfig
	if ctx.ValitailEnabled {
		additionalTokenSyncConfigs = append(additionalTokenSyncConfigs, nodeagentv1alpha1.TokenSecretSyncConfig{
			SecretName: valiconstants.ValitailTokenSecretName,
			Path:       valitail.PathAuthToken,
		})
	}

	files, err := Files(ComponentConfig(ctx.Key, ctx.KubernetesVersion, ctx.APIServerURL, caBundle, additionalTokenSyncConfigs))
	if err != nil {
		return nil, nil, fmt.Errorf("failed generating files: %w", err)
	}

	files = append(files, extensionsv1alpha1.File{
		Path:        PathBinary,
		Permissions: ptr.To[uint32](0755),
		Content: extensionsv1alpha1.FileContent{
			ImageRef: &extensionsv1alpha1.FileContentImageRef{
				Image:           ctx.Images[imagevector.ContainerImageNameGardenerNodeAgent].String(),
				FilePathInImage: "/gardener-node-agent",
			},
		},
	})

	units := []extensionsv1alpha1.Unit{{
		Name:      nodeagentv1alpha1.UnitName,
		Enable:    ptr.To(true),
		Content:   ptr.To(UnitContent()),
		FilePaths: extensionsv1alpha1helper.FilePathsFrom(files),
	}}

	return units, files, nil
}

// UnitContent returns the systemd unit content for the gardener-node-agent unit.
func UnitContent() string {
	return `[Unit]
Description=Gardener Node Agent
After=network-online.target

[Service]
LimitMEMLOCK=infinity
ExecStart=` + nodeagentv1alpha1.BinaryDir + `/gardener-node-agent --config=` + nodeagentv1alpha1.ConfigFilePath + `
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target`
}

// ComponentConfig returns the component configuration for the gardener-node-agent.
func ComponentConfig(
	oscSecretName string,
	kubernetesVersion *semver.Version,
	apiServerURL string,
	caBundle []byte,
	additionalTokenSyncConfigs []nodeagentv1alpha1.TokenSecretSyncConfig,
) *nodeagentv1alpha1.NodeAgentConfiguration {
	return &nodeagentv1alpha1.NodeAgentConfiguration{
		APIServer: nodeagentv1alpha1.APIServer{
			Server:   apiServerURL,
			CABundle: caBundle,
		},
		Controllers: nodeagentv1alpha1.ControllerConfiguration{
			OperatingSystemConfig: nodeagentv1alpha1.OperatingSystemConfigControllerConfig{
				SecretName:        oscSecretName,
				KubernetesVersion: kubernetesVersion,
			},
			Token: nodeagentv1alpha1.TokenControllerConfig{
				SyncConfigs: append([]nodeagentv1alpha1.TokenSecretSyncConfig{{
					SecretName: nodeagentv1alpha1.AccessSecretName,
					Path:       nodeagentv1alpha1.TokenFilePath,
				}}, additionalTokenSyncConfigs...),
				// It is enough to sync the access tokens every 12h to the disk because they are only rotated roughly
				// each 12h. Furthermore, they are valid for 30d, so there should be enough head time to sync an updated
				// token.
				SyncPeriod: &metav1.Duration{Duration: 12 * time.Hour},
			},
		},
	}
}

// Files returns the files related to the gardener-node-agent unit.
func Files(config *nodeagentv1alpha1.NodeAgentConfiguration) ([]extensionsv1alpha1.File, error) {
	configRaw, err := runtime.Encode(codec, config)
	if err != nil {
		return nil, fmt.Errorf("failed encoding component config: %w", err)
	}

	return []extensionsv1alpha1.File{{
		Path:        nodeagentv1alpha1.ConfigFilePath,
		Permissions: ptr.To[uint32](0600),
		Content:     extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "b64", Data: utils.EncodeBase64(configRaw)}},
	}}, nil
}
