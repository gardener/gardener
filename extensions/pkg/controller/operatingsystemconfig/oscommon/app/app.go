// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package app

import (
	"context"
	"os"

	extcontroller "github.com/gardener/gardener/extensions/pkg/controller"
	controllercmd "github.com/gardener/gardener/extensions/pkg/controller/cmd"
	"github.com/gardener/gardener/extensions/pkg/controller/operatingsystemconfig/oscommon"
	oscommoncmd "github.com/gardener/gardener/extensions/pkg/controller/operatingsystemconfig/oscommon/cmd"
	"github.com/gardener/gardener/extensions/pkg/controller/operatingsystemconfig/oscommon/generator"
	"github.com/gardener/gardener/extensions/pkg/util"

	"github.com/spf13/cobra"
	componentbaseconfig "k8s.io/component-base/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// NewControllerCommand creates a new command for running an OS controller.
func NewControllerCommand(ctx context.Context, ctrlName string, osTypes []string, generator generator.Generator) *cobra.Command {
	var (
		restOpts = &controllercmd.RESTOptions{}
		mgrOpts  = &controllercmd.ManagerOptions{
			LeaderElection:          true,
			LeaderElectionID:        controllercmd.LeaderElectionNameID(ctrlName),
			LeaderElectionNamespace: os.Getenv("LEADER_ELECTION_NAMESPACE"),
		}
		ctrlOpts = &controllercmd.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}

		reconcileOpts = &controllercmd.ReconcilerOptions{}

		controllerSwitches = oscommoncmd.SwitchOptions(ctrlName, osTypes, generator)

		aggOption = controllercmd.NewOptionAggregator(
			restOpts,
			mgrOpts,
			ctrlOpts,
			reconcileOpts,
			controllerSwitches,
		)
	)

	cmd := &cobra.Command{
		Use: "os-" + ctrlName + "-controller-manager",

		Run: func(cmd *cobra.Command, args []string) {
			if err := aggOption.Complete(); err != nil {
				controllercmd.LogErrAndExit(err, "Error completing options")
			}

			// TODO: Make these flags configurable via command line parameters or component config file.
			util.ApplyClientConnectionConfigurationToRESTConfig(&componentbaseconfig.ClientConnectionConfiguration{
				QPS:   100.0,
				Burst: 130,
			}, restOpts.Completed().Config)

			mgr, err := manager.New(restOpts.Completed().Config, mgrOpts.Completed().Options())
			if err != nil {
				controllercmd.LogErrAndExit(err, "Could not instantiate manager")
			}

			if err := extcontroller.AddToScheme(mgr.GetScheme()); err != nil {
				controllercmd.LogErrAndExit(err, "Could not update manager scheme")
			}

			ctrlOpts.Completed().Apply(&oscommon.DefaultAddOptions.Controller)

			reconcileOpts.Completed().Apply(&oscommon.DefaultAddOptions.IgnoreOperationAnnotation)

			if err := controllerSwitches.Completed().AddToManager(mgr); err != nil {
				controllercmd.LogErrAndExit(err, "Could not add controller to manager")
			}

			if err := mgr.Start(ctx.Done()); err != nil {
				controllercmd.LogErrAndExit(err, "Error running manager")
			}
		},
	}

	aggOption.AddFlags(cmd.Flags())

	return cmd
}
