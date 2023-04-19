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

package controlplane

import (
	"bytes"
	"context"
	_ "embed"
	"text/template"

	"github.com/Masterminds/semver"
	"github.com/Masterminds/sprig"
	"github.com/go-logr/logr"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"
	"k8s.io/utils/pointer"

	extensionscontextwebhook "github.com/gardener/gardener/extensions/pkg/webhook/context"
	"github.com/gardener/gardener/extensions/pkg/webhook/controlplane/genericmutator"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
)

const pathContainerdConfigScript = v1beta1constants.OperatingSystemConfigFilePathBinaries + "/init-containerd-with-registry-mirrors"

var (
	tplNameInitializer = "init"
	//go:embed templates/scripts/configure-containerd.tpl.sh
	tplContentInitializer string
	tplInitializer        *template.Template
)

func init() {
	var err error
	tplInitializer, err = template.
		New(tplNameInitializer).
		Funcs(sprig.TxtFuncMap()).
		Parse(tplContentInitializer)
	if err != nil {
		panic(err)
	}
}

// NewEnsurer creates a new controlplane ensurer.
func NewEnsurer(logger logr.Logger) genericmutator.Ensurer {
	return &ensurer{
		logger: logger.WithName("local-controlplane-ensurer"),
	}
}

type ensurer struct {
	genericmutator.NoopEnsurer
	logger logr.Logger
}

func (e *ensurer) EnsureKubeletConfiguration(_ context.Context, _ extensionscontextwebhook.GardenContext, _ *semver.Version, newObj, _ *kubeletconfigv1beta1.KubeletConfiguration) error {
	newObj.FailSwapOn = pointer.Bool(false)
	newObj.CgroupDriver = "systemd"
	return nil
}

// EnsureAdditionalFiles ensures that additional required system files are added.
func (e *ensurer) EnsureAdditionalFiles(ctx context.Context, gc extensionscontextwebhook.GardenContext, new, _ *[]extensionsv1alpha1.File) error {
	cluster, err := gc.GetCluster(ctx)
	if err != nil {
		return err
	}

	kindClusterName := "gardener-local-control-plane"
	if v1beta1helper.IsHAControlPlaneConfigured(cluster.Shoot) {
		kindClusterName = "gardener-local-ha-control-plane"
	}

	var script bytes.Buffer
	if err := tplInitializer.Execute(&script, map[string]interface{}{
		"kindClusterName": kindClusterName,
	}); err != nil {
		return err
	}

	appendUniqueFile(new, extensionsv1alpha1.File{
		Path:        pathContainerdConfigScript,
		Permissions: pointer.Int32(0744),
		Content: extensionsv1alpha1.FileContent{
			Inline: &extensionsv1alpha1.FileContentInline{
				Encoding: "b64",
				Data:     utils.EncodeBase64(script.Bytes()),
			},
		},
	})
	return nil
}

func (e *ensurer) EnsureAdditionalUnits(_ context.Context, _ extensionscontextwebhook.GardenContext, new, _ *[]extensionsv1alpha1.Unit) error {
	const unitNameInitializer = "containerd-configuration-local-setup.service"
	unit := extensionsv1alpha1.Unit{
		Name:    unitNameInitializer,
		Command: pointer.String("start"),
		Enable:  pointer.Bool(true),
		Content: pointer.String(`[Unit]
Description=Containerd config configuration for local-setup

[Install]
WantedBy=multi-user.target

[Unit]
After=containerd-initializer.service
Requires=containerd-initializer.service

[Service]
Type=oneshot
RemainAfterExit=yes
ExecStart=` + pathContainerdConfigScript)}

	appendUniqueUnit(new, unit)

	return nil
}

// appendUniqueFile appends a unit file only if it does not exist, otherwise overwrite content of previous files
func appendUniqueFile(files *[]extensionsv1alpha1.File, file extensionsv1alpha1.File) {
	resFiles := make([]extensionsv1alpha1.File, 0, len(*files))

	for _, f := range *files {
		if f.Path != file.Path {
			resFiles = append(resFiles, f)
		}
	}

	*files = append(resFiles, file)
}

// appendUniqueUnit appends a unit only if it does not exist, otherwise overwrite content of previous unit
func appendUniqueUnit(units *[]extensionsv1alpha1.Unit, unit extensionsv1alpha1.Unit) {
	resFiles := make([]extensionsv1alpha1.Unit, 0, len(*units))

	for _, f := range *units {
		if f.Name != unit.Name {
			resFiles = append(resFiles, f)
		}
	}

	*units = append(resFiles, unit)
}
