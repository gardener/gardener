package output

// S3 defines the parameters for out_s3 output plugin
type S3 struct {
	// The AWS access key id.
	AwsKeyId *string `json:"awsKeyId,omitempty"`
	// The AWS secret key.
	AwsSecKey *string `json:"awsSecKey,omitempty"`
	// The Amazon S3 bucket name.
	S3Bucket *string `json:"s3Bucket,omitempty"`
	// The Amazon S3 region name
	S3Region *string `json:"s3Region,omitempty"`
	// The path prefix of the files on S3.
	Path *string `json:"path,omitempty"`
	// The actual S3 path. This is interpolated to the actual path.
	S3ObjectKeyFormat *string `json:"s3ObjectKeyFormat,omitempty"`
	// The compression type.
	// +kubebuilder:validation:Enum:= gzip;lzo;json;txt
	StoreAs *string `json:"storeAs,omitempty"`
	// The proxy URL.
	ProxyUri *string `json:"proxyUri,omitempty"`
	// Verify the SSL certificate of the endpoint.
	SslVerifyPeer *bool `json:"sslVerifyPeer,omitempty"`
}
