// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package features

import (
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/gardener/gardener/pkg/features"
)

// RegisterFeatureGates registers the feature gates of gardener-operator.
func RegisterFeatureGates() {
	utilruntime.Must(features.DefaultFeatureGate.Add(features.GetFeatures(
		features.BackupEntryForGarden,
		features.DefaultSeccompProfile,
		features.IstioTLSTermination,
		features.DoNotCopyBackupCredentials,
		features.VictoriaLogsBackend,
		features.VPAInPlaceUpdates,
		features.PrometheusHealthChecks,
		features.RemoveVali,
		features.DisableNginxIngressInGarden,
		// OpenTelemetryCollector is registered here so the operator can wire the
		// OTel→VictoriaLogs pipeline (see newFluentCustomResources). This also
		// flips the garden fluent-bit init image and the static ClusterOutput to
		// the OTLP path; see #13961 for the rollout context.
		features.OpenTelemetryCollector,
	)))
}
