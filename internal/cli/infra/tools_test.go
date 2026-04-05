package infra

import (
	"context"
	"strings"
	"testing"
	"time"
)

type stubCloudflareZoneService struct {
	zones        []cloudflareZone
	err          error
	requestedKey string
	waitForCtx   bool
}

func (s *stubCloudflareZoneService) ListZones(ctx context.Context, apiToken string) ([]cloudflareZone, error) {
	s.requestedKey = apiToken
	if s.waitForCtx {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if s.err != nil {
		return nil, s.err
	}
	return append([]cloudflareZone(nil), s.zones...), nil
}

func TestRunToolsFlowRequiresProviderWhenNonInteractive(t *testing.T) {
	err := runToolsFlow(&config{Command: commandTools, NonInteractive: true})
	if err == nil || !strings.Contains(err.Error(), "requires a provider") {
		t.Fatalf("expected provider requirement error, got %v", err)
	}
}

func TestRunToolsFlowCloudflareZonesUsesEnvToken(t *testing.T) {
	t.Setenv("CLOUDFLARE_API_TOKEN", "env-cloudflare-token")

	stub := &stubCloudflareZoneService{zones: []cloudflareZone{{ID: "zone-1", Name: "example.com"}}}
	original := cloudflareZonesClient
	cloudflareZonesClient = stub
	defer func() { cloudflareZonesClient = original }()

	err := runToolsFlow(&config{Command: commandTools, ToolsProvider: toolsProviderCloudflare, ToolsAction: toolsActionCloudflareZone, NonInteractive: true})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if stub.requestedKey != "env-cloudflare-token" {
		t.Fatalf("expected env token to be used, got %q", stub.requestedKey)
	}
}

func TestRunToolsFlowCloudflareZonesUsesExplicitToken(t *testing.T) {
	t.Setenv("CLOUDFLARE_API_TOKEN", "ignored-env-token")

	stub := &stubCloudflareZoneService{zones: []cloudflareZone{{ID: "zone-1", Name: "example.com"}}}
	original := cloudflareZonesClient
	cloudflareZonesClient = stub
	defer func() { cloudflareZonesClient = original }()

	err := runToolsFlow(&config{Command: commandTools, ToolsProvider: toolsProviderCloudflare, ToolsAction: toolsActionCloudflareZone, CloudflareToken: "flag-token", NonInteractive: true})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if stub.requestedKey != "flag-token" {
		t.Fatalf("expected explicit token to be used, got %q", stub.requestedKey)
	}
}

func TestRunToolsFlowCloudflareZonesRequiresTokenWhenNonInteractive(t *testing.T) {
	t.Setenv("CLOUDFLARE_API_TOKEN", "")

	err := runToolsFlow(&config{Command: commandTools, ToolsProvider: toolsProviderCloudflare, ToolsAction: toolsActionCloudflareZone, NonInteractive: true})
	if err == nil || !strings.Contains(err.Error(), "cloudflare API token is required") {
		t.Fatalf("expected token requirement error, got %v", err)
	}
}

func TestRunToolsFlowCloudflareZonesRejectsExplicitEmptyToken(t *testing.T) {
	err := runToolsFlow(&config{Command: commandTools, ToolsProvider: toolsProviderCloudflare, ToolsAction: toolsActionCloudflareZone, CloudflareToken: "   ", NonInteractive: true, Provided: providedFlags{CloudflareToken: true}})
	if err == nil || !strings.Contains(err.Error(), "--token must not be empty") {
		t.Fatalf("expected explicit empty token error, got %v", err)
	}
}

func TestRunToolsFlowCloudflareZonesHonorsTimeout(t *testing.T) {
	stub := &stubCloudflareZoneService{waitForCtx: true}

	originalClient := cloudflareZonesClient
	originalTimeout := cloudflareRequestTimeout
	cloudflareZonesClient = stub
	cloudflareRequestTimeout = 1 * time.Millisecond
	defer func() {
		cloudflareZonesClient = originalClient
		cloudflareRequestTimeout = originalTimeout
	}()

	err := runToolsFlow(&config{Command: commandTools, ToolsProvider: toolsProviderCloudflare, ToolsAction: toolsActionCloudflareZone, CloudflareToken: "token", NonInteractive: true, Provided: providedFlags{CloudflareToken: true}})
	if err == nil || !strings.Contains(err.Error(), "list cloudflare zones") || !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Fatalf("expected list cloudflare zones error, got %v", err)
	}
}
