// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package varlibkubeletmount

import (
	"k8s.io/utils/ptr"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/kubelet"
)

// UnitName is the name of the var-lib-kubelet-mount unit.
const UnitName = "var-lib-kubelet.mount"

type component struct{}

// New returns a new var-lib-kubelet-mount component.
func New() *component {
	return &component{}
}

func (component) Name() string {
	return "var-lib-kubelet-mount"
}

func (component) Config(ctx components.Context) ([]extensionsv1alpha1.Unit, []extensionsv1alpha1.File, error) {
	if ctx.KubeletDataVolumeName == nil {
		return nil, nil, nil
	}

	return []extensionsv1alpha1.Unit{
		{
			Name: UnitName,
			Content: ptr.To(`[Unit]
Description=mount ` + kubelet.PathKubeletDirectory + ` on kubelet data device
Before=` + kubelet.UnitName + `
[Mount]
What=/dev/disk/by-label/KUBEDEV
Where=` + kubelet.PathKubeletDirectory + `
Type=ext4
Options=defaults,prjquota,errors=remount-ro
[Install]
WantedBy=local-fs.target`),
		},
	}, nil, nil
}
