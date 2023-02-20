// Copyright (c) 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package endpointslicehints

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// Handler handles admission requests and sets hints in EndpointSlice resouces.
type Handler struct {
	Logger logr.Logger
}

// Default defaults the hints of the endpoints in the provided EndpointSlice.
func (h *Handler) Default(ctx context.Context, obj runtime.Object) error {
	endpointSlice, ok := obj.(*discoveryv1.EndpointSlice)
	if !ok {
		return fmt.Errorf("expected *discoveryv1.EndpointSlice but got %T", obj)
	}

	req, err := admission.RequestFromContext(ctx)
	if err != nil {
		return err
	}

	log := h.Logger.WithValues("endpointSlice", kubernetesutils.ObjectKeyForCreateWebhooks(endpointSlice, req))
	log.Info("Mutating endpoints' hints to the corresponding endpoint's zone")

	for i := range endpointSlice.Endpoints {
		endpoint := &endpointSlice.Endpoints[i]

		if endpoint.Zone != nil && len(*endpoint.Zone) > 0 {
			endpoint.Hints = &discoveryv1.EndpointHints{
				ForZones: []discoveryv1.ForZone{
					{
						Name: *endpoint.Zone,
					},
				},
			}
		}
	}

	return nil
}
