package handlers

type CachedResponse struct {
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
}
