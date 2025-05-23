// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package original

import (
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/containerd"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/gardeneruser"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/journald"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/kernelconfig"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/kubelet"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/nodeagent"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/rootcertificates"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/sshdensurer"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/valitail"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/varlibkubeletmount"
)

// ComponentsFn is a function that returns the list of original operating system config components.
// Exposed for testing.
var ComponentsFn = Components

// Config returns the units and the files for the OperatingSystemConfig that contains actual cloud-config user data.
func Config(ctx components.Context) ([]extensionsv1alpha1.Unit, []extensionsv1alpha1.File, error) {
	var (
		units []extensionsv1alpha1.Unit
		files []extensionsv1alpha1.File
	)

	for _, component := range ComponentsFn(ctx.SSHAccessEnabled) {
		u, f, err := component.Config(ctx)
		if err != nil {
			return nil, nil, err
		}

		units = append(units, u...)
		files = append(files, f...)
	}

	return units, files, nil
}

// Components computes the original operating system config components.
func Components(sshAccessEnabled bool) []components.Component {
	components := []components.Component{
		valitail.New(),
		varlibkubeletmount.New(),
		rootcertificates.New(),
		containerd.New(),
		journald.New(),
		kernelconfig.New(),
		kubelet.New(),
		sshdensurer.New(),
		nodeagent.New(),
	}

	if sshAccessEnabled {
		components = append(components, gardeneruser.New())
	}

	return components
}
