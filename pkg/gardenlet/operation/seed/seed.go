// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seed

import (
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/utils"
)

// NewBuilder returns a new Builder.
func NewBuilder() *Builder {
	return &Builder{
		seedObjectFunc: func(_ context.Context) (*gardencorev1beta1.Seed, error) {
			return nil, errors.New("seed object is required but not set")
		},
	}
}

// WithSeedObject sets the seedObjectFunc attribute at the Builder.
func (b *Builder) WithSeedObject(seedObject *gardencorev1beta1.Seed) *Builder {
	b.seedObjectFunc = func(_ context.Context) (*gardencorev1beta1.Seed, error) { return seedObject, nil }
	return b
}

// WithSeedObjectFrom sets the seedObjectFunc attribute at the Builder after fetching it from the given lister.
func (b *Builder) WithSeedObjectFrom(gardenClient client.Reader, seedName string) *Builder {
	b.seedObjectFunc = func(ctx context.Context) (*gardencorev1beta1.Seed, error) {
		seed := &gardencorev1beta1.Seed{}
		return seed, gardenClient.Get(ctx, client.ObjectKey{Name: seedName}, seed)
	}
	return b
}

// Build initializes a new Seed object.
func (b *Builder) Build(ctx context.Context) (*Seed, error) {
	seed := &Seed{}

	seedObject, err := b.seedObjectFunc(ctx)
	if err != nil {
		return nil, err
	}
	seed.SetInfo(seedObject)

	return seed, nil
}

// GetInfo returns the seed resource of this Seed in a concurrency safe way.
// This method should be used only for reading the data of the returned seed resource. The returned seed
// resource MUST NOT BE MODIFIED (except in test code) since this might interfere with other concurrent reads and writes.
// To properly update the seed resource of this Seed use UpdateInfo or UpdateInfoStatus.
func (s *Seed) GetInfo() *gardencorev1beta1.Seed {
	return s.info.Load().(*gardencorev1beta1.Seed)
}

// SetInfo sets the seed resource of this Seed in a concurrency safe way.
// This method is not protected by a mutex and does not update the seed resource in the cluster and so
// should be used only in exceptional situations, or as a convenience in test code. The seed passed as a parameter
// MUST NOT BE MODIFIED after the call to SetInfo (except in test code) since this might interfere with other concurrent reads and writes.
// To properly update the seed resource of this Seed use UpdateInfo or UpdateInfoStatus.
func (s *Seed) SetInfo(seed *gardencorev1beta1.Seed) {
	s.info.Store(seed)
}

// UpdateInfo updates the seed resource of this Seed in a concurrency safe way,
// using the given context, client, and mutate function.
// It copies the current seed resource and then uses the copy to patch the resource in the cluster
// using either client.MergeFrom or client.StrategicMergeFrom depending on useStrategicMerge.
// This method is protected by a mutex, so only a single UpdateInfo or UpdateInfoStatus operation can be
// executed at any point in time.
func (s *Seed) UpdateInfo(ctx context.Context, c client.Client, useStrategicMerge bool, f func(*gardencorev1beta1.Seed) error) error {
	s.infoMutex.Lock()
	defer s.infoMutex.Unlock()

	seed := s.info.Load().(*gardencorev1beta1.Seed).DeepCopy()

	var patch client.Patch
	if useStrategicMerge {
		patch = client.StrategicMergeFrom(seed.DeepCopy())
	} else {
		patch = client.MergeFrom(seed.DeepCopy())
	}
	if err := f(seed); err != nil {
		return err
	}
	if err := c.Patch(ctx, seed, patch); err != nil {
		return err
	}
	s.info.Store(seed)
	return nil
}

// UpdateInfoStatus updates the status of the seed resource of this Seed in a concurrency safe way,
// using the given context, client, and mutate function.
// It copies the current seed resource and then uses the copy to patch the resource in the cluster
// using either client.MergeFrom or client.StrategicMergeFrom depending on useStrategicMerge.
// This method is protected by a mutex, so only a single UpdateInfo or UpdateInfoStatus operation can be
// executed at any point in time.
func (s *Seed) UpdateInfoStatus(ctx context.Context, c client.Client, useStrategicMerge bool, f func(*gardencorev1beta1.Seed) error) error {
	s.infoMutex.Lock()
	defer s.infoMutex.Unlock()

	seed := s.info.Load().(*gardencorev1beta1.Seed).DeepCopy()

	var patch client.Patch
	if useStrategicMerge {
		patch = client.StrategicMergeFrom(seed.DeepCopy())
	} else {
		patch = client.MergeFrom(seed.DeepCopy())
	}
	if err := f(seed); err != nil {
		return err
	}
	if err := c.Status().Patch(ctx, seed, patch); err != nil {
		return err
	}
	s.info.Store(seed)
	return nil
}

// GetIngressFQDN returns the fully qualified domain name of ingress sub-resource for the Seed cluster. The
// end result is '<subDomain>.<shootName>.<projectName>.<seed-ingress-domain>'.
func (s *Seed) GetIngressFQDN(subDomain string) string {
	return fmt.Sprintf("%s.%s", subDomain, s.IngressDomain())
}

// IngressDomain returns the ingress domain for the seed.
func (s *Seed) IngressDomain() string {
	seed := s.GetInfo()
	if seed.Spec.Ingress != nil {
		return seed.Spec.Ingress.Domain
	}
	return ""
}

// GetValidVolumeSize is to get a valid volume size.
// If the given size is smaller than the minimum volume size permitted by cloud provider on which seed cluster is running, it will return the minimum size.
func (s *Seed) GetValidVolumeSize(size string) string {
	seed := s.GetInfo()
	if seed.Spec.Volume == nil || seed.Spec.Volume.MinimumSize == nil {
		return size
	}

	qs, err := resource.ParseQuantity(size)
	if err == nil && qs.Cmp(*seed.Spec.Volume.MinimumSize) < 0 {
		return seed.Spec.Volume.MinimumSize.String()
	}

	return size
}

// GetLoadBalancerServiceAnnotations returns the load balancer annotations set for the seed if any.
func (s *Seed) GetLoadBalancerServiceAnnotations() map[string]string {
	seed := s.GetInfo()
	if seed.Spec.Settings != nil && seed.Spec.Settings.LoadBalancerServices != nil {
		// return copy of annotations to prevent any accidental mutation by components
		return utils.MergeStringMaps(seed.Spec.Settings.LoadBalancerServices.Annotations)
	}
	return nil
}

// GetLoadBalancerServiceExternalTrafficPolicy indicates the external traffic policy for the seed if any.
func (s *Seed) GetLoadBalancerServiceExternalTrafficPolicy() *corev1.ServiceExternalTrafficPolicy {
	seed := s.GetInfo()
	if seed.Spec.Settings != nil && seed.Spec.Settings.LoadBalancerServices != nil {
		return seed.Spec.Settings.LoadBalancerServices.ExternalTrafficPolicy
	}
	return nil
}

// GetZonalLoadBalancerServiceAnnotations returns the zonal load balancer annotations set for the seed if any.
func (s *Seed) GetZonalLoadBalancerServiceAnnotations(zone string) map[string]string {
	seed := s.GetInfo()
	if seed.Spec.Settings != nil && seed.Spec.Settings.LoadBalancerServices != nil {
		for _, zoneSettings := range seed.Spec.Settings.LoadBalancerServices.Zones {
			if zoneSettings.Name == zone {
				// return copy of annotations to prevent any accidental mutation by components
				return utils.MergeStringMaps(zoneSettings.Annotations)
			}
		}
	}
	return s.GetLoadBalancerServiceAnnotations()
}

// GetZonalLoadBalancerServiceExternalTrafficPolicy indicates the zonal external traffic policy for the seed if any.
func (s *Seed) GetZonalLoadBalancerServiceExternalTrafficPolicy(zone string) *corev1.ServiceExternalTrafficPolicy {
	seed := s.GetInfo()
	if seed.Spec.Settings != nil && seed.Spec.Settings.LoadBalancerServices != nil {
		for _, zoneSettings := range seed.Spec.Settings.LoadBalancerServices.Zones {
			if zoneSettings.Name == zone {
				return zoneSettings.ExternalTrafficPolicy
			}
		}
	}
	return s.GetLoadBalancerServiceExternalTrafficPolicy()
}

// GetLoadBalancerServiceProxyProtocolTermination indicates if the seed allows proxy protocol termination for load balancer services.
func (s *Seed) GetLoadBalancerServiceProxyProtocolTermination() *bool {
	seed := s.GetInfo()
	if seed.Spec.Settings != nil && seed.Spec.Settings.LoadBalancerServices != nil && seed.Spec.Settings.LoadBalancerServices.ProxyProtocol != nil {
		return &seed.Spec.Settings.LoadBalancerServices.ProxyProtocol.Allowed
	}
	return nil
}

// GetZonalLoadBalancerServiceProxyProtocolTermination indicates if the seed allows proxy protocol termination for load balancer services for the specified zone.
func (s *Seed) GetZonalLoadBalancerServiceProxyProtocolTermination(zone string) *bool {
	seed := s.GetInfo()
	if seed.Spec.Settings != nil && seed.Spec.Settings.LoadBalancerServices != nil {
		for _, zoneSettings := range seed.Spec.Settings.LoadBalancerServices.Zones {
			if zoneSettings.Name == zone {
				if zoneSettings.ProxyProtocol != nil {
					return &zoneSettings.ProxyProtocol.Allowed
				}
				break
			}
		}
	}
	return s.GetLoadBalancerServiceProxyProtocolTermination()
}

// IsDualStack checks if the seed is a dual-stack seed.
func (s *Seed) IsDualStack() bool {
	seed := s.GetInfo()
	return len(seed.Spec.Networks.IPFamilies) == 2
}

// GetNodeCIDR returns the node CIDR of the seed.
func (s *Seed) GetNodeCIDR() *string {
	return s.GetInfo().Spec.Networks.Nodes
}
