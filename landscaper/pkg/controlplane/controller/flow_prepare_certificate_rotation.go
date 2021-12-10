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

package controller

import (
	"context"
)

// PrepareCompleteCertificateRotation clears are CA and TLS certificates in the import configuration to subsequently regenerate them.
func (o *operation) PrepareCompleteCertificateRotation(ctx context.Context) error {
	// Gardener API Server CA
	o.imports.GardenerAPIServer.ComponentConfiguration.CA.Crt = nil
	o.exports.GardenerAPIServerCA.Rotated = true

	o.imports.GardenerAPIServer.ComponentConfiguration.TLS.Crt = nil
	o.exports.GardenerAPIServerTLSServing.Rotated = true

	// Gardener Controller Manager
	o.imports.GardenerControllerManager.ComponentConfiguration.TLS.Crt = nil
	o.exports.GardenerControllerManagerTLSServing.Rotated = true

	// Gardener Admission Controller
	if o.imports.GardenerAdmissionController.Enabled {
		o.imports.GardenerAdmissionController.ComponentConfiguration.CA.Crt = nil
		o.exports.GardenerAdmissionControllerCA.Rotated = true

		o.imports.GardenerAdmissionController.ComponentConfiguration.TLS.Crt = nil
		o.exports.GardenerAdmissionControllerTLSServing.Rotated = true
	}

	return nil
}
