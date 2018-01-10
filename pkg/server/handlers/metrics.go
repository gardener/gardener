// Copyright 2018 The Gardener Authors.
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

package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/apis/garden/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation/common"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
)

type metrics struct {
	k8sGardenClient kubernetes.Client
	interval        time.Duration
}

// InitMetrics takes an Kubernetes <client> for a Garden cluster and initiate the
// collection of Gardener and Shoot related metrics. It returns a <http.Handler> to
// register on a webserver, which provides the collected metrics in the Prometheus format.
func InitMetrics(client kubernetes.Client, scrapeInterval time.Duration) http.Handler {
	m := metrics{
		k8sGardenClient: client,
		interval:        scrapeInterval,
	}
	m.initShootMetrics()
	m.initProjectCountMetric()
	m.initUserCountMetric()
	return promhttp.Handler()
}

func (m metrics) initShootMetrics() {
	metricShootState := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "garden_shoot_state",
		Help: "State of a Shoot cluster",
	}, []string{"name", "project", "cloud", "version", "region", "seed", "createdAt"})

	metricShootNodeCount := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "garden_shoot_nodes_count",
		Help: "Node count of a Shoot cluster",
	}, []string{"name", "project"})

	prometheus.Register(metricShootState)
	prometheus.Register(metricShootNodeCount)

	m.collect(func() {
		var state float64
		shoots, err := m.k8sGardenClient.ListShoots(metav1.NamespaceAll)
		if err != nil {
			logger.Logger.Info("Unable to fetch shoots. skip shoot metric set...")
			return
		}

		for _, shoot := range shoots.Items {
			state = 4 // unknown
			if shoot.Status.LastOperation != nil {
				switch shoot.Status.LastOperation.State {
				case gardenv1beta1.ShootLastOperationStateSucceeded:
					state = 0
				case gardenv1beta1.ShootLastOperationStateProcessing:
					state = 1
				case gardenv1beta1.ShootLastOperationStateError:
					state = 2
				case gardenv1beta1.ShootLastOperationStateFailed:
					state = 3
				}
			}

			cloud, err := helper.DetermineCloudProviderInShoot(shoot.Spec.Cloud)
			if err != nil {
				logger.Logger.Infof("Unable to determine cloud provider for Shoot %s", shoot.Name)
				return
			}

			shootVar := shoot
			shootObj := &shootpkg.Shoot{Info: &shootVar}
			nodeCount := shootObj.GetNodeCount()

			metricShootState.With(prometheus.Labels{
				"name":      shoot.Name,
				"project":   shoot.Namespace,
				"cloud":     string(cloud),
				"region":    shoot.Spec.Cloud.Region,
				"version":   shoot.Spec.Kubernetes.Version,
				"seed":      *(shoot.Spec.Cloud.Seed),
				"createdAt": strconv.FormatInt(shoot.CreationTimestamp.UTC().Unix(), 10),
			}).Set(state)

			metricShootNodeCount.With(prometheus.Labels{
				"name":    shoot.Name,
				"project": shoot.Namespace,
			}).Set(float64(nodeCount))
		}
	})
}

func (m metrics) initProjectCountMetric() {
	metricProjectCount := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "garden_project_count",
		Help: "Count of projects",
	})
	prometheus.MustRegister(metricProjectCount)
	projectsLabel := fmt.Sprintf("%s=%s", common.GardenRole, common.GardenRoleProject)

	m.collect(func() {
		projects, err := m.k8sGardenClient.ListNamespaces(metav1.ListOptions{
			LabelSelector: projectsLabel,
		})
		if err != nil {
			logger.Logger.Info("Unable to fetch project namespaces. skip metric...")
			return
		}
		metricProjectCount.Set(float64(len(projects.Items)))
	})
}

func (m metrics) initUserCountMetric() {
	metricUserCount := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "garden_user_count",
		Help: "Count of users",
	})
	prometheus.MustRegister(metricUserCount)
	usersLabel := fmt.Sprintf("%s=%s", common.GardenRole, common.GardenRoleMembers)

	m.collect(func() {
		roleBindings, err := m.k8sGardenClient.ListRoleBindings(metav1.NamespaceAll, metav1.ListOptions{
			LabelSelector: usersLabel,
		})
		if err != nil {
			logger.Logger.Info("Unable to fetch user rolebindings. skip metric...")
			return
		}
		var userCount float64
		for _, rb := range roleBindings {
			for _, subject := range rb.Subjects {
				if subject.Kind == "User" {
					userCount++
				}
			}
		}
		metricUserCount.Set(userCount)
	})
}

func (m metrics) collect(metricsCollector func()) {
	go func() {
		for {
			metricsCollector()
			time.Sleep(m.interval)
		}
	}()
}
