// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package ingress

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/extensions/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/provider-local/local"
	"github.com/gardener/gardener/pkg/utils"
)

type reconciler struct {
	client client.Client
}

func (r *reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	ingress := &networkingv1.Ingress{}
	if err := r.client.Get(ctx, req.NamespacedName, ingress); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	ip := ipForIngress(ingress)
	if ip == "" {
		log.Info("Skipping ingress because it doesn't have a LoadBalancer IP")
		return reconcile.Result{}, nil
	}

	log.Info("Reconciling ingress")
	dnsRecords, err := dnsRecordsForIngress(ingress, ip, r.client.Scheme())
	if err != nil {
		return reconcile.Result{}, err
	}

	for _, record := range dnsRecords {
		log.Info("Applying DNSRecord", "dnsRecord", client.ObjectKeyFromObject(record), "host", record.Spec.Name)
		if err = r.client.Patch(ctx, record, client.Apply, local.FieldOwner, client.ForceOwnership); err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

func ipForIngress(ingress *networkingv1.Ingress) string {
	for _, ing := range ingress.Status.LoadBalancer.Ingress {
		if ing.IP != "" {
			return ing.IP
		}
	}
	return ""
}

func dnsRecordsForIngress(ingress *networkingv1.Ingress, ip string, scheme *runtime.Scheme) ([]*extensionsv1alpha1.DNSRecord, error) {
	var dnsRecords []*extensionsv1alpha1.DNSRecord

	for _, rule := range ingress.Spec.Rules {
		host := rule.Host
		record := &extensionsv1alpha1.DNSRecord{
			TypeMeta: metav1.TypeMeta{
				APIVersion: extensionsv1alpha1.SchemeGroupVersion.String(),
				Kind:       extensionsv1alpha1.DNSRecordResource,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      ingress.Name + "-" + utils.ComputeSHA256Hex([]byte(host))[:8],
				Namespace: ingress.Namespace,
				Labels: map[string]string{
					"origin": "provider-local",
					"for":    "ingress",
				},
				Annotations: map[string]string{
					// skip deletion protection, otherwise garbage collector won't be able to delete this DNSRecord object
					v1beta1constants.ConfirmationDeletion: "true",
				},
			},
			Spec: extensionsv1alpha1.DNSRecordSpec{
				DefaultSpec: extensionsv1alpha1.DefaultSpec{
					Type: local.Type,
				},
				RecordType: helper.GetDNSRecordType(ip),
				Name:       host,
				Values:     []string{ip},
				SecretRef: corev1.SecretReference{
					Name: "provider-local-is-awesome",
				},
			},
		}

		if err := controllerutil.SetControllerReference(ingress, record, scheme); err != nil {
			return nil, err
		}
		dnsRecords = append(dnsRecords, record)
	}

	return dnsRecords, nil
}
