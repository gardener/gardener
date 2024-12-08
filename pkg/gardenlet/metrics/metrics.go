// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	runtimemetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/gardener/gardener/pkg/utils/flow"
)

func init() {
	flow.RegisterMetrics(runtimemetrics.Registry)
}
