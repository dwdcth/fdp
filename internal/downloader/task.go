package downloader

type Task struct {
	Digest    string
	Size      int64
	MediaType string
	Path      string
	Kind      string
	URL       string
	Headers   []string
}
