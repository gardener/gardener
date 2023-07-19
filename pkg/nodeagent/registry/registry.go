package registry

type Extractor interface {
	ExtractFromLayer(image, pathSuffix, dest string) error
}

func NewExtractor() Extractor {
	return remoteExtractor{}
}
