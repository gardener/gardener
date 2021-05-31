/*
Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file

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

	authenticationapi "github.com/gardener/gardener/pkg/apis/authentication"
	authenticationapiv1alpha1 "github.com/gardener/gardener/pkg/apis/authentication/v1alpha1"
	authenticationvalidation "github.com/gardener/gardener/pkg/apis/authentication/validation"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils/infodata"
	"github.com/gardener/gardener/pkg/utils/secrets"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/authentication/user"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
)

// AdminKubeconfigREST implements a RESTStorage for shoots/adminkubeconfig.
type AdminKubeconfigREST struct {
	shootStateStorage    getter
	shootStorage         getter
	now                  func() time.Time
	maxExpirationSeconds int64
}

var (
	_ = rest.NamedCreater(&AdminKubeconfigREST{})
	_ = rest.GroupVersionKindProvider(&AdminKubeconfigREST{})

	gvk = schema.GroupVersionKind{
		Group:   authenticationapiv1alpha1.SchemeGroupVersion.Group,
		Version: authenticationapiv1alpha1.SchemeGroupVersion.Version,
		Kind:    "AdminKubeconfigRequest",
	}
)

// New returns and instance of AdminKubeconfigRequest
func (r *AdminKubeconfigREST) New() runtime.Object {
	return &authenticationapi.AdminKubeconfigRequest{}
}

// Create returns a AdminKubeconfigRequest with kubeconfig based on
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
		return nil, errors.NewInvalid(gvk.GroupKind(), "", errs)
	}

	userInfo, ok := genericapirequest.UserFrom(ctx)
	if !ok {
		return nil, errors.NewBadRequest("no user in context")
	}

	shootStateObj, err := r.shootStateStorage.Get(ctx, name, &metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	shootState := &gardencorev1alpha1.ShootState{}

	err = kubernetes.GardenScheme.Convert(shootStateObj, shootState, nil)
	if err != nil {
		return nil, err
	}

	shootObj, err := r.shootStorage.Get(ctx, name, &metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	shoot, ok := shootObj.(*core.Shoot)
	if !ok {
		return nil, errors.NewInternalError(fmt.Errorf("cannot convert to *core.Shoot object - got type %T", shootObj))
	}

	if len(shoot.Status.AdvertisedAddresses) == 0 {
		return nil, errors.NewBadRequest("no kube-apiserver advertised addresses in Shoot .status.advertisedAddresses")
	}

	ca, err := infodata.GetInfoData(shootState.Spec.Gardener, v1beta1constants.SecretNameCACluster)
	if err != nil {
		return nil, errors.NewInternalError(err)
	}

	if ca == nil {
		return nil, errors.NewBadRequest("certificate authority not yet provisioned")
	}

	caInfoData, ok := ca.(*secrets.CertificateInfoData)
	if !ok {
		return nil, errors.NewInternalError(fmt.Errorf("could not convert InfoData entry ca to CertificateInfoData"))
	}

	caCert, err := secrets.LoadCertificate("", caInfoData.PrivateKey, caInfoData.Certificate)
	if err != nil {
		return nil, errors.NewInternalError(err)
	}

	if r.maxExpirationSeconds > 0 && out.Spec.ExpirationSeconds > r.maxExpirationSeconds {
		out.Spec.ExpirationSeconds = r.maxExpirationSeconds
	}

	validity := time.Duration(out.Spec.ExpirationSeconds) * time.Second
	authName := fmt.Sprintf("%s--%s", shoot.Namespace, shoot.Name)

	cpsc := secrets.ControlPlaneSecretConfig{
		CertificateSecretConfig: &secrets.CertificateSecretConfig{
			Name:         authName,
			CommonName:   userInfo.GetName(),
			Organization: []string{user.SystemPrivilegedGroup},
			CertType:     secrets.ClientCert,
			Validity:     &validity,
			SigningCA:    caCert,
			Now:          r.now,
		},
	}

	for _, address := range shoot.Status.AdvertisedAddresses {
		u, err := url.Parse(address.URL)
		if err != nil {
			return nil, err
		}

		cpsc.KubeConfigRequests = append(cpsc.KubeConfigRequests, secrets.KubeConfigRequest{
			ClusterName:   fmt.Sprintf("%s-%s", authName, address.Name),
			APIServerHost: u.Host,
		})
	}

	cp, err := cpsc.GenerateControlPlane()
	if err != nil {
		return nil, err
	}

	out.Status.Kubeconfig = cp.Kubeconfig
	out.Status.ExpirationTimestamp = metav1.Time{Time: cp.Certificate.Certificate.NotAfter}

	return out, nil
}

// GroupVersionKind returns authentication.gardener.cloud/v1alpha1 for AdminKubeconfigRequest.
func (r *AdminKubeconfigREST) GroupVersionKind(schema.GroupVersion) schema.GroupVersionKind {
	return gvk
}

type getter interface {
	Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error)
}
