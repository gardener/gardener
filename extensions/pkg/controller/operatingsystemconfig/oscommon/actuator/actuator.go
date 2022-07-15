// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package actuator

import (
	"github.com/gardener/gardener/extensions/pkg/controller/operatingsystemconfig"
	"github.com/gardener/gardener/extensions/pkg/controller/operatingsystemconfig/oscommon/generator"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Actuator uses a generator to render an OperatingSystemConfiguration for an Operating System
type Actuator struct {
	scheme    *runtime.Scheme
	client    client.Client
	osName    string
	generator generator.Generator
}

// NewActuator creates a new actuator with the given logger.
func NewActuator(osName string, generator generator.Generator) operatingsystemconfig.Actuator {
	return &Actuator{
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
