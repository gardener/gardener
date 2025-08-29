// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package opentelemetrycollector_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components"
	. "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/opentelemetrycollector"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/imagevector"
)

var _ = Describe("OpenTelemetryCollector", func() {
	Describe("#Config", func() {
		var (
			cABundle       = "exampleCABundle"
			clusterDomain  = "exampleClusterDomain.com"
			apiServerURL   = "https://api.example-cluster.com"
			otelImageName  = "opentelemetry-collector"
			otelRepository = "eu.gcr.io/gardener-project"
			otelImageTag   = "v0.69.0"
			otelImage      = &imagevector.Image{
				Name:       otelImageName,
				Repository: &otelRepository,
				Tag:        &otelImageTag,
			}
			otelIngress = "ingress.otel.exampleClusterDomain"
		)

		It("should return the expected units and files when OpenTelemetry is enabled", func() {
			ctx := components.Context{
				CABundle:      cABundle,
				ClusterDomain: clusterDomain,
				Images: map[string]*imagevector.Image{
					"opentelemetry-collector": otelImage,
				},
				OpenTelemetryCollectorLogShipperEnabled: true,
				OpenTelemetryCollectorIngressHostName:   otelIngress,
				APIServerURL:                            apiServerURL,
			}

			units, files, err := New().Config(ctx)
			Expect(err).NotTo(HaveOccurred())

			unitContent := `[Unit]
Description=opentelemetry-collector daemon
Documentation=https://github.com/open-telemetry/opentelemetry-collector
[Install]
WantedBy=multi-user.target
[Service]
CPUAccounting=yes
MemoryAccounting=yes
CPUQuota=3%
CPUQuotaPeriodSec=1000ms
MemoryMin=29M
MemoryHigh=400M
MemoryMax=800M
MemorySwapMax=0
Restart=always
RestartSec=5
EnvironmentFile=/etc/environment
ExecStartPre=/bin/sh -c "systemctl set-environment HOSTNAME=$(hostname | tr [:upper:] [:lower:])"
ExecStart=/opt/bin/opentelemetry-collector --config=` + PathConfig

			otelDaemonUnit := extensionsv1alpha1.Unit{
				Name:    UnitName,
				Command: ptr.To(extensionsv1alpha1.CommandStart),
				Enable:  ptr.To(true),
				Content: ptr.To(unitContent),
			}

			fileTargets := ""
			for _, shootComponent := range ShootComponents {
				fileTargets += "/var/log/pods/kube-system_" + shootComponent + "*/*/*.log,"
			}

			otelConfigFile := extensionsv1alpha1.File{
				Path:        "/var/lib/opentelemetry-collector/config/config",
				Permissions: ptr.To[uint32](0644),
				Content: extensionsv1alpha1.FileContent{
					Inline: &extensionsv1alpha1.FileContentInline{
						Encoding: "b64",
						Data: utils.EncodeBase64([]byte(`extensions:
  file_storage:
    directory: /var/log/otelcol
    create_directory: true

receivers:
  journald/journal:
    start_at: beginning
    storage: file_storage
    operators:
      - type: move
        from: body.SYSLOG_IDENTIFIER
        to: resource.unit
      - type: move
        from: body._HOSTNAME
        to: resource.nodename
      - type: retain
        fields:
          - body.MESSAGE

  filelog/pods:
    include: [` + fileTargets + `]
    storage: file_storage
    include_file_path: true
    operators:
      - type: container
        format: containerd
        add_metadata_from_filepath: true

processors:
  batch:
    timeout: 10s

  resourcedetection/system:
    detectors: ["system"]
    system:
      hostname_sources: ["os"]

  filter/drop_localhost_journal:
    logs:
      exclude:
        match_type: strict
        resource_attributes:
          - key: _HOSTNAME
            value: localhost

  filter/keep_units_journal:
    logs:
      include:
        match_type: strict
        resource_attributes:
          - key: SYSLOG_IDENTIFIER
            value: kernel
          - key: _SYSTEMD_UNIT
            value: kubelet.service
          - key: _SYSTEMD_UNIT
            value: docker.service
          - key: _SYSTEMD_UNIT
            value: containerd.service
          - key: _SYSTEMD_UNIT
            value: gardener-node-agent.service

  filter/drop_units_combine:
    logs:
      exclude:
        match_type: strict
        resource_attributes:
          - key: SYSLOG_IDENTIFIER
            value: kernel
          - key: _SYSTEMD_UNIT
            value: kubelet.service
          - key: _SYSTEMD_UNIT
            value: docker.service
          - key: _SYSTEMD_UNIT
            value: containerd.service
          - key: _SYSTEMD_UNIT
            value: gardener-node-agent.service

  resource/journal:
    attributes:
      - action: insert
        key: origin
        value: systemd-journal
      - key: loki.resource.labels
        value: unit, nodename, origin
        action: insert
      - key: loki.format
        value: logfmt
        action: insert

  resource/pod_labels:
    attributes:
      - key: origin
        value: "shoot-system"
        action: insert
      - key: namespace_name
        value: "kube-system"
        action: insert
      - key: pod_name
        from_attribute: k8s.pod.name
        action: insert
      - key: container_name
        from_attribute: k8s.container.name
        action: insert
      - key: loki.resource.labels
        value: pod_name, container_name, origin, namespace_name, nodename, host.name
        action: insert
      - key: loki.format
        value: logfmt
        action: insert

exporters:
  loki:
    endpoint: https://ingress.otel.exampleClusterDomain/loki/api/v1/push
    headers:
      Authorization: "Bearer ${file:/var/lib/opentelemetry-collector/auth-token}"
    tls:
      ca_file: /var/lib/opentelemetry-collector/ca.crt

  debug:
    verbosity: detailed

service:
  extensions: [file_storage]
  pipelines:
    logs/journal:
      receivers: [journald/journal]
      processors: [filter/drop_localhost_journal, filter/keep_units_journal, resource/journal, batch]
      exporters: [loki]
    logs/combine_journal:
      receivers: [journald/journal]
      processors: [filter/drop_localhost_journal, filter/drop_units_combine, resource/journal, batch]
      exporters: [loki]
    logs/pods:
      receivers: [filelog/pods]
      processors: [resourcedetection/system, resource/pod_labels, batch]
      exporters: [loki, debug]
`)),
					},
				},
			}

			otelBinaryFile := extensionsv1alpha1.File{
				Path:        "/opt/bin/opentelemetry-collector",
				Permissions: ptr.To[uint32](0755),
				Content: extensionsv1alpha1.FileContent{
					ImageRef: &extensionsv1alpha1.FileContentImageRef{
						Image:           ctx.Images["opentelemetry-collector"].String(),
						FilePathInImage: "/otelcol-contrib",
					},
				},
			}

			caBundleFile := extensionsv1alpha1.File{
				Path:        "/var/lib/opentelemetry-collector/ca.crt",
				Permissions: ptr.To[uint32](0644),
				Content: extensionsv1alpha1.FileContent{
					Inline: &extensionsv1alpha1.FileContentInline{
						Encoding: "b64",
						Data:     utils.EncodeBase64([]byte(cABundle)),
					},
				},
			}

			expectedFiles := []extensionsv1alpha1.File{otelConfigFile, caBundleFile, otelBinaryFile}

			otelDaemonUnit.FilePaths = []string{
				"/var/lib/opentelemetry-collector/config/config",
				"/var/lib/opentelemetry-collector/ca.crt",
				"/opt/bin/opentelemetry-collector",
			}

			expectedUnits := []extensionsv1alpha1.Unit{otelDaemonUnit}

			Expect(units).To(ConsistOf(expectedUnits))
			Expect(files).To(ConsistOf(expectedFiles))
		})

		It("should return the expected units and files when OpenTelemetry is disabled", func() {
			ctx := components.Context{
				CABundle:      cABundle,
				ClusterDomain: clusterDomain,
				Images: map[string]*imagevector.Image{
					"opentelemetrycollector": otelImage,
				},
				OpenTelemetryCollectorLogShipperEnabled: false,
				OpenTelemetryCollectorIngressHostName:   otelIngress,
			}

			units, files, err := New().Config(ctx)
			Expect(err).NotTo(HaveOccurred())

			Expect(units).To(BeEmpty())
			Expect(files).To(BeEmpty())
		})

		It("should return error when OpenTelemetry ingress is not specified", func() {
			ctx := components.Context{
				CABundle:      cABundle,
				ClusterDomain: clusterDomain,
				Images: map[string]*imagevector.Image{
					"opentelemetrycollector": otelImage,
				},
				OpenTelemetryCollectorLogShipperEnabled: true,
				OpenTelemetryCollectorIngressHostName:   "",
			}

			units, files, err := New().Config(ctx)
			Expect(err).To(MatchError(ContainSubstring("opentelemetry-collector ingress url is missing")))
			Expect(units).To(BeNil())
			Expect(files).To(BeNil())
		})
	})
})
