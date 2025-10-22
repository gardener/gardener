// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package dnsrecord

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/component/networking/coredns"
)

// Actuator implements the DNSRecord actuator for the local DNS provider.
type Actuator struct {
	Client client.Client
}

// shouldHandleDNSRecord returns true for DNSRecords that are implemented in provider-local by writing to the custom
// coredns ConfigMap (e.g., DNSRecords for shoot API servers). Other DNSRecords (e.g., for the Garden) are implemented
// outside this controller.
func shouldHandleDNSRecord(dnsRecord *extensionsv1alpha1.DNSRecord) bool {
	return strings.HasPrefix(dnsRecord.Spec.Name, "api.") && strings.HasSuffix(dnsRecord.Spec.Name, ".local.gardener.cloud") &&
		ptr.Deref(dnsRecord.Spec.Class, "") != extensionsv1alpha1.ExtensionClassGarden
}

// Reconcile ensures that the DNS record is correctly represented in the CoreDNS config map.
func (a *Actuator) Reconcile(ctx context.Context, _ logr.Logger, dnsRecord *extensionsv1alpha1.DNSRecord, cluster *extensionscontroller.Cluster) error {
	if !shouldHandleDNSRecord(dnsRecord) {
		return nil
	}

	config, err := a.configForDNSRecord(ctx, dnsRecord, cluster)
	if err != nil {
		return err
	}

	return a.patchCoreDNSConfigMap(ctx, func(configMap *corev1.ConfigMap) {
		configMap.Data[keyForDNSRecord(dnsRecord)] = config
	})
}

// Delete removes the DNS record from the CoreDNS config map.
func (a *Actuator) Delete(ctx context.Context, _ logr.Logger, dnsRecord *extensionsv1alpha1.DNSRecord, _ *extensionscontroller.Cluster) error {
	if !shouldHandleDNSRecord(dnsRecord) {
		return nil
	}

	return a.patchCoreDNSConfigMap(ctx, func(configMap *corev1.ConfigMap) {
		delete(configMap.Data, keyForDNSRecord(dnsRecord))
	})
}

// ForceDelete is the same as Delete for the local DNS provider.
func (a *Actuator) ForceDelete(ctx context.Context, log logr.Logger, dnsRecord *extensionsv1alpha1.DNSRecord, cluster *extensionscontroller.Cluster) error {
	return a.Delete(ctx, log, dnsRecord, cluster)
}

// Migrate removes the DNS record from the CoreDNS config map if the shoot is not self-hosted.
func (a *Actuator) Migrate(ctx context.Context, log logr.Logger, dnsRecord *extensionsv1alpha1.DNSRecord, cluster *extensionscontroller.Cluster) error {
	if v1beta1helper.IsShootSelfHosted(cluster.Shoot.Spec.Provider.Workers) {
		// Do nothing when migrating DNSRecord of self-hosted shoot with managed infrastructure. The CoreDNS
		// rewrite rules are still needed for the control plane machines to resolve the kube-apiserver domain.
		return nil
	}

	return a.Delete(ctx, log, dnsRecord, cluster)
}

// Restore is the same as Reconcile for the local DNS provider.
func (a *Actuator) Restore(ctx context.Context, log logr.Logger, dnsRecord *extensionsv1alpha1.DNSRecord, cluster *extensionscontroller.Cluster) error {
	return a.Reconcile(ctx, log, dnsRecord, cluster)
}

func (a *Actuator) patchCoreDNSConfigMap(ctx context.Context, mutate func(configMap *corev1.ConfigMap)) error {
	configMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: coredns.CustomConfigMapName, Namespace: "gardener-extension-provider-local-coredns"}}
	_, err := controllerutil.CreateOrPatch(ctx, a.Client, configMap, func() error {
		mutate(configMap)
		return nil
	})
	return err
}

func keyForDNSRecord(dnsRecord *extensionsv1alpha1.DNSRecord) string {
	return dnsRecord.Spec.Name + ".override"
}

func (a *Actuator) configForDNSRecord(ctx context.Context, dnsRecord *extensionsv1alpha1.DNSRecord, cluster *extensionscontroller.Cluster) (string, error) {
	if v1beta1helper.IsShootSelfHosted(cluster.Shoot.Spec.Provider.Workers) {
		switch dnsRecord.Spec.RecordType {
		case extensionsv1alpha1.DNSRecordTypeA, extensionsv1alpha1.DNSRecordTypeAAAA:
			// We need to use the `template` plugin because the `hosts` plugin can only be used once per server block.
			config := fmt.Sprintf(`template IN %[1]s local.gardener.cloud {
  match "^%[2]s\.$"
  answer "{{ .Name }} %[3]d IN %[1]s %[4]s"
  fallthrough
}
`, dnsRecord.Spec.RecordType, regexp.QuoteMeta(dnsRecord.Spec.Name), ptr.Deref(dnsRecord.Spec.TTL, 120), strings.Join(dnsRecord.Spec.Values, " "))

			return config, nil
		default:
			return "", fmt.Errorf("unsupported record type %q for self-hosted shoot, only A and AAAA are supported", dnsRecord.Spec.RecordType)
		}
	}

	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: dnsRecord.Namespace}}
	if err := a.Client.Get(ctx, client.ObjectKeyFromObject(namespace), namespace); err != nil {
		return "", err
	}

	var zone string
	if zones, ok := namespace.Annotations[resourcesv1alpha1.HighAvailabilityConfigZones]; ok &&
		!strings.Contains(zones, ",") && len(cluster.Seed.Spec.Provider.Zones) > 1 &&
		v1beta1helper.SeedSettingZonalIngressEnabled(cluster.Seed.Spec.Settings) {
		zone = zones
	}
	istioNamespaceSuffix := ""
	if zone != "" {
		istioNamespaceSuffix = "--" + zone
	}

	return "rewrite stop name regex " + regexp.QuoteMeta(dnsRecord.Spec.Name) + " istio-ingressgateway.istio-ingress" + istioNamespaceSuffix + ".svc.cluster.local answer auto", nil
}
