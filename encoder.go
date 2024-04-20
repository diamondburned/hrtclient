package hrtclient

import (
	"net/http"

	"libdb.so/hrt"
)

// Encoder describes an encoder that can encode a request.
// It is the inverse counterpart of [hrt.Encoder].
type Encoder interface {
	// Encode encodes the given value into the given request.
	Encode(*http.Request, any) error
}

// MethodEncoder is an encoder that chooses another encoder based on the request
// method. It accepts a wildcard "*" method that is used when no method is found.
type MethodEncoder map[string]Encoder

func (e MethodEncoder) Encode(r *http.Request, v any) error {
	ec, ok := e[r.Method]
	if !ok {
		ec, ok = e["*"]
	}
	if !ok {
		return hrt.NewHTTPError(http.StatusMethodNotAllowed, "method not allowed")
	}
	return ec.Encode(r, v)
}

// TODO: implement URLEncoder
