// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extensions

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
)

// SyncClusterResourceToSeed creates or updates the `extensions.gardener.cloud/v1alpha1.Cluster` resource in the seed
// cluster by adding the shoot, seed, and cloudprofile specification.
func SyncClusterResourceToSeed(
	ctx context.Context,
	c client.Client,
	clusterName string,
	shoot *gardencorev1beta1.Shoot,
	cloudProfile *gardencorev1beta1.CloudProfile,
	seed *gardencorev1beta1.Seed,
) error {
	var (
		cluster = &extensionsv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterName,
			},
		}

		cloudProfileObj *gardencorev1beta1.CloudProfile
		seedObj         *gardencorev1beta1.Seed
		shootObj        *gardencorev1beta1.Shoot
	)

	if cloudProfile != nil {
		cloudProfileObj = cloudProfile.DeepCopy()
		cloudProfileObj.TypeMeta = metav1.TypeMeta{
			APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
			Kind:       "CloudProfile",
		}
		cloudProfileObj.ManagedFields = nil
	}

	if seed != nil {
		seedObj = seed.DeepCopy()
		seedObj.TypeMeta = metav1.TypeMeta{
			APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
			Kind:       "Seed",
		}
		seedObj.ManagedFields = nil
	}

	if shoot != nil {
		shootObj = shoot.DeepCopy()
		shootObj.TypeMeta = metav1.TypeMeta{
			APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
			Kind:       "Shoot",
		}
		shootObj.ManagedFields = nil
	}

	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, c, cluster, func() error {
		if cloudProfileObj != nil {
			cluster.Spec.CloudProfile = runtime.RawExtension{Object: cloudProfileObj}
		}
		if seedObj != nil {
			cluster.Spec.Seed = runtime.RawExtension{Object: seedObj}
		}
		if shootObj != nil {
			cluster.Spec.Shoot = runtime.RawExtension{Object: shootObj}
		}
		return nil
	})
	return err
}

// Cluster contains the decoded resources of Gardener's extension Cluster resource.
type Cluster struct {
	ObjectMeta   metav1.ObjectMeta
	CloudProfile *gardencorev1beta1.CloudProfile
	Seed         *gardencorev1beta1.Seed
	Shoot        *gardencorev1beta1.Shoot
}

// GetCluster tries to read Gardener's Cluster extension resource in the given namespace.
func GetCluster(ctx context.Context, c client.Reader, namespace string) (*Cluster, error) {
	cluster := &extensionsv1alpha1.Cluster{}
	if err := c.Get(ctx, client.ObjectKey{Name: namespace}, cluster); err != nil {
		return nil, err
	}

	cloudProfile, err := CloudProfileFromCluster(cluster)
	if err != nil {
		return nil, err
	}
	seed, err := SeedFromCluster(cluster)
	if err != nil {
		return nil, err
	}
	shoot, err := ShootFromCluster(cluster)
	if err != nil {
		return nil, err
	}

	return &Cluster{cluster.ObjectMeta, cloudProfile, seed, shoot}, nil
}

// CloudProfileFromCluster returns the CloudProfile resource inside the Cluster resource.
func CloudProfileFromCluster(cluster *extensionsv1alpha1.Cluster) (*gardencorev1beta1.CloudProfile, error) {
	var (
		decoder      = kubernetes.GardenCodec.UniversalDeserializer()
		cloudProfile = &gardencorev1beta1.CloudProfile{}
	)

	if cluster.Spec.CloudProfile.Raw == nil {
		return nil, nil
	}
	if _, _, err := decoder.Decode(cluster.Spec.CloudProfile.Raw, nil, cloudProfile); err != nil {
		return nil, err
	}

	return cloudProfile, nil
}

// SeedFromCluster returns the Seed resource inside the Cluster resource.
func SeedFromCluster(cluster *extensionsv1alpha1.Cluster) (*gardencorev1beta1.Seed, error) {
	var (
		decoder = kubernetes.GardenCodec.UniversalDeserializer()
		seed    = &gardencorev1beta1.Seed{}
	)

	if cluster.Spec.Seed.Raw == nil {
		return nil, nil
	}
	if _, _, err := decoder.Decode(cluster.Spec.Seed.Raw, nil, seed); err != nil {
		return nil, err
	}

	return seed, nil
}

// ShootFromCluster returns the Shoot resource inside the Cluster resource.
func ShootFromCluster(cluster *extensionsv1alpha1.Cluster) (*gardencorev1beta1.Shoot, error) {
	var (
		decoder = kubernetes.GardenCodec.UniversalDeserializer()
		shoot   = &gardencorev1beta1.Shoot{}
	)

	if cluster.Spec.Shoot.Raw == nil {
		return nil, nil
	}
	if _, _, err := decoder.Decode(cluster.Spec.Shoot.Raw, nil, shoot); err != nil {
		return nil, err
	}

	return shoot, nil
}

// GenericTokenKubeconfigSecretNameFromCluster reads the generic-token-kubeconfig.secret.gardener.cloud/name annotation
// and returns its value. If the annotation is not present then it falls back to the deprecated
// SecretNameGenericTokenKubeconfig.
func GenericTokenKubeconfigSecretNameFromCluster(cluster *Cluster) string {
	if v, ok := cluster.ObjectMeta.Annotations[v1beta1constants.AnnotationKeyGenericTokenKubeconfigSecretName]; ok {
		return v
	}
	return v1beta1constants.SecretNameGenericTokenKubeconfig
}

// GetShootStateForCluster retrieves the ShootState and the Shoot resources for a given Cluster name by first fetching
// the *extensionsv1alpha1.Cluster object in the seed, extracting the Shoot resource from it and then fetching the
// *gardencorev1beta1.ShootState resource from the garden.
func GetShootStateForCluster(
	ctx context.Context,
	gardenClient client.Client,
	seedClient client.Client,
	clusterName string,
) (
	*gardencorev1beta1.ShootState,
	*gardencorev1beta1.Shoot,
	error,
) {
	cluster := &extensionsv1alpha1.Cluster{}
	if err := seedClient.Get(ctx, client.ObjectKey{Name: clusterName}, cluster); err != nil {
		return nil, nil, err
	}

	shoot, err := ShootFromCluster(cluster)
	if err != nil {
		return nil, nil, err
	}

	if shoot == nil {
		return nil, nil, fmt.Errorf("cluster resource %s doesn't contain shoot resource in raw format", cluster.Name)
	}

	shootState := &gardencorev1beta1.ShootState{}
	if err := gardenClient.Get(ctx, client.ObjectKey{Namespace: shoot.Namespace, Name: shoot.Name}, shootState); err != nil {
		return nil, nil, err
	}

	return shootState, shoot, nil
}

// GetShoot tries to read Gardener's Cluster extension resource in the given namespace and return the embedded Shoot resource.
func GetShoot(ctx context.Context, c client.Reader, namespace string) (*gardencorev1beta1.Shoot, error) {
	cluster := &extensionsv1alpha1.Cluster{}
	if err := c.Get(ctx, client.ObjectKey{Name: namespace}, cluster); err != nil {
		return nil, err
	}

	return ShootFromCluster(cluster)
}
