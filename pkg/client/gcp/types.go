// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package gcp

import (
	"context"
	"strings"

	compute "google.golang.org/api/compute/v1"
)

// ClientInterface is an interface which must be implemented by GCP clients.
type ClientInterface interface {
	ListKubernetesFirewallRulesForNetwork(ctx context.Context, project, networkName string) ([]string, error)
	ListKubernetesRoutesForNetwork(ctx context.Context, project, networkName, namespace string) ([]string, error)
	DeleteFirewallRule(ctx context.Context, project, firewallRuleName string) error
	DeleteRoute(ctx context.Context, project, routeName string) error
}

const (
	fwNamePrefix string = "k8s"
	routePrefix  string = "shoot--"
)

// ListKubernetesFirewallRulesForNetwork returns a list of all k8s created firewall rules within the shoot network.
func (c *Client) ListKubernetesFirewallRulesForNetwork(ctx context.Context, project, network string) ([]string, error) {
	var firewalls []string
	if err := c.computeService.Firewalls.List(project).Pages(ctx, func(page *compute.FirewallList) error {
		for _, firewall := range page.Items {
			if strings.HasSuffix(firewall.Network, network) && strings.HasPrefix(firewall.Name, fwNamePrefix) {
				firewalls = append(firewalls, firewall.Name)
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return firewalls, nil
}

// ListKubernetesRoutesForNetwork returns a list of all routes within the shoot network which have the namespace as prefix.
func (c *Client) ListKubernetesRoutesForNetwork(ctx context.Context, project, network, namespace string) ([]string, error) {
	var routes []string
	if err := c.computeService.Routes.List(project).Pages(ctx, func(page *compute.RouteList) error {
		for _, route := range page.Items {
			if strings.HasPrefix(route.Name, routePrefix) && strings.HasSuffix(route.Network, network) {
				routes = append(routes, route.Name)
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return routes, nil
}

// DeleteFirewallRule deletes the firewall rule with the specific name. If it does not exist,
// no error is returned.
func (c *Client) DeleteFirewallRule(ctx context.Context, project, firewall string) error {
	_, err := c.computeService.Firewalls.Delete(project, firewall).Context(ctx).Do()
	return err
}

// DeleteRoute deletes the route with the specific name. If it does not exist,
// no error is returned.
func (c *Client) DeleteRoute(ctx context.Context, project, route string) error {
	_, err := c.computeService.Routes.Delete(project, route).Context(ctx).Do()
	return err
}
