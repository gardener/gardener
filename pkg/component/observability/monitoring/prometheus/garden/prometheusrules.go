// Copyright 2024 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package garden

import (
	_ "embed"

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
	garden     *monitoringv1.PrometheusRule

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

	garden = &monitoringv1.PrometheusRule{}
	utilruntime.Must(runtime.DecodeInto(monitoringutils.Decoder, gardenYAML, garden))

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
func CentralPrometheusRules() []*monitoringv1.PrometheusRule {
	return []*monitoringv1.PrometheusRule{
		auditLog,
		etcd,
		garden,
		metering,
		recording,
		seed,
		shoot,
	}
}
