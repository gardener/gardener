// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package rootcertificates

import (
	"bytes"
	_ "embed"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	extensionsv1alpha1helper "github.com/gardener/gardener/pkg/api/extensions/v1alpha1/helper"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components"
	"github.com/gardener/gardener/pkg/utils"
)

const (
	// PathLocalSSLRootCerts is the path to the Gardener CAs. It can be used as trigger for other components to reload the CAs.
	PathLocalSSLRootCerts = PathLocalSSLCerts + "/ROOTcerts.crt"
	// PathLocalSSLCerts is the directory for local CA certificates.
	PathLocalSSLCerts = "/var/lib/ca-certificates-local"
	// PathLocalSSLRegistryCACerts is the path to the registry CA certificate written during node init and OSC reconciliation.
	PathLocalSSLRegistryCACerts = PathLocalSSLCerts + "/registry-ca.crt"
	// PathPKITrustAnchors is the directory for PKI trust anchor certificates (RedHat/SUSE systems).
	PathPKITrustAnchors = "/etc/pki/trust/anchors"
	// PathPKITrustAnchorsRegistryCACerts is the path to the registry CA certificate in the PKI trust anchors (RedHat/SUSE systems).
	PathPKITrustAnchorsRegistryCACerts = PathPKITrustAnchors + "/registry-ca.pem"

	// PathUpdateLocalCACertificates is the path to the script that updates the local CA certificates.
	PathUpdateLocalCACertificates = "/var/lib/ssl/update-local-ca-certificates.sh"
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
	updateLocalCaCertificatesScriptFile, err := UpdateLocalCACertificatesScriptFile()
	if err != nil {
		return nil, nil, err
	}

	const pathEtcSSLCerts = "/etc/ssl/certs"

	caBundleBase64 := utils.EncodeBase64([]byte(ctx.CABundle))

	updateCACertsFiles := []extensionsv1alpha1.File{
		updateLocalCaCertificatesScriptFile,
		// This file contains Gardener CAs for Debian based OS
		{
			Path:        PathLocalSSLRootCerts,
			Permissions: new(uint32(0644)),
			Content: extensionsv1alpha1.FileContent{
				Inline: &extensionsv1alpha1.FileContentInline{
					Encoding: "b64",
					Data:     caBundleBase64,
				},
			},
		},
		// This file contains Gardener CAs for Redhat/SUSE OS
		{
			Path:        PathPKITrustAnchors + "/ROOTcerts.pem",
			Permissions: new(uint32(0644)),
			Content: extensionsv1alpha1.FileContent{
				Inline: &extensionsv1alpha1.FileContentInline{
					Encoding: "b64",
					Data:     caBundleBase64,
				},
			},
		},
	}

	if ctx.RegistryCABundle != nil {
		updateCACertsFiles = append(updateCACertsFiles,
			extensionsv1alpha1.File{
				Path:        PathLocalSSLRegistryCACerts,
				Permissions: new(uint32(0644)),
				Content: extensionsv1alpha1.FileContent{
					Inline: &extensionsv1alpha1.FileContentInline{
						Encoding: "b64",
						Data:     utils.EncodeBase64([]byte(*ctx.RegistryCABundle)),
					},
				},
			},
			extensionsv1alpha1.File{
				Path:        PathPKITrustAnchorsRegistryCACerts,
				Permissions: new(uint32(0644)),
				Content: extensionsv1alpha1.FileContent{
					Inline: &extensionsv1alpha1.FileContentInline{
						Encoding: "b64",
						Data:     utils.EncodeBase64([]byte(*ctx.RegistryCABundle)),
					},
				},
			},
		)
	}

	updateCACertsUnit := extensionsv1alpha1.Unit{
		Name:    "updatecacerts.service",
		Command: new(extensionsv1alpha1.CommandStart),
		Content: new(`[Unit]
Description=Update local certificate authorities
# Since other services depend on the certificate store run this early
DefaultDependencies=no
Wants=systemd-tmpfiles-setup.service clean-ca-certificates.service
After=systemd-tmpfiles-setup.service clean-ca-certificates.service
Before=sysinit.target ` + v1beta1constants.OperatingSystemConfigUnitNameKubeletService + `
ConditionPathIsReadWrite=` + pathEtcSSLCerts + `
ConditionPathIsReadWrite=` + PathLocalSSLCerts + `
[Service]
Type=oneshot
ExecStart=` + PathUpdateLocalCACertificates + `
[Install]
WantedBy=multi-user.target`),
		FilePaths: extensionsv1alpha1helper.FilePathsFrom(updateCACertsFiles),
	}

	return []extensionsv1alpha1.Unit{updateCACertsUnit}, updateCACertsFiles, nil
}

// UpdateLocalCACertificatesScriptFile returns the file for the update-local-ca-certificates script.
func UpdateLocalCACertificatesScriptFile() (extensionsv1alpha1.File, error) {
	var script bytes.Buffer
	if err := tplUpdateLocalCaCertificates.Execute(&script, map[string]any{
		"pathLocalSSLCerts": PathLocalSSLCerts,
	}); err != nil {
		return extensionsv1alpha1.File{}, err
	}

	return extensionsv1alpha1.File{
		Path:        PathUpdateLocalCACertificates,
		Permissions: new(uint32(0744)),
		Content: extensionsv1alpha1.FileContent{
			Inline: &extensionsv1alpha1.FileContentInline{
				Encoding: "b64",
				Data:     utils.EncodeBase64(script.Bytes()),
			},
		},
	}, nil
}
