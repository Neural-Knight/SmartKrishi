package tools

import (
	"encoding/json"
	"fmt"
	"io"
)

// httpStatusError is returned when a tool's HTTP call has a non-2xx status.
type httpStatusError struct {
	Code int
	URL  string
}

func (e *httpStatusError) Error() string {
	return fmt.Sprintf("http status %d from %s", e.Code, e.URL)
}

func decodeJSON(r io.Reader, into any) error {
	return json.NewDecoder(r).Decode(into)
}
