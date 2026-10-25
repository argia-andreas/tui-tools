package fixture

import (
	"context"
	"errors"
	"slices"
	"strings"

	"servicebusexplorer/internal/explorer"
)

// NewScoped keeps the original local-lab inventory and adds independent sample
// namespaces for trying environment selection. No Azure credentials or network.
func NewScoped() (*Store, error) {
	s, err := New()
	if err != nil {
		return nil, err
	}
	seeds := slices.Clone(s.resources)
	for _, ns := range sampleNamespaces()[1:] {
		for _, original := range seeds[:3] {
			r := original
			r.Namespace, r.AzureSubscriptionID, r.ResourceGroup = ns.Address, ns.AzureSubscriptionID, ns.ResourceGroup
			s.resources = append(s.resources, r)
			active, dlq := 9, 0
			if ns.Name == "prod-core" {
				active, dlq = 128, 3
			}
			s.entries[r] = entry{messages(r, active, false), messages(r, dlq, true)}
		}
	}
	return s, nil
}

func sampleNamespaces() []explorer.Namespace {
	return []explorer.Namespace{
		{Profile: "fixture", Address: "local-lab", Name: "local-lab"},
		{Profile: "fixture", Address: "prod-core", Name: "prod-core", AzureSubscriptionID: "sample-production", AzureSubscriptionName: "Sample production", ResourceGroup: "rg-core"},
		{Profile: "fixture", Address: "prod-archive", Name: "prod-archive", AzureSubscriptionID: "sample-production", AzureSubscriptionName: "Sample production", ResourceGroup: "rg-core"},
		{Profile: "fixture", Address: "prod-payments", Name: "prod-payments", AzureSubscriptionID: "sample-production", AzureSubscriptionName: "Sample production", ResourceGroup: "rg-payments"},
		{Profile: "fixture", Address: "prod-notifications", Name: "prod-notifications", AzureSubscriptionID: "sample-shared", AzureSubscriptionName: "Sample shared services", ResourceGroup: "rg-shared"},
		{Profile: "fixture", Address: "stage-core", Name: "stage-core", AzureSubscriptionID: "sample-staging", AzureSubscriptionName: "Sample staging", ResourceGroup: "rg-core"},
	}
}

func (*Store) Subscriptions(ctx context.Context) ([]explorer.AzureSubscription, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return []explorer.AzureSubscription{{ID: "local", Name: "Local fixture"}, {ID: "sample-production", Name: "Sample production"}, {ID: "sample-shared", Name: "Sample shared services"}, {ID: "sample-staging", Name: "Sample staging"}}, nil
}

func (*Store) Namespaces(ctx context.Context, subscription string) ([]explorer.Namespace, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var result []explorer.Namespace
	for _, ns := range sampleNamespaces() {
		if ns.AzureSubscriptionID == subscription || ns.AzureSubscriptionID == "" && subscription == "local" {
			result = append(result, ns)
		}
	}
	return result, nil
}

func (*Store) ResolveNamespace(ctx context.Context, input string) (explorer.Namespace, error) {
	if err := ctx.Err(); err != nil {
		return explorer.Namespace{}, err
	}
	for _, ns := range sampleNamespaces() {
		if ns.Address == strings.TrimSpace(input) {
			return ns, nil
		}
	}
	return explorer.Namespace{}, errors.New("unknown fixture namespace; use local-lab, prod-core, prod-archive, prod-payments, prod-notifications or stage-core")
}

func (s *Store) DiscoverScope(ctx context.Context, namespaces []explorer.Namespace) []explorer.NamespaceDiscovery {
	var results []explorer.NamespaceDiscovery
	for _, ns := range namespaces {
		r := explorer.NamespaceDiscovery{Namespace: ns, Err: ctx.Err()}
		if !slices.Contains(sampleNamespaces(), ns) {
			r.Err = errors.New("fixture namespace identity is no longer available")
		}
		if r.Err == nil {
			for _, entity := range s.resources {
				if entity.Profile == ns.Profile && entity.Namespace == ns.Address && entity.AzureSubscriptionID == ns.AzureSubscriptionID && entity.ResourceGroup == ns.ResourceGroup {
					r.Resources = append(r.Resources, entity)
				}
			}
		}
		results = append(results, r)
	}
	return results
}

var _ explorer.ScopeProvider = (*Store)(nil)
