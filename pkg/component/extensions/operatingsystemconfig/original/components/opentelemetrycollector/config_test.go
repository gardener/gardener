// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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
Environment=KUBECONFIG=/var/lib/opentelemetry-collector/kubeconfig
ExecStartPre=/bin/sh -c "systemctl set-environment HOSTNAME=$(hostname | tr [:upper:] [:lower:])"
ExecStart=/opt/bin/opentelemetry-collector --config=` + PathConfig

			otelDaemonUnit := extensionsv1alpha1.Unit{
				Name:    UnitName,
				Command: ptr.To(extensionsv1alpha1.CommandStart),
				Enable:  ptr.To(true),
				Content: ptr.To(unitContent),
			}

			otelConfigFile := extensionsv1alpha1.File{
				Path:        "/var/lib/opentelemetry-collector/config/config",
				Permissions: ptr.To[uint32](0400),
				Content: extensionsv1alpha1.FileContent{
					Inline: &extensionsv1alpha1.FileContentInline{
						Encoding: "b64",
						Data: utils.EncodeBase64([]byte(`extensions:
  file_storage:
    directory: /var/log/otelcol
    create_directory: true
  bearertokenauth:
    filename: /var/lib/opentelemetry-collector/auth-token

receivers:
  journald/journal:
    start_at: beginning
    storage: file_storage
    units:
      - kernel
      - kubelet.service
      - containerd.service
      - gardener-node-agent.service
    operators:
      - type: move
        from: body._SYSTEMD_UNIT
        to: resource.unit
      - type: move
        from: body._HOSTNAME
        to: resource.nodename
      - type: move
        from: body.MESSAGE
        to: body

  filelog/pods:
    include: /var/log/pods/kube-system_*/*/*.log
    storage: file_storage
    include_file_path: true
    operators:
      - type: container
        format: containerd
        add_metadata_from_filepath: true

processors:
  batch:
    timeout: 10s

  # Include resource attributes from the Kubernetes environment.
  # The Shoot KAPI server is queried for pods in the kube-system namespace
  # which are labeled with "resources.gardener.cloud/managed-by=gardener".
  k8sattributes:
    auth_type: "kubeConfig"
    context: "shoot"
    wait_for_metadata: true
    wait_for_metadata_timeout: 30s
    filter:
        namespace: kube-system
        labels:
          - key: resources.gardener.cloud/managed-by
            op: equals
            value: gardener
    pod_association:
      - sources:
          - from: resource_attribute
            name: k8s.pod.name

  # If the log came from a pod that is managed by Gardener, the 'k8sattributes' processor
  # will successfully associate the datapoint (log) with the a pod. Since we only
  # watch the kube-system namespace for pods that are managed by Gardener, we can
  # simply drop all logs that do not have a specific label that we know should have been 
  # added by the 'k8sattributes' processor. In this case, we check for the
  # existence of the 'k8s.node.name' attribute.
  filter/drop_non_gardener:
    error_mode: ignore
    logs:
      log_record:
        - resource.attributes["k8s.node.name"] == nil

  resource/journal:
    attributes:
      - action: insert
        key: origin
        value: systemd_journal

  resource/pod_labels:
    attributes:
      - key: origin
        value: "shoot_system"
        action: insert
      - key: namespace_name
        value: "kube-system"
        action: insert

exporters:
  otlp:
    endpoint: ingress.otel.exampleClusterDomain:443
    auth:
      authenticator: bearertokenauth
    tls:
      ca_file: /var/lib/opentelemetry-collector/ca.crt

service:
  extensions: [file_storage, bearertokenauth]
  pipelines:
    logs/journal:
      receivers: [journald/journal]
      processors: [resource/journal, batch]
      exporters: [otlp]
    logs/pods:
      receivers: [filelog/pods]
      processors: [k8sattributes, filter/drop_non_gardener, resource/pod_labels, batch]
      exporters: [otlp]
`)),
					},
				},
			}

			otelBinaryFile := extensionsv1alpha1.File{
				Path:        "/opt/bin/opentelemetry-collector",
				Permissions: ptr.To[uint32](0700),
				Content: extensionsv1alpha1.FileContent{
					ImageRef: &extensionsv1alpha1.FileContentImageRef{
						Image:           ctx.Images["opentelemetry-collector"].String(),
						FilePathInImage: "/bin/otelcol",
					},
				},
			}

			caBundleFile := extensionsv1alpha1.File{
				Path:        "/var/lib/opentelemetry-collector/ca.crt",
				Permissions: ptr.To[uint32](0400),
				Content: extensionsv1alpha1.FileContent{
					Inline: &extensionsv1alpha1.FileContentInline{
						Encoding: "b64",
						Data:     utils.EncodeBase64([]byte(cABundle)),
					},
				},
			}
			kubeconfig := extensionsv1alpha1.File{
				Path:        "/var/lib/opentelemetry-collector/kubeconfig",
				Permissions: ptr.To[uint32](0600),
				Content: extensionsv1alpha1.FileContent{
					Inline: &extensionsv1alpha1.FileContentInline{
						Encoding: "",
						Data: `apiVersion: v1
clusters:
- cluster:
    certificate-authority: /var/lib/opentelemetry-collector/ca.crt
    server: https://api.example-cluster.com
  name: shoot
contexts:
- context:
    cluster: shoot
    user: shoot
  name: shoot
current-context: shoot
kind: Config
preferences: {}
users:
- name: shoot
  user:
    tokenFile: /var/lib/opentelemetry-collector/auth-token
`,
					},
				},
			}

			expectedFiles := []extensionsv1alpha1.File{otelConfigFile, caBundleFile, otelBinaryFile, kubeconfig}

			otelDaemonUnit.FilePaths = []string{
				"/var/lib/opentelemetry-collector/config/config",
				"/var/lib/opentelemetry-collector/ca.crt",
				"/opt/bin/opentelemetry-collector",
				"/var/lib/opentelemetry-collector/kubeconfig",
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
					"opentelemetry-collector": otelImage,
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
					"opentelemetry-collector": otelImage,
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
