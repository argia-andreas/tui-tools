// Package kafka implements broker reads only. It never joins a group or writes.
package kafka

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl/plain"
	"github.com/twmb/franz-go/pkg/sasl/scram"
	"streams/internal/config"
	"streams/internal/stream"
)

const catalogLimit = 10000

type Client struct {
	mu       sync.Mutex
	ctx      context.Context
	cancel   context.CancelFunc
	closed   bool
	metadata *kgo.Client
	admin    *kadm.Client
	opts     []kgo.Opt
	active   *reader
}

var _ stream.Backend = (*Client)(nil)

func New(p config.Profile) (*Client, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	opts := []kgo.Opt{
		kgo.SeedBrokers(p.Brokers...), kgo.ClientID("streams-read-only"),
		kgo.BrokerMaxReadBytes(16 << 20), kgo.MaxDecompressBatchBytes(8 << 20),
		kgo.FetchMaxBytes(4 << 20), kgo.FetchMaxPartitionBytes(1 << 20),
		kgo.MaxConcurrentFetches(0), kgo.FetchMaxWait(250 * time.Millisecond),
		kgo.FetchIsolationLevel(kgo.ReadCommitted()), kgo.ConsumeResetOffset(kgo.NoResetOffset()),
	}
	if p.TLS {
		t := &tls.Config{MinVersion: tls.VersionTLS12}
		if p.CAFile != "" {
			b, err := config.ReadPrivateFile(p.CAFile)
			if err != nil {
				return nil, err
			}
			t.RootCAs = x509.NewCertPool()
			if !t.RootCAs.AppendCertsFromPEM(b) {
				return nil, errors.New("invalid CA certificate file")
			}
		}
		if p.CertFile != "" {
			cert, err := config.ReadPrivateFile(p.CertFile)
			if err != nil {
				return nil, err
			}
			key, err := config.ReadPrivateFile(p.KeyFile)
			if err != nil {
				return nil, err
			}
			pair, err := tls.X509KeyPair(cert, key)
			if err != nil {
				return nil, errors.New("invalid client certificate or private key")
			}
			t.Certificates = []tls.Certificate{pair}
		}
		opts = append(opts, kgo.DialTLSConfig(t))
	} else {
		// Also constrain advertised addresses, not just the bootstrap list.
		opts = append(opts, kgo.Dialer(func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, _, err := net.SplitHostPort(addr)
			if err != nil || (host != "localhost" && !net.ParseIP(host).IsLoopback()) {
				return nil, stream.ErrTransport
			}
			return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, addr)
		}))
	}
	if p.SASL != "" {
		b, err := config.ReadPrivateFile(p.PasswordFile)
		if err != nil {
			return nil, err
		}
		// Permit the single line terminator commonly written by secret-file tools;
		// spaces and all other bytes are significant.
		password := strings.TrimSuffix(strings.TrimSuffix(string(b), "\n"), "\r")
		if password == "" {
			return nil, errors.New("empty SASL password file")
		}
		switch p.SASL {
		case "PLAIN":
			opts = append(opts, kgo.SASL(plain.Auth{User: p.Username, Pass: password}.AsMechanism()))
		case "SCRAM-SHA-256":
			opts = append(opts, kgo.SASL(scram.Auth{User: p.Username, Pass: password}.AsSha256Mechanism()))
		case "SCRAM-SHA-512":
			opts = append(opts, kgo.SASL(scram.Auth{User: p.Username, Pass: password}.AsSha512Mechanism()))
		}
	}
	k, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, safe(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Client{ctx: ctx, cancel: cancel, metadata: k, admin: kadm.NewClient(k), opts: opts}, nil
}

func (c *Client) operation(ctx context.Context) (context.Context, func()) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	stop := context.AfterFunc(c.ctx, cancel)
	return ctx, func() { stop(); cancel() }
}

func (c *Client) Close() {
	c.cancel()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	c.closed = true
	if c.active != nil {
		c.active.Close()
	}
	c.metadata.Close()
}

func topicRef(t kadm.TopicDetail) stream.TopicRef {
	id := ""
	if t.ID != (kadm.TopicID{}) {
		id = t.ID.String()
	}
	return stream.TopicRef{Name: t.Topic, ID: id}
}

func (c *Client) Topics(ctx context.Context) ([]stream.Topic, error) {
	ctx, done := c.operation(ctx)
	defer done()
	ts, err := c.admin.ListTopicsWithInternal(ctx)
	if err != nil {
		return nil, safe(err)
	}
	if len(ts) > catalogLimit {
		return nil, stream.ErrLimit
	}
	out := make([]stream.Topic, 0, len(ts))
	for _, name := range ts.Names() {
		t := ts[name]
		if t.Err != nil {
			return out, safe(t.Err)
		}
		x := stream.Topic{Ref: topicRef(t), Internal: t.IsInternal}
		for _, p := range t.Partitions.Sorted() {
			x.Partitions = append(x.Partitions, stream.Partition{ID: p.Partition, Leader: p.Leader, Replicas: append([]int32(nil), p.Replicas...), InSync: append([]int32(nil), p.ISR...), Err: safe(p.Err)})
		}
		out = append(out, x)
	}
	return out, nil
}

func (c *Client) Groups(ctx context.Context) ([]string, error) {
	ctx, done := c.operation(ctx)
	defer done()
	gs, err := c.admin.ListGroups(ctx)
	if len(gs) > catalogLimit {
		return nil, stream.ErrLimit
	}
	names := gs.Groups()
	sort.Strings(names)
	if err != nil {
		return names, safe(err)
	}
	return names, nil
}

// Never return raw library messages: brokers can supply arbitrary error text.
func safe(err error) error {
	if err == nil {
		return nil
	}
	for _, e := range []error{context.Canceled, context.DeadlineExceeded, stream.ErrClosed, stream.ErrPermission, stream.ErrAuthentication, stream.ErrTransport, stream.ErrTopic, stream.ErrExpired, stream.ErrLimit} {
		if errors.Is(err, e) {
			return e
		}
	}
	for _, e := range []error{kerr.TopicAuthorizationFailed, kerr.GroupAuthorizationFailed, kerr.ClusterAuthorizationFailed} {
		if errors.Is(err, e) {
			return stream.ErrPermission
		}
	}
	for _, e := range []error{kerr.SaslAuthenticationFailed, kerr.UnsupportedSaslMechanism, kerr.IllegalSaslState} {
		if errors.Is(err, e) {
			return stream.ErrAuthentication
		}
	}
	for _, e := range []error{kerr.UnknownTopicOrPartition, kerr.UnknownTopicID, kerr.InconsistentTopicID} {
		if errors.Is(err, e) {
			return stream.ErrTopic
		}
	}
	for _, e := range []error{kerr.OffsetOutOfRange, kerr.FencedLeaderEpoch, kerr.UnknownLeaderEpoch} {
		if errors.Is(err, e) {
			return stream.ErrExpired
		}
	}
	var loss *kgo.ErrDataLoss
	if errors.As(err, &loss) {
		return stream.ErrExpired
	}
	// The pinned client does not export its wire-response-size sentinel.
	// Recognize its exact fixed message without exposing any underlying text.
	if errors.Is(err, kgo.ErrMaxDecompress) || err.Error() == "response exceeds BrokerMaxReadBytes" {
		return stream.ErrLimit
	}
	return stream.ErrTransport
}
