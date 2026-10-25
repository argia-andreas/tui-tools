package azure

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"servicebusexplorer/internal/explorer"
)

const cloudTestTenant = "11111111-1111-1111-1111-111111111111"
const cloudTestSub = "22222222-2222-2222-2222-222222222222"
const cloudTestPrincipal = "33333333-3333-3333-3333-333333333333"
const cloudTestARM = "/subscriptions/" + cloudTestSub + "/resourceGroups/group/providers/Microsoft.ServiceBus/namespaces/chosen"

type cloudHTTPFunc func(*http.Request) (*http.Response, error)

func (f cloudHTTPFunc) Do(r *http.Request) (*http.Response, error) { return f(r) }
func cloudResponse(status int, body string) (*http.Response, error) {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
}

func cloudTokenResponse(t *testing.T, claims any) []byte {
	t.Helper()
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	// Deliberately unsigned: production inspects trusted CLI output, not signatures.
	token := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`)) + "." + base64.RawURLEncoding.EncodeToString(payload) + "."
	b, err := json.Marshal(map[string]any{"accessToken": token, "tenant": cloudTestTenant, "expires_on": time.Now().Add(time.Hour).Unix()})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func cloudFakeCommand(t *testing.T, user *string) cloudCommand {
	t.Helper()
	return func(ctx context.Context, args ...string) ([]byte, error) {
		if args[0] != "account" {
			t.Fatalf("unexpected CLI command %v", args)
		}
		switch args[1] {
		case "show":
			return json.Marshal(map[string]any{"tenantId": cloudTestTenant, "user": map[string]string{"name": *user, "type": "user"}})
		case "get-access-token":
			if strings.Join(args[2:6], " ") != "--resource https://management.azure.com --tenant "+cloudTestTenant && strings.Join(args[2:6], " ") != "--resource https://servicebus.azure.net/ --tenant "+cloudTestTenant && strings.Join(args[2:6], " ") != "--resource https://servicebus.azure.net --tenant "+cloudTestTenant {
				t.Fatalf("unscoped token request %v", args)
			}
			return cloudTokenResponse(t, map[string]string{"oid": cloudTestPrincipal, "tid": cloudTestTenant, "aud": args[3]}), nil
		default:
			t.Fatalf("unexpected CLI mutation %v", args)
			return nil, errors.New("unexpected command")
		}
	}
}

func cloudFixture(t *testing.T, f cloudHTTPFunc) *Cloud {
	t.Helper()
	user := "reader@example.test"
	c, err := newCloud(context.Background(), cloudTestTenant, cloudFakeCommand(t, &user), f)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestCloudMetadataPagingAndTenantFilter(t *testing.T) {
	calls := 0
	c := cloudFixture(t, func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Method != "GET" || r.URL.Host != "management.azure.com" || r.Header.Get("Authorization") == "" {
			t.Fatal("not a metadata-only authenticated GET")
		}
		if calls == 1 {
			return cloudResponse(200, `{"value":[{"subscriptionId":"`+cloudTestSub+`","displayName":"one","tenantId":"`+cloudTestTenant+`"}],"nextLink":"https://management.azure.com/subscriptions?api-version=2022-12-01&page=2"}`)
		}
		return cloudResponse(200, `{"value":[{"subscriptionId":"`+cloudTestSub+`","tenantId":"33333333-3333-3333-3333-333333333333"}]}`)
	})
	subs, err := c.Subscriptions(context.Background())
	if err != nil || len(subs) != 1 || calls != 2 {
		t.Fatalf("subscriptions=%v err=%v calls=%d", subs, err, calls)
	}
}

func TestCloudPagingFailsClosed(t *testing.T) {
	for _, next := range []string{"https://evil.test/steal", "https://management.azure.com@evil.test/", "http://management.azure.com/", "https://management.azure.com:443/", "https://management.azure.com/%2e%2e/private", "https://management.azure.com/subscriptions?api-version=2022-12-01"} {
		t.Run(next, func(t *testing.T) {
			calls := 0
			c := cloudFixture(t, func(r *http.Request) (*http.Response, error) {
				calls++
				return cloudResponse(200, `{"value":[],"nextLink":`+fmt.Sprintf("%q", next)+`}`)
			})
			list, err := c.Subscriptions(context.Background())
			if err == nil || list != nil || calls != 1 {
				t.Fatalf("partial/unsafe success %v %v calls=%d", list, err, calls)
			}
		})
	}
	c := cloudFixture(t, func(r *http.Request) (*http.Response, error) { return cloudResponse(200, `{}`) })
	if _, err := c.Subscriptions(context.Background()); err == nil {
		t.Fatal("missing list accepted")
	}
}

func TestCloudPagingStaysInSelectedCollection(t *testing.T) {
	const collection = "https://management.azure.com/subscriptions/" + cloudTestSub + "/providers/Microsoft.ServiceBus/namespaces"
	for _, next := range []string{
		"https://management.azure.com/subscriptions?api-version=2022-12-01",
		"https://management.azure.com/subscriptions/" + cloudTestSub + "/providers/Microsoft.Compute/virtualMachines?api-version=2024-01-01",
		strings.Replace(collection, cloudTestSub, cloudTestPrincipal, 1) + "?api-version=2024-01-01",
		collection + "/chosen?api-version=2024-01-01",
	} {
		t.Run(next, func(t *testing.T) {
			calls := 0
			c := cloudFixture(t, func(r *http.Request) (*http.Response, error) {
				calls++
				if calls != 1 {
					t.Fatal("queried continuation outside the selected collection")
				}
				return cloudResponse(200, `{"value":[{"id":"`+cloudTestARM+`"}],"nextLink":`+fmt.Sprintf("%q", next)+`}`)
			})
			result, err := c.Namespaces(context.Background(), cloudTestSub)
			if err == nil || result != nil || calls != 1 {
				t.Fatalf("partial/unsafe success: %v %v calls=%d", result, err, calls)
			}
		})
	}
	calls := 0
	c := cloudFixture(t, func(r *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return cloudResponse(200, `{"value":[{"id":"`+cloudTestARM+`"}],"nextLink":"`+collection+`?api-version=2024-01-01&$skiptoken=next"}`)
		}
		return cloudResponse(200, `{"value":[]}`)
	})
	result, err := c.Namespaces(context.Background(), cloudTestSub)
	if err != nil || len(result) != 1 || calls != 2 {
		t.Fatalf("valid continuation rejected: %v %v calls=%d", result, err, calls)
	}
}

func TestCloudTokenClaimsPinned(t *testing.T) {
	for name, claims := range map[string]map[string]string{
		"different principal same name": {"oid": cloudTestSub, "tid": cloudTestTenant, "aud": "https://management.azure.com"},
		"different tenant":              {"oid": cloudTestPrincipal, "tid": cloudTestSub, "aud": "https://management.azure.com"},
		"wrong audience":                {"oid": cloudTestPrincipal, "tid": cloudTestTenant, "aud": "https://graph.microsoft.com"},
		"missing oid":                   {"tid": cloudTestTenant, "aud": "https://management.azure.com"},
		"invalid oid":                   {"oid": "not-an-object-id", "tid": cloudTestTenant, "aud": "https://management.azure.com"},
		"missing tid":                   {"oid": cloudTestPrincipal, "aud": "https://management.azure.com"},
		"missing audience":              {"oid": cloudTestPrincipal, "tid": cloudTestTenant},
	} {
		t.Run(name, func(t *testing.T) {
			user := "same@example.test"
			base := cloudFakeCommand(t, &user)
			changed := false
			run := func(ctx context.Context, args ...string) ([]byte, error) {
				if changed && args[1] == "get-access-token" {
					return cloudTokenResponse(t, claims), nil
				}
				return base(ctx, args...)
			}
			c, err := newCloud(context.Background(), cloudTestTenant, run, cloudHTTPFunc(func(*http.Request) (*http.Response, error) {
				t.Fatal("HTTP request after token identity/audience mismatch")
				return nil, nil
			}))
			if err != nil {
				t.Fatal(err)
			}
			profile := c.Profile()
			changed = true
			if token, err := c.credential.GetToken(context.Background(), policy.TokenRequestOptions{Scopes: []string{"https://management.azure.com/.default"}}); err == nil || token.Token != "" {
				t.Fatal("accepted inconsistent token")
			}
			if _, err := c.Subscriptions(context.Background()); err == nil {
				t.Fatal("accepted changed CLI token")
			}
			n := explorer.Namespace{Profile: profile, Name: "chosen", Address: "chosen.servicebus.windows.net"}
			if result := c.DiscoverScope(context.Background(), []explorer.Namespace{n}); len(result) != 1 || result[0].Err == nil {
				t.Fatal("namespace discovery accepted changed CLI token")
			}
			if c.Profile() != profile {
				t.Fatal("existing connection profile silently changed")
			}
			if name != "different principal same name" {
				if _, err := newCloud(context.Background(), cloudTestTenant, run, nil); err == nil {
					t.Fatal("connected with invalid token claims")
				}
				return
			}
			reconnected, err := newCloud(context.Background(), cloudTestTenant, run, nil)
			if err != nil || reconnected.Profile() == profile {
				t.Fatalf("new principal reused saved connection identity: %v", err)
			}
		})
	}
}

func TestCloudProfileUsesImmutableIdentityAndServiceBusScopes(t *testing.T) {
	user := "before@example.test"
	run := cloudFakeCommand(t, &user)
	c, err := newCloud(context.Background(), cloudTestTenant, run, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, scope := range []string{"https://servicebus.azure.net/.default", "https://servicebus.azure.net//.default"} {
		if _, err := c.credential.GetToken(context.Background(), policy.TokenRequestOptions{Scopes: []string{scope}}); err != nil {
			t.Fatalf("supported scope %s rejected: %v", scope, err)
		}
	}
	user = "after@example.test"
	renamed, err := newCloud(context.Background(), cloudTestTenant, run, nil)
	if err != nil || renamed.Profile() != c.Profile() {
		t.Fatalf("same immutable principal changed profile after rename: %v", err)
	}
}

func TestCloudServiceBusTokenRequiresServiceBusAudience(t *testing.T) {
	user := "reader@example.test"
	base := cloudFakeCommand(t, &user)
	run := func(ctx context.Context, args ...string) ([]byte, error) {
		if args[1] == "get-access-token" && strings.HasPrefix(args[3], "https://servicebus.azure.net") {
			return cloudTokenResponse(t, map[string]string{"oid": cloudTestPrincipal, "tid": cloudTestTenant, "aud": "https://management.azure.com"}), nil
		}
		return base(ctx, args...)
	}
	c, err := newCloud(context.Background(), cloudTestTenant, run, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, scope := range []string{"https://servicebus.azure.net/.default", "https://servicebus.azure.net//.default"} {
		if token, err := c.credential.GetToken(context.Background(), policy.TokenRequestOptions{Scopes: []string{scope}}); err == nil || token.Token != "" {
			t.Fatalf("accepted ARM audience for Service Bus scope %s", scope)
		}
	}
}

func TestCloudNamespaceMetadataAndKnownResolution(t *testing.T) {
	calls := 0
	c := cloudFixture(t, func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Host != "management.azure.com" {
			t.Fatal("entity discovery during metadata listing")
		}
		return cloudResponse(200, `{"value":[{"id":"`+cloudTestARM+`"}]}`)
	})
	ns, err := c.Namespaces(context.Background(), cloudTestSub)
	if err != nil || len(ns) != 1 || ns[0].ResourceGroup != "group" || ns[0].AzureSubscriptionID != cloudTestSub || ns[0].Profile != c.Profile() {
		t.Fatalf("metadata %v %v", ns, err)
	}
	n, err := c.ResolveNamespace(context.Background(), "manual.servicebus.windows.net")
	if err != nil || calls != 1 || n.AzureSubscriptionID != "" {
		t.Fatalf("known FQDN caused broad listing: %v %v", n, err)
	}
	for _, bad := range []string{"https://chosen.servicebus.windows.net/", "chosen.servicebus.windows.net.evil.test", cloudTestARM + "/queues/x", strings.Replace(cloudTestARM, "group", "..", 1)} {
		if _, err := c.ResolveNamespace(context.Background(), bad); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
	if _, err := c.Namespaces(context.Background(), "../other"); err == nil {
		t.Fatal("subscription injection accepted")
	}
}

func TestCloudARMIdentityAndResourceMapping(t *testing.T) {
	c := cloudFixture(t, func(r *http.Request) (*http.Response, error) {
		return cloudResponse(200, `{"id":"`+strings.Replace(cloudTestARM, "group", "moved", 1)+`"}`)
	})
	if _, err := c.ResolveNamespace(context.Background(), cloudTestARM); err == nil {
		t.Fatal("changed ARM identity accepted")
	}
	r := explorer.Resource{Profile: c.Profile(), Namespace: "chosen.servicebus.windows.net", Queue: "queue", AzureSubscriptionID: cloudTestSub, ResourceGroup: "group"}
	if _, err := c.Counts(context.Background(), r); err == nil {
		t.Fatal("counts accepted moved namespace")
	}
	if _, err := c.Peek(context.Background(), r, false, 0, 1); err == nil {
		t.Fatal("peek accepted moved namespace")
	}
}

const cloudEmptyFeed = `<feed xmlns="http://www.w3.org/2005/Atom"></feed>`

func TestCloudPeekMessageRejectsInvalidIdentityAndCancellationBeforeIO(t *testing.T) {
	c := cloudFixture(t, func(*http.Request) (*http.Response, error) {
		t.Fatal("unexpected HTTP request")
		return nil, nil
	})
	c.credential.run = func(context.Context, ...string) ([]byte, error) {
		t.Fatal("invalid full peek reached credential validation")
		return nil, nil
	}
	valid := explorer.Resource{Profile: c.Profile(), Namespace: "chosen.servicebus.windows.net", Queue: "queue"}
	for _, dlq := range []bool{false, true} {
		for _, test := range []struct {
			name   string
			change func(*explorer.Resource)
		}{
			{"profile", func(r *explorer.Resource) { r.Profile = "other" }},
			{"namespace", func(r *explorer.Resource) { r.Namespace = "evil.test" }},
			{"queue path", func(r *explorer.Resource) { r.Queue = "../queue" }},
			{"mixed entity", func(r *explorer.Resource) { r.Topic = "topic"; r.Subscription = "sub" }},
			{"missing subscription", func(r *explorer.Resource) { r.Queue = ""; r.Topic = "topic" }},
		} {
			t.Run(fmt.Sprintf("%s/dlq=%t", test.name, dlq), func(t *testing.T) {
				r := valid
				test.change(&r)
				message, err := c.PeekMessage(context.Background(), r, dlq, 123)
				if err == nil || message.Body != nil || message.ID != "" {
					t.Fatalf("invalid identity returned content: %+v %v", message, err)
				}
			})
		}
		if _, err := c.PeekMessage(context.Background(), valid, dlq, -1); err == nil {
			t.Fatal("negative full peek sequence accepted")
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := c.PeekMessage(ctx, valid, dlq, 123); !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled full peek reached IO: %v", err)
		}
	}
	if len(c.clients) != 0 {
		t.Fatal("invalid full peek created messaging clients")
	}
}

func TestCloudPeekMessageRejectsMovedManagementIdentity(t *testing.T) {
	for _, metadata := range []string{
		`{"id":"` + strings.Replace(cloudTestARM, "group", "moved", 1) + `"}`,
		`{"id":"` + strings.Replace(cloudTestARM, cloudTestSub, cloudTestPrincipal, 1) + `"}`,
		`{"id":"` + strings.Replace(cloudTestARM, "chosen", "another", 1) + `"}`,
		`{"id":"` + cloudTestARM + `","properties":{"serviceBusEndpoint":"https://another.servicebus.windows.net/"}}`,
	} {
		for _, dlq := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/dlq=%t", metadata, dlq), func(t *testing.T) {
				calls := 0
				c := cloudFixture(t, func(r *http.Request) (*http.Response, error) {
					calls++
					if r.Method != http.MethodGet || r.URL.Host != "management.azure.com" || r.URL.Path != cloudTestARM {
						t.Fatalf("unexpected validation request: %s %s", r.Method, r.URL)
					}
					return cloudResponse(200, metadata)
				})
				r := explorer.Resource{Profile: c.Profile(), Namespace: "chosen.servicebus.windows.net", Topic: "topic", Subscription: "sub", AzureSubscriptionID: cloudTestSub, ResourceGroup: "group"}
				message, err := c.PeekMessage(context.Background(), r, dlq, 123)
				if err == nil || message.Body != nil || calls != 1 || len(c.clients) != 0 {
					t.Fatalf("moved namespace reached messaging: %+v err=%v calls=%d clients=%d", message, err, calls, len(c.clients))
				}
			})
		}
	}
}

func TestCloudPeekMessageRejectsChangedCredentialAndSanitizesErrors(t *testing.T) {
	user := "reader@example.test"
	c, err := newCloud(context.Background(), cloudTestTenant, cloudFakeCommand(t, &user), cloudHTTPFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("unexpected HTTP request")
		return nil, nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	r := explorer.Resource{Profile: c.Profile(), Namespace: "chosen.servicebus.windows.net", Queue: "queue"}
	user = "changed@example.test"
	if _, err := c.PeekMessage(context.Background(), r, false, 123); err == nil {
		t.Fatal("changed CLI identity accepted for full peek")
	}
	c.credential.run = func(context.Context, ...string) ([]byte, error) {
		return []byte("secret-output"), errors.New("secret-error")
	}
	if _, err := c.PeekMessage(context.Background(), r, true, 123); err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatalf("full peek leaked raw CLI failure: %v", err)
	}
	if len(c.clients) != 0 {
		t.Fatal("failed credential validation reached messaging")
	}
}

func TestCloudExplicitTargetsPartialRoleFailure(t *testing.T) {
	var hosts []string
	c := cloudFixture(t, func(r *http.Request) (*http.Response, error) {
		hosts = append(hosts, r.URL.Host)
		if r.URL.Host == "denied.servicebus.windows.net" {
			return cloudResponse(403, "sensitive failure details")
		}
		if r.URL.Host != "chosen.servicebus.windows.net" {
			t.Fatalf("unselected namespace %s", r.URL.Host)
		}
		return cloudResponse(200, cloudEmptyFeed)
	})
	n := explorer.Namespace{Profile: c.Profile(), Name: "chosen", Address: "chosen.servicebus.windows.net"}
	denied := n
	denied.Name = "denied"
	denied.Address = "denied.servicebus.windows.net"
	wrong := n
	wrong.Profile = "other"
	results := c.DiscoverScope(context.Background(), []explorer.Namespace{n, denied, wrong})
	if len(results) != 3 || results[0].Err != nil || results[1].Err == nil || results[2].Err == nil {
		t.Fatalf("bad per-target results %+v", results)
	}
	if strings.Contains(results[1].Err.Error(), "sensitive") || len(hosts) != 3 {
		t.Fatalf("unsafe error or incorrect request count: %v %v", results, hosts)
	}
	if out := c.DiscoverScope(context.Background(), nil); len(out) != 0 || len(hosts) != 3 {
		t.Fatal("empty scope enumerated")
	}
}

func TestCloudIdentityPinnedAndErrorsSanitized(t *testing.T) {
	user := "one@example.test"
	c, err := newCloud(context.Background(), cloudTestTenant, cloudFakeCommand(t, &user), cloudHTTPFunc(func(*http.Request) (*http.Response, error) { t.Fatal("request after identity switch"); return nil, nil }))
	if err != nil {
		t.Fatal(err)
	}
	profile := c.Profile()
	user = "two@example.test"
	if _, err := c.Subscriptions(context.Background()); err == nil {
		t.Fatal("identity switch accepted")
	}
	if c.Profile() != profile {
		t.Fatal("profile changed silently")
	}
	c.credential.run = func(context.Context, ...string) ([]byte, error) {
		return []byte("secret-output"), errors.New("secret-error")
	}
	_, err = c.credential.GetToken(context.Background(), policy.TokenRequestOptions{Scopes: []string{"https://management.azure.com/.default"}})
	if err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatalf("raw CLI failure leaked: %v", err)
	}
}

func TestCloudReadOnlyAndInvalidResources(t *testing.T) {
	c := cloudFixture(t, func(*http.Request) (*http.Response, error) { t.Fatal("unexpected HTTP request"); return nil, nil })
	ctx := context.Background()
	r := explorer.Resource{Profile: c.Profile(), Namespace: "evil.test", Queue: "q"}
	if _, err := c.Discover(ctx); err == nil {
		t.Fatal("global discovery enabled")
	}
	if _, err := c.ReceiveNextDLQ(ctx, r); err == nil {
		t.Fatal("receive enabled")
	}
	if _, err := c.PrepareDLQCopy(ctx, r, 1); err == nil {
		t.Fatal("copy enabled")
	}
	if _, err := c.ReceiveSelectedDLQ(ctx, r, 1); err == nil {
		t.Fatal("selected receive enabled")
	}
	if _, err := c.Counts(ctx, r); err == nil {
		t.Fatal("invalid counts accepted")
	}
	if _, err := c.Peek(ctx, r, false, 0, 1); err == nil {
		t.Fatal("invalid peek accepted")
	}
	if err := c.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := c.messaging("chosen.servicebus.windows.net"); err == nil {
		t.Fatal("opened client after close")
	}
}

func TestCloudCountPresenceGuard(t *testing.T) {
	for _, details := range []string{"", `<CountDetails><ActiveMessageCount>0</ActiveMessageCount></CountDetails>`, `<CountDetails><ActiveMessageCount>0</ActiveMessageCount><DeadLetterMessageCount>0</DeadLetterMessageCount></CountDetails>`} {
		t.Run(details, func(t *testing.T) {
			c := cloudFixture(t, func(*http.Request) (*http.Response, error) {
				return cloudResponse(200, `<entry xmlns="http://www.w3.org/2005/Atom"><content type="application/xml"><QueueDescription xmlns="http://schemas.microsoft.com/netservices/2010/10/servicebus/connect"><CreatedAt>2026-01-01T00:00:00Z</CreatedAt><UpdatedAt>2026-01-01T00:00:00Z</UpdatedAt><AccessedAt>2026-01-01T00:00:00Z</AccessedAt>`+details+`</QueueDescription></content></entry>`)
			})
			counts, err := c.Counts(context.Background(), explorer.Resource{Profile: c.Profile(), Namespace: "chosen.servicebus.windows.net", Queue: "queue"})
			complete := strings.Contains(details, "DeadLetterMessageCount")
			if complete && (err != nil || counts.State != "available") {
				t.Fatalf("complete zero counts unavailable: %+v %v", counts, err)
			}
			if !complete && (err == nil || counts.State != "unavailable") {
				t.Fatalf("missing counts became zeros: %+v %v", counts, err)
			}
		})
	}
}

func TestCloudSDKDiscoveryPagingPreservesManagementIdentity(t *testing.T) {
	pages := map[string]int{}
	c := cloudFixture(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "management.azure.com" {
			return cloudResponse(200, `{"id":"`+cloudTestARM+`"}`)
		}
		pages[r.URL.Path]++
		if pages[r.URL.Path] == 2 {
			return cloudResponse(200, cloudEmptyFeed)
		}
		var name, description string
		switch r.URL.Path {
		case "/$Resources/Queues":
			name, description = "queue", "QueueDescription"
		case "/$Resources/Topics":
			name, description = "topic", "TopicDescription"
		case "/topic/Subscriptions":
			name, description = "subscriber", "SubscriptionDescription"
		default:
			t.Fatalf("unexpected SDK path %s", r.URL.Path)
		}
		return cloudResponse(200, `<feed xmlns="http://www.w3.org/2005/Atom"><entry><title>`+name+`</title><content type="application/xml"><`+description+` xmlns="http://schemas.microsoft.com/netservices/2010/10/servicebus/connect"></`+description+`></content></entry></feed>`)
	})
	n, err := c.ResolveNamespace(context.Background(), cloudTestARM)
	if err != nil {
		t.Fatal(err)
	}
	results := c.DiscoverScope(context.Background(), []explorer.Namespace{n})
	if len(results) != 1 || results[0].Err != nil || len(results[0].Resources) != 3 {
		t.Fatalf("discovery %+v", results)
	}
	for _, r := range results[0].Resources {
		if r.Profile != c.Profile() || r.AzureSubscriptionID != cloudTestSub || r.ResourceGroup != "group" {
			t.Fatalf("management identity lost: %+v", r)
		}
	}
	for path, count := range pages {
		if count != 2 {
			t.Fatalf("did not exhaust %s: %d", path, count)
		}
	}
	if results[0].Resources[2].Subscription != "subscriber" {
		t.Fatal("topic subscription identity lost")
	}
}
