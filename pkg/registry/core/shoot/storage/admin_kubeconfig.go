/*
Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package storage

import (
	"context"
	"fmt"
	"net/url"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/authentication/user"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	kubecorev1listers "k8s.io/client-go/listers/core/v1"

	authenticationapi "github.com/gardener/gardener/pkg/apis/authentication"
	authenticationv1alpha1 "github.com/gardener/gardener/pkg/apis/authentication/v1alpha1"
	authenticationvalidation "github.com/gardener/gardener/pkg/apis/authentication/validation"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/internalversion"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/secrets"
)

// AdminKubeconfigREST implements a RESTStorage for shoots/adminkubeconfig.
type AdminKubeconfigREST struct {
	secretLister         kubecorev1listers.SecretLister
	internalSecretLister gardencorelisters.InternalSecretLister
	shootStorage         getter
	maxExpirationSeconds int64
}

var (
	_ = rest.NamedCreater(&AdminKubeconfigREST{})
	_ = rest.GroupVersionKindProvider(&AdminKubeconfigREST{})

	gvk = schema.GroupVersionKind{
		Group:   authenticationv1alpha1.SchemeGroupVersion.Group,
		Version: authenticationv1alpha1.SchemeGroupVersion.Version,
		Kind:    "AdminKubeconfigRequest",
	}
)

// New returns and instance of AdminKubeconfigRequest
func (r *AdminKubeconfigREST) New() runtime.Object {
	return &authenticationapi.AdminKubeconfigRequest{}
}

// Destroy cleans up its resources on shutdown.
func (r *AdminKubeconfigREST) Destroy() {
	// Given that underlying store is shared with REST,
	// we don't destroy it here explicitly.
}

// Create returns an AdminKubeconfigRequest with kubeconfig based on
// - shoot's advertised addresses
// - shoot's certificate authority
// - user making the request
func (r *AdminKubeconfigREST) Create(ctx context.Context, name string, obj runtime.Object, createValidation rest.ValidateObjectFunc, _ *metav1.CreateOptions) (runtime.Object, error) {
	if createValidation != nil {
		if err := createValidation(ctx, obj.DeepCopyObject()); err != nil {
			return nil, err
		}
	}

	out := obj.(*authenticationapi.AdminKubeconfigRequest)
	if errs := authenticationvalidation.ValidateAdminKubeconfigRequest(out); len(errs) != 0 {
		return nil, apierrors.NewInvalid(gvk.GroupKind(), "", errs)
	}

	userInfo, ok := genericapirequest.UserFrom(ctx)
	if !ok {
		return nil, apierrors.NewBadRequest("no user in context")
	}

	// prepare: get shoot object
	shootObj, err := r.shootStorage.Get(ctx, name, &metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	shoot, ok := shootObj.(*core.Shoot)
	if !ok {
		return nil, apierrors.NewInternalError(fmt.Errorf("cannot convert to *core.Shoot object - got type %T", shootObj))
	}

	if len(shoot.Status.AdvertisedAddresses) == 0 {
		fieldErr := field.Invalid(field.NewPath("status", "status"), shoot.Status.AdvertisedAddresses, "no kube-apiserver advertised addresses in Shoot .status.advertisedAddresses")
		return nil, apierrors.NewInvalid(gvk.GroupKind(), shoot.Name, field.ErrorList{fieldErr})
	}

	// prepare: get cluster and client CA
	caClientSecret, err := r.internalSecretLister.InternalSecrets(shoot.Namespace).Get(gardenerutils.ComputeShootProjectSecretName(shoot.Name, gardenerutils.ShootProjectSecretSuffixCAClient))
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("could not get client CA secret: %w", err))
	}

	clientCACertificate, err := secrets.LoadCertificate("", caClientSecret.Data[secrets.DataKeyPrivateKeyCA], caClientSecret.Data[secrets.DataKeyCertificateCA])
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("could not load client CA certificate from secret: %w", err))
	}

	caClusterSecret, err := r.secretLister.Secrets(shoot.Namespace).Get(gardenerutils.ComputeShootProjectSecretName(shoot.Name, gardenerutils.ShootProjectSecretSuffixCACluster))
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("could not get cluster CA secret: %w", err))
	}
	clusterCABundle := caClusterSecret.Data[secrets.DataKeyCertificateCA]

	if len(clusterCABundle) == 0 {
		return nil, apierrors.NewInternalError(fmt.Errorf("could not load cluster CA bundle from secret"))
	}

	// generate kubeconfig with client certificate
	if r.maxExpirationSeconds > 0 && out.Spec.ExpirationSeconds > r.maxExpirationSeconds {
		out.Spec.ExpirationSeconds = r.maxExpirationSeconds
	}

	var (
		validity = time.Duration(out.Spec.ExpirationSeconds) * time.Second
		authName = fmt.Sprintf("%s--%s", shoot.Namespace, shoot.Name)
		cpsc     = secrets.ControlPlaneSecretConfig{
			Name: authName,
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				CommonName:   userInfo.GetName(),
				Organization: []string{user.SystemPrivilegedGroup},
				CertType:     secrets.ClientCert,
				Validity:     &validity,
				SigningCA:    clientCACertificate,
			},
		}
	)

	for _, address := range shoot.Status.AdvertisedAddresses {
		u, err := url.Parse(address.URL)
		if err != nil {
			return nil, err
		}

		cpsc.KubeConfigRequests = append(cpsc.KubeConfigRequests, secrets.KubeConfigRequest{
			ClusterName:   fmt.Sprintf("%s-%s", authName, address.Name),
			APIServerHost: u.Host,
			CAData:        clusterCABundle,
		})
	}

	cp, err := cpsc.Generate()
	if err != nil {
		return nil, err
	}
	controlPlaneSecret := cp.(*secrets.ControlPlane)

	// return generated kubeconfig in status
	out.Status.Kubeconfig = controlPlaneSecret.Kubeconfig
	out.Status.ExpirationTimestamp = metav1.Time{Time: controlPlaneSecret.Certificate.Certificate.NotAfter}

	return out, nil
}

// GroupVersionKind returns authentication.gardener.cloud/v1alpha1 for AdminKubeconfigRequest.
func (r *AdminKubeconfigREST) GroupVersionKind(schema.GroupVersion) schema.GroupVersionKind {
	return gvk
}

type getter interface {
	Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error)
}
