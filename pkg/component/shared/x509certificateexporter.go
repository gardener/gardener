// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shared

import (
	"time"

	"github.com/Masterminds/semver/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	"github.com/gardener/gardener/pkg/component"
	x "github.com/gardener/gardener/pkg/component/observability/monitoring/x509certificateexporter"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
)

// NewX509CertificateExporter instantiates a new `x509-certificate-exporter` component.
func NewX509CertificateExporter(
	c client.Client,
	gardebNamespaceName string,
	runtimeVersion *semver.Version,
	priorityClassName string,
	suffix string,
	prometheusInstanceName string,
) (
	component.DeployWaiter,
	error,
) {
	image, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameX509CertificateExporter, imagevectorutils.TargetVersion(runtimeVersion.String()))
	if err != nil {
		return nil, err
	}

	return x.New(c, nil, gardebNamespaceName, x.Values{
		// TODO(mimiteto): Allow host mounts
		SecretTypes: x.SecretTypeList{
			x.SecretType{Type: "kubernetes.io/tls", Key: `.*.crt`},
			x.SecretType{Type: "istio.io/ca-root", Key: `.*cert.*\.pem`},
		},
		ConfigMapKeys:             x.ConfigMapKeys{"ca.crt", "root-cert.pem"},
		CacheDuration:             metav1.Duration{Duration: 24 * time.Hour},
		Image:                     image.String(),
		PriorityClassName:         priorityClassName,
		Replicas:                  1,
		NameSuffix:                suffix,
		CertificateRenewalDays:    14,
		CertificateExpirationDays: 7,
		PrometheusInstance:        prometheusInstanceName,
	}), nil
}
