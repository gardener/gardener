// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package test

import (
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// FakeManager fakes a manager.Manager.
type FakeManager struct {
	manager.Manager

	Client        client.Client
	Cache         cache.Cache
	EventRecorder events.EventRecorder
	APIReader     client.Reader
	Scheme        *runtime.Scheme
	Logger        logr.Logger
	// AddFunc is an optional callback invoked by Add. If nil, Add is a no-op.
	AddFunc func(manager.Runnable) error
}

// GetClient returns the client of the FakeManager.
func (f FakeManager) GetClient() client.Client {
	return f.Client
}

// GetCache returns the cache of the FakeManager.
func (f FakeManager) GetCache() cache.Cache {
	return f.Cache
}

// GetEventRecorder returns the eventRecorder of the FakeManager.
func (f FakeManager) GetEventRecorder(_ string) events.EventRecorder {
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

// GetLogger returns the Logger of the FakeManager.
func (f FakeManager) GetLogger() logr.Logger {
	return f.Logger
}

// Add calls AddFunc if set, otherwise is a no-op.
func (f FakeManager) Add(runnable manager.Runnable) error {
	if f.AddFunc != nil {
		return f.AddFunc(runnable)
	}
	return nil
}
