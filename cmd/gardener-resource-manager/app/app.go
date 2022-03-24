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
	goruntime "runtime"
	"sync"

	gardenerhealthz "github.com/gardener/gardener/pkg/healthz"
	resourcemanagercmd "github.com/gardener/gardener/pkg/resourcemanager/cmd"
	garbagecollectorcontroller "github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector"
	healthcontroller "github.com/gardener/gardener/pkg/resourcemanager/controller/health"
	resourcecontroller "github.com/gardener/gardener/pkg/resourcemanager/controller/managedresource"
	rootcacontroller "github.com/gardener/gardener/pkg/resourcemanager/controller/rootcapublisher"
	secretcontroller "github.com/gardener/gardener/pkg/resourcemanager/controller/secret"
	tokeninvalidatorcontroller "github.com/gardener/gardener/pkg/resourcemanager/controller/tokeninvalidator"
	tokenrequestorcontroller "github.com/gardener/gardener/pkg/resourcemanager/controller/tokenrequestor"
	resourcemanagerhealthz "github.com/gardener/gardener/pkg/resourcemanager/healthz"
	projectedtokenmountwebhook "github.com/gardener/gardener/pkg/resourcemanager/webhook/projectedtokenmount"
	tokeninvalidatorwebhook "github.com/gardener/gardener/pkg/resourcemanager/webhook/tokeninvalidator"
	"github.com/gardener/gardener/pkg/server/routes"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/component-base/version"
	"k8s.io/component-base/version/verflag"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	runtimelog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var log = runtimelog.Log

// NewResourceManagerCommand creates a new command for running gardener resource manager controllers.
func NewResourceManagerCommand() *cobra.Command {
	var (
		managerOpts       = &resourcemanagercmd.ManagerOptions{}
		profilingOpts     = &resourcemanagercmd.ProfilingOption{}
		sourceClientOpts  = &resourcemanagercmd.SourceClientOptions{}
		targetClusterOpts = &resourcemanagercmd.TargetClusterOptions{}

		resourceControllerOpts         = &resourcecontroller.ControllerOptions{}
		secretControllerOpts           = &secretcontroller.ControllerOptions{}
		healthControllerOpts           = &healthcontroller.ControllerOptions{}
		gcControllerOpts               = &garbagecollectorcontroller.ControllerOptions{}
		tokenInvalidatorControllerOpts = &tokeninvalidatorcontroller.ControllerOptions{}
		tokenRequestorControllerOpts   = &tokenrequestorcontroller.ControllerOptions{}
		rootCAControllerOpts           = &rootcacontroller.ControllerOptions{}
		projectedTokenMountWebhookOpts = &projectedtokenmountwebhook.WebhookOptions{}

		cmd = &cobra.Command{
			Use: "gardener-resource-manager",

			RunE: func(cmd *cobra.Command, args []string) error {
				verflag.PrintAndExitIfRequested()

				ctx, cancel := context.WithCancel(cmd.Context())
				defer cancel()

				log.Info("Starting gardener-resource-manager", "version", version.Get().GitVersion)
				cmd.Flags().VisitAll(func(flag *pflag.Flag) {
					log.Info(fmt.Sprintf("FLAG: --%s=%s", flag.Name, flag.Value)) //nolint:logcheck
				})

				if err := resourcemanagercmd.CompleteAll(
					managerOpts,
					sourceClientOpts,
					targetClusterOpts,
					resourceControllerOpts,
					secretControllerOpts,
					healthControllerOpts,
					gcControllerOpts,
					tokenInvalidatorControllerOpts,
					tokenRequestorControllerOpts,
					rootCAControllerOpts,
					projectedTokenMountWebhookOpts,
				); err != nil {
					return err
				}

				var managerOptions manager.Options
				resourcemanagerhealthz.DefaultAddOptions.Ctx = ctx

				managerOpts.Completed().Apply(&managerOptions)
				sourceClientOpts.Completed().ApplyManagerOptions(&managerOptions)
				sourceClientOpts.Completed().ApplyClientSet(&resourcemanagerhealthz.DefaultAddOptions.ClientSet)
				resourceControllerOpts.Completed().TargetCluster = targetClusterOpts.Completed().Cluster
				secretControllerOpts.Completed().ClassFilter = *resourceControllerOpts.Completed().ClassFilter
				healthControllerOpts.Completed().ClassFilter = *resourceControllerOpts.Completed().ClassFilter
				resourceControllerOpts.Completed().GarbageCollectorActivated = gcControllerOpts.Completed().SyncPeriod > 0
				if err := resourceControllerOpts.Completed().ApplyDefaultClusterId(ctx, log, sourceClientOpts.Completed().RESTConfig); err != nil {
					return err
				}
				healthControllerOpts.Completed().TargetCluster = targetClusterOpts.Completed().Cluster
				gcControllerOpts.Completed().TargetCluster = targetClusterOpts.Completed().Cluster
				tokenInvalidatorControllerOpts.Completed().TargetCluster = targetClusterOpts.Completed().Cluster
				tokenRequestorControllerOpts.Completed().TargetCluster = targetClusterOpts.Completed().Cluster
				rootCAControllerOpts.Completed().TargetCluster = targetClusterOpts.Completed().Cluster
				projectedTokenMountWebhookOpts.Completed().TargetCluster = targetClusterOpts.Completed().Cluster

				// setup manager
				mgr, err := manager.New(sourceClientOpts.Completed().RESTConfig, managerOptions)
				if err != nil {
					return fmt.Errorf("could not instantiate manager: %w", err)
				}

				if err := mgr.Add(targetClusterOpts.Completed().Cluster); err != nil {
					return fmt.Errorf("could not add target cluster to manager: %w", err)
				}

				log.Info("Setting up healthcheck endpoints")
				if err := mgr.AddReadyzCheck("informer-sync", gardenerhealthz.NewCacheSyncHealthz(mgr.GetCache())); err != nil {
					return err
				}
				if err := mgr.AddHealthzCheck("ping", healthz.Ping); err != nil {
					return err
				}

				log.Info("Setting up readycheck for webhook server")
				if err := mgr.AddReadyzCheck("webhook-server", mgr.GetWebhookServer().StartedChecker()); err != nil {
					return err
				}

				if profilingOpts.EnableProfiling {
					if err := (routes.Profiling{}).AddToManager(mgr); err != nil {
						return fmt.Errorf("failed adding profiling handlers to manager: %w", err)
					}
					if profilingOpts.EnableContentionProfiling {
						goruntime.SetBlockProfileRate(1)
					}
				}

				// add controllers, health endpoint and webhooks to manager
				if err := resourcemanagercmd.AddAllToManager(mgr,
					// controllers
					resourcecontroller.AddToManager,
					secretcontroller.AddToManager,
					healthcontroller.AddToManager,
					garbagecollectorcontroller.AddToManager,
					tokeninvalidatorcontroller.AddToManager,
					tokenrequestorcontroller.AddToManager,
					rootcacontroller.AddToManager,
					// health endpoints
					resourcemanagerhealthz.AddToManager,
					// webhooks
					tokeninvalidatorwebhook.AddToManager,
					projectedtokenmountwebhook.AddToManager,
				); err != nil {
					return err
				}

				// start manager and exit if there was an error
				var wg sync.WaitGroup
				errChan := make(chan error)

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
					log.Info("Stop signal received, shutting down")
					wg.Wait()
					return nil
				}
			},
			SilenceUsage: true,
		}
	)

	resourcemanagercmd.AddAllFlags(
		cmd.Flags(),
		managerOpts,
		profilingOpts,
		sourceClientOpts,
		targetClusterOpts,
		resourceControllerOpts,
		secretControllerOpts,
		healthControllerOpts,
		gcControllerOpts,
		tokenInvalidatorControllerOpts,
		tokenRequestorControllerOpts,
		rootCAControllerOpts,
		projectedTokenMountWebhookOpts,
	)
	verflag.AddFlags(cmd.Flags())

	return cmd
}
