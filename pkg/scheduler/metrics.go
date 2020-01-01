// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package scheduler

import (
	gardenmetrics "github.com/gardener/gardener/pkg/controllerutils/metrics"
)

var (
	// ControllerWorkerSum is a metric descriptor which collects the current amount of workers per controller.
	ControllerWorkerSum = gardenmetrics.NewMetricDescriptor("garden_scheduler_worker_amount", "Count of currently running controller workers")

	// ScrapeFailures is a metric descriptor which counts the amount scrape issues grouped by kind.
	ScrapeFailures = gardenmetrics.NewCounterVec("garden_scheduler_scrape_failure_total", "Total count of scraping failures, grouped by kind/group of metric(s)")
)
