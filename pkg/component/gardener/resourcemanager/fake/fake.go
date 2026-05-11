// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package fake

import (
	"context"

	"github.com/gardener/gardener/pkg/component/gardener/resourcemanager"
)

// ResourceManager is a test fake for resourcemanager.Interface.
type ResourceManager struct {
	Replicas    *int32
	DeployError error
	Secrets     resourcemanager.Secrets
	Values      resourcemanager.Values

	DeployCalled  bool
	DestroyCalled bool
}

// GetReplicas returns the Replicas field.
func (f *ResourceManager) GetReplicas() *int32 { return f.Replicas }

// SetReplicas sets the Replicas field.
func (f *ResourceManager) SetReplicas(r *int32) { f.Replicas = r }

// SetSecrets sets the Secrets field.
func (f *ResourceManager) SetSecrets(s resourcemanager.Secrets) { f.Secrets = s }

// GetValues returns the Values field.
func (f *ResourceManager) GetValues() resourcemanager.Values { return f.Values }

// SetBootstrapControlPlaneNode is a no-op.
func (f *ResourceManager) SetBootstrapControlPlaneNode(bool) {}

// Deploy records that it was called and returns DeployError.
func (f *ResourceManager) Deploy(_ context.Context) error {
	f.DeployCalled = true
	err := f.DeployError
	f.DeployError = nil
	return err
}

// Destroy records that it was called.
func (f *ResourceManager) Destroy(_ context.Context) error {
	f.DestroyCalled = true
	return nil
}

// Wait is a no-op.
func (f *ResourceManager) Wait(_ context.Context) error { return nil }

// WaitCleanup is a no-op.
func (f *ResourceManager) WaitCleanup(_ context.Context) error { return nil }
