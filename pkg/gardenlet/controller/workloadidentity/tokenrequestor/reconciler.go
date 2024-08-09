// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package tokenrequestor

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	securityv1alpha1constants "github.com/gardener/gardener/pkg/apis/security/v1alpha1/constants"
	securityclientset "github.com/gardener/gardener/pkg/client/security/clientset/versioned"
	"github.com/gardener/gardener/pkg/controllerutils"
)

const (
	tokenRenewTimestamp   = securityv1alpha1constants.WorkloadIdentityPrefix + "token-renew-timestamp"
	maxExpirationDuration = 24 * time.Hour
	expirationDuration    = 6 * time.Hour // short enough to be secure and long enough to be resilient to disruptions
)

// Reconciler requests and refreshes tokens via the TokenRequest API.
type Reconciler struct {
	SeedClient           client.Client
	GardenClient         client.Client
	GardenSecurityClient securityclientset.Interface
	ConcurrentSyncs      int
	Clock                clock.Clock
	JitterFunc           func(time.Duration, float64) time.Duration
}

// Reconcile requests and populates tokens.
func (r *Reconciler) Reconcile(reconcileCtx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(reconcileCtx)

	ctx, cancel := controllerutils.GetMainReconciliationContext(reconcileCtx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

	secret := &corev1.Secret{}
	if err := r.SeedClient.Get(ctx, req.NamespacedName, secret); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if !r.isRelevantSecret(secret) {
		return reconcile.Result{}, nil
	}

	mustRequeue, requeueAfter, err := r.requeue(secret)
	if err != nil {
		return reconcile.Result{}, err
	}
	if mustRequeue {
		log.Info("No need to generate new token, renewal is scheduled", "after", requeueAfter)
		return reconcile.Result{Requeue: true, RequeueAfter: requeueAfter}, nil
	}

	log.Info("Requesting new token")

	var contextObject *securityv1alpha1.ContextObject
	if v, ok := secret.Annotations[securityv1alpha1constants.AnnotationWorkloadIdentityContextObject]; ok {
		contextObject = &securityv1alpha1.ContextObject{}
		if err := json.Unmarshal([]byte(v), contextObject); err != nil {
			return reconcile.Result{}, fmt.Errorf("cannot parse context object: %w", err)
		}
	}

	wi := &securityv1alpha1.WorkloadIdentity{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secret.Annotations[securityv1alpha1constants.AnnotationWorkloadIdentityName],
			Namespace: secret.Annotations[securityv1alpha1constants.AnnotationWorkloadIdentityNamespace],
		},
	}

	if err := r.GardenClient.Get(ctx, client.ObjectKeyFromObject(wi), wi); err != nil {
		return reconcile.Result{}, err
	}

	tokenRequest, err := r.GardenSecurityClient.SecurityV1alpha1().WorkloadIdentities(wi.Namespace).CreateToken(ctx, wi.Name, &securityv1alpha1.TokenRequest{
		Spec: securityv1alpha1.TokenRequestSpec{
			ContextObject:     contextObject,
			ExpirationSeconds: ptr.To((int64(expirationDuration / time.Second))),
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return reconcile.Result{}, err
	}

	renewDuration := r.renewDuration(tokenRequest.Status.ExpirationTimestamp.Time)

	if err := r.reconcileSecret(ctx, log, secret, tokenRequest.Status.Token, renewDuration); err != nil {
		return reconcile.Result{}, fmt.Errorf("could not update Secret with token: %w", err)
	}

	log.Info("Successfully requested token and scheduled renewal", "after", renewDuration)
	return reconcile.Result{Requeue: true, RequeueAfter: renewDuration}, nil
}

func (r *Reconciler) reconcileSecret(ctx context.Context, log logr.Logger, secret *corev1.Secret, token string, renewDuration time.Duration) error {
	patch := client.MergeFrom(secret.DeepCopy())
	metav1.SetMetaDataAnnotation(&secret.ObjectMeta, tokenRenewTimestamp, r.Clock.Now().UTC().Add(renewDuration).Format(time.RFC3339))

	log.Info("Populating the token to secret")
	if err := r.populateToken(log, secret, token)(); err != nil {
		return err
	}

	return r.SeedClient.Patch(ctx, secret, patch)
}

func (r *Reconciler) populateToken(log logr.Logger, secret *corev1.Secret, token string) func() error {
	return func() error {
		if secret.Data == nil {
			secret.Data = make(map[string][]byte, 1)
		}

		log.Info("Writing token to data")
		secret.Data["token"] = []byte(token)
		return nil
	}
}

func (r *Reconciler) requeue(secret *corev1.Secret) (bool, time.Duration, error) {
	renewTimestamp := secret.Annotations[tokenRenewTimestamp]
	if len(renewTimestamp) == 0 {
		return false, 0, nil
	}

	if _, ok := secret.Data["token"]; !ok {
		return false, 0, nil
	}

	renewTime, err := time.Parse(time.RFC3339, renewTimestamp)
	if err != nil {
		return false, 0, fmt.Errorf("could not parse renew timestamp: %w", err)
	}

	if r.Clock.Now().UTC().Before(renewTime.UTC()) {
		return true, renewTime.UTC().Sub(r.Clock.Now().UTC()), nil
	}

	return false, 0, nil
}

func (r *Reconciler) renewDuration(expirationTimestamp time.Time) time.Duration {
	expirationDuration := expirationTimestamp.UTC().Sub(r.Clock.Now().UTC())
	if expirationDuration >= maxExpirationDuration {
		expirationDuration = maxExpirationDuration
	}

	return r.JitterFunc(expirationDuration*80/100, 0.05)
}
