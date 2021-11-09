// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package tokenrequestor

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/clock"
	corev1clientset "k8s.io/client-go/kubernetes/typed/core/v1"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
)

const (
	defaultExpirationDuration = 12 * time.Hour
	maxExpirationDuration     = 24 * time.Hour
)

// NewReconciler returns a new instance of the reconciler.
func NewReconciler(
	clock clock.Clock,
	jitter func(time.Duration, float64) time.Duration,
	targetClient client.Client,
	targetCoreV1Client corev1clientset.CoreV1Interface,
) reconcile.Reconciler {
	return &reconciler{
		clock:              clock,
		jitter:             jitter,
		targetClient:       targetClient,
		targetCoreV1Client: targetCoreV1Client,
	}
}

type reconciler struct {
	clock              clock.Clock
	jitter             func(time.Duration, float64) time.Duration
	log                logr.Logger
	targetClient       client.Client
	targetCoreV1Client corev1clientset.CoreV1Interface
	client             client.Client
}

func (r *reconciler) InjectLogger(l logr.Logger) error {
	r.log = l.WithName(ControllerName)
	return nil
}

func (r *reconciler) InjectClient(c client.Client) error {
	r.client = c
	return nil
}

func (r *reconciler) Reconcile(reconcileCtx context.Context, req reconcile.Request) (reconcile.Result, error) {
	ctx, cancel := context.WithTimeout(reconcileCtx, time.Minute)
	defer cancel()

	log := r.log.WithValues("object", req)

	secret := &corev1.Secret{}
	if err := r.client.Get(ctx, req.NamespacedName, secret); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Stopping reconciliation of Secret, as it has been deleted")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("could not fetch Secret: %w", err)
	}

	if !isRelevantSecret(secret) {
		return reconcile.Result{}, nil
	}

	mustRequeue, requeueAfter, err := r.requeue(secret.Annotations[resourcesv1alpha1.ServiceAccountTokenRenewTimestamp])
	if err != nil {
		return reconcile.Result{}, err
	}
	if mustRequeue {
		return reconcile.Result{Requeue: true, RequeueAfter: requeueAfter}, nil
	}

	serviceAccount, err := r.reconcileServiceAccount(ctx, secret)
	if err != nil {
		return reconcile.Result{}, err
	}

	expirationSeconds, err := tokenExpirationSeconds(secret)
	if err != nil {
		return reconcile.Result{}, err
	}

	tokenRequest, err := r.createServiceAccountToken(ctx, serviceAccount, expirationSeconds)
	if err != nil {
		return reconcile.Result{}, err
	}

	renewDuration := r.renewDuration(tokenRequest.Status.ExpirationTimestamp.Time)

	if err := r.reconcileSecret(ctx, secret, tokenRequest.Status.Token, renewDuration); err != nil {
		return reconcile.Result{}, fmt.Errorf("could not update Secret with token: %w", err)
	}

	return reconcile.Result{Requeue: true, RequeueAfter: renewDuration}, nil
}

func (r *reconciler) reconcileServiceAccount(ctx context.Context, secret *corev1.Secret) (*corev1.ServiceAccount, error) {
	serviceAccount := getServiceAccountFromAnnotations(secret.Annotations)

	if _, err := controllerutil.CreateOrUpdate(ctx, r.targetClient, serviceAccount, func() error {
		serviceAccount.AutomountServiceAccountToken = pointer.Bool(false)
		return nil
	}); err != nil {
		return nil, err
	}

	return serviceAccount, nil
}

func (r *reconciler) reconcileSecret(ctx context.Context, secret *corev1.Secret, token string, renewDuration time.Duration) error {
	patch := client.MergeFrom(secret.DeepCopy())

	if secret.Data == nil {
		secret.Data = make(map[string][]byte, 1)
	}

	if err := updateTokenInSecretData(secret.Data, token); err != nil {
		return err
	}
	metav1.SetMetaDataAnnotation(&secret.ObjectMeta, resourcesv1alpha1.ServiceAccountTokenRenewTimestamp, r.clock.Now().UTC().Add(renewDuration).Format(time.RFC3339))

	return r.client.Patch(ctx, secret, patch)
}

func (r *reconciler) createServiceAccountToken(ctx context.Context, sa *corev1.ServiceAccount, expirationSeconds int64) (*authenticationv1.TokenRequest, error) {
	tokenRequest := &authenticationv1.TokenRequest{
		Spec: authenticationv1.TokenRequestSpec{
			Audiences:         []string{v1beta1constants.GardenerAudience},
			ExpirationSeconds: &expirationSeconds,
		},
	}

	return r.targetCoreV1Client.ServiceAccounts(sa.Namespace).CreateToken(ctx, sa.Name, tokenRequest, metav1.CreateOptions{})
}

func (r *reconciler) requeue(renewTimestamp string) (bool, time.Duration, error) {
	if len(renewTimestamp) == 0 {
		return false, 0, nil
	}

	renewTime, err := time.Parse(time.RFC3339, renewTimestamp)
	if err != nil {
		return false, 0, fmt.Errorf("could not parse renew timestamp: %w", err)
	}

	if r.clock.Now().UTC().Before(renewTime.UTC()) {
		return true, renewTime.UTC().Sub(r.clock.Now().UTC()), nil
	}

	return false, 0, nil
}

func (r *reconciler) renewDuration(expirationTimestamp time.Time) time.Duration {
	expirationDuration := expirationTimestamp.UTC().Sub(r.clock.Now().UTC())
	if expirationDuration >= maxExpirationDuration {
		expirationDuration = maxExpirationDuration
	}

	return r.jitter(expirationDuration*80/100, 0.05)
}

func tokenExpirationSeconds(secret *corev1.Secret) (int64, error) {
	var (
		expirationDuration = defaultExpirationDuration
		err                error
	)

	if v, ok := secret.Annotations[resourcesv1alpha1.ServiceAccountTokenExpirationDuration]; ok {
		expirationDuration, err = time.ParseDuration(v)
		if err != nil {
			return 0, err
		}
	}

	return int64(expirationDuration / time.Second), nil
}

func getServiceAccountFromAnnotations(annotations map[string]string) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      annotations[resourcesv1alpha1.ServiceAccountName],
			Namespace: annotations[resourcesv1alpha1.ServiceAccountNamespace],
		},
	}
}

func updateTokenInSecretData(data map[string][]byte, token string) error {
	if _, ok := data[resourcesv1alpha1.DataKeyKubeconfig]; !ok {
		data[resourcesv1alpha1.DataKeyToken] = []byte(token)
		return nil
	}

	kubeconfig := &clientcmdv1.Config{}
	if _, _, err := clientcmdlatest.Codec.Decode(data[resourcesv1alpha1.DataKeyKubeconfig], nil, kubeconfig); err != nil {
		return err
	}

	var userName string
	for _, namedContext := range kubeconfig.Contexts {
		if namedContext.Name == kubeconfig.CurrentContext {
			userName = namedContext.Context.AuthInfo
			break
		}
	}

	for i, users := range kubeconfig.AuthInfos {
		if users.Name == userName {
			kubeconfig.AuthInfos[i].AuthInfo.Token = token
			break
		}
	}

	kubeconfigEncoded, err := runtime.Encode(clientcmdlatest.Codec, kubeconfig)
	if err != nil {
		return err
	}

	data[resourcesv1alpha1.DataKeyKubeconfig] = kubeconfigEncoded
	return nil
}
