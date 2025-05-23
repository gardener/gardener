// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package dnsrecord

import (
	"context"

	"github.com/go-logr/logr"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// Actuator acts upon DNSRecord resources.
type Actuator interface {
	// Reconcile reconciles the DNSRecord.
	Reconcile(context.Context, logr.Logger, *extensionsv1alpha1.DNSRecord, *extensionscontroller.Cluster) error
	// Delete deletes the DNSRecord.
	Delete(context.Context, logr.Logger, *extensionsv1alpha1.DNSRecord, *extensionscontroller.Cluster) error
	// ForceDelete forcefully deletes the DNSRecord.
	ForceDelete(context.Context, logr.Logger, *extensionsv1alpha1.DNSRecord, *extensionscontroller.Cluster) error
	// Restore restores the DNSRecord.
	Restore(context.Context, logr.Logger, *extensionsv1alpha1.DNSRecord, *extensionscontroller.Cluster) error
	// Migrate migrates the DNSRecord.
	Migrate(context.Context, logr.Logger, *extensionsv1alpha1.DNSRecord, *extensionscontroller.Cluster) error
}
