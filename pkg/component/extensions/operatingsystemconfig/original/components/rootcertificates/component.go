// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package rootcertificates

import (
	"bytes"
	_ "embed"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	extensionsv1alpha1helper "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components"
	"github.com/gardener/gardener/pkg/utils"
)

const (
	// PathLocalSSLRootCerts is the path to the Gardener CAs. It can be used as trigger for other components to reload the CAs.
	PathLocalSSLRootCerts = pathLocalSSLCerts + "/ROOTcerts.crt"

	pathLocalSSLCerts             = "/var/lib/ca-certificates-local"
	pathUpdateLocalCaCertificates = "/var/lib/ssl/update-local-ca-certificates.sh"
)

var (
	tplNameUpdateLocalCaCertificates = "update-local-ca-certificates"
	//go:embed templates/scripts/update-local-ca-certificates.tpl.sh
	tplContentUpdateLocalCaCertificates string
	tplUpdateLocalCaCertificates        *template.Template
)

func init() {
	var err error
	tplUpdateLocalCaCertificates, err = template.
		New(tplNameUpdateLocalCaCertificates).
		Funcs(sprig.TxtFuncMap()).
		Parse(tplContentUpdateLocalCaCertificates)
	utilruntime.Must(err)
}

type component struct{}

// New returns a new root CA certificates component.
func New() *component {
	return &component{}
}

func (component) Name() string {
	return "root-certificates"
}

func (component) Config(ctx components.Context) ([]extensionsv1alpha1.Unit, []extensionsv1alpha1.File, error) {
	updateLocalCaCertificatesScriptFile, err := updateLocalCACertificatesScriptFile()
	if err != nil {
		return nil, nil, err
	}

	const pathEtcSSLCerts = "/etc/ssl/certs"
	var caBundleBase64 = utils.EncodeBase64([]byte(ctx.CABundle))

	updateCACertsFiles := []extensionsv1alpha1.File{
		updateLocalCaCertificatesScriptFile,
		// This file contains Gardener CAs for Debian based OS
		{
			Path:        PathLocalSSLRootCerts,
			Permissions: ptr.To[uint32](0644),
			Content: extensionsv1alpha1.FileContent{
				Inline: &extensionsv1alpha1.FileContentInline{
					Encoding: "b64",
					Data:     caBundleBase64,
				},
			},
		},
		// This file contains Gardener CAs for Redhat/SUSE OS
		{
			Path:        "/etc/pki/trust/anchors/ROOTcerts.pem",
			Permissions: ptr.To[uint32](0644),
			Content: extensionsv1alpha1.FileContent{
				Inline: &extensionsv1alpha1.FileContentInline{
					Encoding: "b64",
					Data:     caBundleBase64,
				},
			},
		},
	}

	updateCACertsUnit := extensionsv1alpha1.Unit{
		Name:    "updatecacerts.service",
		Command: ptr.To(extensionsv1alpha1.CommandStart),
		Content: ptr.To(`[Unit]
Description=Update local certificate authorities
# Since other services depend on the certificate store run this early
DefaultDependencies=no
Wants=systemd-tmpfiles-setup.service clean-ca-certificates.service
After=systemd-tmpfiles-setup.service clean-ca-certificates.service
Before=sysinit.target ` + v1beta1constants.OperatingSystemConfigUnitNameKubeletService + `
ConditionPathIsReadWrite=` + pathEtcSSLCerts + `
ConditionPathIsReadWrite=` + pathLocalSSLCerts + `
[Service]
Type=oneshot
ExecStart=` + pathUpdateLocalCaCertificates + `
[Install]
WantedBy=multi-user.target`),
		FilePaths: extensionsv1alpha1helper.FilePathsFrom(updateCACertsFiles),
	}

	return []extensionsv1alpha1.Unit{updateCACertsUnit}, updateCACertsFiles, nil
}

func updateLocalCACertificatesScriptFile() (extensionsv1alpha1.File, error) {
	var script bytes.Buffer
	if err := tplUpdateLocalCaCertificates.Execute(&script, map[string]any{
		"pathLocalSSLCerts": pathLocalSSLCerts,
	}); err != nil {
		return extensionsv1alpha1.File{}, err
	}

	return extensionsv1alpha1.File{
		Path:        pathUpdateLocalCaCertificates,
		Permissions: ptr.To[uint32](0744),
		Content: extensionsv1alpha1.FileContent{
			Inline: &extensionsv1alpha1.FileContentInline{
				Encoding: "b64",
				Data:     utils.EncodeBase64(script.Bytes()),
			},
		},
	}, nil
}
