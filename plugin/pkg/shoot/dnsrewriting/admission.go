// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package dnsrewriting

import (
	"context"
	"errors"
	"fmt"
	"io"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apiserver/pkg/admission"

	"github.com/gardener/gardener/pkg/apis/core"
	plugin "github.com/gardener/gardener/plugin/pkg"
	"github.com/gardener/gardener/plugin/pkg/shoot/dnsrewriting/apis/shootdnsrewriting/validation"
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(plugin.PluginNameShootDNSRewriting, func(config io.Reader) (admission.Interface, error) {
		cfg, err := LoadConfiguration(config)
		if err != nil {
			return nil, err
		}

		if err := validation.ValidateConfiguration(cfg); err != nil {
			return nil, fmt.Errorf("invalid config: %+v", err)
		}

		return New(cfg.CommonSuffixes), nil
	})
}

// DNSRewriting contains required information to process admission requests.
type DNSRewriting struct {
	*admission.Handler
	commonSuffixes []string
}

// New creates a new ShootDNSRewriting admission plugin.
func New(commonSuffixes []string) admission.MutationInterface {
	return &DNSRewriting{
		Handler:        admission.NewHandler(admission.Create),
		commonSuffixes: commonSuffixes,
	}
}

// Admit defaults spec.systemComponents.coreDNS.rewriting.commonSuffixes to the configured values for new shoot clusters.
func (c *DNSRewriting) Admit(_ context.Context, a admission.Attributes, _ admission.ObjectInterfaces) error {
	switch {
	case a.GetKind().GroupKind() != core.Kind("Shoot"),
		a.GetOperation() != admission.Create,
		a.GetSubresource() != "":
		return nil
	}

	shoot, ok := a.GetObject().(*core.Shoot)
	if !ok {
		return apierrors.NewInternalError(errors.New("could not convert resource into Shoot object"))
	}

	if len(c.commonSuffixes) == 0 {
		return nil
	}

	if shoot.Spec.SystemComponents == nil {
		shoot.Spec.SystemComponents = &core.SystemComponents{}
	}

	if shoot.Spec.SystemComponents.CoreDNS == nil {
		shoot.Spec.SystemComponents.CoreDNS = &core.CoreDNS{}
	}

	if shoot.Spec.SystemComponents.CoreDNS.Rewriting == nil {
		shoot.Spec.SystemComponents.CoreDNS.Rewriting = &core.CoreDNSRewriting{}
	}

	shoot.Spec.SystemComponents.CoreDNS.Rewriting.CommonSuffixes = append(shoot.Spec.SystemComponents.CoreDNS.Rewriting.CommonSuffixes, c.commonSuffixes...)

	return nil
}
