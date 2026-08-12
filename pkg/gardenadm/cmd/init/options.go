// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package init

import (
	"fmt"
	"os"

	"github.com/spf13/pflag"

	"github.com/gardener/gardener/pkg/gardenadm"
	"github.com/gardener/gardener/pkg/gardenadm/cmd"
)

// Options contains options for this command.
type Options struct {
	*cmd.Options
	cmd.ManifestOptions

	// Force forces the execution of the init command, even if the control plane is already initialized (default: false).
	Force bool
	// UseBootstrapEtcd indicates whether to use the bootstrap etcd instead of transitioning to etcd-druid
	// (default: false). This helps `gardenadm init` to run faster.
	UseBootstrapEtcd bool
	// UseHostNetwork indicates whether to run gardener-resource-manager and extensions in host network (instead of
	// redeploying them into the pod network after bootstrapping) (default: false). This helps `gardenadm init` to run
	// faster.
	UseHostNetwork bool
	// Zone is the availability zone in which the new node is being initialized.
	// It is validated against the `.spec.provider.workers[].zones` field of the Shoot manifest.
	// If the worker pool has multiple zones configured, this flag is required.
	// If it has exactly one zone configured, that zone is automatically applied and the flag is optional.
	// If it has no zones configured, this flag must not be set.
	Zone string
}

// ParseArgs parses the arguments to the options.
func (o *Options) ParseArgs(args []string) error {
	return o.ManifestOptions.ParseArgs(args)
}

// Validate validates the options.
func (o *Options) Validate() error {
	if err := o.ManifestOptions.Validate(); err != nil {
		return err
	}

	resources, err := gardenadm.ReadManifests(o.Log, os.DirFS(o.ConfigDir))
	if err != nil {
		return fmt.Errorf("failed loading resources for zone validation: %w", err)
	}

	effectiveZone, err := cmd.ValidateAndDetermineControlPlaneZone(resources.Shoot, o.Zone)
	if err != nil {
		return err
	}
	o.Zone = effectiveZone

	return nil
}

// Complete completes the options.
func (o *Options) Complete() error {
	return o.ManifestOptions.Complete()
}

func (o *Options) addFlags(fs *pflag.FlagSet) {
	o.ManifestOptions.AddFlags(fs)
	fs.BoolVar(&o.UseBootstrapEtcd, "use-bootstrap-etcd", false, "If set, the control plane continues using the bootstrap etcd instead of transitioning to etcd-druid. This can be useful for testing purposes to save time.")
	fs.BoolVar(&o.UseHostNetwork, "use-host-network", false, "If set, gardener-resource-manager and extensions continue to run in host network instead of getting redeployed into the pod network after bootstrapping. This can be useful for testing purposes to save time.")
	fs.StringVarP(&o.Zone, "zone", "z", "", "Availability zone for the new node. Required if the control plane worker pool in the Shoot has multiple zones configured. Optional if exactly one zone is configured (applied automatically). Must not be set if no zones are configured.")
	fs.BoolVar(&o.Force, "force", false, "If set, the init command will be executed even if the control plane is already initialized.")
}
