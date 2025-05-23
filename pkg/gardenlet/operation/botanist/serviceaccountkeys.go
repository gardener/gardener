// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// SyncPublicServiceAccountKeys retrieves the responses of /.well-known/openid-configuration and /openid/v1/jwks
// from the shoot kube-apiserver and writes them in a secret in the gardener-system-shoot-issuer namespace in the Garden cluster.
func (b *Botanist) SyncPublicServiceAccountKeys(ctx context.Context) error {
	var (
		retrieveBytes = func(ctx context.Context, relativePath string) ([]byte, error) {
			request := b.ShootClientSet.RESTClient().Get()
			request.RequestURI(relativePath)
			return request.DoRaw(ctx)
		}
	)

	// paths copied from https://github.com/kubernetes/kubernetes/blob/7ea3d0245a63fbbba698f1cb939831fe8143db3e/pkg/serviceaccount/openidmetadata.go#L34-L45
	oidReqBytes, err := retrieveBytes(ctx, "/.well-known/openid-configuration")
	if err != nil {
		return err
	}

	jwksReqBytes, err := retrieveBytes(ctx, "/openid/v1/jwks")
	if err != nil {
		return err
	}

	secret := b.emptyPublicServiceAccountKeysSecret()
	_, err = controllerutil.CreateOrUpdate(ctx, b.GardenClient, secret, func() error {
		secret.Labels = map[string]string{
			v1beta1constants.ProjectName:         b.Garden.Project.Name,
			v1beta1constants.LabelShootName:      b.Shoot.GetInfo().Name,
			v1beta1constants.LabelShootNamespace: b.Shoot.GetInfo().Namespace,
			v1beta1constants.LabelPublicKeys:     v1beta1constants.LabelPublicKeysServiceAccount,
		}
		secret.Data = map[string][]byte{
			"openid-config": oidReqBytes,
			"jwks":          jwksReqBytes,
		}
		return nil
	})
	return err
}

// DeletePublicServiceAccountKeys deletes the secret containing the public info
// of the shoot's service account issuer from the gardener-system-shoot-issuer namespace in the Garden cluster.
func (b *Botanist) DeletePublicServiceAccountKeys(ctx context.Context) error {
	return client.IgnoreNotFound(b.GardenClient.Delete(ctx, b.emptyPublicServiceAccountKeysSecret()))
}

func (b *Botanist) emptyPublicServiceAccountKeysSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gardenerutils.ComputeManagedShootIssuerSecretName(b.Garden.Project.Name, b.Shoot.GetInfo().UID),
			Namespace: gardencorev1beta1.GardenerShootIssuerNamespace,
		},
	}
}
