// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package cmd

import (
	"context"
	"errors"

	goflag "flag"

	"github.com/gardener/gardener/pkg/version/verflag"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"k8s.io/klog/v2"
)

// NewKonnectivityServerReloaderCommand creates the root command of the konnectivity-server-reloader.
func NewKonnectivityServerReloaderCommand(ctx context.Context) *cobra.Command {
	opts := &options{}

	cmd := &cobra.Command{
		Use:   "konnectivity-server-reloader [flags] -- COMMAND [args...] INJECTED-REPLICA-COUNT",
		Short: "Executes a command depending on the deployment's replica count",
		Example: `
    $(terminal-1) kubectl create deployment --image=nginx nginx
    deployment.apps/nginx created

and running

    $(terminal-2) konnectivity-server-reloader --namespace=default --deployment-name=nginx -- sleep

would start a "sleep 1" process.
If the watched deployment is scaled, then the controller stops the previous process and
starts a new one:

    $(terminal-1) kubectl scale deployment my-dep --replicas=10
    deployment.apps/my-dep scaled

    $(terminal-1) ps | grep sleep
    61191 ttys003    0:00.00 sleep 10`,
		RunE: func(cmd *cobra.Command, args []string) error {
			verflag.PrintAndExitIfRequested()

			argsLenAtDash := cmd.ArgsLenAtDash()

			if argsLenAtDash > -1 && len(args[argsLenAtDash:]) > 0 {
				opts.command = args[argsLenAtDash:]
			} else {
				return errors.New("COMMAND should be passed e.g. konnectivity-server-reloader --namespace=default -- sleep")
			}

			if opts.namespace == "" {
				return errors.New("namespace is required")
			}

			if opts.deploymentName == "" {
				return errors.New("deployment-name is required")
			}

			if opts.jitter < 0 {
				return errors.New("jitter cannot be negative")
			}

			if opts.jitterFactor < 0 {
				return errors.New("jitter-factor cannot be negative")
			}

			if opts.jitterFactor > 0 && opts.jitter == 0 {
				return errors.New("jitter must be set when specifying jitter-factor")
			}

			return opts.start(ctx)
		},
	}

	flags := cmd.Flags()

	klog.InitFlags(goflag.CommandLine)
	flag.CommandLine.AddGoFlagSet(goflag.CommandLine)
	verflag.AddFlags(flags)

	flags.StringVarP(&opts.namespace, "namespace", "n", "", "namespace of the deployment")
	flags.StringVar(&opts.deploymentName, "deployment-name", "kube-apiserver", "name of the deployment")
	flags.DurationVar(&opts.jitter, "jitter", 0, "duration between receiving a scale event and process restart")
	flags.Float64Var(&opts.jitterFactor, "jitter-factor", 0.0, "adds random factor to jitter. Requires jitter to be set")

	return cmd
}
