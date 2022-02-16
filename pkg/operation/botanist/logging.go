// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package botanist

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/gardener/gardener/charts"
	gardencore "github.com/gardener/gardener/pkg/apis/core"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/features"
	gardenlethelper "github.com/gardener/gardener/pkg/gardenlet/apis/config/helper"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/seed"
	"github.com/gardener/gardener/pkg/utils/images"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeploySeedLogging will install the Helm release "seed-bootstrap/charts/loki" in the Seed clusters.
func (b *Botanist) DeploySeedLogging(ctx context.Context) error {
	// check if loki is enabled in gardenlet config, default is true
	var lokiEnabled = gardenlethelper.IsLokiEnabled(b.Config)

	if !b.Shoot.IsShootControlPlaneLoggingEnabled(b.Config) || !lokiEnabled {
		return b.destroyShootLoggingStack(ctx)
	}

	if err := b.Shoot.Components.Logging.ShootRBACProxy.Deploy(ctx); err != nil {
		return err
	}

	seedImages, err := b.InjectSeedSeedImages(map[string]interface{}{},
		images.ImageNameLoki,
		images.ImageNameLokiCurator,
		images.ImageNameKubeRbacProxy,
		images.ImageNameTelegraf,
	)
	if err != nil {
		return err
	}

	lokiValues := map[string]interface{}{
		"global":   seedImages,
		"replicas": b.Shoot.GetReplicas(1),
	}

	hvpaValues := make(map[string]interface{})
	hvpaEnabled := gardenletfeatures.FeatureGate.Enabled(features.HVPA)
	if b.ManagedSeed != nil {
		hvpaEnabled = gardenletfeatures.FeatureGate.Enabled(features.HVPAForShootedSeed)
	}

	ingressClass, err := seed.ComputeNginxIngressClass(b.Seed, b.Seed.GetInfo().Status.KubernetesVersion)
	if err != nil {
		return err
	}

	if b.isShootNodeLoggingEnabled() {
		lokiValues["rbacSidecarEnabled"] = true
		lokiValues["podAnnotations"] = map[string]interface{}{
			"checksum/secret-" + v1beta1constants.SecretNameGenericTokenKubeconfig: b.LoadCheckSum(v1beta1constants.SecretNameGenericTokenKubeconfig),
		}
		lokiValues["ingress"] = map[string]interface{}{
			"class": ingressClass,
			"hosts": []map[string]interface{}{
				{
					"hostName":    b.ComputeLokiHost(),
					"secretName":  common.LokiTLS,
					"serviceName": "loki",
					"servicePort": 8080,
					"backendPath": "/loki/api/v1/push",
				},
			},
		}
	} else {
		if err := b.destroyShootNodeLogging(ctx); err != nil {
			return err
		}
	}

	hvpaValues["enabled"] = hvpaEnabled
	lokiValues["hvpa"] = hvpaValues

	if hvpaEnabled {
		currentResources, err := kutil.GetContainerResourcesInStatefulSet(ctx, b.K8sSeedClient.Client(), kutil.Key(b.Shoot.SeedNamespace, "loki"))
		if err != nil {
			return err
		}
		if len(currentResources) != 0 && currentResources["loki"] != nil {
			lokiValues["resources"] = map[string]interface{}{
				"loki": currentResources["loki"],
			}
		}
	}

	if err := b.K8sSeedClient.ChartApplier().Apply(ctx, filepath.Join(charts.Path, "seed-bootstrap", "charts", "loki"), b.Shoot.SeedNamespace, fmt.Sprintf("%s-logging", b.Shoot.SeedNamespace), kubernetes.Values(lokiValues)); err != nil {
		return err
	}

	// TODO(rfranzke): Remove in a future release.
	return kutil.DeleteObjects(ctx, b.K8sSeedClient.Client(),
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: b.Shoot.SeedNamespace, Name: "loki-config"}},
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: b.Shoot.SeedNamespace, Name: "telegraf-config"}},
		// Follow-up of https://github.com/gardener/gardener/pull/5010 (loki `ServiceAccount` got removed and was never
		// used.
		&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: b.Shoot.SeedNamespace, Name: "loki"}},
	)
}

func (b *Botanist) destroyShootLoggingStack(ctx context.Context) error {
	if err := b.destroyShootNodeLogging(ctx); err != nil {
		return err
	}

	return common.DeleteLoki(ctx, b.K8sSeedClient.Client(), b.Shoot.SeedNamespace)
}

func (b *Botanist) destroyShootNodeLogging(ctx context.Context) error {
	if err := b.Shoot.Components.Logging.ShootRBACProxy.Destroy(ctx); err != nil {
		return err
	}

	return kutil.DeleteObjects(ctx, b.K8sSeedClient.Client(),
		&extensionsv1beta1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "loki", Namespace: b.Shoot.SeedNamespace}},
		&networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "loki", Namespace: b.Shoot.SeedNamespace}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: common.LokiTLS, Namespace: b.Shoot.SeedNamespace}},
		&networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-from-prometheus-to-loki-telegraf", Namespace: b.Shoot.SeedNamespace}},
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "telegraf-config", Namespace: b.Shoot.SeedNamespace}},
	)
}

func (b *Botanist) isShootNodeLoggingEnabled() bool {
	if b.Shoot != nil && b.Shoot.IsShootControlPlaneLoggingEnabled(b.Config) &&
		gardenlethelper.IsLokiEnabled(b.Config) && b.Config != nil &&
		b.Config.Logging != nil && b.Config.Logging.ShootNodeLogging != nil {

		for _, purpose := range b.Config.Logging.ShootNodeLogging.ShootPurposes {
			if gardencore.ShootPurpose(b.Shoot.Purpose) == purpose {
				return true
			}
		}
	}
	return false
}
