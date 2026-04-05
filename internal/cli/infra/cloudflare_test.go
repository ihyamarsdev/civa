package infra

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type stubCloudflareZonesService struct {
	listFn   func(ctx context.Context, apiToken string) ([]cloudflareZone, error)
	createFn func(ctx context.Context, apiToken string, body cloudflareZoneCreateRequest) (cloudflareZone, error)
	updateFn func(ctx context.Context, apiToken string, zoneID string, body cloudflareZoneUpdateRequest) (cloudflareZone, error)
	deleteFn func(ctx context.Context, apiToken string, zoneID string) (cloudflareZone, error)
}

func (s *stubCloudflareZonesService) ListZones(ctx context.Context, apiToken string) ([]cloudflareZone, error) {
	if s == nil || s.listFn == nil {
		return nil, nil
	}
	return s.listFn(ctx, apiToken)
}

func (s *stubCloudflareZonesService) CreateZone(ctx context.Context, apiToken string, body cloudflareZoneCreateRequest) (cloudflareZone, error) {
	if s == nil || s.createFn == nil {
		return cloudflareZone{}, nil
	}
	return s.createFn(ctx, apiToken, body)
}

func (s *stubCloudflareZonesService) UpdateZone(ctx context.Context, apiToken string, zoneID string, body cloudflareZoneUpdateRequest) (cloudflareZone, error) {
	if s == nil || s.updateFn == nil {
		return cloudflareZone{}, nil
	}
	return s.updateFn(ctx, apiToken, zoneID, body)
}

func (s *stubCloudflareZonesService) DeleteZone(ctx context.Context, apiToken string, zoneID string) (cloudflareZone, error) {
	if s == nil || s.deleteFn == nil {
		return cloudflareZone{}, nil
	}
	return s.deleteFn(ctx, apiToken, zoneID)
}

func TestRunAuthFlowCloudflareSetAndGet(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg := defaultConfig(commandAuth)
	cfg.AuthProvider = authProviderCloudflare
	cfg.AuthAction = authActionSet
	cfg.AuthProfile = "default"
	cfg.AuthToken = "secret-token"
	cfg.Provided.AuthToken = true
	if err := runAuthFlow(&cfg); err != nil {
		t.Fatalf("set unexpected error: %v", err)
	}
	cfg.AuthAction = authActionGet
	if err := runAuthFlow(&cfg); err != nil {
		t.Fatalf("get unexpected error: %v", err)
	}
}

func TestRunAuthFlowCloudflareListAndRemove(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := writeSecretValue(cloudflareAuthSecretName("default"), "token"); err != nil {
		t.Fatalf("write secret: %v", err)
	}
	cfg := defaultConfig(commandAuth)
	cfg.AuthProvider = authProviderCloudflare
	cfg.AuthAction = authActionList
	if err := runAuthFlow(&cfg); err != nil {
		t.Fatalf("list unexpected error: %v", err)
	}
	if err := runAuthFlow(&config{Command: commandAuth, AuthProvider: authProviderCloudflare, AuthAction: authActionRemove, AuthProfile: "default"}); err != nil {
		t.Fatalf("remove unexpected error: %v", err)
	}
	profiles, err := listCloudflareAuthProfiles()
	if err != nil {
		t.Fatalf("list profiles: %v", err)
	}
	if len(profiles) != 0 {
		t.Fatalf("expected 0 profiles, got %d", len(profiles))
	}
}

func TestResolveCloudflareAuthTokenForToolsMissing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg := defaultConfig(commandTools)
	cfg.NonInteractive = true
	if _, _, err := resolveCloudflareAuthTokenForTools(&cfg); err == nil {
		t.Fatalf("expected missing profile error")
	} else if !errors.Is(err, errSecretNotFound) && !strings.Contains(err.Error(), "not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunToolsFlowCloudflareZonesListUsesProfileToken(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := writeSecretValue(cloudflareAuthSecretName("default"), "token"); err != nil {
		t.Fatalf("write secret: %v", err)
	}
	var usedToken string
	stub := &stubCloudflareZonesService{
		listFn: func(ctx context.Context, apiToken string) ([]cloudflareZone, error) {
			usedToken = apiToken
			return []cloudflareZone{{ID: "zone-1", Name: "example.com", Type: "full"}}, nil
		},
	}
	original := cloudflareZonesClient
	cloudflareZonesClient = stub
	t.Cleanup(func() { cloudflareZonesClient = original })
	cfg := defaultConfig(commandTools)
	cfg.NonInteractive = true
	cfg.ToolsProvider = toolsProviderCloudflare
	cfg.ToolsAction = toolsActionCloudflareZone
	cfg.ToolsOperation = toolsOperationList
	if err := runToolsFlow(&cfg); err != nil {
		t.Fatalf("run tools flow: %v", err)
	}
	if usedToken != "token" {
		t.Fatalf("expected token used, got %q", usedToken)
	}
}

func TestRunToolsFlowCloudflareZonesCreate(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := writeSecretValue(cloudflareAuthSecretName("default"), "token"); err != nil {
		t.Fatalf("write secret: %v", err)
	}
	var lastRequest cloudflareZoneCreateRequest
	stub := &stubCloudflareZonesService{
		createFn: func(ctx context.Context, apiToken string, body cloudflareZoneCreateRequest) (cloudflareZone, error) {
			lastRequest = body
			return cloudflareZone{ID: "zone-1", Name: body.Name}, nil
		},
	}
	original := cloudflareZonesClient
	cloudflareZonesClient = stub
	t.Cleanup(func() { cloudflareZonesClient = original })
	cfg := defaultConfig(commandTools)
	cfg.NonInteractive = true
	cfg.ToolsProvider = toolsProviderCloudflare
	cfg.ToolsAction = toolsActionCloudflareZone
	cfg.ToolsOperation = toolsOperationCreate
	cfg.CloudflareZoneName = "example.com"
	cfg.CloudflareAccountID = "account-1"
	cfg.CloudflareZoneType = "Partial"
	if err := runToolsFlow(&cfg); err != nil {
		t.Fatalf("run create: %v", err)
	}
	if lastRequest.Name != "example.com" {
		t.Fatalf("unexpected name %q", lastRequest.Name)
	}
	if lastRequest.Account.ID != "account-1" {
		t.Fatalf("unexpected account %q", lastRequest.Account.ID)
	}
	if lastRequest.Type != "partial" {
		t.Fatalf("unexpected type %q", lastRequest.Type)
	}
}

func TestRunToolsFlowCloudflareZonesUpdateRequiresFields(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := writeSecretValue(cloudflareAuthSecretName("default"), "token"); err != nil {
		t.Fatalf("write secret: %v", err)
	}
	cfg := defaultConfig(commandTools)
	cfg.NonInteractive = true
	cfg.ToolsProvider = toolsProviderCloudflare
	cfg.ToolsAction = toolsActionCloudflareZone
	cfg.ToolsOperation = toolsOperationUpdate
	cfg.CloudflareZoneID = "zone-1"
	if err := runToolsFlow(&cfg); err == nil {
		t.Fatalf("expected error for missing update fields")
	}
}

func TestRunToolsFlowCloudflareZonesUpdateType(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := writeSecretValue(cloudflareAuthSecretName("default"), "token"); err != nil {
		t.Fatalf("write secret: %v", err)
	}
	var capturedType string
	stub := &stubCloudflareZonesService{
		updateFn: func(ctx context.Context, apiToken string, zoneID string, body cloudflareZoneUpdateRequest) (cloudflareZone, error) {
			if body.Type != nil {
				capturedType = *body.Type
			}
			return cloudflareZone{ID: zoneID}, nil
		},
	}
	original := cloudflareZonesClient
	cloudflareZonesClient = stub
	t.Cleanup(func() { cloudflareZonesClient = original })
	cfg := defaultConfig(commandTools)
	cfg.NonInteractive = true
	cfg.ToolsProvider = toolsProviderCloudflare
	cfg.ToolsAction = toolsActionCloudflareZone
	cfg.ToolsOperation = toolsOperationUpdate
	cfg.CloudflareZoneID = "zone-1"
	cfg.CloudflareZoneType = "FULL"
	if err := runToolsFlow(&cfg); err != nil {
		t.Fatalf("run update: %v", err)
	}
	if capturedType != "full" {
		t.Fatalf("expected type normalized to full, got %q", capturedType)
	}
}

func TestRunToolsFlowCloudflareZonesDelete(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := writeSecretValue(cloudflareAuthSecretName("default"), "token"); err != nil {
		t.Fatalf("write secret: %v", err)
	}
	var deletedID string
	stub := &stubCloudflareZonesService{
		deleteFn: func(ctx context.Context, apiToken string, zoneID string) (cloudflareZone, error) {
			deletedID = zoneID
			return cloudflareZone{ID: zoneID}, nil
		},
	}
	original := cloudflareZonesClient
	cloudflareZonesClient = stub
	t.Cleanup(func() { cloudflareZonesClient = original })
	cfg := defaultConfig(commandTools)
	cfg.NonInteractive = true
	cfg.ToolsProvider = toolsProviderCloudflare
	cfg.ToolsAction = toolsActionCloudflareZone
	cfg.ToolsOperation = toolsOperationDelete
	cfg.CloudflareZoneID = "zone-1"
	if err := runToolsFlow(&cfg); err != nil {
		t.Fatalf("run delete: %v", err)
	}
	if deletedID != "zone-1" {
		t.Fatalf("unexpected zone ID %q", deletedID)
	}
}
