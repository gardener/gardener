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

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	namespace = "gardener_admission_controller"

	// ReasonSizeExceeded is the value for reason when a resource exceeded the maximum allowed size.
	ReasonSizeExceeded = "Size Exceeded"

	// ReasonRejectedKubeconfig is the value for reason when a kubeconfig was rejected.
	ReasonRejectedKubeconfig = "Rejected Kubeconfig"
)

var (
	// RejectedResources defines the counter rejected_resources_total.
	RejectedResources = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "rejected_resources_total",
			Help:      "Total number of resources rejected.",
		},
		[]string{
			"operation",
			"kind",
			"namespace",
			"reason",
		},
	)

	// InvalidWebhookRequest defines the counter invalid_webhook_requests_total.
	InvalidWebhookRequest = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "invalid_webhook_requests_total",
			Help:      "Total number of invalid webhook requests.",
		},
		[]string{},
	)
)
