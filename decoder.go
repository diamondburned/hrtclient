package hrtclient

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"libdb.so/hrt"
)

// Decoder describes a decoder that can decode a response.
// It is the inverse counterpart of [hrt.Decoder].
type Decoder interface {
	// Decode decodes the given value from the given response.
	Decode(*http.Response, any) error
}

// DefaultDecoder is the default decoder to use.
// It is the inverse counterpart of [hrt.DefaultOpts].
var DefaultDecoder = ValidatedDecoder(ErrorHandledDecoder{
	Success: JSONCodec,
	Error:   JSONErrorDecoder("error"),
})

// ErrorHandledDecoder is a decoder that chooses another decoder based on the
// response status code. If the status code matches neither 2xx nor 4xx/5xx,
// then no decoder is used.
type ErrorHandledDecoder struct {
	// Success is the decoder to use for 2xx status codes.
	Success Decoder
	// Error is the decoder to use for 4xx and 5xx status codes.
	Error Decoder
}

func (e ErrorHandledDecoder) Decode(r *http.Response, v any) error {
	return (StatusDecoder{
		Status1xx: NoDecoder,
		Status2xx: e.Success,
		Status3xx: NoDecoder,
		Status4xx: e.Error,
		Status5xx: e.Error,
	}).Decode(r, v)
}

// StatusDecoderKey is the key used to choose a status decoder.
// Normal status codes are used as-is, while special status codes are defined as
// negative constants below.
type StatusDecoderKey int

const (
	// Status1xx is a special status code that matches all 1xx status codes.
	Status1xx StatusDecoderKey = -1
	// Status2xx is a special status code that matches all 2xx status codes.
	Status2xx StatusDecoderKey = -2
	// Status3xx is a special status code that matches all 3xx status codes.
	Status3xx StatusDecoderKey = -3
	// Status4xx is a special status code that matches all 4xx status codes.
	Status4xx StatusDecoderKey = -4
	// Status5xx is a special status code that matches all 5xx status codes.
	Status5xx StatusDecoderKey = -5
)

// Status is an alias for [StatusDecoderKey]. It exists to make it easier to use
// the status decoder with a regular status code.
type Status = StatusDecoderKey

// StatusDecoder is a decoder that chooses another decoder based on the response
// status code.
type StatusDecoder map[StatusDecoderKey]Decoder

func (e StatusDecoder) Decode(r *http.Response, v any) error {
	ec, ok := e[StatusDecoderKey(r.StatusCode)]
	if !ok {
		s := -r.StatusCode / 100
		ec, ok = e[StatusDecoderKey(s)]
	}
	if !ok {
		// Detect if we have a body at all. If we don't, then not having a
		// decoder is acceptable.
		// Ideally, the user should use [NoDecoder], but this is a good
		// fallback.
		b, err := io.ReadAll(r.Body)
		if err != nil {
			return fmt.Errorf("read body error: %w", err)
		}
		if len(b) == 0 {
			return nil
		}
		return fmt.Errorf("no decoder for status code %d (%s)", r.StatusCode, b)
	}
	return ec.Decode(r, v)
}

// NoDecoder is a decoder that does nothing.
var NoDecoder Decoder = noDecoder{}

type noDecoder struct{}

func (noDecoder) Decode(_ *http.Response, _ any) error {
	return nil
}

type validatedDecoder struct{ dec Decoder }

// ValidatedDecoder creates a decoder that validates the response after decoding.
// Types that implement [hrt.Validator] will be validated after decoding.
func ValidatedDecoder(dec Decoder) Decoder {
	return validatedDecoder{dec}
}

func (e validatedDecoder) Decode(r *http.Response, v any) error {
	if err := e.dec.Decode(r, v); err != nil {
		return err
	}

	if validator, ok := v.(hrt.Validator); ok {
		return validator.Validate()
	}

	return nil
}

type textErrorDecoder struct{}

// TextErrorDecoder is a decoder that decodes the body of the response body as
// an error to return.
var TextErrorDecoder Decoder = textErrorDecoder{}

func (textErrorDecoder) Decode(r *http.Response, v any) error {
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("read body error: %w", err)
	}
	return NewHTTPError(r.StatusCode, strings.TrimSpace(string(b)))
}

type jsonErrorDecoder struct{ field string }

// JSONErrorDecoder creates a decoder that decodes the error from the
// given JSON field in the response body. It is the inverse of
// [hrt.JSONErrorWriter].
func JSONErrorDecoder(field string) Decoder {
	return jsonErrorDecoder{field}
}

func (e jsonErrorDecoder) Decode(r *http.Response, v any) error {
	var err struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(r.Body).Decode(&err); err != nil {
		return fmt.Errorf("decode error: %w", err)
	}
	return NewHTTPError(r.StatusCode, err.Error)
}

type stringError string

func (e stringError) Error() string { return string(e) }

// NewHTTPError creates a new HTTPError with the given status code and message.
// It behaves exactly like [hrt.NewHTTPError], except if str begins with
// "{code}: ", then that prefix is removed.
func NewHTTPError(code int, str string) error {
	prefix := fmt.Sprintf("%d: ", code)
	str = strings.TrimPrefix(str, prefix)
	return hrt.NewHTTPError(code, str)
}
