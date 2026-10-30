package azure

import (
	"errors"
	"net/http"
)

// These are the disposable project's fixed loopback ports, not cloud endpoints.
const (
	MessagingAddress  = "127.0.0.1:5673"
	managementAddress = "127.0.0.1:5301"
	// The emulator's published development key is not an Azure credential.
	emulatorCredentials = ";SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=SAS_KEY_VALUE;UseDevelopmentEmulator=true;"
)

// The Go administration client constructs HTTPS URLs even in emulator mode.
// Downgrade only this exact loopback authority at the final transport boundary.
// Never install this transport in a cloud client or follow server redirects.
type emulatorTransport struct {
	client *http.Client
}

func newEmulatorTransport() emulatorTransport {
	return emulatorTransport{client: &http.Client{
		Transport: &http.Transport{Proxy: nil},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return errors.New("emulator administration redirects are forbidden")
		},
	}}
}

func (t emulatorTransport) Do(req *http.Request) (*http.Response, error) {
	if req.URL.Scheme != "https" || req.URL.Host != managementAddress || req.URL.User != nil || req.URL.Opaque != "" || req.Method != http.MethodGet {
		return nil, errors.New("administration request is outside the read-only loopback emulator boundary")
	}
	clone := req.Clone(req.Context())
	clone.URL.Scheme = "http"
	return t.client.Do(clone)
}
