// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package healthcheck

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ShootClient is an interface to be used to receive a shoot client.
type ShootClient interface {
	// InjectShootClient injects the shoot client
	InjectShootClient(client.Client)
}

// SeedClient is an interface to be used to receive a seed client.
type SeedClient interface {
	// InjectSeedClient injects the seed client
	InjectSeedClient(client.Client)
}

// ShootClientInto will set the shoot client on i if i implements ShootClient.
func ShootClientInto(client client.Client, i interface{}) bool {
	if s, ok := i.(ShootClient); ok {
		s.InjectShootClient(client)
		return true
	}
	return false
}

// SeedClientInto will set the seed client on i if i implements SeedClient.
func SeedClientInto(client client.Client, i interface{}) bool {
	if s, ok := i.(SeedClient); ok {
		s.InjectSeedClient(client)
		return true
	}
	return false
}
