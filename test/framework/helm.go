// Copyright 2019 Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
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

package framework

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gardener/gardener/pkg/client/kubernetes"

	"github.com/mholt/archiver"
	"github.com/onsi/ginkgo/v2"
	"k8s.io/helm/pkg/downloader"
	"k8s.io/helm/pkg/getter"
	"k8s.io/helm/pkg/helm/environment"
	"k8s.io/helm/pkg/helm/helmpath"
	"k8s.io/helm/pkg/repo"
)

const (
	stableRepository = "stable"
)

// RenderAndDeployChart downloads a helm chart from helm stable repo url available in resources/repositories and deploys it on the test cluster
func (f *CommonFramework) RenderAndDeployChart(ctx context.Context, k8sClient kubernetes.Interface, c Chart, values map[string]interface{}) error {
	helmRepo := Helm(f.ResourcesDir)
	err := EnsureRepositoryDirectories(helmRepo)
	if err != nil {
		return err
	}

	ginkgo.By("Downloading chart artifacts")
	err = f.DownloadChartArtifacts(ctx, helmRepo, f.ChartDir, c.Name, c.Version)
	if err != nil {
		return fmt.Errorf("unable to download chart artifacts for chart %s with version %s: %w", c.Name, c.Version, err)
	}

	return f.DeployChart(ctx, k8sClient, c.Namespace, f.ChartDir, c.ReleaseName, values)
}

// DeployChart deploys it on the test cluster
func (f *CommonFramework) DeployChart(ctx context.Context, k8sClient kubernetes.Interface, namespace, chartRepoDestination, chartNameToDeploy string, values map[string]interface{}) error {
	chartPathToRender := filepath.Join(chartRepoDestination, chartNameToDeploy)
	return k8sClient.ChartApplier().Apply(ctx, chartPathToRender, namespace, chartNameToDeploy, kubernetes.Values(values), kubernetes.ForceNamespace)
}

// DownloadChartArtifacts downloads a helm chart from helm stable repo url available in resources/repositories
func (f *CommonFramework) DownloadChartArtifacts(ctx context.Context, helm Helm, chartRepoDestination, chartNameToDownload, chartVersionToDownload string) error {
	exists, err := Exists(chartRepoDestination)
	if err != nil {
		return err
	}

	if !exists {
		if err := os.MkdirAll(chartRepoDestination, 0755); err != nil {
			return err
		}
	}

	rf, err := repo.LoadRepositoriesFile(helm.RepositoryFile())
	if err != nil {
		return err
	}

	if len(rf.Repositories) == 0 {
		return ErrNoRepositoriesFound
	}

	stableRepo := rf.Repositories[0]
	var chartPath string

	chartDownloaded, err := Exists(filepath.Join(chartRepoDestination, strings.Split(chartNameToDownload, "/")[1]))
	if err != nil {
		return err
	}

	if !chartDownloaded {
		chartPath, err = downloadChart(ctx, chartNameToDownload, chartVersionToDownload, chartRepoDestination, stableRepo.URL, HelmAccess{
			HelmPath: helm,
		})
		if err != nil {
			return err
		}
		f.Logger.Info("Chart downloaded", "chartPath", chartPath)
	}
	return nil
}

// Chart represents a external helm chart with a specific version and namespace
type Chart struct {
	Name        string
	ReleaseName string
	Namespace   string
	Version     string
}

// Helm is the home for the HELM repo
type Helm string

// Path returns Helm path with elements appended.
func (h Helm) Path(elem ...string) string {
	p := []string{h.String()}
	p = append(p, elem...)
	return filepath.Join(p...)
}

// Path returns the home for the helm repo with.
func (h Helm) String(elem ...string) string {
	return string(h)
}

// Repository returns the path to the local repository.
func (h Helm) Repository() string {
	return h.Path("repository")
}

// RepositoryFile returns the path to the repositories.yaml file.
func (h Helm) RepositoryFile() string {
	return h.Path("repository", "repositories.yaml")
}

// CacheIndex returns the path to an index for the given named repository.
func (h Helm) CacheIndex(name string) string {
	target := fmt.Sprintf("%s-index.yaml", name)
	return h.Path("repository", "cache", target)
}

// HelmAccess is a struct that holds the helm home
type HelmAccess struct {
	HelmPath Helm
}

// EnsureRepositoryDirectories creates the repository directory which holds the repositories.yaml config file
func EnsureRepositoryDirectories(helm Helm) error {
	configDirectories := []string{
		helm.String(),
		helm.Repository(),
	}
	for _, p := range configDirectories {
		fi, err := os.Stat(p)
		if err != nil {
			if os.IsNotExist(err) {
				if err := os.MkdirAll(p, os.ModePerm); err != nil {
					return fmt.Errorf("unable to create %s: %w", p, err)
				}
				continue
			}
			return err
		}
		if !fi.IsDir() {
			return fmt.Errorf("%s must be a directory", p)
		}
	}
	return nil
}

// downloadChart downloads a native chart with <name> to <downloadDestination> from <stableRepoURL>
func downloadChart(ctx context.Context, name, version, downloadDestination, stableRepoURL string, helmSettings HelmAccess) (string, error) {
	providers := getter.All(environment.EnvSettings{})
	dl := downloader.ChartDownloader{
		Getters:  providers,
		HelmHome: helmpath.Home(helmSettings.HelmPath),
		Out:      os.Stdout,
	}

	err := ensureCacheIndex(ctx, helmSettings, stableRepoURL, providers)
	if err != nil {
		return "", err
	}

	// Download the chart
	filename, _, err := dl.DownloadTo(name, version, downloadDestination)
	if err != nil {
		return "", err
	}

	lname, err := filepath.Abs(filename)
	if err != nil {
		return "", err
	}

	err = archiver.Unarchive(lname, downloadDestination)
	if err != nil {
		return "", err
	}

	err = os.Remove(lname)
	if err != nil {
		return "", err
	}
	return lname, nil
}

func ensureCacheIndex(ctx context.Context, helmSettings HelmAccess, stableRepoURL string, providers getter.Providers) error {
	// This will download the cache index file only if it does not exist
	stableRepoCacheIndexPath := helmSettings.HelmPath.CacheIndex(stableRepository)
	if _, err := os.Stat(stableRepoCacheIndexPath); err != nil {
		if os.IsNotExist(err) {
			directory := filepath.Dir(stableRepoCacheIndexPath)
			err := os.MkdirAll(directory, os.ModePerm)
			if err != nil {
				return err
			}
			_, err = downloadCacheIndex(ctx, stableRepoCacheIndexPath, stableRepoURL, providers)
			if err != nil {
				return err
			}
			return nil
		}
		return err
	}
	return nil
}

// downloadCacheIndex downloads the cache index for repository
func downloadCacheIndex(ctx context.Context, cacheFile, stableRepositoryURL string, providers getter.Providers) (*repo.Entry, error) {
	c := repo.Entry{
		Name:  stableRepository,
		URL:   stableRepositoryURL,
		Cache: cacheFile,
	}

	r, err := repo.NewChartRepository(&c, providers)
	if err != nil {
		return nil, err
	}

	if err := r.DownloadIndexFile(""); err != nil {
		return nil, fmt.Errorf("looks like %q is not a valid chart repository or cannot be reached: %s", stableRepositoryURL, err.Error())
	}
	return &c, nil
}
