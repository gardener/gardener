// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bootstrap

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/gardenadm/botanist"
	"github.com/gardener/gardener/pkg/gardenadm/cmd"
)

// NewCommand creates a new cobra.Command.
func NewCommand(globalOpts *cmd.Options) *cobra.Command {
	opts := &Options{Options: globalOpts}

	cmd := &cobra.Command{
		Use:   "bootstrap",
		Short: "Bootstrap the infrastructure for an Autonomous Shoot Cluster",
		Long:  "Bootstrap the infrastructure for an Autonomous Shoot Cluster (networks, machines, etc.)",

		Example: `# Bootstrap the infrastructure
gardenadm bootstrap --kubeconfig ~/.kube/config`,

		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.ParseArgs(args); err != nil {
				return err
			}

			if err := opts.Validate(); err != nil {
				return err
			}

			if err := opts.Complete(); err != nil {
				return err
			}

			return run(cmd.Context(), opts)
		},
	}

	opts.addFlags(cmd.Flags())

	return cmd
}

// NewClientSetFromFile in alias for botanist.NewClientSetFromFile.
// Exposed for unit testing.
var NewClientSetFromFile = botanist.NewClientSetFromFile

func run(ctx context.Context, opts *Options) error {
	clientSet, err := NewClientSetFromFile(opts.Kubeconfig)
	if err != nil {
		return fmt.Errorf("failed creating client: %w", err)
	}

	if err := ensureNoGardenletOrOperator(ctx, clientSet.Client()); err != nil {
		return err
	}

	opts.Log.Info("Command is work in progress")
	return nil
}

// ensureNoGardenletOrOperator is a safety check that prevents operators from accidentally executing
// `gardenadm bootstrap` on a cluster that is already used as a runtime cluster with gardener-operator or as a seed
// cluster. Doing so would lead to conflicts when `gardenadm bootstrap` starts deploying components like provider
// extensions.
func ensureNoGardenletOrOperator(ctx context.Context, c client.Reader) error {
	for _, key := range []client.ObjectKey{
		{Namespace: v1beta1constants.GardenNamespace, Name: "gardener-operator"},
		{Namespace: v1beta1constants.GardenNamespace, Name: "gardenlet"},
	} {
		if err := c.Get(ctx, key, &appsv1.Deployment{}); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return fmt.Errorf("failed checking if %q deployment exists: %w", key, err)
		}

		return fmt.Errorf("deployment %q exists on the targeted cluster. "+
			"`gardenadm bootstrap` does not support targeting a cluster that is already used as a runtime cluster with gardener-operator or as a seed cluster. "+
			"Please consult the gardenadm documentation", key)
	}

	return nil
}
