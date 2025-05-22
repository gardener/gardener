// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package components

import (
	"github.com/Masterminds/semver/v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
	CABundle                string
	ClusterDNSAddresses     []string
	ClusterDomain           string
	CRIName                 extensionsv1alpha1.CRIName
	Images                  map[string]*imagevector.Image
	NodeLabels              map[string]string
	NodeMonitorGracePeriod  metav1.Duration
	KubeletCABundle         []byte
	KubeletCLIFlags         ConfigurableKubeletCLIFlags
	KubeletConfigParameters ConfigurableKubeletConfigParameters
	KubeletDataVolumeName   *string
	KubeProxyEnabled        bool
	KubernetesVersion       *semver.Version
	SSHPublicKeys           []string
	SSHAccessEnabled        bool
	ValiIngress             string
	ValitailEnabled         bool
	APIServerURL            string
	Sysctls                 map[string]string
	PreferIPv6              bool
	Taints                  []corev1.Taint
}
