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

package varlibmount

import (
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components/kubelet"

	"k8s.io/utils/pointer"
)

// UnitName is the name of the var-lib-mount unit.
const UnitName = "var-lib.mount"

type component struct{}

// New returns a new var-lib-mount component.
func New() *component {
	return &component{}
}

func (component) Name() string {
	return "var-lib-mount"
}

func (component) Config(ctx components.Context) ([]extensionsv1alpha1.Unit, []extensionsv1alpha1.File, error) {
	if ctx.KubeletDataVolumeName == nil {
		return nil, nil, nil
	}

	const pathVarLib = "/var/lib"

	return []extensionsv1alpha1.Unit{
		{
			Name: "var-lib.mount",
			Content: pointer.String(`[Unit]
Description=mount ` + pathVarLib + ` on kubelet data device
Before=` + kubelet.UnitName + `
[Mount]
What=/dev/disk/by-label/kubeletdev
Where=` + pathVarLib + `
Type=xfs
Options=defaults
[Install]
WantedBy=local-fs.target`),
		},
	}, nil, nil
}
