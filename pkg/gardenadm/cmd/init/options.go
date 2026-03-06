// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package init

import (
	"fmt"
	"os"

	"github.com/spf13/pflag"

	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
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

	return o.validateZone()
}

// validateZone validates the zone configuration against the shoot specification.
func (o *Options) validateZone() error {
	resources, err := gardenadm.ReadManifests(o.Log, os.DirFS(o.ConfigDir))
	if err != nil {
		return fmt.Errorf("failed loading resources for zone validation: %w", err)
	}

	if v1beta1helper.HasManagedInfrastructure(resources.Shoot) {
		if o.Zone != "" {
			return fmt.Errorf("zone can't be configured for shoot with managed infrastructure")
		}
		return nil
	}

	if resources.Shoot == nil {
		return fmt.Errorf("zone validation failed shoot resource is missing in the manifests")
	}

	// init command is only for control plane node, therefore we look for the control plane pool
	var controlPlanePool *gardencorev1beta1.Worker
	if controlPlanePool = v1beta1helper.ControlPlaneWorkerPoolForShoot(resources.Shoot.Spec.Provider.Workers); controlPlanePool == nil {
		return fmt.Errorf("zone validation failed, shoot doesn't have a control plane worker pool configured")
	}

	effectiveZone, err := cmd.DetermineZone(*controlPlanePool, o.Zone)
	if err != nil {
		return fmt.Errorf("failed determining zone for control plane worker pool %q: %w", controlPlanePool.Name, err)
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
	fs.StringVarP(&o.Zone, "zone", "z", "", "Availability zone for the new node. Required if the control plane worker pool in the `Shoot` has multiple zones configured. Optional if exactly one zone is configured (applied automatically). Must not be set if no zones are configured.")
}
