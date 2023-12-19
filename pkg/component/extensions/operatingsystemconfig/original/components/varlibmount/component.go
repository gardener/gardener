// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package varlibmount

import (
	"k8s.io/utils/pointer"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/kubelet"
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
