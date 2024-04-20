package hrtclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Codec combines an encoder and decoder into one.
type Codec interface {
	Encoder
	Decoder
}

// CombinedCodec combines an encoder and decoder pair into one.
type CombinedCodec struct {
	Encoder
	Decoder
}

var _ Codec = CombinedCodec{}

type jsonCodec struct{}

// JSONCodec is an encoder that encodes and decodes JSON.
// It is the inverse counterpart of [hrt.JSONEncoder].
var JSONCodec Codec = jsonCodec{}

func (e jsonCodec) Encode(r *http.Request, v any) error {
	// TODO: care more and use io.Pipe
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("encode error: %w", err)
	}

	r.Header.Set("Content-Type", "application/json")
	r.ContentLength = int64(len(b))
	r.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(b)), nil
	}
	r.Body, _ = r.GetBody()

	return nil
}

func (e jsonCodec) Decode(r *http.Response, v any) error {
	contentType := r.Header.Get("Content-Type")
	if contentType != "" && contentType != "application/json" {
		return fmt.Errorf("unexpected content type: %q", contentType)
	}

	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return fmt.Errorf("decode error: %w", err)
	}

	return nil
}
