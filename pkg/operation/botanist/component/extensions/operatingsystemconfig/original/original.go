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

package original

import (
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components/containerd"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components/docker"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components/gardeneruser"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components/journald"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components/kernelconfig"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components/kubelet"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components/promtail"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components/rootcertificates"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components/varlibmount"
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

	for _, component := range ComponentsFn(ctx.CRIName) {
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
func Components(criName extensionsv1alpha1.CRIName) []components.Component {
	components := []components.Component{
		promtail.New(),
		varlibmount.New(),
		rootcertificates.New(),
		getContainerRuntimeComponent(criName),
		journald.New(),
		kernelconfig.New(),
		kubelet.New(),
		gardeneruser.New(),
	}

	if criName == extensionsv1alpha1.CRINameContainerD {
		components = append(components, containerd.NewInitializer())
	}

	return components
}

func getContainerRuntimeComponent(criName extensionsv1alpha1.CRIName) components.Component {
	if criName == extensionsv1alpha1.CRINameContainerD {
		return containerd.New()
	}
	return docker.New()
}
