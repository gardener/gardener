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

package seed

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apiserver/pkg/authorization/authorizer"
)

// PluginName is the name of this authorization plugin.
const PluginName = "seedauthorizer"

// NewAuthorizer returns a new seed authorizer which never has an opinion on the request.
func NewAuthorizer(logger logr.Logger) *plugin {
	return &plugin{
		logger: logger,
	}
}

// Authorizer authorizes requests from gardenlets. Only resources related to a Seed are allowed.
type plugin struct {
	logger logr.Logger
}

var _ = authorizer.Authorizer(&plugin{})

func (p *plugin) Authorize(_ context.Context, _ authorizer.Attributes) (authorizer.Decision, string, error) {
	return authorizer.DecisionNoOpinion, "", nil
}
