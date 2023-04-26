package output

import (
	"github.com/fluent/fluent-operator/v2/apis/fluentd/v1alpha1/plugins/common"
)

// Http defines the parameters for out_http output plugin
type Http struct {
	// Auth section for this plugin
	*common.Auth `json:"auth,omitempty"`
	// Endpoint defines the endpoint for HTTP request. If you want to use HTTPS, use https prefix.
	Endpoint *string `json:"endpoint,omitempty"`
	// HttpMethod defines the method for HTTP request.
	// +kubebuilder:validation:Enum:=post;put
	HttpMethod *string `json:"httpMethod,omitempty"`
	// Proxy defines the proxy for HTTP request.
	Proxy *string `json:"proxy,omitempty"`
	// ContentType defines Content-Type for HTTP request. out_http automatically set Content-Type for built-in formatters when this parameter is not specified.
	ContentType *string `json:"contentType,omitempty"`
	// JsonArray defines whether to use the array format of JSON or not
	JsonArray *bool `json:"jsonArray,omitempty"`
	// Headers defines the additional headers for HTTP request.
	Headers *string `json:"headers,omitempty"`
	// Additional placeholder based headers for HTTP request. If you want to use tag or record field, use this parameter instead of headers.
	HeadersFromPlaceholders *string `json:"headersFromPlaceholders,omitempty"`
	// OpenTimeout defines the connection open timeout in seconds.
	OpenTimeout *uint16 `json:"openTimeout,omitempty"`
	// ReadTimeout defines the read timeout in seconds.
	ReadTimeout *uint16 `json:"readTimeout,omitempty"`
	// SslTimeout defines the TLS timeout in seconds.
	SslTimeout *uint16 `json:"sslTimeout,omitempty"`
	// TlsCaCertPath defines the CA certificate path for TLS.
	TlsCaCertPath *string `json:"tlsCaCertPath,omitempty"`
	// TlsClientCertPath defines the client certificate path for TLS.
	TlsClientCertPath *string `json:"tlsClientCertPath,omitempty"`
	// TlsPrivateKeyPath defines the client private key path for TLS.
	TlsPrivateKeyPath *string `json:"tlsPrivateKeyPath,omitempty"`
	// TlsPrivateKeyPassphrase defines the client private key passphrase for TLS.
	TlsPrivateKeyPassphrase *string `json:"tlsPrivateKeyPassphrase,omitempty"`
	// TlsVerifyMode defines the verify mode of TLS.
	// +kubebuilder:validation:Enum:=peer;none
	TlsVerifyMode *string `json:"tlsVerifyMode,omitempty"`
	// TlsVersion defines the default version of TLS transport.
	// +kubebuilder:validation:Enum:=TLSv1_1;TLSv1_2
	TlsVersion *string `json:"tlsVersion,omitempty"`
	// TlsCiphers defines the cipher suites configuration of TLS.
	TlsCiphers *string `json:"tlsCiphers,omitempty"`
	// Raise UnrecoverableError when the response code is not SUCCESS.
	ErrorResponseAsUnrecoverable *bool `json:"errorResponseAsUnrecoverable,omitempty"`
	// The list of retryable response codes. If the response code is included in this list, out_http retries the buffer flush.
	RetryableResponseCodes *string `json:"retryableResponseCodes,omitempty"`
}
