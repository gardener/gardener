// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package test

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// FakeManager fakes a manager.Manager.
type FakeManager struct {
	manager.Manager

	Client        client.Client
	Cache         cache.Cache
	EventRecorder record.EventRecorder
	APIReader     client.Reader
	Scheme        *runtime.Scheme
}

// GetClient returns the client of the FakeManager.
func (f FakeManager) GetClient() client.Client {
	return f.Client
}

// GetCache returns the cache of the FakeManager.
func (f FakeManager) GetCache() cache.Cache {
	return f.Cache
}

// GetEventRecorderFor returns the eventRecorder of the FakeManager.
func (f FakeManager) GetEventRecorderFor(_ string) record.EventRecorder {
	return f.EventRecorder
}

// GetAPIReader returns the apiReader of the FakeManager.
func (f FakeManager) GetAPIReader() client.Reader {
	return f.APIReader
}

// GetScheme returns the Scheme of the FakeManager.
func (f FakeManager) GetScheme() *runtime.Scheme {
	return f.Scheme
}
