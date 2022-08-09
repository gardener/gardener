// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package seed

import (
	"context"
	"encoding/json"
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1helper "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/dnsrecord"
	"github.com/gardener/gardener/pkg/operation/botanist/component/nginxingress"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/images"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	versionutils "github.com/gardener/gardener/pkg/utils/version"

	"github.com/Masterminds/semver"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	networkingv1 "k8s.io/api/networking/v1"
	networkingv1beta1 "k8s.io/api/networking/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const annotationSeedIngressClass = "seed.gardener.cloud/ingress-class"

func managedIngress(seed *Seed) bool {
	return seed.GetInfo().Spec.DNS.Provider != nil && seed.GetInfo().Spec.Ingress != nil && seed.GetInfo().Spec.Ingress.Controller.Kind == v1beta1constants.IngressKindNginx
}

func defaultNginxIngress(c client.Client, imageVector imagevector.ImageVector, kubernetesVersion *semver.Version, ingressClass string, config map[string]string) (component.DeployWaiter, error) {
	imageController, err := imageVector.FindImage(images.ImageNameNginxIngressControllerSeed, imagevector.TargetVersion(kubernetesVersion.String()))
	if err != nil {
		return nil, err
	}
	imageDefaultBackend, err := imageVector.FindImage(images.ImageNameIngressDefaultBackend, imagevector.TargetVersion(kubernetesVersion.String()))
	if err != nil {
		return nil, err
	}

	values := nginxingress.Values{
		ImageController:     imageController.String(),
		ImageDefaultBackend: imageDefaultBackend.String(),
		KubernetesVersion:   kubernetesVersion.String(),
		IngressClass:        ingressClass,
		ConfigData:          config,
	}

	return nginxingress.New(c, v1beta1constants.GardenNamespace, values), nil
}

func getManagedIngressDNSRecord(log logr.Logger, seedClient client.Client, dnsConfig gardencorev1beta1.SeedDNS, secretData map[string][]byte, seedFQDN string, loadBalancerAddress string) component.DeployMigrateWaiter {
	values := &dnsrecord.Values{
		Name:                         "seed-ingress",
		SecretName:                   "seed-ingress",
		Namespace:                    v1beta1constants.GardenNamespace,
		SecretData:                   secretData,
		DNSName:                      seedFQDN,
		RecordType:                   extensionsv1alpha1helper.GetDNSRecordType(loadBalancerAddress),
		ReconcileOnlyOnChangeOrError: true,
	}

	if dnsConfig.Provider != nil {
		values.Type = dnsConfig.Provider.Type
		if dnsConfig.Provider.Zones != nil && len(dnsConfig.Provider.Zones.Include) == 1 {
			values.Zone = &dnsConfig.Provider.Zones.Include[0]
		}
	}

	if loadBalancerAddress != "" {
		values.Values = []string{loadBalancerAddress}
	}

	return dnsrecord.New(
		log,
		seedClient,
		values,
		dnsrecord.DefaultInterval,
		dnsrecord.DefaultSevereThreshold,
		dnsrecord.DefaultTimeout,
	)
}

func migrateIngressClassForShootIngresses(ctx context.Context, gardenClient, seedClient client.Client, seed *Seed, newClass string, kubernetesVersion *semver.Version) error {
	if oldClass, ok := seed.GetInfo().Annotations[annotationSeedIngressClass]; ok && oldClass == newClass {
		return nil
	}

	shootNamespaces := &corev1.NamespaceList{}
	if err := seedClient.List(ctx, shootNamespaces, client.MatchingLabels{v1beta1constants.GardenRole: v1beta1constants.GardenRoleShoot}); err != nil {
		return err
	}

	if err := switchIngressClass(ctx, seedClient, kutil.Key(v1beta1constants.GardenNamespace, "aggregate-prometheus"), newClass, kubernetesVersion); err != nil {
		return err
	}
	if err := switchIngressClass(ctx, seedClient, kutil.Key(v1beta1constants.GardenNamespace, "grafana"), newClass, kubernetesVersion); err != nil {
		return err
	}

	for _, ns := range shootNamespaces.Items {
		if err := switchIngressClass(ctx, seedClient, kutil.Key(ns.Name, "alertmanager"), newClass, kubernetesVersion); err != nil {
			return err
		}
		if err := switchIngressClass(ctx, seedClient, kutil.Key(ns.Name, "prometheus"), newClass, kubernetesVersion); err != nil {
			return err
		}
		if err := switchIngressClass(ctx, seedClient, kutil.Key(ns.Name, "grafana-operators"), newClass, kubernetesVersion); err != nil {
			return err
		}
		if err := switchIngressClass(ctx, seedClient, kutil.Key(ns.Name, "grafana-users"), newClass, kubernetesVersion); err != nil {
			return err
		}
	}

	return seed.UpdateInfo(ctx, gardenClient, false, func(seed *gardencorev1beta1.Seed) error {
		metav1.SetMetaDataAnnotation(&seed.ObjectMeta, annotationSeedIngressClass, newClass)
		return nil
	})
}

func switchIngressClass(ctx context.Context, seedClient client.Client, ingressKey types.NamespacedName, newClass string, kubernetesVersion *semver.Version) error {
	// We need to use `versionutils.CompareVersions` because this function normalizes the seed version first.
	// This is especially necessary if the seed cluster is a non Gardener managed cluster and thus might have some
	// custom version suffix.
	lessEqual121, err := versionutils.CompareVersions(kubernetesVersion.String(), "<=", "1.21.x")
	if err != nil {
		return err
	}
	if lessEqual121 {
		ingress := &extensionsv1beta1.Ingress{}

		if err := seedClient.Get(ctx, ingressKey, ingress); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}

		annotations := ingress.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}
		annotations[networkingv1beta1.AnnotationIngressClass] = newClass
		ingress.SetAnnotations(annotations)

		return seedClient.Update(ctx, ingress)
	}

	ingress := &networkingv1.Ingress{}

	if err := seedClient.Get(ctx, ingressKey, ingress); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	ingress.Spec.IngressClassName = &newClass
	delete(ingress.Annotations, networkingv1beta1.AnnotationIngressClass)

	return seedClient.Update(ctx, ingress)
}

func getConfig(seed *Seed) (map[string]string, error) {
	var (
		defaultConfig = map[string]interface{}{
			"server-name-hash-bucket-size": "256",
			"use-proxy-protocol":           "false",
			"worker-processes":             "2",
		}
		providerConfig = map[string]interface{}{}
	)
	if seed.GetInfo().Spec.Ingress != nil && seed.GetInfo().Spec.Ingress.Controller.ProviderConfig != nil {
		if err := json.Unmarshal(seed.GetInfo().Spec.Ingress.Controller.ProviderConfig.Raw, &providerConfig); err != nil {
			return nil, err
		}
	}

	return interfaceMapToStringMap(utils.MergeMaps(defaultConfig, providerConfig)), nil
}

// ComputeNginxIngressClass returns the IngressClass for the Nginx Ingress controller
func ComputeNginxIngressClass(seed *Seed, kubernetesVersion *string) (string, error) {
	managed := managedIngress(seed)

	if kubernetesVersion == nil {
		return "", fmt.Errorf("kubernetes version is missing in status for seed %q", seed.GetInfo().Name)
	}
	// We need to use `versionutils.CompareVersions` because this function normalizes the seed version first.
	// This is especially necessary if the seed cluster is a non Gardener managed cluster and thus might have some
	// custom version suffix.
	greaterEqual122, err := versionutils.CompareVersions(*kubernetesVersion, ">=", "1.22")
	if err != nil {
		return "", err
	}

	if managed && greaterEqual122 {
		return v1beta1constants.SeedNginxIngressClass122, nil
	}
	if managed {
		return v1beta1constants.SeedNginxIngressClass, nil
	}
	return v1beta1constants.NginxIngressClass, nil
}

func interfaceMapToStringMap(in map[string]interface{}) map[string]string {
	m := make(map[string]string, len(in))
	for k, v := range in {
		m[k] = fmt.Sprint(v)
	}
	return m
}
