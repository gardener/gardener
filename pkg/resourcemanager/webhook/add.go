// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package webhook

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/pkg/resourcemanager/apis/config"
	"github.com/gardener/gardener/pkg/resourcemanager/webhook/podschedulername"
)

// AddToManager adds all webhook handlers to the given manager.
func AddToManager(mgr manager.Manager, sourceCluster, targetCluster cluster.Cluster, cfg *config.ResourceManagerConfiguration) error {
	if cfg.Webhooks.PodSchedulerName.Enabled {
		if err := (&podschedulername.Handler{
			SchedulerName: *cfg.Webhooks.PodSchedulerName.SchedulerName,
		}).AddToManager(mgr); err != nil {
			return fmt.Errorf("failed adding %s webhook handler: %w", podschedulername.HandlerName, err)
		}
	}

	return nil
}
