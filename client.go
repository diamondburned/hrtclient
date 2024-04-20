// Package hrtclient makes interacting with hrt-defined APIs easier.
package hrtclient

import (
	"context"
	"fmt"
	"net/http"
	"reflect"

	"libdb.so/ctxt"
	"libdb.so/hrt"
)

// Client is a higher-level wrapper around [http.Client] that provides encoding
// and decoding of data.
type Client struct {
	client  *http.Client
	header  http.Header
	codec   Codec
	baseURL string
}

// NewClient creates a new client with the given base URL and codec.
// The codec will be used to encode and decode data.
// [http.DefaultClient] is used as the default HTTP client. To use a custom
// client, use [NewCustomClient].
func NewClient(baseURL string, codec Codec) *Client {
	return NewCustomClient(baseURL, codec, nil)
}

// NewCustomClient is like [NewClient], but allows you to specify a custom HTTP
// client.
func NewCustomClient(baseURL string, codec Codec, client *http.Client) *Client {
	if client == nil {
		client = http.DefaultClient
	}
	return &Client{
		client:  client,
		codec:   codec,
		baseURL: baseURL,
	}
}

// WithHeader returns a new client with the given headers.
// Existing headers are overridden.
func (c *Client) WithHeader(header http.Header) *Client {
	cc := *c
	if cc.header != nil {
		cc.header = cc.header.Clone()
		for k, v := range header {
			cc.header[k] = v
		}
	} else {
		cc.header = header
	}
	return &cc
}

// Do performs the request with the given method and URL. If requestIn is not nil,
// it is encoded into the request. If responseOut is not nil, it is decoded into
// the response.
func (c *Client) Do(ctx context.Context, method, path string, requestIn, responseOut any) error {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	if requestIn != nil {
		if err := c.codec.Encode(req, requestIn); err != nil {
			return err
		}
	}

	for k, v := range c.header {
		req.Header[k] = v
	}

	h := ctxt.FromOr(ctx, contextHeader{})
	for k, v := range h.h {
		req.Header[k] = v
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if err := c.codec.Decode(resp, responseOut); err != nil {
		return err
	}

	return nil
}

type contextHeader struct {
	h http.Header
}

// WithHeader adds the given HTTP headers into the context to be used in
// [Client.Do]. Headers specified here will override any headers set in the
// context.
func WithHeader(ctx context.Context, header http.Header) context.Context {
	ch := ctxt.FromOr(ctx, contextHeader{})
	if ch.h == nil {
		ch.h = header
	} else {
		ch.h = ch.h.Clone()
		for k, v := range header {
			ch.h[k] = v
		}
	}
	return ctxt.With(ctx, ch)
}

// DoFunc describes a function that accepts a [Client] and a request value
// and returns a response value or an error.
type DoFunc[ReqT, RespT any] func(ctx context.Context, client *Client, in ReqT) (RespT, error)

// Endpoint describes an API endpoint along with its input and output types.
// Endpoint defines a new [Endpoint].
func Endpoint[ReqT, RespT any](method, path string) DoFunc[ReqT, RespT] {
	var newValue func() any
	var indirect bool

	if _, isNone := any(zero[RespT]()).(hrt.None); !isNone {
		t := reflect.TypeFor[RespT]()
		if t.Kind() == reflect.Pointer {
			// Special case: if RespT = *T, then we create an instance of T and
			// take the pointer of that.
			newValue = func() any { return reflect.New(t.Elem()).Interface() }
		} else {
			// Otherwise, we create a new pointer to a RespT.
			newValue = func() any { return reflect.New(t).Interface() }
			indirect = true
		}
	}

	return func(ctx context.Context, client *Client, in ReqT) (RespT, error) {
		var out any
		if newValue != nil {
			out = newValue()
		}

		err := client.Do(ctx, method, path, in, out)

		if out == nil {
			return zero[RespT](), err
		}

		if indirect {
			return *(out.(*RespT)), err
		}
		return out.(RespT), err
	}
}

// GET is a shorthand for [Endpoint] with method "GET".
func GET[ReqT, RespT any](path string) DoFunc[ReqT, RespT] {
	return Endpoint[ReqT, RespT]("GET", path)
}

// POST is a shorthand for [Endpoint] with method "POST".
func POST[ReqT, RespT any](path string) DoFunc[ReqT, RespT] {
	return Endpoint[ReqT, RespT]("POST", path)
}

// PUT is a shorthand for [Endpoint] with method "PUT".
func PUT[ReqT, RespT any](path string) DoFunc[ReqT, RespT] {
	return Endpoint[ReqT, RespT]("PUT", path)
}

// DELETE is a shorthand for [Endpoint] with method "DELETE".
func DELETE[ReqT, RespT any](path string) DoFunc[ReqT, RespT] {
	return Endpoint[ReqT, RespT]("DELETE", path)
}

// PATCH is a shorthand for [Endpoint] with method "PATCH".
func PATCH[ReqT, RespT any](path string) DoFunc[ReqT, RespT] {
	return Endpoint[ReqT, RespT]("PATCH", path)
}

func zero[T any]() T {
	var z T
	return z
}
