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

package test

import (
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
}

// GetClient returns the given client.
func (f FakeManager) GetClient() client.Client {
	return f.Client
}

// GetCache returns the given cache.
func (f FakeManager) GetCache() cache.Cache {
	return f.Cache
}

// GetEventRecorderFor returns the given eventRecorder.
func (f FakeManager) GetEventRecorderFor(name string) record.EventRecorder {
	return f.EventRecorder
}

// GetAPIReader returns the given apiReader.
func (f FakeManager) GetAPIReader() client.Reader {
	return f.APIReader
}
