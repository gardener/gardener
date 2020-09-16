// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package actuator

import (
	"github.com/gardener/gardener/extensions/pkg/controller/operatingsystemconfig"
	"github.com/gardener/gardener/extensions/pkg/controller/operatingsystemconfig/oscommon/generator"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Actuator uses a generator to render an OperatingSystemConfiguration for an Operating System
type Actuator struct {
	scheme    *runtime.Scheme
	client    client.Client
	logger    logr.Logger
	osName    string
	generator generator.Generator
}

// NewActuator creates a new actuator with the given logger.
func NewActuator(osName string, generator generator.Generator) operatingsystemconfig.Actuator {
	return &Actuator{
		logger:    log.Log.WithName(osName + "-operatingsystemconfig-actuator"),
		osName:    osName,
		generator: generator,
	}
}

// InjectScheme injects a runtime Scheme to the Actuator
func (a *Actuator) InjectScheme(scheme *runtime.Scheme) error {
	a.scheme = scheme
	return nil
}

// InjectClient injects a Client to the Actuator
func (a *Actuator) InjectClient(client client.Client) error {
	a.client = client
	return nil
}
