// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package restore

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

	// BackupDataPath is the local path on the node where the etcd backup data is stored.
	// When set, the bootstrap etcd will be initialized from this path using the Local storage provider.
	// The path is expected to have the structure: <backupBucketsRoot>/<bucketName>/<namespace>--<uid>/etcd-main/v2.
	//
	// TODO(Kostov6): Support specifying the backup root path after backup resource manifests are imported.
	BackupDataPath string
	// PriorNodeName defines the name of the node that is going to be replaced.
	PriorNodeName string
	// Zone is the availability zone of the new machine where the prior node is being restored to.
	// The zone of the new machine can differ from the zone of the prior node.
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
	if o.BackupDataPath == "" {
		return fmt.Errorf("must provide --backup-data-path")
	}
	if o.PriorNodeName == "" {
		return fmt.Errorf("must provide --prior-node-name")
	}

	resources, err := gardenadm.ReadManifests(o.Log, os.DirFS(o.ConfigDir))
	if err != nil {
		return fmt.Errorf("failed loading resources for gardenadm restore validation: %w", err)
	}
	if resources.ShootState == nil {
		return fmt.Errorf("gardenadm restore requires a ShootState resource in the config directory, but none was found")
	}
	if resources.Shoot == nil || resources.Shoot.Status.UID == "" {
		// The Shoot .status.uid must match the UID from the original cluster because it determines the
		// BackupBucket and BackupEntry names and identifies the cluster in the cluster-identity ConfigMap.
		// If it were empty, a fresh UID would be generated, which would lead to undesired behaviour and inconsistency in the system.
		return fmt.Errorf("gardenadm restore requires the Shoot manifest in the config directory to have .status.uid set")
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
	// TODO(Kostov6): Support specifying the backup root path after backup resource manifests are imported.
	fs.StringVar(&o.BackupDataPath, "backup-data-path", "", "Local path on the node where the etcd backup data is stored. Expected structure: <backupBucketsRoot>/<bucketName>/<namespace>--<uid>/etcd-main/v2")
	fs.StringVar(&o.PriorNodeName, "prior-node-name", "", "The name of the prior control plane node. Required in order to cleanup stale resources.")
	fs.StringVarP(&o.Zone, "zone", "z", "", "Availability zone of the new machine where the prior node is being restored to. Required if the control plane worker pool in the Shoot has multiple zones configured. Optional if exactly one zone is configured (applied automatically). Must not be set if no zones are configured.")
}
