// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package prometheus

import (
	"context"
	"fmt"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/utils"
)

var (
	// runtimeGardenPrometheuses are managed by gardener-operator in the runtime garden cluster.
	runtimeGardenPrometheuses = sets.New("garden", "longterm")
	// seedPrometheuses are managed by gardenlet in the seed cluster.
	seedPrometheuses = sets.New("seed")

	// handledPrometheusNames are the Prometheus instances this webhook reconciles.
	handledPrometheusNames = runtimeGardenPrometheuses.Union(seedPrometheuses)
)

type mutator struct {
	client          client.Client
	remoteWriteURLs []string
	externalLabels  map[string]string
}

func (m *mutator) Mutate(_ context.Context, newObj, _ client.Object) error {
	if newObj.GetDeletionTimestamp() != nil {
		return nil
	}

	prometheus, ok := newObj.(*monitoringv1.Prometheus)
	if !ok {
		return fmt.Errorf("unexpected object, got %T wanted *monitoringv1.Prometheus", newObj)
	}

	if !handledPrometheusNames.Has(prometheus.Name) {
		return nil
	}

	// prometheus-operator defaults TerminationGracePeriodSeconds to 600s. In e2e the
	// `Wait for PersistentVolumes to be cleaned up` step waits for PV reclaim, which
	// is gated on the pod releasing the PVC. Shrink the grace period so cleanup waits
	// stay short and PV reclaim is what we're actually measuring.
	prometheus.Spec.TerminationGracePeriodSeconds = ptr.To[int64](60)

	if !seedPrometheuses.Has(prometheus.Name) {
		return nil
	}

	// Add the configured external labels
	prometheus.Spec.ExternalLabels = utils.MergeStringMaps(prometheus.Spec.ExternalLabels, m.externalLabels)

	// Add the configured remote write URLs
	// When pushing metrics to a remote write endpoint in the prow cluster, prometheus needs to talk to private networks.
	prometheus.Spec.PodMetadata.Labels[v1beta1constants.LabelNetworkPolicyToPrivateNetworks] = v1beta1constants.LabelNetworkPolicyAllowed

	var remoteWriteSpecs []monitoringv1.RemoteWriteSpec
	for _, remoteWriteURL := range m.remoteWriteURLs {
		remoteWriteSpecs = append(remoteWriteSpecs, monitoringv1.RemoteWriteSpec{
			URL: monitoringv1.URL(remoteWriteURL),
			WriteRelabelConfigs: []monitoringv1.RelabelConfig{
				{
					SourceLabels: []monitoringv1.LabelName{"job"},
					Regex:        "gardenlet",
					Action:       "keep",
				},
				{
					SourceLabels: []monitoringv1.LabelName{"__name__"},
					Regex:        "(up|flow_.+|gardenlet_.+)",
					Action:       "keep",
				},
			},
		})
	}
	prometheus.Spec.RemoteWrite = remoteWriteSpecs

	return nil
}
