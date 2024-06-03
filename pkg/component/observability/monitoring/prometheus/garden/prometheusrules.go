// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package garden

import (
	"bytes"
	_ "embed"
	"text/template"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
)

var (
	//go:embed assets/prometheusrules/auditlog.yaml
	auditLogYAML []byte
	auditLog     *monitoringv1.PrometheusRule

	//go:embed assets/prometheusrules/etcd.yaml
	etcdYAML []byte
	etcd     *monitoringv1.PrometheusRule

	//go:embed assets/prometheusrules/garden.yaml
	gardenYAML []byte

	//go:embed assets/prometheusrules/metering-meta.yaml
	meteringYAML []byte
	metering     *monitoringv1.PrometheusRule

	//go:embed assets/prometheusrules/recording.yaml
	recordingYAML []byte
	recording     *monitoringv1.PrometheusRule

	//go:embed assets/prometheusrules/seed.yaml
	seedYAML []byte
	seed     *monitoringv1.PrometheusRule

	//go:embed assets/prometheusrules/shoot.yaml
	shootYAML []byte
	shoot     *monitoringv1.PrometheusRule
)

func init() {
	auditLog = &monitoringv1.PrometheusRule{}
	utilruntime.Must(runtime.DecodeInto(monitoringutils.Decoder, auditLogYAML, auditLog))

	etcd = &monitoringv1.PrometheusRule{}
	utilruntime.Must(runtime.DecodeInto(monitoringutils.Decoder, etcdYAML, etcd))

	metering = &monitoringv1.PrometheusRule{}
	utilruntime.Must(runtime.DecodeInto(monitoringutils.Decoder, meteringYAML, metering))

	recording = &monitoringv1.PrometheusRule{}
	utilruntime.Must(runtime.DecodeInto(monitoringutils.Decoder, recordingYAML, recording))

	seed = &monitoringv1.PrometheusRule{}
	utilruntime.Must(runtime.DecodeInto(monitoringutils.Decoder, seedYAML, seed))

	shoot = &monitoringv1.PrometheusRule{}
	utilruntime.Must(runtime.DecodeInto(monitoringutils.Decoder, shootYAML, shoot))
}

// CentralPrometheusRules returns the central PrometheusRule resources for the garden prometheus.
func CentralPrometheusRules(isGardenerDiscoveryServerEnabled bool) []*monitoringv1.PrometheusRule {
	gardenTmpl, err := template.New("garden").Delims("<<", ">>").Parse(string(gardenYAML))
	utilruntime.Must(err)

	var (
		gardenConfig = struct {
			GardenerDiscoveryServerEnabled bool
		}{
			GardenerDiscoveryServerEnabled: isGardenerDiscoveryServerEnabled,
		}
		gardenRaw bytes.Buffer
	)

	utilruntime.Must(gardenTmpl.Execute(&gardenRaw, gardenConfig))

	garden := &monitoringv1.PrometheusRule{}
	utilruntime.Must(runtime.DecodeInto(monitoringutils.Decoder, gardenRaw.Bytes(), garden))

	return []*monitoringv1.PrometheusRule{
		auditLog.DeepCopy(),
		etcd.DeepCopy(),
		garden.DeepCopy(),
		metering.DeepCopy(),
		recording.DeepCopy(),
		seed.DeepCopy(),
		shoot.DeepCopy(),
	}
}
