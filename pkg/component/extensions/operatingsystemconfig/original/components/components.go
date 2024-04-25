// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package components

import (
	"github.com/Masterminds/semver/v3"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/imagevector"
)

// Component is an interface which can be implemented by operating system config components.
type Component interface {
	// Name returns the name of the component.
	Name() string
	// Config takes a configuration context and returns units and files.
	Config(Context) ([]extensionsv1alpha1.Unit, []extensionsv1alpha1.File, error)
}

// Context contains configuration for the components.
type Context struct {
	Key                     string
	CABundle                *string
	ClusterDNSAddress       string
	ClusterDomain           string
	CRIName                 extensionsv1alpha1.CRIName
	Images                  map[string]*imagevector.Image
	NodeLabels              map[string]string
	KubeletCABundle         []byte
	KubeletCLIFlags         ConfigurableKubeletCLIFlags
	KubeletConfigParameters ConfigurableKubeletConfigParameters
	KubeletDataVolumeName   *string
	KubernetesVersion       *semver.Version
	SSHPublicKeys           []string
	SSHAccessEnabled        bool
	ValiIngress             string
	ValitailEnabled         bool
	APIServerURL            string
	Sysctls                 map[string]string
	PreferIPv6              bool
}
