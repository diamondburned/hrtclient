package hrtclient_test

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/alecthomas/assert/v2"
	"github.com/go-chi/chi/v5"
	"libdb.so/hrt"
	"libdb.so/hrtclient"
)

func newTestClient(t *testing.T) *hrtclient.Client {
	r := chi.NewMux()
	r.Use(hrt.Use(hrt.Opts{
		Encoder:     hrt.JSONEncoder,
		ErrorWriter: hrt.TextErrorWriter,
	}))
	r.Get("/echo", hrt.Wrap(func(ctx context.Context, r EchoRequest) (EchoResponse, error) {
		return EchoResponse{Message: r.Message}, nil
	}))
	r.Get("/error/400", hrt.Wrap(func(ctx context.Context, _ hrt.None) (hrt.None, error) {
		return hrt.Empty, hrt.NewHTTPError(400, "bad request")
	}))
	r.Get("/error/500", hrt.Wrap(func(ctx context.Context, _ hrt.None) (hrt.None, error) {
		return hrt.Empty, hrt.NewHTTPError(500, "internal server error")
	}))

	server := httptest.NewServer(r)
	t.Cleanup(server.Close)

	return hrtclient.NewClient(server.URL, hrtclient.CombinedCodec{
		Encoder: hrtclient.ValidatedEncoder(hrtclient.JSONCodec),
		Decoder: hrtclient.ValidatedDecoder(hrtclient.StatusDecoder{
			hrtclient.Status2xx: hrtclient.JSONCodec,
			hrtclient.Status4xx: hrtclient.TextErrorDecoder,
			hrtclient.Status5xx: hrtclient.TextErrorDecoder,
		}),
	})
}

func TestClient(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	t.Run("echo", func(t *testing.T) {
		var out EchoResponse
		err := client.Do(ctx, "GET", "/echo", EchoRequest{"hi"}, &out)
		assert.NoError(t, err)
		assert.Equal(t, EchoResponse{Message: "hi"}, out)

		err = client.Do(ctx, "GET", "/echo", EchoRequest{""}, &out)
		assert.Error(t, err)
	})

	t.Run("error/400", func(t *testing.T) {
		err := client.Do(ctx, "GET", "/error/400", nil, nil)
		assert.Error(t, err)
		assert.Equal(t, "400: bad request", err.Error())
	})

	t.Run("error/500", func(t *testing.T) {
		err := client.Do(ctx, "GET", "/error/500", nil, nil)
		assert.Error(t, err)
		assert.Equal(t, "500: internal server error", err.Error())
	})
}

func TestEndpoint(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	echo := hrtclient.GET[EchoRequest, EchoResponse]("/echo")
	echoPtr := hrtclient.GET[*EchoRequest, *EchoResponse]("/echo")
	error400 := hrtclient.GET[hrt.None, hrt.None]("/error/400")
	error500 := hrtclient.GET[hrt.None, hrt.None]("/error/500")

	t.Run("echo", func(t *testing.T) {
		resp, err := echo(ctx, client, EchoRequest{"hi"})
		assert.NoError(t, err)
		assert.Equal(t, EchoResponse{Message: "hi"}, resp)

		resp, err = echo(ctx, client, EchoRequest{""})
		assert.Error(t, err)
	})

	t.Run("echoPtr", func(t *testing.T) {
		resp, err := echoPtr(ctx, client, &EchoRequest{"hi"})
		assert.NoError(t, err)
		assert.Equal(t, &EchoResponse{Message: "hi"}, resp)

		resp, err = echoPtr(ctx, client, &EchoRequest{""})
		assert.Error(t, err)
	})

	t.Run("error/400", func(t *testing.T) {
		_, err := error400(ctx, client, hrt.Empty)
		assert.Error(t, err)
		assert.Equal(t, "400: bad request", err.Error())
	})

	t.Run("error/500", func(t *testing.T) {
		_, err := error500(ctx, client, hrt.Empty)
		assert.Error(t, err)
		assert.Equal(t, "500: internal server error", err.Error())
	})
}
