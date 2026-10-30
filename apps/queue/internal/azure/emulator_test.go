package azure

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestEmulatorTransportBoundary(t *testing.T) {
	for _, target := range []string{
		"https://namespace.servicebus.windows.net/queue",
		"https://127.0.0.1:5300/queue", "https://localhost:5301/queue",
		"http://127.0.0.1:5301/queue", "https://127.0.0.1.evil.test:5301/queue",
		"https://user@127.0.0.1:5301/queue",
	} {
		t.Run(target, func(t *testing.T) {
			transport := newEmulatorTransport()
			transport.client.Transport = transportFunc(func(*http.Request) (*http.Response, error) {
				t.Fatal("an out-of-boundary request reached the network transport")
				return nil, nil
			})
			req, err := http.NewRequest(http.MethodGet, target, nil)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := transport.Do(req); err == nil {
				t.Fatal("unsafe administration target accepted")
			}
		})
	}
	transport := newEmulatorTransport()
	transport.client.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != "http://127.0.0.1:5301/queues?api-version=2024-05" || r.Header.Get("Authorization") != "test-signature" {
			t.Fatalf("signed request path, query or authorization changed: %s", r.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("ok")), Header: make(http.Header)}, nil
	})
	req, _ := http.NewRequest(http.MethodGet, "https://127.0.0.1:5301/queues?api-version=2024-05", nil)
	req.Header.Set("Authorization", "test-signature")
	response, err := transport.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if req.URL.Scheme != "https" {
		t.Fatal("transport mutated the SDK request")
	}
	req.Method = http.MethodDelete
	if _, err := transport.Do(req); err == nil {
		t.Fatal("read-only transport accepted a write")
	}

	transport = newEmulatorTransport()
	transport.client.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != managementAddress {
			t.Fatal("redirect escaped emulator boundary")
		}
		return &http.Response{StatusCode: 302, Header: http.Header{"Location": {"https://external.invalid/"}}, Body: io.NopCloser(strings.NewReader(""))}, nil
	})
	req.Method = http.MethodGet
	response, err = transport.Do(req)
	if response != nil {
		response.Body.Close()
	}
	if err == nil {
		t.Fatal("emulator redirect accepted")
	}
}
