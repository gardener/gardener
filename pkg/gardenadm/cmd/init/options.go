// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package init

import (
	"fmt"
	"os"

	"github.com/spf13/pflag"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/gardenadm"
	"github.com/gardener/gardener/pkg/gardenadm/cmd"
)

// Options contains options for this command.
type Options struct {
	*cmd.Options
	cmd.ManifestOptions

	// UseBootstrapEtcd indicates whether to use the bootstrap etcd instead of transitioning to etcd-druid.
	UseBootstrapEtcd bool
	// Zone is the availability zone in which the new node is being initialized.
	// For shoot's worker with multiple zones configured, this flag is required.
	// For shoot's worker with a single zone configured, this zone is automatically applied.
	// For shoot's worker with no zones, this flag is optional.
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

	return o.validateZone()
}

// validateZone validates the zone configuration against the shoot specification.
func (o *Options) validateZone() error {
	resources, err := gardenadm.ReadManifests(o.Log, os.DirFS(o.ConfigDir))
	if err != nil {
		return fmt.Errorf("failed loading resources for zone validation: %w", err)
	}

	if resources.Shoot.Spec.CredentialsBindingName != nil || resources.Shoot.Spec.SecretBindingName != nil {
		if o.Zone != "" {
			return fmt.Errorf("zone can't be configured for shoot with managed infrastrcture")
		}
		return nil
	}

	if resources.Shoot == nil {
		return fmt.Errorf("zone validation failed shoot resource is missing in the manifests")
	}

	// init command is only for control plane node, therefore we look for the control plane worker
	var worker gardencorev1beta1.Worker
	for _, w := range resources.Shoot.Spec.Provider.Workers {
		if w.ControlPlane != nil {
			worker = w
		}
	}

	effectiveZone, err := cmd.ValidateZone(worker, o.Zone)
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
	fs.BoolVar(&o.UseBootstrapEtcd, "use-bootstrap-etcd", false, "If set, the control plane continues using the bootstrap etcd instead of transitioning to etcd-druid. This is useful for testing purposes to save time.")
	fs.StringVarP(&o.Zone, "zone", "z", "", "Zone of the node in which this new node is being initialized")
}
