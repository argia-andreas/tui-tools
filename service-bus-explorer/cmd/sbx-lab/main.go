// sbx-lab is a disposable, fixed-loopback lab tool, never an Azure admin CLI.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	"servicebusexplorer/internal/explorer"
	"servicebusexplorer/internal/fixture"
)

// These are Microsoft's PUBLIC emulator credentials, not a user's SAS secret.
// Never read SBX_CONNECTION_STRING or accept an endpoint/connection-string flag.
const connectionString = "Endpoint=sb://127.0.0.1:5673;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=SAS_KEY_VALUE;UseDevelopmentEmulator=true;"

type options struct {
	command, entity, body, id, reason string
	max                               int
	from                              int64
	dlq, confirm, eula, sudo, orb     bool
}

func parse(argv []string, output io.Writer) (options, error) {
	var o options
	if len(argv) == 0 || argv[0] == "--help" || argv[0] == "-h" {
		fmt.Fprintln(output, `Usage: sbx-lab COMMAND [flags]
Commands: seed, send, peek, dead-letter, drain
Target is ONLY sbx-explorer-lab: AMQP 127.0.0.1:5673; management 5301.
Run from service-bus-explorer/ (or a subdirectory); Python 3 and Docker Compose required.
Lifecycle: python3 scripts/lab.py [--sudo] [--orb] [--accept-eula] [--confirm-disposable] up|status|down|reset
Seed ALWAYS destroys/recreates this lab via Python reset, then publishes fixtures.
No cloud credentials, endpoint overrides or runtime counter claims. Peek is non-destructive.
Examples:
  sbx-lab seed --accept-eula --confirm-disposable --sudo --orb
  sbx-lab peek --entity orders --max 20 --from 0
  sbx-lab peek --entity events/shipping --dlq
  sbx-lab send --entity events --body '{"event":"test"}' --id manual-1 --confirm-disposable
  sbx-lab dead-letter --entity orders --max 1 --confirm-disposable
  sbx-lab drain --entity orders --dlq --max 100 --confirm-disposable
Use COMMAND --help for flags. In this orb add --sudo; lifecycle/seed also add --orb.
Drain/dead-letter report actual acknowledged settlements, not inferred totals.
An unsuccessful send/settlement may have an UNKNOWN outcome; do not blindly retry.`)
		return o, flag.ErrHelp
	}
	o.command = argv[0]
	fs := flag.NewFlagSet(o.command, flag.ContinueOnError)
	fs.SetOutput(output)
	fs.StringVar(&o.entity, "entity", "orders", "configured queue, events, or events/billing|shipping (topic itself is send-only)")
	fs.StringVar(&o.body, "body", "lab message", "UTF-8 body for send; no file/endpoint interpretation")
	fs.StringVar(&o.id, "id", "", "message ID for send; defaults to lab-<UnixNano>")
	fs.StringVar(&o.reason, "reason", "LabDeadLetter", "reason for explicit dead-letter")
	fs.IntVar(&o.max, "max", 20, "maximum messages; 1..10000, peek limited to 100")
	fs.Int64Var(&o.from, "from", 0, "explicit starting sequence for peek (0 = beginning)")
	fs.BoolVar(&o.dlq, "dlq", false, "peek/drain the dead-letter subqueue")
	fs.BoolVar(&o.confirm, "confirm-disposable", false, "authorize mutations to this disposable lab only")
	fs.BoolVar(&o.eula, "accept-eula", false, "explicitly accept emulator/SQL terms for seed reset")
	fs.BoolVar(&o.sudo, "sudo", false, "use sudo docker in Python lab guard/reset")
	fs.BoolVar(&o.orb, "orb", false, "supervise foreground Compose during seed reset via amp orb service")
	if err := fs.Parse(argv[1:]); err != nil {
		return o, err
	}
	if fs.NArg() != 0 {
		return o, errors.New("unexpected positional arguments")
	}
	switch o.command {
	case "seed", "send", "dead-letter", "drain":
		if !o.confirm {
			return o, errors.New("mutation requires --confirm-disposable")
		}
	case "peek":
	default:
		return o, errors.New("unknown command; use --help")
	}
	if o.command == "seed" && !o.eula {
		return o, errors.New("seed requires --accept-eula and always resets the lab")
	}
	if o.max < 1 || o.max > 10000 || (o.command == "peek" && o.max > 100) || o.from < 0 {
		return o, errors.New("invalid bounded max/from")
	}
	if o.dlq && o.command != "peek" && o.command != "drain" {
		return o, errors.New("--dlq is only supported for peek/drain")
	}
	_, err := resource(o.entity, o.command == "send")
	return o, err
}

func resource(name string, send bool) (explorer.Resource, error) {
	s, err := fixture.New()
	if err != nil {
		return explorer.Resource{}, err
	}
	resources, err := s.Discover(context.Background())
	if err != nil {
		return explorer.Resource{}, err
	}
	for _, r := range resources {
		if r.Name() == name && (!send || r.Queue != "") {
			return r, nil
		}
	}
	if send && name == "events" {
		return explorer.Resource{Topic: "events"}, nil
	}
	return explorer.Resource{}, errors.New("entity must be a configured lab queue/subscription (send accepts events topic, not subscriptions)")
}

func moduleRoot() (string, error) {
	path, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		module, err := os.ReadFile(filepath.Join(path, "go.mod"))
		if err == nil && strings.HasPrefix(string(module), "module servicebusexplorer\n") {
			if _, err := os.Stat(filepath.Join(path, "scripts", "lab.py")); err == nil {
				return path, nil
			}
		}
		parent := filepath.Dir(path)
		if parent == path {
			return "", errors.New("run sbx-lab from service-bus-explorer or a subdirectory")
		}
		path = parent
	}
}

func python(o options, command string) error {
	root, err := moduleRoot()
	if err != nil {
		return err
	}
	args := []string{filepath.Join(root, "scripts", "lab.py")}
	if o.sudo {
		args = append(args, "--sudo")
	}
	if o.orb {
		args = append(args, "--orb")
	}
	if command == "reset" {
		args = append(args, "--accept-eula", "--confirm-disposable")
	}
	args = append(args, command)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "python3", args...)
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	if err := cmd.Run(); err != nil {
		return errors.New("local lab guard/reset failed; no messaging attempted")
	}
	return nil
}

func receiver(client *azservicebus.Client, r explorer.Resource, dlq bool) (*azservicebus.Receiver, error) {
	opts := &azservicebus.ReceiverOptions{ReceiveMode: azservicebus.ReceiveModePeekLock}
	if dlq {
		opts.SubQueue = azservicebus.SubQueueDeadLetter
	}
	if r.Queue != "" {
		return client.NewReceiverForQueue(r.Queue, opts)
	}
	return client.NewReceiverForSubscription(r.Topic, r.Subscription, opts)
}

func closeReceiver(r *azservicebus.Receiver) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = r.Close(ctx)
}

func emit(value any) { _ = json.NewEncoder(os.Stdout).Encode(value) }

func send(client *azservicebus.Client, destination string, messages []explorer.Message) error {
	sender, err := client.NewSender(destination, nil)
	if err != nil {
		return err
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = sender.Close(ctx)
	}()
	for len(messages) > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		batch, err := sender.NewMessageBatch(ctx, nil)
		n := min(20, len(messages))
		if err == nil {
			for _, m := range messages[:n] {
				id, subject, contentType := m.ID, m.Subject, m.ContentType
				err = batch.AddMessage(&azservicebus.Message{MessageID: &id, Subject: &subject, ContentType: &contentType, Body: m.Body, ApplicationProperties: m.Properties}, nil)
				if err != nil {
					break
				}
			}
		}
		if err == nil {
			err = sender.SendMessageBatch(ctx, batch, nil)
		}
		cancel()
		if err != nil {
			return errors.New("send failed; outcome may be unknown; inspect destination before retrying")
		}
		messages = messages[n:]
		// The development broker throttles bursts during a large seed. Pace
		// acknowledged batches instead of retrying an ambiguous send result.
		time.Sleep(500 * time.Millisecond)
	}
	return nil
}

// settle receives at most one at a time in PeekLock. It reports only successful
// acknowledgements. Empty timeout is a bounded observation, never a queue total.
func settle(client *azservicebus.Client, r explorer.Resource, dlq, deadLetter bool, max int, reason string) (int, error) {
	rx, err := receiver(client, r, dlq)
	if err != nil {
		return 0, err
	}
	defer closeReceiver(rx)
	return settleReceiver(rx, deadLetter, max, reason)
}

type settlingReceiver interface {
	ReceiveMessages(context.Context, int, *azservicebus.ReceiveMessagesOptions) ([]*azservicebus.ReceivedMessage, error)
	CompleteMessage(context.Context, *azservicebus.ReceivedMessage, *azservicebus.CompleteMessageOptions) error
	DeadLetterMessage(context.Context, *azservicebus.ReceivedMessage, *azservicebus.DeadLetterOptions) error
}

func settleReceiver(rx settlingReceiver, deadLetter bool, max int, reason string) (int, error) {
	settled := 0
	for settled < max {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		messages, err := rx.ReceiveMessages(ctx, 1, nil)
		cancel()
		if err != nil && !errors.Is(err, context.DeadlineExceeded) {
			return settled, errors.New("receive failed; settled count includes acknowledgements only")
		}
		if len(messages) == 0 {
			return settled, nil
		}
		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
		if deadLetter {
			description := "Payment provider timed out after repeated delivery attempts."
			err = rx.DeadLetterMessage(ctx, messages[0], &azservicebus.DeadLetterOptions{Reason: &reason, ErrorDescription: &description})
		} else {
			err = rx.CompleteMessage(ctx, messages[0], nil)
		}
		cancel()
		if err != nil {
			return settled, errors.New("settlement outcome unknown; reported count excludes unacknowledged attempt")
		}
		settled++
	}
	return settled, nil
}

func seedMessages(s *fixture.Store, r explorer.Resource, dlq bool) ([]explorer.Message, error) {
	var result []explorer.Message
	from := int64(0)
	for {
		page, err := s.Peek(context.Background(), r, dlq, from, 20)
		if err != nil {
			return nil, err
		}
		result = append(result, page.Messages...)
		if page.NextSequence == nil {
			break
		}
		from = *page.NextSequence
	}
	// Two controlled text variants exercise newline and terminal-control safety.
	// All other bodies and IDs are byte-for-byte the embedded fixture scenario.
	for i := range result {
		if result[i].ID == "notifications-active-0001" {
			result[i].Body = []byte("Local lab notification\nsecond line\n")
			result[i].ContentType = "text/plain"
		}
		if result[i].ID == "notifications-active-0002" {
			result[i].Body = []byte("untrusted \x1b[31mred\x1b[0m \x1b]0;not-a-title\x07\nvisible next line")
			result[i].ContentType = "text/plain"
			result[i].Subject = strings.Repeat("long-notification-subject-", 6)
		}
	}
	return result, nil
}

func seed(client *azservicebus.Client) error {
	s, err := fixture.New()
	if err != nil {
		return err
	}
	resources, err := s.Discover(context.Background())
	if err != nil {
		return err
	}
	for _, r := range resources {
		if r.Queue == "" {
			continue
		}
		dead, err := seedMessages(s, r, true)
		if err != nil {
			return err
		}
		active, err := seedMessages(s, r, false)
		if err != nil {
			return err
		}
		// Dead-letter the first cohort before adding active messages: no guessing
		// which peeked sequence a receive-next operation will acquire.
		if err := send(client, r.Queue, dead); err != nil {
			return err
		}
		n, err := settle(client, r, false, true, len(dead), "MaxDeliveryCountExceeded")
		if err != nil {
			return err
		}
		if n != len(dead) {
			return errors.New("seed dead-letter cohort not fully received; reset before retrying")
		}
		if err := send(client, r.Queue, active); err != nil {
			return err
		}
		emit(map[string]any{"entity": r.Name(), "inserted_active": len(active), "settled_dead_letter": n, "counts": "seed expectation, not runtime counters"})
	}
	// An unfiltered topic fans every publication out to BOTH subscriptions.
	// Use billing's 8-message cohort once, not independent synthetic backlogs.
	billing, _ := resource("events/billing", false)
	dead, err := seedMessages(s, billing, true)
	if err != nil {
		return err
	}
	active, err := seedMessages(s, billing, false)
	if err != nil {
		return err
	}
	all := append(dead, active...)
	if err := send(client, "events", all); err != nil {
		return err
	}
	for _, r := range resources {
		if r.Topic != "events" {
			continue
		}
		counts, err := s.Counts(context.Background(), r)
		if err != nil {
			return err
		}
		n, err := settle(client, r, false, true, int(counts.DeadLetter), "MaxDeliveryCountExceeded")
		if err != nil {
			return err
		}
		if n != int(counts.DeadLetter) {
			return errors.New("seed topic cohort not fully received; reset before retrying")
		}
		emit(map[string]any{"entity": r.Name(), "inserted_active": len(all) - n, "settled_dead_letter": n, "counts": "fanout-derived seed expectation, not runtime counters"})
	}
	return nil
}

func run(o options) error {
	if o.command == "seed" {
		if err := python(o, "reset"); err != nil {
			return err
		}
	}
	if err := python(o, "status"); err != nil {
		return err
	}
	client, err := azservicebus.NewClientFromConnectionString(connectionString, &azservicebus.ClientOptions{RetryOptions: azservicebus.RetryOptions{MaxRetries: -1}})
	if err != nil {
		return errors.New("cannot initialize fixed local emulator client")
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = client.Close(ctx)
	}()
	if o.command == "seed" {
		return seed(client)
	}
	r, err := resource(o.entity, o.command == "send")
	if err != nil {
		return err
	}
	switch o.command {
	case "send":
		if o.id == "" {
			o.id = fmt.Sprintf("lab-%d", time.Now().UnixNano())
		}
		if err := send(client, o.entity, []explorer.Message{{ID: o.id, Body: []byte(o.body), ContentType: "text/plain"}}); err != nil {
			return err
		}
		emit(map[string]any{"sent": 1, "id": o.id, "entity": o.entity})
	case "peek":
		rx, err := receiver(client, r, o.dlq)
		if err != nil {
			return err
		}
		defer closeReceiver(rx)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		messages, err := rx.PeekMessages(ctx, o.max, &azservicebus.PeekMessagesOptions{FromSequenceNumber: &o.from})
		if err != nil {
			return errors.New("peek failed at fixed local emulator")
		}
		for _, m := range messages {
			emit(map[string]any{"id": m.MessageID, "body": string(m.Body), "sequence": m.SequenceNumber, "subject": m.Subject, "content_type": m.ContentType, "properties": m.ApplicationProperties, "dead_letter_reason": m.DeadLetterReason, "dead_letter_description": m.DeadLetterErrorDescription})
		}
	case "drain", "dead-letter":
		n, err := settle(client, r, o.dlq, o.command == "dead-letter", o.max, o.reason)
		emit(map[string]any{"entity": o.entity, "dlq": o.dlq, "operation": o.command, "settled": n, "maximum": o.max, "outcome_unknown": err != nil})
		return err
	}
	return nil
}

func main() {
	o, err := parse(os.Args[1:], os.Stderr)
	if errors.Is(err, flag.ErrHelp) {
		return
	}
	if err == nil {
		err = run(o)
	}
	if err != nil {
		// SDK errors can contain signed URIs. Only our fixed diagnostics are safe.
		fmt.Fprintln(os.Stderr, "sbx-lab:", err)
		os.Exit(1)
	}
}
