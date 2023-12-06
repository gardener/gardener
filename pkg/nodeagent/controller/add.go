// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/pkg/nodeagent/apis/config"
	"github.com/gardener/gardener/pkg/nodeagent/controller/node"
	"github.com/gardener/gardener/pkg/nodeagent/controller/operatingsystemconfig"
	"github.com/gardener/gardener/pkg/nodeagent/controller/token"
)

// AddToManager adds all controllers to the given manager.
func AddToManager(cancel context.CancelFunc, mgr manager.Manager, cfg *config.NodeAgentConfiguration, hostName string) error {
	if err := (&node.Reconciler{}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding node controller: %w", err)
	}

	if err := (&operatingsystemconfig.Reconciler{
		Config:        cfg.Controllers.OperatingSystemConfig,
		HostName:      hostName,
		CancelContext: cancel,
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding operating system config controller: %w", err)
	}

	if err := (&token.Reconciler{
		Config: cfg.Controllers.Token,
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding token controller: %w", err)
	}

	return nil
}
