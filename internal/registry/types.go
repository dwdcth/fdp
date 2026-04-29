package registry

type Platform struct {
	OS           string
	Architecture string
	Variant      string
}

type BlobRequest struct {
	Digest string
	Kind   string
}
