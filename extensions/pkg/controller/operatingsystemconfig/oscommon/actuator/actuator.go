// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package actuator

import (
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/extensions/pkg/controller/operatingsystemconfig"
	"github.com/gardener/gardener/extensions/pkg/controller/operatingsystemconfig/oscommon/generator"
)

// Actuator uses a generator to render an OperatingSystemConfiguration for an Operating System
type Actuator struct {
	scheme    *runtime.Scheme
	client    client.Client
	osName    string
	generator generator.Generator
}

// NewActuator creates a new actuator with the given logger.
func NewActuator(mgr manager.Manager, osName string, generator generator.Generator) operatingsystemconfig.Actuator {
	return &Actuator{
		scheme:    mgr.GetScheme(),
		client:    mgr.GetClient(),
		osName:    osName,
		generator: generator,
	}
}
