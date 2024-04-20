package hrtclient_test

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"

	"github.com/go-chi/chi/v5"
	"libdb.so/hrt"
	"libdb.so/hrtclient"
)

// EchoRequest is the request for the /echo endpoint.
type EchoRequest struct {
	Message string `json:"message"`
}

var _ hrt.Validator = EchoRequest{}

// Validate validates the request.
func (r EchoRequest) Validate() error {
	if r.Message == "" {
		return fmt.Errorf("message is required")
	}
	return nil
}

// EchoResponse is the response for the /echo endpoint.
type EchoResponse struct {
	Message    string `json:"message"`
	ExtraStuff string `json:"extra_stuff"` // from X-Extra-Stuff header
}

// Echo describes the GET /echo endpoint.
var Echo = hrtclient.GET[EchoRequest, EchoResponse]("/echo")

func Example() {
	// Create a new server for our examples.
	server := newServer()
	defer server.Close()

	client := hrtclient.NewClient(server.URL, hrtclient.CombinedCodec{
		Encoder: hrtclient.ValidatedEncoder(hrtclient.JSONCodec),
		Decoder: hrtclient.ValidatedDecoder(hrtclient.StatusDecoder{
			hrtclient.Status2xx: hrtclient.JSONCodec,
			hrtclient.Status4xx: hrtclient.TextErrorDecoder,
			hrtclient.Status5xx: hrtclient.TextErrorDecoder,
		}),
	})

	ctx := context.Background()

	// Inject a header to just this request.
	// You may also use [Client.WithHeader] to inject to all requests.
	ctx = hrtclient.WithHeader(ctx, http.Header{
		"X-Extra-Stuff": {"from-client"},
	})

	resp, err := Echo(ctx, client, EchoRequest{Message: "hello"})
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Printf("%+v\n", resp)
	// Output:
	// {Message:hello ExtraStuff:from-client}
}

// newServer creates a new test server for the examples.
// This server defines our REST API handlers.
func newServer() *httptest.Server {
	r := chi.NewMux()
	r.Use(hrt.Use(hrt.Opts{
		Encoder:     hrt.JSONEncoder,
		ErrorWriter: hrt.JSONErrorWriter("error"),
	}))
	r.Get("/echo", hrt.Wrap(func(ctx context.Context, req EchoRequest) (EchoResponse, error) {
		httpReq := hrt.RequestFromContext(ctx)
		return EchoResponse{
			Message:    req.Message,
			ExtraStuff: httpReq.Header.Get("X-Extra-Stuff"),
		}, nil
	}))

	server := httptest.NewServer(r)
	return server
}
