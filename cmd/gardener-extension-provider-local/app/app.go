// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"net"
	"os"

	"github.com/gardener/gardener/extensions/pkg/controller"
	controllercmd "github.com/gardener/gardener/extensions/pkg/controller/cmd"
	genericcontrolplaneactuator "github.com/gardener/gardener/extensions/pkg/controller/controlplane/genericactuator"
	"github.com/gardener/gardener/extensions/pkg/controller/operatingsystemconfig/oscommon"
	"github.com/gardener/gardener/extensions/pkg/controller/worker"
	webhookcmd "github.com/gardener/gardener/extensions/pkg/webhook/cmd"
	localinstall "github.com/gardener/gardener/pkg/provider-local/apis/local/install"
	localcontrolplane "github.com/gardener/gardener/pkg/provider-local/controller/controlplane"
	localdnsprovider "github.com/gardener/gardener/pkg/provider-local/controller/dnsprovider"
	localdnsrecord "github.com/gardener/gardener/pkg/provider-local/controller/dnsrecord"
	localhealthcheck "github.com/gardener/gardener/pkg/provider-local/controller/healthcheck"
	localinfrastructure "github.com/gardener/gardener/pkg/provider-local/controller/infrastructure"
	localnetwork "github.com/gardener/gardener/pkg/provider-local/controller/network"
	localnode "github.com/gardener/gardener/pkg/provider-local/controller/node"
	localservice "github.com/gardener/gardener/pkg/provider-local/controller/service"
	localworker "github.com/gardener/gardener/pkg/provider-local/controller/worker"
	"github.com/gardener/gardener/pkg/provider-local/local"

	dnsv1alpha1 "github.com/gardener/external-dns-management/pkg/apis/dns/v1alpha1"
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	autoscalingv1beta2 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1beta2"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var hostIP string

func init() {
	addrs, err := net.InterfaceAddrs()
	utilruntime.Must(err)

	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				hostIP = ipnet.IP.String()
				break
			}
		}
	}
}

// NewControllerManagerCommand creates a new command for running a local provider controller.
func NewControllerManagerCommand(ctx context.Context) *cobra.Command {
	var (
		restOpts = &controllercmd.RESTOptions{}
		mgrOpts  = &controllercmd.ManagerOptions{
			LeaderElection:             true,
			LeaderElectionResourceLock: resourcelock.LeasesResourceLock,
			LeaderElectionID:           controllercmd.LeaderElectionNameID(local.Name),
			LeaderElectionNamespace:    os.Getenv("LEADER_ELECTION_NAMESPACE"),
			WebhookServerPort:          443,
			WebhookCertDir:             "/tmp/gardener-extensions-cert",
		}
		generalOpts = &controllercmd.GeneralOptions{}

		// options for the health care controller
		healthCheckCtrlOpts = &controllercmd.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}

		// options for the controlplane controller
		controlPlaneCtrlOpts = &controllercmd.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}

		// options for the dnsprovider controller
		dnsProviderCtrlOpts = &controllercmd.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}

		// options for the dnsrecord controller
		dnsRecordCtrlOpts = &controllercmd.ControllerOptions{
			MaxConcurrentReconciles: 1,
		}

		// options for the service controller
		serviceCtrlOpts = &localservice.ControllerOptions{
			MaxConcurrentReconciles: 5,
			HostIP:                  hostIP,
			APIServerSNIEnabled:     true,
		}

		// options for the node controller
		nodeCtrlOpts = &localnode.ControllerOptions{
			MaxConcurrentReconciles: 1,
		}

		// options for the operatingsystemconfig controller
		operatingSystemConfigCtrlOpts = &controllercmd.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}

		// options for the network controller
		networkCtrlOpts = &controllercmd.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}

		// options for the infrastructure controller
		infraCtrlOpts = &controllercmd.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}
		reconcileOpts = &controllercmd.ReconcilerOptions{}

		// options for the worker controller
		workerCtrlOpts = &controllercmd.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}
		workerReconcileOpts = &worker.Options{
			DeployCRDs: true,
		}
		workerCtrlOptsUnprefixed = controllercmd.NewOptionAggregator(workerCtrlOpts, workerReconcileOpts)

		// options for the webhook server
		webhookServerOptions = &webhookcmd.ServerOptions{
			Namespace: os.Getenv("WEBHOOK_CONFIG_NAMESPACE"),
		}

		controllerSwitches = ControllerSwitchOptions()
		webhookSwitches    = WebhookSwitchOptions()
		webhookOptions     = webhookcmd.NewAddToManagerOptions(local.Name, webhookServerOptions, webhookSwitches)

		aggOption = controllercmd.NewOptionAggregator(
			restOpts,
			mgrOpts,
			generalOpts,
			controllercmd.PrefixOption("controlplane-", controlPlaneCtrlOpts),
			controllercmd.PrefixOption("dnsprovider-", dnsProviderCtrlOpts),
			controllercmd.PrefixOption("dnsrecord-", dnsRecordCtrlOpts),
			controllercmd.PrefixOption("infrastructure-", infraCtrlOpts),
			controllercmd.PrefixOption("worker-", &workerCtrlOptsUnprefixed),
			controllercmd.PrefixOption("service-", serviceCtrlOpts),
			controllercmd.PrefixOption("node-", nodeCtrlOpts),
			controllercmd.PrefixOption("operatingsystemconfig-", operatingSystemConfigCtrlOpts),
			controllercmd.PrefixOption("network-", networkCtrlOpts),
			controllercmd.PrefixOption("healthcheck-", healthCheckCtrlOpts),
			controllerSwitches,
			reconcileOpts,
			webhookOptions,
		)
	)

	cmd := &cobra.Command{
		Use: fmt.Sprintf("%s-controller-manager", local.Name),

		Run: func(cmd *cobra.Command, args []string) {
			if err := aggOption.Complete(); err != nil {
				controllercmd.LogErrAndExit(err, "Error completing options")
			}

			if workerReconcileOpts.Completed().DeployCRDs {
				if err := worker.ApplyMachineResourcesForConfig(ctx, restOpts.Completed().Config); err != nil {
					controllercmd.LogErrAndExit(err, "Error ensuring the machine CRDs")
				}
			}

			mgr, err := manager.New(restOpts.Completed().Config, mgrOpts.Completed().Options())
			if err != nil {
				controllercmd.LogErrAndExit(err, "Could not instantiate manager")
			}

			scheme := mgr.GetScheme()
			if err := controller.AddToScheme(scheme); err != nil {
				controllercmd.LogErrAndExit(err, "Could not update manager scheme")
			}
			if err := localinstall.AddToScheme(scheme); err != nil {
				controllercmd.LogErrAndExit(err, "Could not update manager scheme")
			}
			if err := autoscalingv1beta2.AddToScheme(scheme); err != nil {
				controllercmd.LogErrAndExit(err, "Could not update manager scheme")
			}
			if err := machinev1alpha1.AddToScheme(scheme); err != nil {
				controllercmd.LogErrAndExit(err, "Could not update manager scheme")
			}
			if err := dnsv1alpha1.AddToScheme(scheme); err != nil {
				controllercmd.LogErrAndExit(err, "Could not update manager scheme")
			}
			// add common meta types to schema for controller-runtime to use v1.ListOptions
			metav1.AddToGroupVersion(scheme, machinev1alpha1.SchemeGroupVersion)

			useTokenRequestor, err := controller.UseTokenRequestor(generalOpts.Completed().GardenerVersion)
			if err != nil {
				controllercmd.LogErrAndExit(err, "Could not determine whether token requestor should be used")
			}
			localworker.DefaultAddOptions.UseTokenRequestor = useTokenRequestor

			useProjectedTokenMount, err := controller.UseServiceAccountTokenVolumeProjection(generalOpts.Completed().GardenerVersion)
			if err != nil {
				controllercmd.LogErrAndExit(err, "Could not determine whether service account token volume projection should be used")
			}
			localworker.DefaultAddOptions.UseProjectedTokenMount = useProjectedTokenMount

			controlPlaneCtrlOpts.Completed().Apply(&localcontrolplane.DefaultAddOptions.Controller)
			dnsProviderCtrlOpts.Completed().Apply(&localdnsprovider.DefaultAddOptions.Controller)
			dnsRecordCtrlOpts.Completed().Apply(&localdnsrecord.DefaultAddOptions.Controller)
			healthCheckCtrlOpts.Completed().Apply(&localhealthcheck.DefaultAddOptions.Controller)
			infraCtrlOpts.Completed().Apply(&localinfrastructure.DefaultAddOptions.Controller)
			networkCtrlOpts.Completed().Apply(&localnetwork.DefaultAddOptions.Controller)
			operatingSystemConfigCtrlOpts.Completed().Apply(&oscommon.DefaultAddOptions.Controller)
			serviceCtrlOpts.Completed().Apply(&localservice.DefaultAddOptions)
			nodeCtrlOpts.Completed().Apply(&localnode.DefaultAddOptions)
			workerCtrlOpts.Completed().Apply(&localworker.DefaultAddOptions.Controller)

			reconcileOpts.Completed().Apply(&localcontrolplane.DefaultAddOptions.IgnoreOperationAnnotation)
			reconcileOpts.Completed().Apply(&localdnsrecord.DefaultAddOptions.IgnoreOperationAnnotation)
			reconcileOpts.Completed().Apply(&localinfrastructure.DefaultAddOptions.IgnoreOperationAnnotation)
			reconcileOpts.Completed().Apply(&localnetwork.DefaultAddOptions.IgnoreOperationAnnotation)
			reconcileOpts.Completed().Apply(&oscommon.DefaultAddOptions.IgnoreOperationAnnotation)
			reconcileOpts.Completed().Apply(&localworker.DefaultAddOptions.IgnoreOperationAnnotation)

			_, shootWebhooks, err := webhookOptions.Completed().AddToManager(ctx, mgr)
			if err != nil {
				controllercmd.LogErrAndExit(err, "Could not add webhooks to manager")
			}
			localcontrolplane.DefaultAddOptions.ShootWebhooks = shootWebhooks

			// Update shoot webhook configuration in case the webhook server port has changed.
			c, err := client.New(restOpts.Completed().Config, client.Options{})
			if err != nil {
				controllercmd.LogErrAndExit(err, "Error creating client for startup tasks")
			}
			if err := genericcontrolplaneactuator.ReconcileShootWebhooksForAllNamespaces(ctx, c, local.Name, local.Type, mgr.GetWebhookServer().Port, shootWebhooks); err != nil {
				controllercmd.LogErrAndExit(err, "Error ensuring shoot webhooks in all namespaces")
			}

			if err := controllerSwitches.Completed().AddToManager(mgr); err != nil {
				controllercmd.LogErrAndExit(err, "Could not add controllers to manager")
			}

			mgr.GetLogger().Info("Started with", "hostIP", hostIP)

			if err := mgr.Start(ctx); err != nil {
				controllercmd.LogErrAndExit(err, "Error running manager")
			}
		},
	}

	aggOption.AddFlags(cmd.Flags())

	return cmd
}
