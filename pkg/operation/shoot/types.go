// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package shoot

import (
	"context"
	"net"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/etcdencryption"
	"github.com/gardener/gardener/pkg/operation/garden"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Builder is an object that builds Shoot objects.
type Builder struct {
	shootObjectFunc  func() (*gardencorev1beta1.Shoot, error)
	cloudProfileFunc func(string) (*gardencorev1beta1.CloudProfile, error)
	shootSecretFunc  func(context.Context, client.Client, string, string) (*corev1.Secret, error)
	projectName      string
	internalDomain   *garden.Domain
	defaultDomains   []*garden.Domain
	disableDNS       bool
}

// Shoot is an object containing information about a Shoot cluster.
type Shoot struct {
	Info         *gardencorev1beta1.Shoot
	Secret       *corev1.Secret
	CloudProfile *gardencorev1beta1.CloudProfile

	SeedNamespace               string
	KubernetesMajorMinorVersion string

	DisableDNS            bool
	InternalClusterDomain string
	ExternalClusterDomain *string
	ExternalDomain        *garden.Domain

	WantsClusterAutoscaler bool
	WantsAlertmanager      bool
	IgnoreAlerts           bool
	HibernationEnabled     bool

	Networks *Networks

	Components *Components

	OperatingSystemConfigsMap map[string]OperatingSystemConfigs
	Extensions                map[string]Extension
	InfrastructureStatus      []byte
	ControlPlaneStatus        []byte
	MachineDeployments        []extensionsv1alpha1.MachineDeployment

	ETCDEncryption *etcdencryption.EncryptionConfig
}

// Components contains different components deployed
type Components struct {
	DNS *DNS
}

// DNS contains references to internal and external DNSProvider and DNSEntry deployers.
type DNS struct {
	ExternalProvider    component.DeployWaiter
	ExternalEntry       component.DeployWaiter
	InternalProvider    component.DeployWaiter
	InternalEntry       component.DeployWaiter
	AdditionalProviders map[string]component.DeployWaiter
	NginxEntry          component.DeployWaiter
}

// Networks contains pre-calculated subnets and IP address for various components.
type Networks struct {
	// Pods subnet
	Pods *net.IPNet
	// Services subnet
	Services *net.IPNet
	// APIServer is the ClusterIP of default/kubernetes Service
	APIServer net.IP
	// CoreDNS is the ClusterIP of kube-system/coredns Service
	CoreDNS net.IP
}

// OperatingSystemConfigs contains operating system configs for the downloader script as well as for the original cloud config.
type OperatingSystemConfigs struct {
	Downloader OperatingSystemConfig
	Original   OperatingSystemConfig
}

// OperatingSystemConfig contains the operating system config's name and data.
type OperatingSystemConfig struct {
	Name string
	Data OperatingSystemConfigData
}

// OperatingSystemConfigData contains the actual content, a command to load it and all units that
// shall be considered for restart on change.
type OperatingSystemConfigData struct {
	Content string
	Command *string
	Units   []string
}

// Extension contains information about the extension api resouce as well as configuration information.
type Extension struct {
	extensionsv1alpha1.Extension
	Timeout time.Duration
}

// IncompleteDNSConfigError is a custom error type.
type IncompleteDNSConfigError struct{}

// Error prints the error message of the IncompleteDNSConfigError error.
func (e *IncompleteDNSConfigError) Error() string {
	return "unable to figure out which secret should be used for dns"
}

// IsIncompleteDNSConfigError returns true if the error indicates that not the DNS config is incomplete.
func IsIncompleteDNSConfigError(err error) bool {
	_, ok := err.(*IncompleteDNSConfigError)
	return ok
}
