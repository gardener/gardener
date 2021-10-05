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
	"fmt"
	"sync"
	"time"

	resourcemanagercmd "github.com/gardener/gardener/pkg/resourcemanager/cmd"
	garbagecollectorcontroller "github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector"
	healthcontroller "github.com/gardener/gardener/pkg/resourcemanager/controller/health"
	resourcecontroller "github.com/gardener/gardener/pkg/resourcemanager/controller/managedresource"
	secretcontroller "github.com/gardener/gardener/pkg/resourcemanager/controller/secret"
	"github.com/gardener/gardener/pkg/resourcemanager/healthz"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/component-base/version"
	"k8s.io/component-base/version/verflag"
	runtimelog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var log = runtimelog.Log.WithName("gardener-resource-manager")

// NewResourceManagerCommand creates a new command for running a gardener resource manager controllers.
func NewResourceManagerCommand() *cobra.Command {
	entryLog := log.WithName("entrypoint")

	managerOpts := &resourcemanagercmd.ManagerOptions{}
	sourceClientOpts := &resourcemanagercmd.SourceClientOptions{}
	targetClientOpts := &resourcemanagercmd.TargetClientOptions{}

	resourceControllerOpts := &resourcecontroller.ControllerOptions{}
	secretControllerOpts := &secretcontroller.ControllerOptions{}
	healthControllerOpts := &healthcontroller.ControllerOptions{}
	gcControllerOpts := &garbagecollectorcontroller.ControllerOptions{}

	cmd := &cobra.Command{
		Use: "gardener-resource-manager",

		RunE: func(cmd *cobra.Command, args []string) error {
			verflag.PrintAndExitIfRequested()

			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()

			entryLog.Info("Starting gardener-resource-manager...", "version", version.Get().GitVersion)
			cmd.Flags().VisitAll(func(flag *pflag.Flag) {
				entryLog.Info(fmt.Sprintf("FLAG: --%s=%s", flag.Name, flag.Value))
			})

			if err := resourcemanagercmd.CompleteAll(
				managerOpts,
				sourceClientOpts,
				targetClientOpts,
				resourceControllerOpts,
				secretControllerOpts,
				healthControllerOpts,
				gcControllerOpts,
			); err != nil {
				return err
			}

			var managerOptions manager.Options
			healthz.DefaultAddOptions.Ctx = ctx

			managerOpts.Completed().Apply(&managerOptions)
			sourceClientOpts.Completed().ApplyManagerOptions(&managerOptions)
			sourceClientOpts.Completed().ApplyClientSet(&healthz.DefaultAddOptions.ClientSet)
			targetClientOpts.Completed().Apply(&resourceControllerOpts.Completed().TargetClientConfig)
			targetClientOpts.Completed().Apply(&healthControllerOpts.Completed().TargetClientConfig)
			resourceControllerOpts.Completed().ApplyClassFilter(&secretControllerOpts.Completed().ClassFilter)
			resourceControllerOpts.Completed().ApplyClassFilter(&healthControllerOpts.Completed().ClassFilter)
			if err := resourceControllerOpts.Completed().ApplyDefaultClusterId(ctx, entryLog, sourceClientOpts.Completed().RESTConfig); err != nil {
				return err
			}
			resourceControllerOpts.Completed().GarbageCollectorActivated = gcControllerOpts.Completed().SyncPeriod > 0

			uncachedTargetClientConfig, err := resourcemanagercmd.NewTargetClientConfig(targetClientOpts.KubeconfigPath, true, 0)
			if err != nil {
				return err
			}
			uncachedTargetClientConfig.Apply(&gcControllerOpts.Completed().TargetClientConfig)

			// setup manager
			mgr, err := manager.New(sourceClientOpts.Completed().RESTConfig, managerOptions)
			if err != nil {
				return fmt.Errorf("could not instantiate manager: %w", err)
			}

			// add controllers and health endpoint to manager
			if err := resourcemanagercmd.AddAllToManager(
				mgr,
				resourcecontroller.AddToManager,
				secretcontroller.AddToManager,
				healthcontroller.AddToManager,
				garbagecollectorcontroller.AddToManager,
				healthz.AddToManager,
			); err != nil {
				return err
			}

			// start the target cache and exit if there was an error
			var wg sync.WaitGroup
			errChan := make(chan error)

			go func() {
				defer wg.Done()

				wg.Add(1)
				if err := targetClientOpts.Completed().Start(ctx); err != nil {
					errChan <- fmt.Errorf("error syncing target cache: %w", err)
				}
			}()

			ctxWaitForCache, cancelWaitForCache := context.WithTimeout(ctx, 5*time.Minute)
			defer cancelWaitForCache()

			if !targetClientOpts.Completed().WaitForCacheSync(ctxWaitForCache) {
				return fmt.Errorf("timed out waiting for target cache to sync")
			}

			go func() {
				defer wg.Done()

				wg.Add(1)
				if err := mgr.Start(ctx); err != nil {
					errChan <- fmt.Errorf("error running manager: %w", err)
				}
			}()

			select {
			case err := <-errChan:
				cancel()
				wg.Wait()
				return err

			case <-cmd.Context().Done():
				entryLog.Info("Stop signal received, shutting down.")
				wg.Wait()
				return nil
			}
		},
	}

	resourcemanagercmd.AddAllFlags(
		cmd.Flags(),
		managerOpts,
		targetClientOpts,
		sourceClientOpts,
		resourceControllerOpts,
		secretControllerOpts,
		healthControllerOpts,
		gcControllerOpts,
	)
	verflag.AddFlags(cmd.Flags())

	return cmd
}
