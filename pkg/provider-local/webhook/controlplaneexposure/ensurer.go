// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package controlplaneexposure

import (
	"context"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	extensionscontextwebhook "github.com/gardener/gardener/extensions/pkg/webhook/context"
	"github.com/gardener/gardener/extensions/pkg/webhook/controlplane/genericmutator"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
)

// NewEnsurer creates a new controlplaneexposure ensurer.
func NewEnsurer() genericmutator.Ensurer {
	return &ensurer{logger: log.Log.WithName("ensurer")}
}

type ensurer struct {
	genericmutator.NoopEnsurer
	logger logr.Logger
}

func (e *ensurer) EnsureKubeAPIServerService(_ context.Context, _ extensionscontextwebhook.GardenContext, newObj, _ *corev1.Service) error {
	if v1beta1helper.IsAPIServerExposureManaged(newObj) {
		return nil
	}

	for i, servicePort := range newObj.Spec.Ports {
		if servicePort.Name == "kube-apiserver" {
			newObj.Spec.Ports[i].NodePort = 30443
			break
		}
	}

	return nil
}
