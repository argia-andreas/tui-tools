package azure

import (
	"context"
	"errors"
	"strings"

	"servicebusexplorer/internal/explorer"
)

func LocalNamespace() explorer.Namespace {
	return explorer.Namespace{Profile: "emulator", Name: "Local lab", Address: MessagingAddress}
}

func (*Store) Subscriptions(ctx context.Context) ([]explorer.AzureSubscription, error) {
	return []explorer.AzureSubscription{{ID: "local", Name: "Local emulator (not Azure)"}}, ctx.Err()
}

func (*Store) Namespaces(ctx context.Context, subscription string) ([]explorer.Namespace, error) {
	if subscription != "local" {
		return nil, errors.New("the local emulator has no Azure subscriptions")
	}
	return []explorer.Namespace{LocalNamespace()}, ctx.Err()
}

func (*Store) ResolveNamespace(ctx context.Context, input string) (explorer.Namespace, error) {
	if strings.TrimSpace(input) != MessagingAddress {
		return explorer.Namespace{}, errors.New("emulator mode permits only the fixed local lab namespace")
	}
	return LocalNamespace(), ctx.Err()
}

func (s *Store) DiscoverScope(ctx context.Context, namespaces []explorer.Namespace) []explorer.NamespaceDiscovery {
	var results []explorer.NamespaceDiscovery
	for _, ns := range namespaces {
		r := explorer.NamespaceDiscovery{Namespace: ns}
		if ns != LocalNamespace() {
			r.Err = errors.New("namespace is outside the fixed local emulator")
		} else {
			r.Resources, r.Err = s.Discover(ctx)
		}
		results = append(results, r)
	}
	return results
}

var _ explorer.ScopeProvider = (*Store)(nil)
