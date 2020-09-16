// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardenlet

import (
	gardenmetrics "github.com/gardener/gardener/pkg/controllerutils/metrics"
)

var (
	// ControllerWorkerSum is a metric descriptor which collects the current amount of workers per controller.
	ControllerWorkerSum = gardenmetrics.NewMetricDescriptor("gardenlet_worker_amount", "Count of currently running controller workers")

	// ScrapeFailures is a metric descriptor which counts the amount scrape issues grouped by kind.
	ScrapeFailures = gardenmetrics.NewCounterVec("gardenlet_scrape_failure_total", "Total count of scraping failures, grouped by kind/group of metric(s)")
)
