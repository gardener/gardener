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

package graph

import (
	"context"
	"time"

	certificatesv1 "k8s.io/api/certificates/v1"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"

	"github.com/gardener/gardener/pkg/admissioncontroller/seedidentity"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

func (g *graph) setupCertificateSigningRequestWatch(_ context.Context, informer cache.Informer) error {
	_, err := informer.AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			if csrV1, ok := obj.(*certificatesv1.CertificateSigningRequest); ok {
				g.handleCertificateSigningRequestCreate(csrV1.Name, csrV1.Spec.Request, csrV1.Spec.Usages)
				return
			}
		},

		DeleteFunc: func(obj interface{}) {
			if tombstone, ok := obj.(toolscache.DeletedFinalStateUnknown); ok {
				obj = tombstone.Obj
			}

			if csrV1, ok := obj.(*certificatesv1.CertificateSigningRequest); ok {
				g.handleCertificateSigningRequestDelete(csrV1.Name)
				return
			}
		},
	})
	return err
}

func (g *graph) handleCertificateSigningRequestCreate(name string, request []byte, usages []certificatesv1.KeyUsage) {
	start := time.Now()
	defer func() {
		metricUpdateDuration.WithLabelValues("CertificateSigningRequest", "Create").Observe(time.Since(start).Seconds())
	}()
	g.lock.Lock()
	defer g.lock.Unlock()

	x509cr, err := utils.DecodeCertificateRequest(request)
	if err != nil {
		return
	}
	if ok, _ := gardenerutils.IsSeedClientCert(x509cr, usages); !ok {
		return
	}
	seedName, _, _ := seedidentity.FromCertificateSigningRequest(x509cr)

	var (
		certificateSigningRequestVertex = g.getOrCreateVertex(VertexTypeCertificateSigningRequest, "", name)
		seedVertex                      = g.getOrCreateVertex(VertexTypeSeed, "", seedName)
	)

	g.addEdge(certificateSigningRequestVertex, seedVertex)
}

func (g *graph) handleCertificateSigningRequestDelete(name string) {
	start := time.Now()
	defer func() {
		metricUpdateDuration.WithLabelValues("CertificateSigningRequest", "Delete").Observe(time.Since(start).Seconds())
	}()
	g.lock.Lock()
	defer g.lock.Unlock()

	g.deleteVertex(VertexTypeCertificateSigningRequest, "", name)
}
