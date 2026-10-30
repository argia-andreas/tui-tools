package azure

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	sb "github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus/admin"
	"servicebusexplorer/internal/explorer"
)

type cloudCommand func(context.Context, ...string) ([]byte, error)
type cloudHTTP interface {
	Do(*http.Request) (*http.Response, error)
}

// Cloud browses explicitly selected namespaces using an existing Azure CLI login.
// It never logs in, changes accounts, or exposes a cloud mutation capability.
type Cloud struct {
	credential *cliCredential
	http       cloudHTTP
	mu         sync.Mutex
	clients    map[string]*sb.Client
	closed     bool
}

type cliAccount struct {
	Tenant string `json:"tenantId"`
	User   struct {
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"user"`
}
type cliCredential struct {
	run       cloudCommand
	account   cliAccount
	principal string
}

var cloudGUID = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
var cloudName = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9-]{4,48}[a-zA-Z0-9]$`)
var cloudGroup = regexp.MustCompile(`^[a-zA-Z0-9_().-]{1,90}$`)

// NewCloud pins the tenant and principal of the current explicit Azure CLI login.
func NewCloud(ctx context.Context, tenant string) (*Cloud, error) {
	return newCloud(ctx, tenant, func(ctx context.Context, args ...string) ([]byte, error) {
		return exec.CommandContext(ctx, "az", args...).Output()
	}, &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("redirect refused") }})
}

func newCloud(ctx context.Context, tenant string, run cloudCommand, transport cloudHTTP) (*Cloud, error) {
	if !cloudGUID.MatchString(tenant) {
		return nil, errors.New("an explicit Azure tenant ID is required")
	}
	a, err := readCLIAccount(ctx, run)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(a.Tenant, tenant) {
		return nil, errors.New("Azure CLI tenant does not match requested tenant")
	}
	credential := &cliCredential{run: run, account: a}
	_, principal, err := credential.token(ctx, "https://management.azure.com/.default")
	if err != nil {
		return nil, err
	}
	credential.principal = principal
	return &Cloud{credential: credential, http: transport, clients: make(map[string]*sb.Client)}, nil
}

func readCLIAccount(ctx context.Context, run cloudCommand) (cliAccount, error) {
	var a cliAccount
	b, err := run(ctx, "account", "show", "--output", "json", "--only-show-errors")
	if err != nil || json.Unmarshal(b, &a) != nil || !cloudGUID.MatchString(a.Tenant) || a.User.Name == "" || a.User.Type == "" {
		return a, errors.New("cannot read existing Azure CLI login")
	}
	return a, nil
}

func (c *cliCredential) check(ctx context.Context) error {
	_, err := c.GetToken(ctx, policy.TokenRequestOptions{Scopes: []string{"https://management.azure.com/.default"}})
	return err
}

func (c *cliCredential) checkAccount(ctx context.Context) error {
	a, err := readCLIAccount(ctx, c.run)
	if err != nil {
		return err
	}
	if !strings.EqualFold(a.Tenant, c.account.Tenant) || !strings.EqualFold(a.User.Name, c.account.User.Name) || a.User.Type != c.account.User.Type {
		return errors.New("Azure CLI identity changed; reconnect explicitly")
	}
	return nil
}

func (c *cliCredential) GetToken(ctx context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error) {
	if len(opts.Scopes) != 1 || (opts.Scopes[0] != "https://management.azure.com/.default" && opts.Scopes[0] != "https://servicebus.azure.net/.default" && opts.Scopes[0] != "https://servicebus.azure.net//.default") {
		return azcore.AccessToken{}, errors.New("unsupported Azure token scope")
	}
	token, principal, err := c.token(ctx, opts.Scopes[0])
	if err != nil {
		return azcore.AccessToken{}, err
	}
	if c.principal == "" || !strings.EqualFold(principal, c.principal) {
		return azcore.AccessToken{}, errors.New("Azure CLI principal changed; reconnect explicitly")
	}
	return token, nil
}

func (c *cliCredential) token(ctx context.Context, scope string) (azcore.AccessToken, string, error) {
	if err := c.checkAccount(ctx); err != nil {
		return azcore.AccessToken{}, "", err
	}
	resource := strings.TrimSuffix(scope, "/.default")
	b, err := c.run(ctx, "account", "get-access-token", "--resource", resource, "--tenant", c.account.Tenant, "--output", "json", "--only-show-errors")
	var token struct {
		Token   string      `json:"accessToken"`
		Expires json.Number `json:"expires_on"`
		Tenant  string      `json:"tenant"`
	}
	if err != nil || json.Unmarshal(b, &token) != nil || token.Token == "" || !strings.EqualFold(token.Tenant, c.account.Tenant) {
		return azcore.AccessToken{}, "", errors.New("cannot obtain Azure CLI access token")
	}
	// The CLI is the trusted token source. Inspect claims only for consistency;
	// this is not signature validation. Azure still validates the token itself.
	parts := strings.Split(token.Token, ".")
	var claims struct {
		OID      string `json:"oid"`
		TID      string `json:"tid"`
		Audience string `json:"aud"`
	}
	if len(parts) != 3 {
		return azcore.AccessToken{}, "", errors.New("invalid Azure CLI token claims")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || json.Unmarshal(payload, &claims) != nil || !cloudGUID.MatchString(claims.OID) || !strings.EqualFold(claims.TID, c.account.Tenant) || strings.TrimSuffix(claims.Audience, "/") != strings.TrimSuffix(resource, "/") {
		return azcore.AccessToken{}, "", errors.New("Azure CLI token identity or audience mismatch")
	}
	seconds, err := token.Expires.Int64()
	if err != nil || seconds <= time.Now().Unix() {
		return azcore.AccessToken{}, "", errors.New("Azure CLI returned an invalid token expiry")
	}
	if err := c.checkAccount(ctx); err != nil {
		return azcore.AccessToken{}, "", err
	}
	return azcore.AccessToken{Token: token.Token, ExpiresOn: time.Unix(seconds, 0)}, claims.OID, nil
}

// Profile is a stable, nonsecret connection identity, independent of subscription selection.
func (c *Cloud) Profile() string {
	a := c.credential.account
	return fmt.Sprintf("azure-cli:%x", sha256.Sum256([]byte(strings.ToLower(a.Tenant+"/"+c.credential.principal))))
}

// ConnectionLabel describes the pinned account and tenant, not an access token.
func (c *Cloud) ConnectionLabel() string {
	return c.credential.account.User.Name + " / " + c.credential.account.Tenant
}

func cloudAddress(address string) bool {
	const suffix = ".servicebus.windows.net"
	return strings.HasSuffix(address, suffix) && cloudName.MatchString(strings.TrimSuffix(address, suffix))
}

func cloudARM(id string) (explorer.Namespace, error) {
	p := strings.Split(id, "/")
	if len(p) != 9 || p[0] != "" || !strings.EqualFold(p[1], "subscriptions") || !cloudGUID.MatchString(p[2]) || !strings.EqualFold(p[3], "resourceGroups") || !cloudGroup.MatchString(p[4]) || p[4] == "." || p[4] == ".." || !strings.EqualFold(p[5], "providers") || !strings.EqualFold(p[6], "Microsoft.ServiceBus") || !strings.EqualFold(p[7], "namespaces") || !cloudName.MatchString(p[8]) {
		return explorer.Namespace{}, errors.New("invalid Service Bus namespace ARM identity")
	}
	return explorer.Namespace{Address: strings.ToLower(p[8]) + ".servicebus.windows.net", Name: p[8], AzureSubscriptionID: p[2], ResourceGroup: p[4]}, nil
}

func namespaceARM(n explorer.Namespace) string {
	return "/subscriptions/" + n.AzureSubscriptionID + "/resourceGroups/" + n.ResourceGroup + "/providers/Microsoft.ServiceBus/namespaces/" + n.Name
}

func managementURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host != "management.azure.com" || u.User != nil || u.Fragment != "" || u.RawPath != "" || strings.ContainsAny(u.Path, "\\%") {
		return false
	}
	for _, segment := range strings.Split(u.Path, "/") {
		if segment == "." || segment == ".." {
			return false
		}
	}
	return true
}

func (c *Cloud) get(ctx context.Context, endpoint string, out any) error {
	if !managementURL(endpoint) {
		return errors.New("invalid Azure management endpoint")
	}
	token, err := c.credential.GetToken(ctx, policy.TokenRequestOptions{Scopes: []string{"https://management.azure.com/.default"}})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return errors.New("invalid Azure metadata request")
	}
	req.Header.Set("Authorization", "Bearer "+token.Token)
	resp, err := c.http.Do(req)
	if err != nil {
		return errors.New("Azure metadata request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.New("Azure metadata unavailable; check tenant and read permissions")
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(out) != nil {
		return errors.New("invalid Azure metadata response")
	}
	return nil
}

func (c *Cloud) list(ctx context.Context, endpoint string) ([]json.RawMessage, error) {
	if !managementURL(endpoint) {
		return nil, errors.New("invalid Azure management endpoint")
	}
	collection, _ := url.Parse(endpoint)
	var all []json.RawMessage
	seen := map[string]bool{}
	for endpoint != "" {
		pageURL, err := url.Parse(endpoint)
		if err != nil || !managementURL(endpoint) || pageURL.Path != collection.Path {
			return nil, errors.New("Azure metadata paging left the selected collection")
		}
		if seen[endpoint] {
			return nil, errors.New("Azure metadata paging cycle")
		}
		seen[endpoint] = true
		var page struct {
			Value *[]json.RawMessage `json:"value"`
			Next  string             `json:"nextLink"`
		}
		if err := c.get(ctx, endpoint, &page); err != nil {
			return nil, err
		}
		if page.Value == nil {
			return nil, errors.New("incomplete Azure metadata response")
		}
		all = append(all, (*page.Value)...)
		endpoint = page.Next
	}
	return all, nil
}

func (c *Cloud) Subscriptions(ctx context.Context) ([]explorer.AzureSubscription, error) {
	items, err := c.list(ctx, "https://management.azure.com/subscriptions?api-version=2022-12-01")
	if err != nil {
		return nil, err
	}
	var result []explorer.AzureSubscription
	for _, item := range items {
		var s struct {
			ID     string `json:"subscriptionId"`
			Name   string `json:"displayName"`
			Tenant string `json:"tenantId"`
		}
		if json.Unmarshal(item, &s) != nil || !cloudGUID.MatchString(s.ID) || !cloudGUID.MatchString(s.Tenant) {
			return nil, errors.New("invalid Azure subscription metadata")
		}
		if strings.EqualFold(s.Tenant, c.credential.account.Tenant) {
			result = append(result, explorer.AzureSubscription{ID: s.ID, Name: s.Name})
		}
	}
	return result, nil
}

func (c *Cloud) decodeNamespace(raw []byte) (explorer.Namespace, error) {
	var data struct {
		ID         string `json:"id"`
		Properties struct {
			Endpoint string `json:"serviceBusEndpoint"`
		} `json:"properties"`
	}
	if json.Unmarshal(raw, &data) != nil {
		return explorer.Namespace{}, errors.New("invalid namespace metadata")
	}
	n, err := cloudARM(data.ID)
	if err != nil {
		return n, err
	}
	if data.Properties.Endpoint != "" && !strings.EqualFold(data.Properties.Endpoint, "https://"+n.Address+"/") {
		return explorer.Namespace{}, errors.New("namespace endpoint does not match ARM identity")
	}
	n.Profile = c.Profile()
	return n, nil
}

func (c *Cloud) Namespaces(ctx context.Context, subscription string) ([]explorer.Namespace, error) {
	if !cloudGUID.MatchString(subscription) {
		return nil, errors.New("invalid Azure subscription ID")
	}
	items, err := c.list(ctx, "https://management.azure.com/subscriptions/"+subscription+"/providers/Microsoft.ServiceBus/namespaces?api-version=2024-01-01")
	if err != nil {
		return nil, err
	}
	var result []explorer.Namespace
	for _, item := range items {
		n, err := c.decodeNamespace(item)
		if err != nil {
			return nil, err
		}
		if !strings.EqualFold(n.AzureSubscriptionID, subscription) {
			return nil, errors.New("namespace subscription identity changed")
		}
		result = append(result, n)
	}
	return result, nil
}

func (c *Cloud) ResolveNamespace(ctx context.Context, address string) (explorer.Namespace, error) {
	if cloudAddress(address) {
		if err := c.credential.check(ctx); err != nil {
			return explorer.Namespace{}, err
		}
		return explorer.Namespace{Profile: c.Profile(), Address: strings.ToLower(address), Name: strings.TrimSuffix(address, ".servicebus.windows.net")}, nil
	}
	n, err := cloudARM(address)
	if err != nil {
		return explorer.Namespace{}, err
	}
	var raw json.RawMessage
	if err := c.get(ctx, "https://management.azure.com"+namespaceARM(n)+"?api-version=2024-01-01", &raw); err != nil {
		return explorer.Namespace{}, err
	}
	resolved, err := c.decodeNamespace(raw)
	if err != nil {
		return explorer.Namespace{}, err
	}
	if !strings.EqualFold(namespaceARM(n), namespaceARM(resolved)) {
		return explorer.Namespace{}, errors.New("namespace ARM identity changed")
	}
	return resolved, nil
}

func (c *Cloud) validateNamespace(ctx context.Context, n explorer.Namespace) error {
	if n.Profile != c.Profile() || !cloudAddress(n.Address) || !strings.EqualFold(n.Name+".servicebus.windows.net", n.Address) {
		return errors.New("invalid Azure namespace snapshot")
	}
	if n.AzureSubscriptionID != "" || n.ResourceGroup != "" {
		resolved, err := c.ResolveNamespace(ctx, namespaceARM(n))
		if err != nil {
			return err
		}
		if resolved.Address != n.Address {
			return errors.New("namespace address mapping changed")
		}
	}
	return c.credential.check(ctx)
}

type cloudCountsKey struct{}
type cloudTransport struct {
	http       cloudHTTP
	host       string
	credential *cliCredential
	mu         sync.Mutex
	seen       map[string]bool
}

// Guard SDK redirects/paging and inspect presence before SDK zero-value decoding.
func (t *cloudTransport) Do(req *http.Request) (*http.Response, error) {
	if req.Method != http.MethodGet || req.URL.Scheme != "https" || req.URL.Host != t.host || req.URL.User != nil || req.URL.Fragment != "" {
		return nil, errors.New("Azure read-only endpoint rejected")
	}
	if err := t.credential.check(req.Context()); err != nil {
		return nil, err
	}
	t.mu.Lock()
	repeated := t.seen[req.URL.String()]
	t.seen[req.URL.String()] = true
	t.mu.Unlock()
	if repeated {
		return nil, errors.New("Azure namespace paging cycle")
	}
	resp, err := t.http.Do(req)
	if err != nil {
		return nil, errors.New("Azure namespace request failed")
	}
	if req.Context().Value(cloudCountsKey{}) == true && resp.StatusCode == http.StatusOK {
		body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
		resp.Body.Close()
		if err != nil {
			return nil, errors.New("Azure runtime counts unavailable")
		}
		var envelope struct {
			Content struct {
				Description struct {
					MessageCount *int64 `xml:"MessageCount"`
					Counts       *struct {
						Active      *int64 `xml:"ActiveMessageCount"`
						DLQ         *int64 `xml:"DeadLetterMessageCount"`
						Transfer    *int64 `xml:"TransferMessageCount"`
						TransferDLQ *int64 `xml:"TransferDeadLetterMessageCount"`
					} `xml:"CountDetails"`
				} `xml:",any"`
			} `xml:"content"`
		}
		if xml.Unmarshal(body, &envelope) != nil || envelope.Content.Description.Counts == nil || envelope.Content.Description.Counts.Active == nil || envelope.Content.Description.Counts.DLQ == nil {
			return nil, errors.New("Azure runtime counts unavailable")
		}
		// The SDK subscription decoder dereferences these additional optional fields.
		if strings.Contains(req.URL.Path, "/Subscriptions/") || strings.Contains(req.URL.Path, "/subscriptions/") {
			d := envelope.Content.Description
			if d.MessageCount == nil || d.Counts.Transfer == nil || d.Counts.TransferDLQ == nil {
				return nil, errors.New("Azure runtime counts unavailable")
			}
		}
		resp.Body = io.NopCloser(bytes.NewReader(body))
	}
	return resp, nil
}

func (c *Cloud) administration(address string) (*admin.Client, error) {
	return admin.NewClient(address, c.credential, &admin.ClientOptions{ClientOptions: azcore.ClientOptions{Transport: &cloudTransport{http: c.http, host: address, credential: c.credential, seen: make(map[string]bool)}, Retry: policy.RetryOptions{MaxRetries: -1}}})
}

func (c *Cloud) DiscoverScope(ctx context.Context, targets []explorer.Namespace) []explorer.NamespaceDiscovery {
	results := make([]explorer.NamespaceDiscovery, 0, len(targets))
	for _, n := range targets {
		r := explorer.NamespaceDiscovery{Namespace: n}
		r.Err = c.validateNamespace(ctx, n)
		if r.Err == nil {
			r.Resources, r.Err = c.discoverNamespace(ctx, n)
		}
		results = append(results, r)
	}
	return results
}

func (c *Cloud) discoverNamespace(ctx context.Context, n explorer.Namespace) ([]explorer.Resource, error) {
	a, err := c.administration(n.Address)
	if err != nil {
		return nil, errors.New("cannot initialize Azure administration client")
	}
	var resources []explorer.Resource
	add := func(q, t, s string) error {
		if (q != "" && !validEntity(q)) || (t != "" && !validEntity(t)) || (s != "" && !validEntity(s)) {
			return errors.New("invalid Azure entity identity")
		}
		resources = append(resources, explorer.Resource{Profile: c.Profile(), Namespace: n.Address, Queue: q, Topic: t, Subscription: s, AzureSubscriptionID: n.AzureSubscriptionID, ResourceGroup: n.ResourceGroup})
		return nil
	}
	queues := a.NewListQueuesPager(nil)
	for queues.More() {
		page, err := queues.NextPage(ctx)
		if err != nil {
			return nil, errors.New("Azure queue listing failed; check namespace read permissions")
		}
		for _, q := range page.Queues {
			if err := add(q.QueueName, "", ""); err != nil {
				return nil, err
			}
		}
	}
	topics := a.NewListTopicsPager(nil)
	for topics.More() {
		page, err := topics.NextPage(ctx)
		if err != nil {
			return nil, errors.New("Azure topic listing failed; check namespace read permissions")
		}
		for _, t := range page.Topics {
			if err := add("", t.TopicName, ""); err != nil {
				return nil, err
			}
			subs := a.NewListSubscriptionsPager(t.TopicName, nil)
			for subs.More() {
				page, err := subs.NextPage(ctx)
				if err != nil {
					return nil, errors.New("Azure topic subscription listing failed; check namespace read permissions")
				}
				for _, s := range page.Subscriptions {
					if err := add("", t.TopicName, s.SubscriptionName); err != nil {
						return nil, err
					}
				}
			}
		}
	}
	return resources, nil
}

func (*Cloud) Discover(context.Context) ([]explorer.Resource, error) {
	return nil, errors.New("choose a saved Azure environment before discovery")
}

func (c *Cloud) validResource(r explorer.Resource) bool {
	return r.Profile == c.Profile() && cloudAddress(r.Namespace) && ((validEntity(r.Queue) && r.Topic == "" && r.Subscription == "") || (r.Queue == "" && validEntity(r.Topic) && validEntity(r.Subscription)))
}

func (c *Cloud) validateResource(ctx context.Context, r explorer.Resource) error {
	return c.validateNamespace(ctx, explorer.Namespace{Profile: r.Profile, Address: r.Namespace, Name: strings.TrimSuffix(r.Namespace, ".servicebus.windows.net"), AzureSubscriptionID: r.AzureSubscriptionID, ResourceGroup: r.ResourceGroup})
}

func (c *Cloud) Counts(ctx context.Context, r explorer.Resource) (explorer.Counts, error) {
	if !c.validResource(r) {
		return explorer.Counts{}, errors.New("invalid Azure resource identity")
	}
	if err := c.validateResource(ctx, r); err != nil {
		return explorer.Counts{}, err
	}
	a, err := c.administration(r.Namespace)
	if err != nil {
		return explorer.Counts{}, errors.New("cannot initialize Azure administration client")
	}
	ctx = context.WithValue(ctx, cloudCountsKey{}, true)
	counts := explorer.Counts{State: "unavailable", Detail: "Azure runtime counts unavailable"}
	if r.Queue != "" {
		q, err := a.GetQueueRuntimeProperties(ctx, r.Queue, nil)
		if err != nil || q == nil {
			return counts, errors.New("Azure queue runtime counts unavailable")
		}
		counts.Active, counts.DeadLetter = int64(q.ActiveMessageCount), int64(q.DeadLetterMessageCount)
	} else {
		s, err := a.GetSubscriptionRuntimeProperties(ctx, r.Topic, r.Subscription, nil)
		if err != nil || s == nil {
			return counts, errors.New("Azure subscription runtime counts unavailable")
		}
		counts.Active, counts.DeadLetter = int64(s.ActiveMessageCount), int64(s.DeadLetterMessageCount)
	}
	counts.State, counts.Detail, counts.ObservedAt = "available", "", time.Now()
	return counts, nil
}

func (c *Cloud) messaging(address string) (*sb.Client, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil, errors.New("Azure connection is closed")
	}
	if client := c.clients[address]; client != nil {
		return client, nil
	}
	client, err := sb.NewClient(address, c.credential, &sb.ClientOptions{RetryOptions: sb.RetryOptions{MaxRetries: -1}})
	if err != nil {
		return nil, errors.New("cannot initialize Azure messaging client")
	}
	c.clients[address] = client
	return client, nil
}

func (c *Cloud) Peek(ctx context.Context, r explorer.Resource, dlq bool, from int64, limit int) (explorer.Page, error) {
	if !c.validResource(r) || from < 0 || limit < 1 || limit > 20 {
		return explorer.Page{}, errors.New("invalid Azure resource, sequence or page limit")
	}
	if err := c.validateResource(ctx, r); err != nil {
		return explorer.Page{}, err
	}
	client, err := c.messaging(r.Namespace)
	if err != nil {
		return explorer.Page{}, err
	}
	opts := &sb.ReceiverOptions{ReceiveMode: sb.ReceiveModePeekLock}
	if dlq {
		opts.SubQueue = sb.SubQueueDeadLetter
	}
	var receiver *sb.Receiver
	if r.Queue != "" {
		receiver, err = client.NewReceiverForQueue(r.Queue, opts)
	} else {
		receiver, err = client.NewReceiverForSubscription(r.Topic, r.Subscription, opts)
	}
	if err != nil {
		return explorer.Page{}, errors.New("cannot open Azure peek receiver")
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = receiver.Close(closeCtx)
	}()
	messages, err := receiver.PeekMessages(ctx, limit+1, &sb.PeekMessagesOptions{FromSequenceNumber: &from})
	if err != nil {
		return explorer.Page{}, errors.New("Azure message peek failed; check namespace permissions")
	}
	var page explorer.Page
	if len(messages) > limit {
		if messages[limit-1].SequenceNumber == nil {
			return page, errors.New("Azure peek cursor unavailable")
		}
		next := *messages[limit-1].SequenceNumber + 1
		page.NextSequence = &next
		messages = messages[:limit]
	}
	for _, m := range messages {
		page.Messages = append(page.Messages, (&delivery{message: m}).Snapshot())
	}
	return page, nil
}

func (c *Cloud) PeekMessage(ctx context.Context, r explorer.Resource, dlq bool, sequence int64) (explorer.Message, error) {
	if err := ctx.Err(); err != nil {
		return explorer.Message{}, err
	}
	if !c.validResource(r) || sequence < 0 {
		return explorer.Message{}, errors.New("invalid Azure resource or sequence")
	}
	if err := c.validateResource(ctx, r); err != nil {
		return explorer.Message{}, err
	}
	client, err := c.messaging(r.Namespace)
	if err != nil {
		return explorer.Message{}, err
	}
	options := &sb.ReceiverOptions{ReceiveMode: sb.ReceiveModePeekLock}
	if dlq {
		options.SubQueue = sb.SubQueueDeadLetter
	}
	var receiver *sb.Receiver
	if r.Queue != "" {
		receiver, err = client.NewReceiverForQueue(r.Queue, options)
	} else {
		receiver, err = client.NewReceiverForSubscription(r.Topic, r.Subscription, options)
	}
	if err != nil {
		return explorer.Message{}, errors.New("cannot open Azure full peek receiver")
	}
	defer closeReceiver(context.Background(), receiver)
	// A peek cursor is inclusive but may return a later message if the selected
	// sequence disappeared. Never substitute that neighbor's content.
	messages, err := receiver.PeekMessages(ctx, 1, &sb.PeekMessagesOptions{FromSequenceNumber: &sequence})
	if err != nil {
		return explorer.Message{}, errors.New("Azure full message peek failed; check namespace permissions")
	}
	if len(messages) != 1 || messages[0] == nil || messages[0].SequenceNumber == nil || *messages[0].SequenceNumber != sequence {
		return explorer.Message{}, errors.New("selected message is no longer available")
	}
	return snapshot(messages[0], false), nil
}

func (*Cloud) ReceiveNextDLQ(context.Context, explorer.Resource) (explorer.Delivery, error) {
	return nil, errors.New("Azure connection is read-only; DLQ receive is disabled")
}
func (*Cloud) PrepareDLQCopy(context.Context, explorer.Resource, int64) (explorer.Replay, error) {
	return nil, errors.New("Azure connection is read-only; DLQ copy is disabled")
}
func (*Cloud) ReceiveSelectedDLQ(context.Context, explorer.Resource, int64) (explorer.Delivery, error) {
	return nil, errors.New("Azure connection is read-only; DLQ receive is disabled")
}

func (c *Cloud) Close(ctx context.Context) error {
	c.mu.Lock()
	c.closed = true
	clients := c.clients
	c.clients = make(map[string]*sb.Client)
	c.mu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var result error
	for _, client := range clients {
		if client.Close(ctx) != nil {
			result = errors.New("Azure messaging client close failed")
		}
	}
	return result
}

var _ explorer.Store = (*Cloud)(nil)
var _ explorer.ScopeProvider = (*Cloud)(nil)
