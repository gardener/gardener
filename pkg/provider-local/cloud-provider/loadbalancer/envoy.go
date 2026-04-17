// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package loadbalancer

import (
	"archive/tar"
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"net/netip"
	"text/template"

	"github.com/docker/docker/api/types/container"
	corev1 "k8s.io/api/core/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

const (
	envoyDir = "/home/envoy/"

	envoyConfigFileName = "envoy.yaml"
	envoyConfigFilePath = envoyDir + envoyConfigFileName

	ldsConfigFileName = "lds.yaml"
	ldsConfigFilePath = envoyDir + ldsConfigFileName

	cdsConfigFileName = "cds.yaml"
	cdsConfigFilePath = envoyDir + cdsConfigFileName

	// envoyAdminSocketPath is the path to the unix domain socket in the load balancer container where Envoy listens for admin
	// API requests. We don't expose the admin API on a port as usual to prevent blocking load balancer ports.
	// For now, we only use the admin API for health checks (configured in the container).
	envoyAdminSocketPath = envoyDir + "admin.sock"
)

var (
	//go:embed templates/envoy.yaml.tpl
	tplContentEnvoyConfig string
	tplEnvoyConfig        *template.Template

	//go:embed templates/lds.yaml.tpl
	tplContentLDS string
	tplLDS        *template.Template

	//go:embed templates/cds.yaml.tpl
	tplContentCDS string
	tplCDS        *template.Template
)

func init() {
	var err error
	tplEnvoyConfig, err = template.
		New("envoy.yaml.tpl").
		Parse(tplContentEnvoyConfig)
	utilruntime.Must(err)

	tplLDS, err = template.
		New("lds.yaml.tpl").
		Parse(tplContentLDS)
	utilruntime.Must(err)

	tplCDS, err = template.
		New("cds.yaml.tpl").
		Parse(tplContentCDS)
	utilruntime.Must(err)
}

func (p *Provider) writeEnvoyStaticConfig(ctx context.Context, name string) error {
	var envoyConfig bytes.Buffer
	if err := tplEnvoyConfig.Execute(&envoyConfig, map[string]any{
		"cdsConfigFilePath":    cdsConfigFilePath,
		"ldsConfigFilePath":    ldsConfigFilePath,
		"envoyAdminSocketPath": envoyAdminSocketPath,
	}); err != nil {
		return err
	}

	if err := p.copyFilesToContainer(ctx, name, envoyDir, map[string][]byte{
		envoyConfigFileName: envoyConfig.Bytes(),
	}); err != nil {
		return fmt.Errorf("failed to write static envoy config to %s: %w", name, err)
	}
	return nil
}

func (p *Provider) writeEnvoyDynamicConfig(ctx context.Context, name string, service *corev1.Service, nodes []*corev1.Node) error {
	ldsConfig, cdsConfig, err := generateEnvoyDynamicConfig(service, nodes)
	if err != nil {
		return fmt.Errorf("failed to generate dynamic envoy config for %s: %w", name, err)
	}

	if err := p.copyFilesToContainer(ctx, name, envoyDir, map[string][]byte{
		ldsConfigFileName: ldsConfig,
		cdsConfigFileName: cdsConfig,
	}); err != nil {
		return fmt.Errorf("failed to write dynamic envoy config to %s: %w", name, err)
	}

	return nil
}

func (p *Provider) copyFilesToContainer(ctx context.Context, containerID, destDir string, files map[string][]byte) error {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	defer tw.Close()

	for name, content := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(content))}); err != nil {
			return err
		}
		if _, err := tw.Write(content); err != nil {
			return err
		}
	}
	if err := tw.Close(); err != nil {
		return err
	}

	return p.DockerClient.CopyToContainer(ctx, containerID, destDir, &buf, container.CopyToContainerOptions{})
}

type envoyConfigData struct {
	ServicePorts map[string]servicePort
}

type servicePort struct {
	Protocol string
	Listener listener
	Cluster  []endpoint
}

type listener struct {
	Port int32
}

type endpoint struct {
	Address string
	Port    int32
}

func generateEnvoyDynamicConfig(service *corev1.Service, nodes []*corev1.Node) (ldsConfig, cdsConfig []byte, err error) {
	allNodeIPs := make([]netip.Addr, 0, len(nodes))
	for _, node := range nodes {
		nodeIPs, err := getInternalNodeIPs(node)
		if err != nil {
			return nil, nil, fmt.Errorf("could not get internal IPs of node %s: %w", node.Name, err)
		}

		allNodeIPs = append(allNodeIPs, nodeIPs.AsSlice()...)
	}

	data := &envoyConfigData{
		ServicePorts: make(map[string]servicePort, len(service.Spec.Ports)),
	}

	for _, port := range service.Spec.Ports {
		if port.NodePort == 0 {
			continue
		}

		sp := servicePort{
			Listener: listener{
				Port: port.Port,
			},
		}

		for _, nodeIP := range allNodeIPs {
			sp.Cluster = append(sp.Cluster, endpoint{
				Address: nodeIP.String(),
				Port:    port.NodePort,
			})
		}

		data.ServicePorts[fmt.Sprintf("%d_%s", port.Port, string(port.Protocol))] = sp
	}

	var ldsBuf, cdsBuf bytes.Buffer
	if err := tplLDS.Execute(&ldsBuf, data); err != nil {
		return nil, nil, fmt.Errorf("error rendering LDS config: %w", err)
	}
	if err := tplCDS.Execute(&cdsBuf, data); err != nil {
		return nil, nil, fmt.Errorf("error rendering CDS config: %w", err)
	}

	return ldsBuf.Bytes(), cdsBuf.Bytes(), nil
}
