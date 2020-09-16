// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"github.com/gardener/gardener/extensions/pkg/controller/operatingsystemconfig/oscommon/cloudinit"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/pkg/errors"
)

// FileContentInlineCodec contains methods for encoding and decoding byte slices
// to and from *extensionsv1alpha1.FileContentInline.
type FileContentInlineCodec interface {
	// Encode encodes the given byte slice into a *extensionsv1alpha1.FileContentInline.
	Encode([]byte, string) (*extensionsv1alpha1.FileContentInline, error)
	// Decode decodes a byte slice from the given *extensionsv1alpha1.FileContentInline.
	Decode(*extensionsv1alpha1.FileContentInline) ([]byte, error)
}

// NewFileContentInlineCodec creates an returns a new FileContentInlineCodec.
func NewFileContentInlineCodec() FileContentInlineCodec {
	return &fileContentInlineCodec{}
}

type fileContentInlineCodec struct{}

// Encode encodes the given byte slice into a *extensionsv1alpha1.FileContentInline.
func (c *fileContentInlineCodec) Encode(data []byte, encoding string) (*extensionsv1alpha1.FileContentInline, error) {
	// Initialize file codec
	fileCodec, err := getFileCodec(encoding)
	if err != nil {
		return nil, err
	}

	// Encode data using the file codec, if needed
	if fileCodec != nil {
		if data, err = fileCodec.Encode(data); err != nil {
			return nil, errors.Wrap(err, "could not encode data using file codec")
		}
	}

	return &extensionsv1alpha1.FileContentInline{
		Encoding: encoding,
		Data:     string(data),
	}, nil
}

// Decode decodes a byte slice from the given *extensionsv1alpha1.FileContentInline.
func (c *fileContentInlineCodec) Decode(fci *extensionsv1alpha1.FileContentInline) ([]byte, error) {
	data := []byte(fci.Data)

	// Initialize file codec
	fileCodec, err := getFileCodec(fci.Encoding)
	if err != nil {
		return nil, err
	}

	// Decode data using the file codec, if needed
	if fileCodec != nil {
		if data, err = fileCodec.Decode(data); err != nil {
			return nil, errors.Wrap(err, "could not decode data using file codec")
		}
	}

	return data, nil
}

func getFileCodec(encoding string) (cloudinit.FileCodec, error) {
	if encoding == "" {
		return nil, nil
	}
	fileCodecID, err := cloudinit.ParseFileCodecID(encoding)
	if err != nil {
		return nil, errors.Wrapf(err, "could not parse file codec ID '%s'", encoding)
	}
	return cloudinit.FileCodecForID(fileCodecID), nil
}
