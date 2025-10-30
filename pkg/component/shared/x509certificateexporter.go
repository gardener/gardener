// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shared

import (
	"context"

	"github.com/Masterminds/semver/v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	"github.com/gardener/gardener/pkg/component"
	x "github.com/gardener/gardener/pkg/component/observability/monitoring/x509certificateexporter"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
)

// NewX509CertificateExporter instantiates a new `x509-certificate-exporter` component.
func NewX509CertificateExporter(
	ctx context.Context,
	c client.Client,
	gardenNamespaceName string,
	runtimeVersion *semver.Version,
	priorityClassName string,
	suffix string,
	prometheusInstanceName string,
	configMapName string,
) (
	component.DeployWaiter,
	error,
) {
	var cm corev1.ConfigMap
	image, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameX509CertificateExporter, imagevectorutils.TargetVersion(runtimeVersion.String()))
	if err != nil {
		return nil, err
	}
	err = c.Get(ctx, types.NamespacedName{
		Namespace: gardenNamespaceName,
		Name:      configMapName,
	}, &cm)

	if err != nil {
		// no cm found, no monitoring will be deployed
		return nil, nil
	}

	configData := cm.Data["config.yaml"]

	if len(configData) == 0 {
		// no monitoring targets for x509 certifica exporter provided, nothing to deploy
		return nil, nil
	}

	return x.New(c, nil, gardenNamespaceName, x.Values{
		Image:              image.String(),
		PriorityClassName:  priorityClassName,
		NameSuffix:         suffix,
		PrometheusInstance: prometheusInstanceName,
		ConfigData:         []byte(configData),
	})
}
