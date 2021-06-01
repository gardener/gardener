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

package graph

import (
	"context"
	"time"

	"github.com/gardener/gardener/pkg/admissioncontroller/seedidentity"
	"github.com/gardener/gardener/pkg/utils"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"

	certificatesv1beta1 "k8s.io/api/certificates/v1beta1"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"
)

func (g *graph) setupCertificateSigningRequestWatch(_ context.Context, informer cache.Informer) {
	informer.AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			certificateSigningRequest, ok := obj.(*certificatesv1beta1.CertificateSigningRequest)
			if !ok {
				return
			}
			g.handleCertificateSigningRequestCreate(certificateSigningRequest)
		},

		DeleteFunc: func(obj interface{}) {
			if tombstone, ok := obj.(toolscache.DeletedFinalStateUnknown); ok {
				obj = tombstone.Obj
			}
			certificateSigningRequest, ok := obj.(*certificatesv1beta1.CertificateSigningRequest)
			if !ok {
				return
			}
			g.handleCertificateSigningRequestDelete(certificateSigningRequest)
		},
	})
}

func (g *graph) handleCertificateSigningRequestCreate(certificateSigningRequest *certificatesv1beta1.CertificateSigningRequest) {
	start := time.Now()
	defer func() {
		metricUpdateDuration.WithLabelValues("CertificateSigningRequest", "Create").Observe(time.Since(start).Seconds())
	}()
	g.lock.Lock()
	defer g.lock.Unlock()

	x509cr, err := utils.DecodeCertificateRequest(certificateSigningRequest.Spec.Request)
	if err != nil {
		return
	}
	if !gutil.IsSeedClientCert(x509cr, certificateSigningRequest.Spec.Usages) {
		return
	}
	seedName, _ := seedidentity.FromCertificateSigningRequest(x509cr)

	var (
		certificateSigningRequestVertex = g.getOrCreateVertex(VertexTypeCertificateSigningRequest, "", certificateSigningRequest.Name)
		seedVertex                      = g.getOrCreateVertex(VertexTypeSeed, "", seedName)
	)

	g.addEdge(certificateSigningRequestVertex, seedVertex)
}

func (g *graph) handleCertificateSigningRequestDelete(certificateSigningRequest *certificatesv1beta1.CertificateSigningRequest) {
	start := time.Now()
	defer func() {
		metricUpdateDuration.WithLabelValues("CertificateSigningRequest", "Delete").Observe(time.Since(start).Seconds())
	}()
	g.lock.Lock()
	defer g.lock.Unlock()

	g.deleteVertex(VertexTypeCertificateSigningRequest, "", certificateSigningRequest.Name)
}
