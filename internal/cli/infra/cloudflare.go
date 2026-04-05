package infra

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	cloudflareAuthSecretPrefix = "cloudflare-auth-"
	cloudflareAPIBaseURL       = "https://api.cloudflare.com/client/v4"
)

var (
	cloudflareZonesClient    cloudflareZonesService = &cloudflareHTTPClient{client: http.DefaultClient}
	cloudflareRequestTimeout                        = 20 * time.Second
)

type cloudflareZone struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Paused  bool   `json:"paused"`
	Status  string `json:"status"`
	Account struct {
		ID string `json:"id"`
	} `json:"account"`
}

type cloudflareZoneCreateRequest struct {
	Name    string                  `json:"name"`
	Type    string                  `json:"type,omitempty"`
	Account cloudflareZoneAccountID `json:"account"`
}

type cloudflareZoneAccountID struct {
	ID string `json:"id"`
}

type cloudflareZoneUpdateRequest struct {
	Type   *string `json:"type,omitempty"`
	Paused *bool   `json:"paused,omitempty"`
}

type cloudflareZonesService interface {
	ListZones(ctx context.Context, apiToken string) ([]cloudflareZone, error)
	CreateZone(ctx context.Context, apiToken string, body cloudflareZoneCreateRequest) (cloudflareZone, error)
	UpdateZone(ctx context.Context, apiToken string, zoneID string, body cloudflareZoneUpdateRequest) (cloudflareZone, error)
	DeleteZone(ctx context.Context, apiToken string, zoneID string) (cloudflareZone, error)
}

type cloudflareHTTPClient struct {
	client *http.Client
}

type cloudflareAPIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type cloudflareAPIEnvelopeRaw struct {
	Success  bool                 `json:"success"`
	Errors   []cloudflareAPIError `json:"errors"`
	Messages []cloudflareAPIError `json:"messages"`
	Result   json.RawMessage      `json:"result"`
}

func runAuthFlow(cfg *config) error {
	provider := strings.ToLower(strings.TrimSpace(cfg.AuthProvider))
	if provider == "" {
		return fmt.Errorf("auth provider is required")
	}

	switch provider {
	case authProviderCloudflare:
		return runCloudflareAuthFlow(cfg)
	default:
		return fmt.Errorf("unknown auth provider: %s", provider)
	}
}

func runCloudflareAuthFlow(cfg *config) error {
	action := strings.ToLower(strings.TrimSpace(cfg.AuthAction))
	if action == "" {
		action = authActionList
	}

	switch action {
	case authActionSet:
		return runCloudflareAuthSetFlow(cfg)
	case authActionGet:
		return runCloudflareAuthGetFlow(cfg)
	case authActionList:
		return runCloudflareAuthListFlow(cfg)
	case authActionRemove:
		return runCloudflareAuthRemoveFlow(cfg)
	default:
		return fmt.Errorf("unknown cloudflare auth action: %s", action)
	}
}

func runCloudflareAuthSetFlow(cfg *config) error {
	profile, err := normalizeCloudflareAuthProfile(cfg.AuthProfile)
	if err != nil {
		return err
	}
	token, err := resolveCloudflareAuthSetToken(cfg)
	if err != nil {
		return err
	}
	secretName := cloudflareAuthSecretName(profile)
	if err := writeSecretValue(secretName, token); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "✅ Cloudflare auth profile %q saved\n", profile)
	return nil
}

func runCloudflareAuthGetFlow(cfg *config) error {
	profile, err := normalizeCloudflareAuthProfile(cfg.AuthProfile)
	if err != nil {
		return err
	}
	secretName := cloudflareAuthSecretName(profile)
	if _, err := readSecretValue(secretName); err != nil {
		if errors.Is(err, errSecretNotFound) {
			return fmt.Errorf("cloudflare auth profile %q not found", profile)
		}
		return err
	}
	printSection("Cloudflare Auth Profile")
	fmt.Fprintf(os.Stderr, "Profile: %s\n", profile)
	fmt.Fprintf(os.Stderr, "Token: stored\n")
	return nil
}

func runCloudflareAuthListFlow(cfg *config) error {
	profiles, err := listCloudflareAuthProfiles()
	if err != nil {
		return err
	}
	if len(profiles) == 0 {
		fmt.Fprintln(os.Stderr, "No Cloudflare auth profiles stored. Use `civa auth cloudflare set <profile>`.")
		return nil
	}
	printSection("Cloudflare Auth Profiles")
	for _, profile := range profiles {
		fmt.Fprintf(os.Stderr, "- %s\n", profile)
	}
	return nil
}

func runCloudflareAuthRemoveFlow(cfg *config) error {
	profile, err := normalizeCloudflareAuthProfile(cfg.AuthProfile)
	if err != nil {
		return err
	}
	secretName := cloudflareAuthSecretName(profile)
	removed, err := removeSecretValue(secretName)
	if err != nil {
		return err
	}
	if !removed {
		fmt.Fprintf(os.Stderr, "Cloudflare auth profile %q was not found\n", profile)
		return nil
	}
	fmt.Fprintf(os.Stderr, "✅ Cloudflare auth profile %q removed\n", profile)
	return nil
}

func normalizeCloudflareAuthProfile(raw string) (string, error) {
	profile := strings.TrimSpace(raw)
	if profile == "" {
		return "", fmt.Errorf("cloudflare auth profile name must not be empty")
	}
	return profile, nil
}

func cloudflareAuthSecretName(profile string) string {
	return cloudflareAuthSecretPrefix + profile
}

func resolveCloudflareAuthSetToken(cfg *config) (string, error) {
	token := strings.TrimSpace(cfg.AuthToken)
	if cfg.Provided.AuthToken {
		if token == "" {
			return "", fmt.Errorf("--token must not be empty")
		}
		return token, nil
	}
	if !shouldPrompt(cfg) {
		return "", fmt.Errorf("cloudflare API token is required; use --token")
	}
	value, err := promptSecretValue("Cloudflare API token")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(value), nil
}

func listCloudflareAuthProfiles() ([]string, error) {
	names, err := listSecretNames()
	if err != nil {
		return nil, err
	}
	profiles := make([]string, 0, len(names))
	for _, name := range names {
		if !strings.HasPrefix(name, cloudflareAuthSecretPrefix) {
			continue
		}
		profile := strings.TrimPrefix(name, cloudflareAuthSecretPrefix)
		if profile == "" {
			continue
		}
		profiles = append(profiles, profile)
	}
	sort.Strings(profiles)
	return profiles, nil
}

func runToolsFlow(cfg *config) error {
	if cfg.ToolsProvider == "" {
		provider, err := resolveToolsProvider(cfg)
		if err != nil {
			return err
		}
		cfg.ToolsProvider = provider
	}
	provider := strings.ToLower(strings.TrimSpace(cfg.ToolsProvider))
	if provider == "" {
		return nil
	}
	switch provider {
	case toolsProviderCloudflare:
		return runCloudflareToolsFlow(cfg)
	default:
		return fmt.Errorf("unknown tools provider: %s", provider)
	}
}

func resolveToolsProvider(cfg *config) (string, error) {
	provider := strings.ToLower(strings.TrimSpace(cfg.ToolsProvider))
	if provider != "" {
		return provider, nil
	}
	if !shouldPrompt(cfg) {
		return "", fmt.Errorf("civa tools requires a provider (available: cloudflare)")
	}
	value, err := promptToolsProvider(toolsProviderCloudflare)
	if err != nil {
		if errors.Is(err, errUserCancelled) {
			return "", nil
		}
		return "", err
	}
	return strings.ToLower(strings.TrimSpace(value)), nil
}

func runCloudflareToolsFlow(cfg *config) error {
	action, err := resolveCloudflareToolsAction(cfg)
	if err != nil {
		return err
	}
	if action == "" {
		return nil
	}
	switch action {
	case toolsActionCloudflareZone:
		return runCloudflareZonesFlow(cfg)
	default:
		return fmt.Errorf("unknown cloudflare tools action: %s", action)
	}
}

func resolveCloudflareToolsAction(cfg *config) (string, error) {
	action := strings.ToLower(strings.TrimSpace(cfg.ToolsAction))
	if action != "" {
		return action, nil
	}
	if !shouldPrompt(cfg) {
		return "", fmt.Errorf("civa tools cloudflare requires an action (available: zones)")
	}
	value, err := promptCloudflareToolsAction(toolsActionCloudflareZone)
	if err != nil {
		if errors.Is(err, errUserCancelled) {
			return "", nil
		}
		return "", err
	}
	return strings.ToLower(strings.TrimSpace(value)), nil
}

func runCloudflareZonesFlow(cfg *config) error {
	operation, err := resolveCloudflareZonesOperation(cfg)
	if err != nil {
		return err
	}
	if operation == "" {
		return nil
	}
	switch operation {
	case toolsOperationList:
		return runCloudflareZonesListFlow(cfg)
	case toolsOperationCreate:
		return runCloudflareZonesCreateFlow(cfg)
	case toolsOperationUpdate:
		return runCloudflareZonesUpdateFlow(cfg)
	case toolsOperationDelete:
		return runCloudflareZonesDeleteFlow(cfg)
	default:
		return fmt.Errorf("unknown cloudflare zones operation: %s", operation)
	}
}

func resolveCloudflareZonesOperation(cfg *config) (string, error) {
	operation := strings.ToLower(strings.TrimSpace(cfg.ToolsOperation))
	if operation != "" {
		return operation, nil
	}
	if !shouldPrompt(cfg) {
		return "", fmt.Errorf("civa tools cloudflare zones requires an operation (list|create|update|delete)")
	}
	value, err := promptCloudflareZonesOperation(toolsOperationList)
	if err != nil {
		if errors.Is(err, errUserCancelled) {
			return "", nil
		}
		return "", err
	}
	return strings.ToLower(strings.TrimSpace(value)), nil
}

func runCloudflareZonesListFlow(cfg *config) error {
	token, profile, err := resolveCloudflareAuthTokenForTools(cfg)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), cloudflareRequestTimeout)
	defer cancel()
	zones, err := cloudflareZonesClient.ListZones(ctx, token)
	if err != nil {
		return fmt.Errorf("list cloudflare zones: %w", err)
	}
	sort.Slice(zones, func(i, j int) bool {
		return zones[i].Name < zones[j].Name
	})
	printSection("Cloudflare Zones")
	fmt.Fprintf(os.Stderr, "Auth profile: %s\n", profile)
	if len(zones) == 0 {
		fmt.Fprintln(os.Stderr, "No zones found for this API token.")
		return nil
	}
	for _, zone := range zones {
		fmt.Fprintf(os.Stderr, "- %s (ID: %s, Type: %s, Paused: %t, Account: %s)\n", zone.Name, zone.ID, zone.Type, zone.Paused, zone.Account.ID)
	}
	return nil
}

func runCloudflareZonesCreateFlow(cfg *config) error {
	if err := ensureCloudflareZoneCreateInputs(cfg); err != nil {
		return err
	}
	token, _, err := resolveCloudflareAuthTokenForTools(cfg)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), cloudflareRequestTimeout)
	defer cancel()
	zoneType, err := normalizeCloudflareZoneType(cfg.CloudflareZoneType)
	if err != nil {
		return err
	}
	req := cloudflareZoneCreateRequest{
		Name: cfg.CloudflareZoneName,
		Account: cloudflareZoneAccountID{
			ID: cfg.CloudflareAccountID,
		},
	}
	if zoneType != "" {
		req.Type = zoneType
	}
	zone, err := cloudflareZonesClient.CreateZone(ctx, token, req)
	if err != nil {
		return fmt.Errorf("create cloudflare zone: %w", err)
	}
	printSection("Cloudflare Zones")
	fmt.Fprintf(os.Stderr, "✅ Created zone %s (ID: %s)\n", zone.Name, zone.ID)
	return nil
}

func ensureCloudflareZoneCreateInputs(cfg *config) error {
	cfg.CloudflareZoneName = strings.TrimSpace(cfg.CloudflareZoneName)
	cfg.CloudflareAccountID = strings.TrimSpace(cfg.CloudflareAccountID)
	if cfg.CloudflareZoneName == "" {
		if !shouldPrompt(cfg) {
			return fmt.Errorf("zone create requires --name")
		}
		value, err := promptNonEmptyString("Cloudflare zone name", "")
		if err != nil {
			return err
		}
		cfg.CloudflareZoneName = value
	}
	if cfg.CloudflareAccountID == "" {
		if !shouldPrompt(cfg) {
			return fmt.Errorf("zone create requires --account-id")
		}
		value, err := promptNonEmptyString("Cloudflare account ID", "")
		if err != nil {
			return err
		}
		cfg.CloudflareAccountID = value
	}
	return nil
}

func runCloudflareZonesUpdateFlow(cfg *config) error {
	if err := ensureCloudflareZoneUpdateInputs(cfg); err != nil {
		return err
	}
	token, _, err := resolveCloudflareAuthTokenForTools(cfg)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), cloudflareRequestTimeout)
	defer cancel()
	zoneType, err := normalizeCloudflareZoneType(cfg.CloudflareZoneType)
	if err != nil {
		return err
	}
	var pausedValue *bool
	if strings.TrimSpace(cfg.CloudflareZonePausedInput) != "" {
		parsed, err := strconv.ParseBool(strings.TrimSpace(cfg.CloudflareZonePausedInput))
		if err != nil {
			return fmt.Errorf("invalid paused value %q: %w", cfg.CloudflareZonePausedInput, err)
		}
		pausedValue = &parsed
	}
	if zoneType == "" && pausedValue == nil {
		return fmt.Errorf("zone update requires --type or --paused")
	}
	req := cloudflareZoneUpdateRequest{Type: nil, Paused: pausedValue}
	if zoneType != "" {
		req.Type = &zoneType
	}
	zone, err := cloudflareZonesClient.UpdateZone(ctx, token, cfg.CloudflareZoneID, req)
	if err != nil {
		return fmt.Errorf("update cloudflare zone: %w", err)
	}
	printSection("Cloudflare Zones")
	fmt.Fprintf(os.Stderr, "✅ Updated zone %s (ID: %s)\n", zone.Name, zone.ID)
	return nil
}

func ensureCloudflareZoneUpdateInputs(cfg *config) error {
	cfg.CloudflareZoneID = strings.TrimSpace(cfg.CloudflareZoneID)
	if cfg.CloudflareZoneID == "" {
		if !shouldPrompt(cfg) {
			return fmt.Errorf("zone update requires --zone-id")
		}
		value, err := promptNonEmptyString("Cloudflare zone ID", "")
		if err != nil {
			return err
		}
		cfg.CloudflareZoneID = value
	}
	if cfg.CloudflareZonePausedInput != "" {
		cfg.CloudflareZonePausedInput = strings.TrimSpace(cfg.CloudflareZonePausedInput)
	}
	if cfg.CloudflareZoneType != "" {
		cfg.CloudflareZoneType = strings.TrimSpace(cfg.CloudflareZoneType)
	}
	return nil
}

func runCloudflareZonesDeleteFlow(cfg *config) error {
	cfg.CloudflareZoneID = strings.TrimSpace(cfg.CloudflareZoneID)
	if cfg.CloudflareZoneID == "" {
		if !shouldPrompt(cfg) {
			return fmt.Errorf("zone delete requires --zone-id")
		}
		value, err := promptNonEmptyString("Cloudflare zone ID", "")
		if err != nil {
			return err
		}
		cfg.CloudflareZoneID = value
	}
	token, _, err := resolveCloudflareAuthTokenForTools(cfg)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), cloudflareRequestTimeout)
	defer cancel()
	z, err := cloudflareZonesClient.DeleteZone(ctx, token, cfg.CloudflareZoneID)
	if err != nil {
		return fmt.Errorf("delete cloudflare zone: %w", err)
	}
	printSection("Cloudflare Zones")
	fmt.Fprintf(os.Stderr, "✅ Deleted zone %s (ID: %s)\n", z.Name, z.ID)
	return nil
}

func resolveCloudflareAuthTokenForTools(cfg *config) (string, string, error) {
	profile := strings.TrimSpace(cfg.AuthProfile)
	if profile == "" {
		profile = "default"
	}
	secretName := cloudflareAuthSecretName(profile)
	token, err := readSecretValue(secretName)
	if err != nil {
		if errors.Is(err, errSecretNotFound) {
			return "", profile, fmt.Errorf("cloudflare auth profile %q not found; run `civa auth cloudflare set %s --token <value>`", profile, profile)
		}
		return "", profile, fmt.Errorf("load cloudflare auth profile %q: %w", profile, err)
	}
	return token, profile, nil
}

func normalizeCloudflareZoneType(raw string) (string, error) {
	trimmed := strings.TrimSpace(strings.ToLower(raw))
	if trimmed == "" {
		return "", nil
	}
	switch trimmed {
	case "full", "partial", "secondary", "internal":
		return trimmed, nil
	default:
		return "", fmt.Errorf("unsupported zone type: %s", raw)
	}
}

func (c *cloudflareHTTPClient) ListZones(ctx context.Context, apiToken string) ([]cloudflareZone, error) {
	var result []cloudflareZone
	if err := c.doCloudflareRequest(ctx, http.MethodGet, "/zones", apiToken, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *cloudflareHTTPClient) CreateZone(ctx context.Context, apiToken string, body cloudflareZoneCreateRequest) (cloudflareZone, error) {
	var result cloudflareZone
	if err := c.doCloudflareRequest(ctx, http.MethodPost, "/zones", apiToken, body, &result); err != nil {
		return cloudflareZone{}, err
	}
	return result, nil
}

func (c *cloudflareHTTPClient) UpdateZone(ctx context.Context, apiToken string, zoneID string, body cloudflareZoneUpdateRequest) (cloudflareZone, error) {
	var result cloudflareZone
	if err := c.doCloudflareRequest(ctx, http.MethodPatch, fmt.Sprintf("/zones/%s", zoneID), apiToken, body, &result); err != nil {
		return cloudflareZone{}, err
	}
	return result, nil
}

func (c *cloudflareHTTPClient) DeleteZone(ctx context.Context, apiToken string, zoneID string) (cloudflareZone, error) {
	var result cloudflareZone
	if err := c.doCloudflareRequest(ctx, http.MethodDelete, fmt.Sprintf("/zones/%s", zoneID), apiToken, nil, &result); err != nil {
		return cloudflareZone{}, err
	}
	return result, nil
}

func (c *cloudflareHTTPClient) doCloudflareRequest(ctx context.Context, method, path, apiToken string, body interface{}, result interface{}) error {
	url := cloudflareAPIBaseURL + path
	var payload io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		payload = bytes.NewBuffer(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, payload)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s %s: %s", method, path, strings.TrimSpace(string(content)))
	}
	var envelope cloudflareAPIEnvelopeRaw
	if err := json.Unmarshal(content, &envelope); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
	if !envelope.Success {
		return fmt.Errorf("%s %s: %s", method, path, formatCloudflareErrors(envelope.Errors, envelope.Messages))
	}
	if result != nil && len(envelope.Result) > 0 {
		if err := json.Unmarshal(envelope.Result, result); err != nil {
			return fmt.Errorf("parse response result: %w", err)
		}
	}
	return nil
}

func formatCloudflareErrors(errorsList, messages []cloudflareAPIError) string {
	parts := make([]string, 0)
	for _, err := range errorsList {
		parts = append(parts, fmt.Sprintf("%d: %s", err.Code, err.Message))
	}
	for _, msg := range messages {
		parts = append(parts, fmt.Sprintf("%d: %s", msg.Code, msg.Message))
	}
	if len(parts) == 0 {
		return "unknown error"
	}
	return strings.Join(parts, "; ")
}
