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

package botanist

import (
	"context"

	"github.com/gardener/gardener/pkg/features"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
)

func (b *Botanist) DeployInternalDNSResources(ctx context.Context) error {
	if gardenletfeatures.FeatureGate.Enabled(features.UseDNSRecords) {
		if err := b.MigrateInternalDNS(ctx); err != nil {
			return err
		}
		return b.DeployOrDestroyInternalDNSRecord(ctx)
	} else {
		if err := b.MigrateInternalDNSRecord(ctx); err != nil {
			return err
		}
		if err := b.DestroyInternalDNSRecord(ctx); err != nil {
			return err
		}
		return b.DeployInternalDNS(ctx)
	}
}

func (b *Botanist) DeployExternalDNSResources(ctx context.Context) error {
	if gardenletfeatures.FeatureGate.Enabled(features.UseDNSRecords) {
		if err := b.MigrateExternalDNS(ctx); err != nil {
			return err
		}
		return b.DeployOrDestroyExternalDNSRecord(ctx)
	} else {
		if err := b.MigrateExternalDNSRecord(ctx); err != nil {
			return err
		}
		if err := b.DestroyExternalDNSRecord(ctx); err != nil {
			return err
		}
		return b.DeployExternalDNS(ctx)
	}
}

func (b *Botanist) DeployIngressDNSResources(ctx context.Context) error {
	if gardenletfeatures.FeatureGate.Enabled(features.UseDNSRecords) {
		if err := b.MigrateIngressDNS(ctx); err != nil {
			return err
		}
		return b.DeployOrDestroyIngressDNSRecord(ctx)
	} else {
		if err := b.MigrateIngressDNSRecord(ctx); err != nil {
			return err
		}
		if err := b.DestroyIngressDNSRecord(ctx); err != nil {
			return err
		}
		return b.DeployIngressDNS(ctx)
	}
}

func (b *Botanist) DestroyInternalDNSResources(ctx context.Context) error {
	if err := b.DestroyInternalDNS(ctx); err != nil {
		return err
	}
	return b.DestroyInternalDNSRecord(ctx)
}

func (b *Botanist) DestroyExternalDNSResources(ctx context.Context) error {
	if err := b.DestroyExternalDNS(ctx); err != nil {
		return err
	}
	return b.DestroyExternalDNSRecord(ctx)
}

func (b *Botanist) DestroyIngressDNSResources(ctx context.Context) error {
	if err := b.DestroyIngressDNS(ctx); err != nil {
		return err
	}
	return b.DestroyIngressDNSRecord(ctx)
}

func (b *Botanist) MigrateInternalDNSResources(ctx context.Context) error {
	if err := b.MigrateInternalDNS(ctx); err != nil {
		return err
	}
	return b.MigrateInternalDNSRecord(ctx)
}

func (b *Botanist) MigrateExternalDNSResources(ctx context.Context) error {
	if err := b.MigrateExternalDNS(ctx); err != nil {
		return err
	}
	return b.MigrateExternalDNSRecord(ctx)
}

func (b *Botanist) MigrateIngressDNSResources(ctx context.Context) error {
	if err := b.MigrateIngressDNS(ctx); err != nil {
		return err
	}
	return b.MigrateIngressDNSRecord(ctx)
}
