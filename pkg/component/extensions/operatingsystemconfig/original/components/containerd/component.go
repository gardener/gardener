// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package containerd

import (
	_ "embed"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/containerd/logrotate"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/rootcertificates"
)

const (
	// UnitName is the name of the containerd service unit.
	UnitName = v1beta1constants.OperatingSystemConfigUnitNameContainerDService
	// PathSocketEndpoint is the path to the containerd unix domain socket.
	PathSocketEndpoint = "unix:///run/containerd/containerd.sock"
	// CgroupPath is the cgroup path the containerd container runtime is isolated in.
	CgroupPath = "/system.slice/containerd.service"
	// ContainerRuntime designates the runtime type
	ContainerRuntime = "containerd"
)

type containerd struct{}

// New returns a new containerd component.
func New() *containerd {
	return &containerd{}
}

func (containerd) Name() string {
	return ContainerRuntime
}

func (containerd) Config(_ components.Context) ([]extensionsv1alpha1.Unit, []extensionsv1alpha1.File, error) {
	const pathLogRotateConfig = "/etc/systemd/containerd.conf"

	logRotateUnits, logRotateFiles := logrotate.Config(pathLogRotateConfig, "/var/log/pods/*/*/*.log", ContainerRuntime)

	// Unit without content to trigger restart of containerd.service when CAs change.
	containerdUnit := extensionsv1alpha1.Unit{
		Name:      UnitName,
		FilePaths: []string{rootcertificates.PathLocalSSLRootCerts},
	}

	return append(logRotateUnits, containerdUnit), logRotateFiles, nil
}
