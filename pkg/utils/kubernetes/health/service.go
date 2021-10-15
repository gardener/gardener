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

package health

import (
	"context"
	"fmt"

	"github.com/gardener/gardener/pkg/utils/kubernetes"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const eventLimit = 5

// CheckService checks whether the given service is healthy.
// A Service is considered unhealthy if it is of type `LoadBalancer` but doesn't have an ingress element in its status.
func CheckService(ctx context.Context, scheme *runtime.Scheme, c client.Client, service *corev1.Service) error {
	if service.Spec.Type != corev1.ServiceTypeLoadBalancer {
		return nil
	}
	if len(service.Status.LoadBalancer.Ingress) > 0 {
		return nil
	}
	// consult service events for more information
	noIngressMsg := "service is missing ingress status"
	eventsMsg, err := kubernetes.FetchEventMessages(ctx, scheme, c, service, corev1.EventTypeWarning, eventLimit)
	if err != nil {
		return fmt.Errorf("%s but couldn't read events for more information: %s", noIngressMsg, err)
	}
	if eventsMsg != "" {
		noIngressMsg = fmt.Sprintf("%s\n\n%s", noIngressMsg, eventsMsg)
	}
	return fmt.Errorf(noIngressMsg)
}
