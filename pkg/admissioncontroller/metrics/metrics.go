// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	runtimemetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

// Namespace is the metric namespace for the gardener-admission-controller.
const Namespace = "gardener_admission_controller"

var (
	// Factory is used for registering metrics in the controller-runtime metrics registry.
	Factory = promauto.With(runtimemetrics.Registry)

	// RejectedResources defines the counter rejected_resources_total.
	RejectedResources = Factory.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace,
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
)
