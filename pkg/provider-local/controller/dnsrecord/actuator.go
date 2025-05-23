// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package dnsrecord

import (
	"context"
	"regexp"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/dnsrecord"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
)

type actuator struct {
	client client.Client
}

// NewActuator creates a new Actuator that updates the status of the handled DNSRecord resources.
func NewActuator(mgr manager.Manager) dnsrecord.Actuator {
	return &actuator{
		client: mgr.GetClient(),
	}
}

func (a *actuator) Reconcile(ctx context.Context, _ logr.Logger, dnsrecord *extensionsv1alpha1.DNSRecord, cluster *extensionscontroller.Cluster) error {
	return a.reconcile(ctx, dnsrecord, cluster, updateCoreDNSRewriteRule)
}

func (a *actuator) Delete(ctx context.Context, _ logr.Logger, dnsrecord *extensionsv1alpha1.DNSRecord, cluster *extensionscontroller.Cluster) error {
	return a.reconcile(ctx, dnsrecord, cluster, deleteCoreDNSRewriteRule)
}

func (a *actuator) ForceDelete(ctx context.Context, log logr.Logger, dnsrecord *extensionsv1alpha1.DNSRecord, cluster *extensionscontroller.Cluster) error {
	return a.Delete(ctx, log, dnsrecord, cluster)
}

func (a *actuator) reconcile(ctx context.Context, dnsRecord *extensionsv1alpha1.DNSRecord, cluster *extensionscontroller.Cluster, mutateCorednsRules func(corednsConfig *corev1.ConfigMap, dnsRecord *extensionsv1alpha1.DNSRecord, zone *string)) error {
	return a.updateCoreDNSRewritingRules(ctx, dnsRecord, cluster, mutateCorednsRules)
}

func (a *actuator) Migrate(ctx context.Context, log logr.Logger, dnsrecord *extensionsv1alpha1.DNSRecord, cluster *extensionscontroller.Cluster) error {
	return a.Delete(ctx, log, dnsrecord, cluster)
}

func (a *actuator) Restore(ctx context.Context, log logr.Logger, dnsrecord *extensionsv1alpha1.DNSRecord, cluster *extensionscontroller.Cluster) error {
	return a.Reconcile(ctx, log, dnsrecord, cluster)
}

func (a *actuator) updateCoreDNSRewritingRules(
	ctx context.Context,
	dnsRecord *extensionsv1alpha1.DNSRecord,
	cluster *extensionscontroller.Cluster,
	mutateCorednsRules func(corednsConfig *corev1.ConfigMap, dnsRecord *extensionsv1alpha1.DNSRecord, zone *string),
) error {
	// Only handle dns records for kube-apiserver
	if dnsRecord == nil || !strings.HasPrefix(dnsRecord.Spec.Name, "api.") || !strings.HasSuffix(dnsRecord.Spec.Name, ".local.gardener.cloud") ||
		(dnsRecord.Spec.Class != nil && *dnsRecord.Spec.Class == extensionsv1alpha1.ExtensionClassGarden) {
		return nil
	}

	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: dnsRecord.Namespace}}
	if err := a.client.Get(ctx, client.ObjectKeyFromObject(namespace), namespace); err != nil {
		return err
	}

	var zone *string
	if zones, ok := namespace.Annotations[resourcesv1alpha1.HighAvailabilityConfigZones]; ok && !strings.Contains(zones, ",") && len(cluster.Seed.Spec.Provider.Zones) > 1 {
		zone = &zones
	}

	corednsConfig := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "coredns-custom", Namespace: "gardener-extension-provider-local-coredns"}}
	if err := a.client.Get(ctx, client.ObjectKeyFromObject(corednsConfig), corednsConfig); err != nil {
		return err
	}

	originalConfig := corednsConfig.DeepCopy()
	mutateCorednsRules(corednsConfig, dnsRecord, zone)

	return a.client.Patch(ctx, corednsConfig, client.MergeFrom(originalConfig))
}

func deleteCoreDNSRewriteRule(corednsConfig *corev1.ConfigMap, dnsRecord *extensionsv1alpha1.DNSRecord, _ *string) {
	delete(corednsConfig.Data, dnsRecord.Spec.Name+".override")
}

func updateCoreDNSRewriteRule(corednsConfig *corev1.ConfigMap, dnsRecord *extensionsv1alpha1.DNSRecord, zone *string) {
	istioNamespaceSuffix := ""
	if zone != nil {
		istioNamespaceSuffix = "--" + *zone
	}
	corednsConfig.Data[dnsRecord.Spec.Name+".override"] =
		"rewrite stop name regex " + regexp.QuoteMeta(dnsRecord.Spec.Name) + " istio-ingressgateway.istio-ingress" + istioNamespaceSuffix + ".svc.cluster.local answer auto"
}
