// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operatingsystemconfig

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

func serialReconciliation(secret *corev1.Secret) bool {
	return secret.Annotations[v1beta1constants.AnnotationNodeAgentSerialOSCReconciliation] == "true"
}

type leaderElector struct {
	log    logr.Logger
	client client.Client
	clock  clock.Clock

	identity string
	lease    *coordinationv1.Lease
}

func newLeaderElectorForSecret(log logr.Logger, c client.Client, clock clock.Clock, secret *corev1.Secret, identity string) *leaderElector {
	var (
		logger = log.WithName("leader-elector")
		lease  *coordinationv1.Lease
	)

	if serialReconciliation(secret) {
		ownerRef := metav1.NewControllerRef(secret, corev1.SchemeGroupVersion.WithKind("Secret"))
		lease = &coordinationv1.Lease{
			ObjectMeta: metav1.ObjectMeta{
				Name:            secret.Name,
				Namespace:       secret.Namespace,
				OwnerReferences: []metav1.OwnerReference{*ownerRef},
			},
		}
		logger = logger.WithValues("lease", client.ObjectKeyFromObject(lease))
	}

	return &leaderElector{
		log:      logger,
		client:   c,
		clock:    clock,
		identity: identity,
		lease:    lease,
	}
}

func (l *leaderElector) logger() logr.Logger {
	log := l.log

	if l.lease.Spec.HolderIdentity != nil {
		log = log.WithValues("holderIdentity", *l.lease.Spec.HolderIdentity)
	}
	if l.lease.Spec.AcquireTime != nil {
		log = log.WithValues("acquireTime", l.lease.Spec.AcquireTime.String())
	}
	if l.lease.Spec.RenewTime != nil {
		log = log.WithValues("acquireTime", l.lease.Spec.RenewTime.String())
	}
	if l.lease.Spec.LeaseDurationSeconds != nil {
		log = log.WithValues("leaseDuration", l.leaseDuration().String())
	}

	return log
}

func (l *leaderElector) acquiredByMe() bool {
	return ptr.Deref(l.lease.Spec.HolderIdentity, "") == l.identity
}

func (l *leaderElector) acquiredByAnotherInstance() bool {
	holder := ptr.Deref(l.lease.Spec.HolderIdentity, "")
	return holder != "" && !l.acquiredByMe()
}

func (l *leaderElector) leaseDuration() time.Duration {
	return time.Duration(ptr.Deref(l.lease.Spec.LeaseDurationSeconds, 0)) * time.Second
}

func (l *leaderElector) renewedInTime() bool {
	if ptr.Deref(l.lease.Spec.LeaseDurationSeconds, 0) < 0 || l.lease.Spec.RenewTime == nil {
		return false
	}
	return l.lease.Spec.RenewTime.Add(l.leaseDuration()).UTC().After(l.clock.Now().UTC())
}

func (l *leaderElector) durationUntilLeaseExpires() time.Duration {
	if ptr.Deref(l.lease.Spec.LeaseDurationSeconds, 0) < 0 || l.lease.Spec.RenewTime == nil {
		return 0
	}
	return l.lease.Spec.RenewTime.UTC().Add(l.leaseDuration()).UTC().Sub(l.clock.Now().UTC())
}

func (l *leaderElector) reload(ctx context.Context) error {
	return client.IgnoreNotFound(l.client.Get(ctx, client.ObjectKeyFromObject(l.lease), l.lease))
}

func (l *leaderElector) tryAcquireOrRenew(ctx context.Context) error {
	l.log.Info("Try to acquire or renew Lease")

	setLeaseSpec := func() {
		if !l.acquiredByMe() {
			l.lease.Spec.HolderIdentity = &l.identity
			l.lease.Spec.LeaseDurationSeconds = ptr.To[int32](600)
			l.lease.Spec.AcquireTime = &metav1.MicroTime{Time: l.clock.Now().UTC()}
		}
		l.lease.Spec.RenewTime = &metav1.MicroTime{Time: l.clock.Now().UTC()}
	}

	if l.lease.ResourceVersion == "" {
		l.log.Info("Lease does not exist, try creating it")
		setLeaseSpec()
		return l.client.Create(ctx, l.lease)
	}

	l.log.Info("Lease exists, try updating it")
	setLeaseSpec()
	return l.client.Update(ctx, l.lease)
}

func (l *leaderElector) release(ctx context.Context) error {
	if l.lease == nil || !l.acquiredByMe() {
		return nil
	}

	l.log.Info("Releasing Lease")
	l.lease.Spec.HolderIdentity = nil
	l.lease.Spec.LeaseDurationSeconds = nil
	l.lease.Spec.AcquireTime = nil
	l.lease.Spec.RenewTime = nil
	return l.client.Update(ctx, l.lease)
}
