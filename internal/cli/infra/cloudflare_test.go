package infra

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func setInteractivePromptMode(t *testing.T, enabled bool) {
	t.Helper()
	original := shouldPromptIsTerminalFn
	shouldPromptIsTerminalFn = func(int) bool { return enabled }
	t.Cleanup(func() { shouldPromptIsTerminalFn = original })
}

func setCloudflarePromptOverrides(t *testing.T) {
	t.Helper()
	originalSecret := promptSecretValueFn
	originalProfile := promptCloudflareAuthProfileFn
	originalZoneSelection := promptCloudflareZoneSelectionFn
	originalZoneType := promptCloudflareZoneTypeFn
	originalZonePaused := promptCloudflareZonePausedFn
	originalUpdateField := promptCloudflareZoneUpdateFieldFn
	originalTunnelOp := promptCloudflareTunnelsOperationFn
	originalTunnelSel := promptCloudflareTunnelSelectionFn
	t.Cleanup(func() {
		promptSecretValueFn = originalSecret
		promptCloudflareAuthProfileFn = originalProfile
		promptCloudflareZoneSelectionFn = originalZoneSelection
		promptCloudflareZoneTypeFn = originalZoneType
		promptCloudflareZonePausedFn = originalZonePaused
		promptCloudflareZoneUpdateFieldFn = originalUpdateField
		promptCloudflareTunnelsOperationFn = originalTunnelOp
		promptCloudflareTunnelSelectionFn = originalTunnelSel
	})
}

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

func TestResolveCloudflareAuthTokenForToolsPromptsProfile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	setInteractivePromptMode(t, true)
	setCloudflarePromptOverrides(t)
	if err := writeSecretValue(cloudflareAuthSecretName("default"), "default-token"); err != nil {
		t.Fatalf("write default secret: %v", err)
	}
	if err := writeSecretValue(cloudflareAuthSecretName("work"), "work-token"); err != nil {
		t.Fatalf("write work secret: %v", err)
	}
	promptCloudflareAuthProfileFn = func(defaultValue string, profiles []string) (string, error) {
		if defaultValue != "default" {
			t.Fatalf("expected default profile, got %q", defaultValue)
		}
		if len(profiles) != 2 {
			t.Fatalf("expected 2 profiles, got %d", len(profiles))
		}
		return "work", nil
	}

	cfg := defaultConfig(commandTools)
	token, profile, err := resolveCloudflareAuthTokenForTools(&cfg)
	if err != nil {
		t.Fatalf("resolve token: %v", err)
	}
	if profile != "work" {
		t.Fatalf("expected prompted profile work, got %q", profile)
	}
	if token != "work-token" {
		t.Fatalf("expected prompted token work-token, got %q", token)
	}
}

func TestRunAuthFlowCloudflareSetCancellationReturnsNil(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	setInteractivePromptMode(t, true)
	setCloudflarePromptOverrides(t)
	cfg := defaultConfig(commandAuth)
	cfg.AuthProvider = authProviderCloudflare
	cfg.AuthAction = authActionSet
	cfg.AuthProfile = "default"
	promptSecretValueFn = func(string) (string, error) {
		return "", errUserCancelled
	}
	if err := runAuthFlow(&cfg); err != nil {
		t.Fatalf("expected clean cancellation, got %v", err)
	}
	profiles, err := listCloudflareAuthProfiles()
	if err != nil {
		t.Fatalf("list profiles: %v", err)
	}
	if len(profiles) != 0 {
		t.Fatalf("expected no saved profiles after cancellation, got %d", len(profiles))
	}
}

func TestRunToolsFlowCloudflareCancellationReturnsNil(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	setInteractivePromptMode(t, true)
	setCloudflarePromptOverrides(t)
	if err := writeSecretValue(cloudflareAuthSecretName("default"), "token"); err != nil {
		t.Fatalf("write secret: %v", err)
	}
	promptCloudflareAuthProfileFn = func(string, []string) (string, error) {
		return "", errUserCancelled
	}
	cfg := defaultConfig(commandTools)
	cfg.ToolsProvider = toolsProviderCloudflare
	cfg.ToolsAction = toolsActionCloudflareZone
	cfg.ToolsOperation = toolsOperationList
	if err := runToolsFlow(&cfg); err != nil {
		t.Fatalf("expected clean cancellation, got %v", err)
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

func TestEnsureCloudflareZoneUpdateInputsPromptsZoneAndPaused(t *testing.T) {
	setInteractivePromptMode(t, true)
	setCloudflarePromptOverrides(t)
	promptCloudflareZoneSelectionFn = func(title string, availableZones []cloudflareZone, defaultValue string) (string, error) {
		if title != "Cloudflare zone to update" {
			t.Fatalf("unexpected selection title %q", title)
		}
		if len(availableZones) != 2 {
			t.Fatalf("expected 2 zones, got %d", len(availableZones))
		}
		return "zone-2", nil
	}
	promptCloudflareZoneUpdateFieldFn = func() (string, error) {
		return "paused", nil
	}
	promptCloudflareZonePausedFn = func(defaultValue string) (string, error) {
		if defaultValue != "false" {
			t.Fatalf("unexpected paused default %q", defaultValue)
		}
		return "true", nil
	}

	original := cloudflareZonesClient
	cloudflareZonesClient = &stubCloudflareZonesService{
		listFn: func(ctx context.Context, apiToken string) ([]cloudflareZone, error) {
			return []cloudflareZone{
				{ID: "zone-2", Name: "beta.example", Type: "full", Status: "active"},
				{ID: "zone-1", Name: "alpha.example", Type: "partial", Status: "pending"},
			}, nil
		},
	}
	t.Cleanup(func() { cloudflareZonesClient = original })

	cfg := defaultConfig(commandTools)
	if err := ensureCloudflareZoneUpdateInputs(context.Background(), &cfg, "token"); err != nil {
		t.Fatalf("ensure update inputs: %v", err)
	}
	if cfg.CloudflareZoneID != "zone-2" {
		t.Fatalf("expected prompted zone id zone-2, got %q", cfg.CloudflareZoneID)
	}
	if cfg.CloudflareZonePausedInput != "true" {
		t.Fatalf("expected prompted paused input true, got %q", cfg.CloudflareZonePausedInput)
	}
	if cfg.CloudflareZoneType != "" {
		t.Fatalf("expected empty zone type, got %q", cfg.CloudflareZoneType)
	}
}

type stubCloudflareTunnelsService struct {
	listFn   func(ctx context.Context, apiToken string, accountID string) ([]cloudflareTunnel, error)
	createFn func(ctx context.Context, apiToken string, accountID string, name string) (cloudflareTunnel, error)
	getFn    func(ctx context.Context, apiToken string, accountID string, tunnelID string) (cloudflareTunnel, error)
	deleteFn func(ctx context.Context, apiToken string, accountID string, tunnelID string) error
	routeFn  func(ctx context.Context, apiToken string, accountID string, tunnelID string, hostname string, serviceURL string, zoneID string) error
}

func (s *stubCloudflareTunnelsService) ListTunnels(ctx context.Context, apiToken string, accountID string) ([]cloudflareTunnel, error) {
	if s == nil || s.listFn == nil {
		return nil, nil
	}
	return s.listFn(ctx, apiToken, accountID)
}

func (s *stubCloudflareTunnelsService) CreateTunnel(ctx context.Context, apiToken string, accountID string, name string) (cloudflareTunnel, error) {
	if s == nil || s.createFn == nil {
		return cloudflareTunnel{}, nil
	}
	return s.createFn(ctx, apiToken, accountID, name)
}

func (s *stubCloudflareTunnelsService) GetTunnel(ctx context.Context, apiToken string, accountID string, tunnelID string) (cloudflareTunnel, error) {
	if s == nil || s.getFn == nil {
		return cloudflareTunnel{}, nil
	}
	return s.getFn(ctx, apiToken, accountID, tunnelID)
}

func (s *stubCloudflareTunnelsService) DeleteTunnel(ctx context.Context, apiToken string, accountID string, tunnelID string) error {
	if s == nil || s.deleteFn == nil {
		return nil
	}
	return s.deleteFn(ctx, apiToken, accountID, tunnelID)
}

func (s *stubCloudflareTunnelsService) RouteTunnel(ctx context.Context, apiToken string, accountID string, tunnelID string, hostname string, serviceURL string, zoneID string) error {
	if s == nil || s.routeFn == nil {
		return nil
	}
	return s.routeFn(ctx, apiToken, accountID, tunnelID, hostname, serviceURL, zoneID)
}

func TestRunCloudflareTunnelsFlows(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := writeSecretValue(cloudflareAuthSecretName("default"), "mock-token"); err != nil {
		t.Fatalf("write secret: %v", err)
	}

	var createdName, deletedID, routedHost, routedService string
	orig := cloudflareTunnelsClient
	cloudflareTunnelsClient = &stubCloudflareTunnelsService{
		listFn: func(ctx context.Context, apiToken string, accountID string) ([]cloudflareTunnel, error) {
			return []cloudflareTunnel{
				{ID: "t-1", Name: "my-tunnel", Status: "healthy"},
			}, nil
		},
		createFn: func(ctx context.Context, apiToken string, accountID string, name string) (cloudflareTunnel, error) {
			createdName = name
			return cloudflareTunnel{ID: "t-2", Name: name, Status: "healthy"}, nil
		},
		getFn: func(ctx context.Context, apiToken string, accountID string, tunnelID string) (cloudflareTunnel, error) {
			return cloudflareTunnel{ID: tunnelID, Name: "my-tunnel", Status: "healthy"}, nil
		},
		deleteFn: func(ctx context.Context, apiToken string, accountID string, tunnelID string) error {
			deletedID = tunnelID
			return nil
		},
		routeFn: func(ctx context.Context, apiToken string, accountID string, tunnelID string, hostname string, serviceURL string, zoneID string) error {
			routedHost = hostname
			routedService = serviceURL
			return nil
		},
	}
	t.Cleanup(func() { cloudflareTunnelsClient = orig })

	cfgList := defaultConfig(commandTools)
	cfgList.ToolsProvider = toolsProviderCloudflare
	cfgList.ToolsAction = toolsActionCloudflareTunnel
	cfgList.ToolsOperation = toolsOperationList
	cfgList.CloudflareAccountID = "acc-123"
	if err := runToolsFlow(&cfgList); err != nil {
		t.Fatalf("tunnels list flow error: %v", err)
	}

	cfgCreate := defaultConfig(commandTools)
	cfgCreate.ToolsProvider = toolsProviderCloudflare
	cfgCreate.ToolsAction = toolsActionCloudflareTunnel
	cfgCreate.ToolsOperation = toolsOperationCreate
	cfgCreate.CloudflareAccountID = "acc-123"
	cfgCreate.CloudflareTunnelName = "new-tunnel"
	if err := runToolsFlow(&cfgCreate); err != nil {
		t.Fatalf("tunnels create flow error: %v", err)
	}
	if createdName != "new-tunnel" {
		t.Fatalf("expected created name new-tunnel, got %q", createdName)
	}

	cfgGet := defaultConfig(commandTools)
	cfgGet.ToolsProvider = toolsProviderCloudflare
	cfgGet.ToolsAction = toolsActionCloudflareTunnel
	cfgGet.ToolsOperation = toolsOperationGet
	cfgGet.CloudflareAccountID = "acc-123"
	cfgGet.CloudflareTunnelID = "t-1"
	if err := runToolsFlow(&cfgGet); err != nil {
		t.Fatalf("tunnels get flow error: %v", err)
	}

	cfgDelete := defaultConfig(commandTools)
	cfgDelete.ToolsProvider = toolsProviderCloudflare
	cfgDelete.ToolsAction = toolsActionCloudflareTunnel
	cfgDelete.ToolsOperation = toolsOperationDelete
	cfgDelete.CloudflareAccountID = "acc-123"
	cfgDelete.CloudflareTunnelID = "t-1"
	if err := runToolsFlow(&cfgDelete); err != nil {
		t.Fatalf("tunnels delete flow error: %v", err)
	}
	if deletedID != "t-1" {
		t.Fatalf("expected deleted id t-1, got %q", deletedID)
	}

	cfgRoute := defaultConfig(commandTools)
	cfgRoute.ToolsProvider = toolsProviderCloudflare
	cfgRoute.ToolsAction = toolsActionCloudflareTunnel
	cfgRoute.ToolsOperation = toolsOperationRoute
	cfgRoute.CloudflareAccountID = "acc-123"
	cfgRoute.CloudflareTunnelID = "t-1"
	cfgRoute.CloudflareHostname = "app.example.com"
	cfgRoute.CloudflareService = "http://localhost:8080"
	if err := runToolsFlow(&cfgRoute); err != nil {
		t.Fatalf("tunnels route flow error: %v", err)
	}
	if routedHost != "app.example.com" || routedService != "http://localhost:8080" {
		t.Fatalf("unexpected route params: host=%q service=%q", routedHost, routedService)
	}
}
