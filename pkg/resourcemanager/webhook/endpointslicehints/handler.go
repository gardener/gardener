// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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

// Handler handles admission requests and sets hints in EndpointSlice resources.
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

	for i, endpoint := range endpointSlice.Endpoints {
		if endpoint.Zone != nil && len(*endpoint.Zone) > 0 {
			endpointSlice.Endpoints[i].Hints = &discoveryv1.EndpointHints{
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
