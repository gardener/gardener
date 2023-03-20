// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package backupoptions

import (
	"github.com/spf13/pflag"
	"sigs.k8s.io/controller-runtime/pkg/controller"
)

const (
	// DefaultBackupPath is the default path to the directory where the backup bucket is created.
	DefaultBackupPath = "dev/local-backupbuckets"
	// DefaultContainerMountPath is the default path to the directory where the backup bucket is mounted on the container.
	DefaultContainerMountPath = "/etc/gardener/local-backupbuckets"
)

// ControllerOptions are command line options that can be set for controller.Options.
type ControllerOptions struct {
	// BackupBucketPath is the path to the backupbucket.
	BackupBucketPath string
	// ContainerMountPath is the path to the directory where the backup bucket is mounted on the container.
	ContainerMountPath string

	config *ControllerConfig
}

// AddOptions are options to apply when adding the backupbucket controller to the manager.
type AddOptions struct {
	// BackupBucketPath is the path to the backupbucket.
	BackupBucketPath string
	// ContainerMountPath is the path to the directory where the backup bucket is mounted on the container.
	ContainerMountPath string
	// Controller are the controller.Options.
	Controller controller.Options
	// IgnoreOperationAnnotation specifies whether to ignore the operation annotation or not.
	IgnoreOperationAnnotation bool
}

// AddFlags implements Flagger.AddFlags.
func (c *ControllerOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&c.BackupBucketPath, "local-dir", c.BackupBucketPath, "Path to the directory where the bucket will be created.")
	fs.StringVar(&c.ContainerMountPath, "container-mount-path", c.ContainerMountPath, "Path to the directory where the backup bucket is mounted on the container.")
}

// Complete implements Completer.Complete.
func (c *ControllerOptions) Complete() error {
	c.config = &ControllerConfig{
		c.BackupBucketPath,
		c.ContainerMountPath,
	}
	return nil
}

// Completed returns the completed ControllerConfig. Only call this if `Complete` was successful.
func (c *ControllerOptions) Completed() *ControllerConfig {
	return c.config
}

// ControllerConfig is a completed controller configuration.
type ControllerConfig struct {
	// BackupBucketPath is the path to the backupbucket.
	BackupBucketPath string
	// ContainerMountPath is the path to the directory where the backup bucket is mounted on the container.
	ContainerMountPath string
}

// Apply sets the values of this ControllerConfig in the given AddOptions.
func (c *ControllerConfig) Apply(opts *AddOptions) {
	opts.BackupBucketPath = c.BackupBucketPath
	opts.ContainerMountPath = c.ContainerMountPath
}
