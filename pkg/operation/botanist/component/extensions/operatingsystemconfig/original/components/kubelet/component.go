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

package kubelet

import (
	"bytes"
	_ "embed"
	"strings"
	"text/template"

	"github.com/Masterminds/semver"
	"github.com/Masterminds/sprig"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/pointer"

	"github.com/gardener/gardener/charts"
	gardencorev1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components/containerd"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components/docker"
	oscutils "github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/utils"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/imagevector"
)

var (
	tplNameHealthMonitor = "health-monitor"
	//go:embed templates/scripts/health-monitor.tpl.sh
	tplContentHealthMonitor string
	tplHealthMonitor        *template.Template
)

func init() {
	var err error
	tplHealthMonitor, err = template.
		New(tplNameHealthMonitor).
		Funcs(sprig.TxtFuncMap()).
		Parse(tplContentHealthMonitor)
	if err != nil {
		panic(err)
	}
}

const (
	// UnitName is the name of the kubelet service.
	UnitName = gardencorev1beta1constants.OperatingSystemConfigUnitNameKubeletService

	// PathKubeletDirectory is the path for the kubelet's directory.
	PathKubeletDirectory = "/var/lib/kubelet"
	// PathKubernetesBinaries is the path for the kubelet and kubectl binaries.
	PathKubernetesBinaries = "/opt/bin"

	// PathKubeconfigBootstrap is the path for the kubelet's bootstrap kubeconfig.
	PathKubeconfigBootstrap = PathKubeletDirectory + "/kubeconfig-bootstrap"
	// PathKubeconfigReal is the path for the kubelet's real kubeconfig (with client certificate after the TLS
	// bootstrapping process finished).
	PathKubeconfigReal = PathKubeletDirectory + "/kubeconfig-real"
	// PathKubeletCACert is the path for the kubelet's certificate authority.
	PathKubeletCACert = PathKubeletDirectory + "/ca.crt"
	// PathKubeletConfig is the path for the kubelet's config file.
	PathKubeletConfig = gardencorev1beta1constants.OperatingSystemConfigFilePathKubeletConfig

	pathVolumePluginDirectory = "/var/lib/kubelet/volumeplugins"
)

type component struct{}

// New returns a new kubelet component.
func New() *component {
	return &component{}
}

func (component) Name() string {
	return "kubelet"
}

func (component) Config(ctx components.Context) ([]extensionsv1alpha1.Unit, []extensionsv1alpha1.File, error) {
	const pathHealthMonitor = "/opt/bin/health-monitor-kubelet"

	var healthMonitorScript bytes.Buffer
	if err := tplHealthMonitor.Execute(&healthMonitorScript, map[string]string{"pathKubeletKubeconfigReal": PathKubeconfigReal}); err != nil {
		return nil, nil, err
	}

	fileContentKubeletConfig, err := getFileContentKubeletConfig(ctx.KubernetesVersion, ctx.ClusterDNSAddress, ctx.ClusterDomain, ctx.KubeletConfigParameters)
	if err != nil {
		return nil, nil, err
	}

	cliFlags := CLIFlags(ctx.KubernetesVersion, ctx.CRIName, ctx.Images[charts.ImageNamePauseContainer], ctx.KubeletCLIFlags)

	return []extensionsv1alpha1.Unit{
			{
				Name:    UnitName,
				Command: pointer.StringPtr("start"),
				Enable:  pointer.BoolPtr(true),
				Content: pointer.StringPtr(`[Unit]
Description=kubelet daemon
Documentation=https://kubernetes.io/docs/admin/kubelet
` + unitConfigAfterCRI(ctx.CRIName) + `
[Install]
WantedBy=multi-user.target
[Service]
Restart=always
RestartSec=5
EnvironmentFile=/etc/environment
EnvironmentFile=-/var/lib/kubelet/extra_args
ExecStartPre=` + execStartPreCopyBinaryFromContainer("kubelet", ctx.Images[charts.ImageNameHyperkube], ctx.KubernetesVersion) + `
ExecStart=` + PathKubernetesBinaries + `/kubelet \
    ` + utils.Indent(strings.Join(cliFlags, " \\\n"), 4) + ` $KUBELET_EXTRA_ARGS`),
			},
			{
				Name:    "kubelet-monitor.service",
				Command: pointer.StringPtr("start"),
				Enable:  pointer.BoolPtr(true),
				Content: pointer.StringPtr(`[Unit]
Description=Kubelet-monitor daemon
After=` + UnitName + `
[Install]
WantedBy=multi-user.target
[Service]
Restart=always
EnvironmentFile=/etc/environment
ExecStartPre=` + execStartPreCopyBinaryFromContainer("kubectl", ctx.Images[charts.ImageNameHyperkube], ctx.KubernetesVersion) + `
ExecStart=` + pathHealthMonitor),
			},
		},
		[]extensionsv1alpha1.File{
			{
				Path:        PathKubeletCACert,
				Permissions: pointer.Int32Ptr(0644),
				Content: extensionsv1alpha1.FileContent{
					Inline: &extensionsv1alpha1.FileContentInline{
						Encoding: "b64",
						Data:     utils.EncodeBase64([]byte(ctx.KubeletCACertificate)),
					},
				},
			},
			{
				Path:        PathKubeletConfig,
				Permissions: pointer.Int32Ptr(0644),
				Content: extensionsv1alpha1.FileContent{
					Inline: fileContentKubeletConfig,
				},
			},
			{
				Path:        pathHealthMonitor,
				Permissions: pointer.Int32Ptr(0755),
				Content: extensionsv1alpha1.FileContent{
					Inline: &extensionsv1alpha1.FileContentInline{
						Encoding: "b64",
						Data:     utils.EncodeBase64(healthMonitorScript.Bytes()),
					},
				},
			},
		},
		nil
}

func getFileContentKubeletConfig(kubernetesVersion *semver.Version, clusterDNSAddress, clusterDomain string, params components.ConfigurableKubeletConfigParameters) (*extensionsv1alpha1.FileContentInline, error) {
	var (
		kubeletConfig = Config(kubernetesVersion, clusterDNSAddress, clusterDomain, params)
		configFCI     = &extensionsv1alpha1.FileContentInline{Encoding: "b64"}
		kcCodec       = NewConfigCodec(oscutils.NewFileContentInlineCodec())
	)

	return kcCodec.Encode(kubeletConfig, configFCI.Encoding)
}

func execStartPreCopyBinaryFromContainer(binaryName string, image *imagevector.Image, kubernetesVersion *semver.Version) string {
	switch {
	case versionConstraintK8sLess117.Check(kubernetesVersion):
		return docker.PathBinary + ` run --rm -v /opt/bin:/opt/bin:rw ` + image.String() + ` /bin/sh -c "cp /usr/local/bin/` + binaryName + ` /opt/bin"`
	case versionConstraintK8sLess119.Check(kubernetesVersion):
		return docker.PathBinary + ` run --rm -v /opt/bin:/opt/bin:rw --entrypoint /bin/sh ` + image.String() + ` -c "cp /usr/local/bin/` + binaryName + ` /opt/bin"`
	}
	return `/usr/bin/env sh -c "ID=\"$(` + docker.PathBinary + ` run --rm -d -v /opt/bin:/opt/bin:rw ` + image.String() + `)\"; ` + docker.PathBinary + ` cp \"$ID\":/` + binaryName + ` /opt/bin; ` + docker.PathBinary + ` stop \"$ID\"; chmod +x /opt/bin/` + binaryName + `"`
}

func unitConfigAfterCRI(criName extensionsv1alpha1.CRIName) string {
	if criName == extensionsv1alpha1.CRINameContainerD {
		return `After=` + containerd.UnitName
	}
	return `After=` + docker.UnitName + `
Wants=docker.socket rpc-statd.service`
}

var (
	versionConstraintK8sLess117         *semver.Constraints
	versionConstraintK8sLess119         *semver.Constraints
	versionConstraintK8sGreaterEqual119 *semver.Constraints
)

func init() {
	var err error

	versionConstraintK8sLess117, err = semver.NewConstraint("< 1.17")
	utilruntime.Must(err)
	versionConstraintK8sLess119, err = semver.NewConstraint("< 1.19")
	utilruntime.Must(err)
	versionConstraintK8sGreaterEqual119, err = semver.NewConstraint(">= 1.19")
	utilruntime.Must(err)
}
