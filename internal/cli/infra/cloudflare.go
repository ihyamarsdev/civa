package infra

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	cloudflare "github.com/cloudflare/cloudflare-go/v6"
	"github.com/cloudflare/cloudflare-go/v6/dns"
	"github.com/cloudflare/cloudflare-go/v6/option"
	"github.com/cloudflare/cloudflare-go/v6/zero_trust"
	"github.com/cloudflare/cloudflare-go/v6/zones"
)

const (
	cloudflareAuthSecretPrefix = "cloudflare-auth-"
)

var (
	cloudflareZonesClient    cloudflareZonesService   = &cloudflareSDKClient{}
	cloudflareTunnelsClient  cloudflareTunnelsService = &cloudflareSDKClient{}
	cloudflareRequestTimeout                          = 20 * time.Second
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

type cloudflareTunnel struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type cloudflareTunnelsService interface {
	ListTunnels(ctx context.Context, apiToken string, accountID string) ([]cloudflareTunnel, error)
	CreateTunnel(ctx context.Context, apiToken string, accountID string, name string) (cloudflareTunnel, error)
	GetTunnel(ctx context.Context, apiToken string, accountID string, tunnelID string) (cloudflareTunnel, error)
	DeleteTunnel(ctx context.Context, apiToken string, accountID string, tunnelID string) error
	RouteTunnel(ctx context.Context, apiToken string, accountID string, tunnelID string, hostname string, serviceURL string, zoneID string) error
}

type cloudflareSDKClient struct{}

func runAuthFlow(cfg *config) error {
	provider := strings.ToLower(strings.TrimSpace(cfg.AuthProvider))
	if provider == "" {
		return fmt.Errorf("auth provider is required")
	}

	var err error
	switch provider {
	case authProviderCloudflare:
		err = runCloudflareAuthFlow(cfg)
	default:
		return fmt.Errorf("unknown auth provider: %s", provider)
	}
	if errors.Is(err, errUserCancelled) {
		return nil
	}
	return err
}

func runCloudflareAuthFlow(cfg *config) error {
	action := strings.ToLower(strings.TrimSpace(cfg.AuthAction))
	if action == "" {
		action = authActionList
	}

	switch action {
	case authActionLogin:
		return runCloudflareAuthLoginFlow(cfg)
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

func runCloudflareAuthListFlow(_ *config) error {
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
	value, err := promptSecretValueFn("Cloudflare API token")
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
	var err error
	switch provider {
	case toolsProviderCloudflare:
		err = runCloudflareToolsFlow(cfg)
	default:
		return fmt.Errorf("unknown tools provider: %s", provider)
	}
	if errors.Is(err, errUserCancelled) {
		return nil
	}
	return err
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
	case toolsActionCloudflareTunnel:
		return runCloudflareTunnelsFlow(cfg)
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
		return "", fmt.Errorf("civa tools cloudflare requires an action (available: zones, tunnels)")
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
	token, _, err := resolveCloudflareAuthTokenForTools(cfg)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), cloudflareRequestTimeout)
	defer cancel()
	if err := ensureCloudflareZoneUpdateInputs(ctx, cfg, token); err != nil {
		return err
	}
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

func ensureCloudflareZoneUpdateInputs(ctx context.Context, cfg *config, apiToken string) error {
	cfg.CloudflareZoneID = strings.TrimSpace(cfg.CloudflareZoneID)
	if cfg.CloudflareZoneID == "" {
		if !shouldPrompt(cfg) {
			return fmt.Errorf("zone update requires --zone-id")
		}
		value, err := promptCloudflareZoneIDForOperation(ctx, apiToken, "Cloudflare zone to update")
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
	if cfg.CloudflareZoneType == "" && cfg.CloudflareZonePausedInput == "" {
		if !shouldPrompt(cfg) {
			return nil
		}
		field, err := promptCloudflareZoneUpdateFieldFn()
		if err != nil {
			return err
		}
		switch field {
		case "type":
			value, err := promptCloudflareZoneTypeFn("", false)
			if err != nil {
				return err
			}
			cfg.CloudflareZoneType = value
		case "paused":
			value, err := promptCloudflareZonePausedFn("false")
			if err != nil {
				return err
			}
			cfg.CloudflareZonePausedInput = value
		}
	}
	return nil
}

func runCloudflareZonesDeleteFlow(cfg *config) error {
	token, _, err := resolveCloudflareAuthTokenForTools(cfg)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), cloudflareRequestTimeout)
	defer cancel()
	if err := ensureCloudflareZoneDeleteInputs(ctx, cfg, token); err != nil {
		return err
	}
	z, err := cloudflareZonesClient.DeleteZone(ctx, token, cfg.CloudflareZoneID)
	if err != nil {
		return fmt.Errorf("delete cloudflare zone: %w", err)
	}
	printSection("Cloudflare Zones")
	fmt.Fprintf(os.Stderr, "✅ Deleted zone %s (ID: %s)\n", z.Name, z.ID)
	return nil
}

func ensureCloudflareZoneDeleteInputs(ctx context.Context, cfg *config, apiToken string) error {
	cfg.CloudflareZoneID = strings.TrimSpace(cfg.CloudflareZoneID)
	if cfg.CloudflareZoneID != "" {
		return nil
	}
	if !shouldPrompt(cfg) {
		return fmt.Errorf("zone delete requires --zone-id")
	}
	value, err := promptCloudflareZoneIDForOperation(ctx, apiToken, "Cloudflare zone to delete")
	if err != nil {
		return err
	}
	cfg.CloudflareZoneID = value
	return nil
}

func resolveCloudflareAuthTokenForTools(cfg *config) (string, string, error) {
	profile, err := resolveCloudflareAuthProfileForTools(cfg)
	if err != nil {
		return "", "", err
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

func resolveCloudflareAuthProfileForTools(cfg *config) (string, error) {
	profile := strings.TrimSpace(cfg.AuthProfile)
	if profile != "" {
		return normalizeCloudflareAuthProfile(profile)
	}
	if !shouldPrompt(cfg) {
		return "default", nil
	}
	profiles, err := listCloudflareAuthProfiles()
	if err != nil {
		return "", err
	}
	if len(profiles) == 0 {
		return "", fmt.Errorf("no cloudflare auth profiles stored; run `civa auth cloudflare set <profile> --token <value>`")
	}
	defaultProfile := profiles[0]
	for _, candidate := range profiles {
		if candidate == "default" {
			defaultProfile = candidate
			break
		}
	}
	selected, err := promptCloudflareAuthProfileFn(defaultProfile, profiles)
	if err != nil {
		return "", err
	}
	return normalizeCloudflareAuthProfile(selected)
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

func promptCloudflareZoneIDForOperation(ctx context.Context, apiToken string, title string) (string, error) {
	availableZones, err := cloudflareZonesClient.ListZones(ctx, apiToken)
	if err != nil {
		return "", fmt.Errorf("load cloudflare zones for selection: %w", err)
	}
	if len(availableZones) == 0 {
		return "", fmt.Errorf("no cloudflare zones found for this API token")
	}
	sort.Slice(availableZones, func(i, j int) bool {
		return availableZones[i].Name < availableZones[j].Name
	})
	return promptCloudflareZoneSelectionFn(title, availableZones, availableZones[0].ID)
}

func (c *cloudflareSDKClient) ListZones(ctx context.Context, apiToken string) ([]cloudflareZone, error) {
	client := newCloudflareClient(apiToken)
	iter := client.Zones.ListAutoPaging(ctx, zones.ZoneListParams{})
	result := make([]cloudflareZone, 0)
	for iter.Next() {
		result = append(result, cloudflareZoneFromSDK(iter.Current()))
	}
	if err := iter.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *cloudflareSDKClient) CreateZone(ctx context.Context, apiToken string, body cloudflareZoneCreateRequest) (cloudflareZone, error) {
	client := newCloudflareClient(apiToken)
	params := zones.ZoneNewParams{
		Account: cloudflare.F(zones.ZoneNewParamsAccount{
			ID: cloudflare.F(body.Account.ID),
		}),
		Name: cloudflare.F(body.Name),
	}
	zoneType, err := sdkCreateZoneType(body.Type)
	if err != nil {
		return cloudflareZone{}, err
	}
	if zoneType != "" {
		params.Type = cloudflare.F(zoneType)
	}
	createdZone, err := client.Zones.New(ctx, params)
	if err != nil {
		return cloudflareZone{}, err
	}
	return cloudflareZoneFromSDK(*createdZone), nil
}

func (c *cloudflareSDKClient) UpdateZone(ctx context.Context, apiToken string, zoneID string, body cloudflareZoneUpdateRequest) (cloudflareZone, error) {
	client := newCloudflareClient(apiToken)
	params := zones.ZoneEditParams{
		ZoneID: cloudflare.F(zoneID),
	}
	if body.Paused != nil {
		params.Paused = cloudflare.F(*body.Paused)
	}
	if body.Type != nil {
		zoneType, err := sdkEditZoneType(*body.Type)
		if err != nil {
			return cloudflareZone{}, err
		}
		params.Type = cloudflare.F(zoneType)
	}
	updatedZone, err := client.Zones.Edit(ctx, params)
	if err != nil {
		return cloudflareZone{}, err
	}
	return cloudflareZoneFromSDK(*updatedZone), nil

}

func (c *cloudflareSDKClient) DeleteZone(ctx context.Context, apiToken string, zoneID string) (cloudflareZone, error) {
	client := newCloudflareClient(apiToken)
	zoneDetails, err := client.Zones.Get(ctx, zones.ZoneGetParams{ZoneID: cloudflare.F(zoneID)})
	if err != nil {
		return cloudflareZone{}, err
	}
	deletedZone, err := client.Zones.Delete(ctx, zones.ZoneDeleteParams{ZoneID: cloudflare.F(zoneID)})
	if err != nil {
		return cloudflareZone{}, err
	}
	zone := cloudflareZoneFromSDK(*zoneDetails)
	if deletedZone != nil && deletedZone.ID != "" {
		zone.ID = deletedZone.ID
	}
	return zone, nil
}

func newCloudflareClient(apiToken string) *cloudflare.Client {
	return cloudflare.NewClient(option.WithAPIToken(apiToken))
}

func cloudflareZoneFromSDK(zone zones.Zone) cloudflareZone {
	result := cloudflareZone{
		ID:     zone.ID,
		Name:   zone.Name,
		Type:   string(zone.Type),
		Paused: zone.Paused,
		Status: string(zone.Status),
	}
	result.Account.ID = zone.Account.ID
	return result
}

func sdkCreateZoneType(raw string) (zones.Type, error) {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "":
		return "", nil
	case "full":
		return zones.TypeFull, nil
	case "partial":
		return zones.TypePartial, nil
	case "secondary":
		return zones.TypeSecondary, nil
	case "internal":
		return zones.TypeInternal, nil
	default:
		return "", fmt.Errorf("unsupported zone type: %s", raw)
	}
}

func sdkEditZoneType(raw string) (zones.ZoneEditParamsType, error) {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "full":
		return zones.ZoneEditParamsTypeFull, nil
	case "partial":
		return zones.ZoneEditParamsTypePartial, nil
	case "secondary":
		return zones.ZoneEditParamsTypeSecondary, nil
	case "internal":
		return zones.ZoneEditParamsTypeInternal, nil
	default:
		return "", fmt.Errorf("unsupported zone type: %s", raw)
	}
}

func runCloudflareTunnelsFlow(cfg *config) error {
	operation, err := resolveCloudflareTunnelsOperation(cfg)
	if err != nil {
		return err
	}
	if operation == "" {
		return nil
	}
	switch operation {
	case toolsOperationList:
		return runCloudflareTunnelsListFlow(cfg)
	case toolsOperationCreate:
		return runCloudflareTunnelsCreateFlow(cfg)
	case toolsOperationGet:
		return runCloudflareTunnelsGetFlow(cfg)
	case toolsOperationDelete:
		return runCloudflareTunnelsDeleteFlow(cfg)
	case toolsOperationRoute:
		return runCloudflareTunnelsRouteFlow(cfg)
	default:
		return fmt.Errorf("unknown cloudflare tunnels operation: %s", operation)
	}
}

func resolveCloudflareTunnelsOperation(cfg *config) (string, error) {
	operation := strings.ToLower(strings.TrimSpace(cfg.ToolsOperation))
	if operation != "" {
		return operation, nil
	}
	if !shouldPrompt(cfg) {
		return "", fmt.Errorf("civa tools cloudflare tunnels requires an operation (list|create|get|delete|route)")
	}
	value, err := promptCloudflareTunnelsOperationFn(toolsOperationList)
	if err != nil {
		if errors.Is(err, errUserCancelled) {
			return "", nil
		}
		return "", err
	}
	return strings.ToLower(strings.TrimSpace(value)), nil
}

func runCloudflareTunnelsListFlow(cfg *config) error {
	if err := ensureCloudflareTunnelAccountInput(cfg); err != nil {
		return err
	}
	token, profile, err := resolveCloudflareAuthTokenForTools(cfg)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), cloudflareRequestTimeout)
	defer cancel()
	tunnels, err := cloudflareTunnelsClient.ListTunnels(ctx, token, cfg.CloudflareAccountID)
	if err != nil {
		return fmt.Errorf("list cloudflare tunnels: %w", err)
	}
	sort.Slice(tunnels, func(i, j int) bool {
		return tunnels[i].Name < tunnels[j].Name
	})
	printSection("Cloudflare Tunnels")
	fmt.Fprintf(os.Stderr, "Auth profile: %s\n", profile)
	fmt.Fprintf(os.Stderr, "Account ID:   %s\n", cfg.CloudflareAccountID)
	if len(tunnels) == 0 {
		fmt.Fprintln(os.Stderr, "No Zero Trust tunnels found for this account.")
		return nil
	}
	for _, t := range tunnels {
		fmt.Fprintf(os.Stderr, "- %s (ID: %s, Status: %s)\n", t.Name, t.ID, t.Status)
	}
	return nil
}

func runCloudflareTunnelsCreateFlow(cfg *config) error {
	if err := ensureCloudflareTunnelCreateInputs(cfg); err != nil {
		return err
	}
	token, _, err := resolveCloudflareAuthTokenForTools(cfg)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), cloudflareRequestTimeout)
	defer cancel()
	tunnel, err := cloudflareTunnelsClient.CreateTunnel(ctx, token, cfg.CloudflareAccountID, cfg.CloudflareTunnelName)
	if err != nil {
		return fmt.Errorf("create cloudflare tunnel: %w", err)
	}
	printSection("Cloudflare Tunnels")
	fmt.Fprintf(os.Stderr, "✅ Created Zero Trust tunnel %s (ID: %s, Status: %s)\n", tunnel.Name, tunnel.ID, tunnel.Status)
	return nil
}

func runCloudflareTunnelsGetFlow(cfg *config) error {
	if err := ensureCloudflareTunnelAccountInput(cfg); err != nil {
		return err
	}
	token, profile, err := resolveCloudflareAuthTokenForTools(cfg)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), cloudflareRequestTimeout)
	defer cancel()
	if err := ensureCloudflareTunnelGetInputs(ctx, cfg, token); err != nil {
		return err
	}
	tunnel, err := cloudflareTunnelsClient.GetTunnel(ctx, token, cfg.CloudflareAccountID, cfg.CloudflareTunnelID)
	if err != nil {
		return fmt.Errorf("get cloudflare tunnel: %w", err)
	}
	printSection("Cloudflare Tunnel Details")
	fmt.Fprintf(os.Stderr, "Auth profile: %s\n", profile)
	fmt.Fprintf(os.Stderr, "Account ID:   %s\n", cfg.CloudflareAccountID)
	fmt.Fprintf(os.Stderr, "Name:         %s\n", tunnel.Name)
	fmt.Fprintf(os.Stderr, "ID:           %s\n", tunnel.ID)
	fmt.Fprintf(os.Stderr, "Status:       %s\n", tunnel.Status)
	fmt.Fprintf(os.Stderr, "CNAME Target: %s.cfargotunnel.com\n", tunnel.ID)
	return nil
}

func runCloudflareTunnelsDeleteFlow(cfg *config) error {
	if err := ensureCloudflareTunnelAccountInput(cfg); err != nil {
		return err
	}
	token, _, err := resolveCloudflareAuthTokenForTools(cfg)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), cloudflareRequestTimeout)
	defer cancel()
	if err := ensureCloudflareTunnelDeleteInputs(ctx, cfg, token); err != nil {
		return err
	}
	if err := cloudflareTunnelsClient.DeleteTunnel(ctx, token, cfg.CloudflareAccountID, cfg.CloudflareTunnelID); err != nil {
		return fmt.Errorf("delete cloudflare tunnel: %w", err)
	}
	printSection("Cloudflare Tunnels")
	fmt.Fprintf(os.Stderr, "✅ Deleted Zero Trust tunnel %s\n", cfg.CloudflareTunnelID)
	return nil
}

func runCloudflareTunnelsRouteFlow(cfg *config) error {
	if err := ensureCloudflareTunnelAccountInput(cfg); err != nil {
		return err
	}
	token, _, err := resolveCloudflareAuthTokenForTools(cfg)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), cloudflareRequestTimeout)
	defer cancel()
	if err := ensureCloudflareTunnelRouteInputs(ctx, cfg, token); err != nil {
		return err
	}
	if err := cloudflareTunnelsClient.RouteTunnel(ctx, token, cfg.CloudflareAccountID, cfg.CloudflareTunnelID, cfg.CloudflareHostname, cfg.CloudflareService, cfg.CloudflareZoneID); err != nil {
		return fmt.Errorf("route cloudflare tunnel: %w", err)
	}
	printSection("Cloudflare Tunnel Route Configured")
	fmt.Fprintf(os.Stderr, "✅ Hostname %s routed to %s via Tunnel %s\n", cfg.CloudflareHostname, cfg.CloudflareService, cfg.CloudflareTunnelID)
	fmt.Fprintf(os.Stderr, "✅ DNS CNAME record created/updated (%s.cfargotunnel.com, proxied)\n", cfg.CloudflareTunnelID)
	return nil
}

func ensureCloudflareTunnelAccountInput(cfg *config) error {
	cfg.CloudflareAccountID = strings.TrimSpace(cfg.CloudflareAccountID)
	if cfg.CloudflareAccountID == "" {
		if !shouldPrompt(cfg) {
			return fmt.Errorf("tunnels operation requires --account-id")
		}
		value, err := promptNonEmptyString("Cloudflare account ID", "")
		if err != nil {
			return err
		}
		cfg.CloudflareAccountID = value
	}
	return nil
}

func ensureCloudflareTunnelCreateInputs(cfg *config) error {
	if err := ensureCloudflareTunnelAccountInput(cfg); err != nil {
		return err
	}
	cfg.CloudflareTunnelName = strings.TrimSpace(cfg.CloudflareTunnelName)
	if cfg.CloudflareTunnelName == "" {
		if !shouldPrompt(cfg) {
			return fmt.Errorf("tunnel create requires --name")
		}
		value, err := promptNonEmptyString("Cloudflare tunnel name", "")
		if err != nil {
			return err
		}
		cfg.CloudflareTunnelName = value
	}
	return nil
}

func ensureCloudflareTunnelGetInputs(ctx context.Context, cfg *config, apiToken string) error {
	cfg.CloudflareTunnelID = strings.TrimSpace(cfg.CloudflareTunnelID)
	if cfg.CloudflareTunnelID != "" {
		return nil
	}
	if !shouldPrompt(cfg) {
		return fmt.Errorf("tunnel get requires --tunnel-id")
	}
	value, err := promptCloudflareTunnelIDForOperation(ctx, apiToken, cfg.CloudflareAccountID, "Cloudflare Zero Trust tunnel details")
	if err != nil {
		return err
	}
	cfg.CloudflareTunnelID = value
	return nil
}

func ensureCloudflareTunnelDeleteInputs(ctx context.Context, cfg *config, apiToken string) error {
	cfg.CloudflareTunnelID = strings.TrimSpace(cfg.CloudflareTunnelID)
	if cfg.CloudflareTunnelID != "" {
		return nil
	}
	if !shouldPrompt(cfg) {
		return fmt.Errorf("tunnel delete requires --tunnel-id")
	}
	value, err := promptCloudflareTunnelIDForOperation(ctx, apiToken, cfg.CloudflareAccountID, "Cloudflare Zero Trust tunnel to delete")
	if err != nil {
		return err
	}
	cfg.CloudflareTunnelID = value
	return nil
}

func ensureCloudflareTunnelRouteInputs(ctx context.Context, cfg *config, apiToken string) error {
	cfg.CloudflareTunnelID = strings.TrimSpace(cfg.CloudflareTunnelID)
	if cfg.CloudflareTunnelID == "" {
		if !shouldPrompt(cfg) {
			return fmt.Errorf("tunnel route requires --tunnel-id")
		}
		value, err := promptCloudflareTunnelIDForOperation(ctx, apiToken, cfg.CloudflareAccountID, "Cloudflare Zero Trust tunnel to route domain to")
		if err != nil {
			return err
		}
		cfg.CloudflareTunnelID = value
	}
	cfg.CloudflareHostname = strings.TrimSpace(cfg.CloudflareHostname)
	if cfg.CloudflareHostname == "" {
		if !shouldPrompt(cfg) {
			return fmt.Errorf("tunnel route requires --hostname")
		}
		value, err := promptNonEmptyString("Public hostname (e.g. app.example.com)", "")
		if err != nil {
			return err
		}
		cfg.CloudflareHostname = value
	}
	cfg.CloudflareService = strings.TrimSpace(cfg.CloudflareService)
	if cfg.CloudflareService == "" {
		if !shouldPrompt(cfg) {
			return fmt.Errorf("tunnel route requires --service")
		}
		value, err := promptNonEmptyString("Target service URL (e.g. http://localhost:8080)", "http://localhost:8080")
		if err != nil {
			return err
		}
		cfg.CloudflareService = value
	}
	return nil
}

func promptCloudflareTunnelIDForOperation(ctx context.Context, apiToken string, accountID string, title string) (string, error) {
	tunnels, err := cloudflareTunnelsClient.ListTunnels(ctx, apiToken, accountID)
	if err != nil {
		return "", fmt.Errorf("load cloudflare tunnels for selection: %w", err)
	}
	if len(tunnels) == 0 {
		return "", fmt.Errorf("no cloudflare zero trust tunnels found for this account")
	}
	sort.Slice(tunnels, func(i, j int) bool {
		return tunnels[i].Name < tunnels[j].Name
	})
	return promptCloudflareTunnelSelectionFn(title, tunnels, tunnels[0].ID)
}

func (c *cloudflareSDKClient) ListTunnels(ctx context.Context, apiToken string, accountID string) ([]cloudflareTunnel, error) {
	client := newCloudflareClient(apiToken)
	iter := client.ZeroTrust.Tunnels.Cloudflared.ListAutoPaging(ctx, zero_trust.TunnelCloudflaredListParams{
		AccountID: cloudflare.F(accountID),
	})
	var result []cloudflareTunnel
	for iter.Next() {
		t := iter.Current()
		var createdAt time.Time
		createdAt = t.CreatedAt
		result = append(result, cloudflareTunnel{
			ID:        t.ID,
			Name:      t.Name,
			Status:    string(t.Status),
			CreatedAt: createdAt,
		})
	}
	if err := iter.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *cloudflareSDKClient) CreateTunnel(ctx context.Context, apiToken string, accountID string, name string) (cloudflareTunnel, error) {
	client := newCloudflareClient(apiToken)
	t, err := client.ZeroTrust.Tunnels.Cloudflared.New(ctx, zero_trust.TunnelCloudflaredNewParams{
		AccountID: cloudflare.F(accountID),
		Name:      cloudflare.F(name),
	})
	if err != nil {
		return cloudflareTunnel{}, err
	}
	var createdAt time.Time
	createdAt = t.CreatedAt
	return cloudflareTunnel{
		ID:        t.ID,
		Name:      t.Name,
		Status:    string(t.Status),
		CreatedAt: createdAt,
	}, nil
}

func (c *cloudflareSDKClient) GetTunnel(ctx context.Context, apiToken string, accountID string, tunnelID string) (cloudflareTunnel, error) {
	client := newCloudflareClient(apiToken)
	t, err := client.ZeroTrust.Tunnels.Cloudflared.Get(ctx, tunnelID, zero_trust.TunnelCloudflaredGetParams{
		AccountID: cloudflare.F(accountID),
	})
	if err != nil {
		return cloudflareTunnel{}, err
	}
	var createdAt time.Time
	createdAt = t.CreatedAt
	return cloudflareTunnel{
		ID:        t.ID,
		Name:      t.Name,
		Status:    string(t.Status),
		CreatedAt: createdAt,
	}, nil
}

func (c *cloudflareSDKClient) DeleteTunnel(ctx context.Context, apiToken string, accountID string, tunnelID string) error {
	client := newCloudflareClient(apiToken)
	_, err := client.ZeroTrust.Tunnels.Cloudflared.Delete(ctx, tunnelID, zero_trust.TunnelCloudflaredDeleteParams{
		AccountID: cloudflare.F(accountID),
	})
	return err
}

func (c *cloudflareSDKClient) RouteTunnel(ctx context.Context, apiToken string, accountID string, tunnelID string, hostname string, serviceURL string, zoneID string) error {
	client := newCloudflareClient(apiToken)

	existingConfig, err := client.ZeroTrust.Tunnels.Cloudflared.Configurations.Get(ctx, tunnelID, zero_trust.TunnelCloudflaredConfigurationGetParams{
		AccountID: cloudflare.F(accountID),
	})
	if err != nil {
		return fmt.Errorf("get tunnel configuration: %w", err)
	}

	var newIngress []zero_trust.TunnelCloudflaredConfigurationUpdateParamsConfigIngress
	updatedExisting := false

	if existingConfig != nil {
		for _, rule := range existingConfig.Config.Ingress {
			if rule.Hostname == hostname {
				newIngress = append(newIngress, zero_trust.TunnelCloudflaredConfigurationUpdateParamsConfigIngress{
					Hostname: cloudflare.F(hostname),
					Service:  cloudflare.F(serviceURL),
				})
				updatedExisting = true
			} else {
				ing := zero_trust.TunnelCloudflaredConfigurationUpdateParamsConfigIngress{
					Service: cloudflare.F(rule.Service),
				}
				if rule.Hostname != "" {
					ing.Hostname = cloudflare.F(rule.Hostname)
				}
				if rule.Path != "" {
					ing.Path = cloudflare.F(rule.Path)
				}
				newIngress = append(newIngress, ing)
			}
		}
	}

	if !updatedExisting {
		newRule := zero_trust.TunnelCloudflaredConfigurationUpdateParamsConfigIngress{
			Hostname: cloudflare.F(hostname),
			Service:  cloudflare.F(serviceURL),
		}
		if len(newIngress) > 0 && newIngress[len(newIngress)-1].Hostname.Value == "" {
			last := newIngress[len(newIngress)-1]
			newIngress[len(newIngress)-1] = newRule
			newIngress = append(newIngress, last)
		} else {
			newIngress = append(newIngress, newRule)
			newIngress = append(newIngress, zero_trust.TunnelCloudflaredConfigurationUpdateParamsConfigIngress{
				Service: cloudflare.F("http_status:404"),
			})
		}
	}

	_, err = client.ZeroTrust.Tunnels.Cloudflared.Configurations.Update(ctx, tunnelID, zero_trust.TunnelCloudflaredConfigurationUpdateParams{
		AccountID: cloudflare.F(accountID),
		Config: cloudflare.F(zero_trust.TunnelCloudflaredConfigurationUpdateParamsConfig{
			Ingress: cloudflare.F(newIngress),
		}),
	})
	if err != nil {
		return fmt.Errorf("update tunnel configuration: %w", err)
	}

	if zoneID == "" {
		zonesList, err := c.ListZones(ctx, apiToken)
		if err != nil {
			return fmt.Errorf("list zones for hostname matching: %w", err)
		}
		var bestMatch string
		for _, z := range zonesList {
			if strings.HasSuffix(hostname, z.Name) || hostname == z.Name {
				if len(z.Name) > len(bestMatch) {
					bestMatch = z.Name
					zoneID = z.ID
				}
			}
		}
		if zoneID == "" {
			return fmt.Errorf("could not auto-detect zone ID for hostname %q; please specify --zone-id", hostname)
		}
	}

	cnameTarget := fmt.Sprintf("%s.cfargotunnel.com", tunnelID)
	recordsIter := client.DNS.Records.ListAutoPaging(ctx, dns.RecordListParams{
		ZoneID: cloudflare.F(zoneID),
		Name: cloudflare.F(dns.RecordListParamsName{
			Exact: cloudflare.F(hostname),
		}),
	})

	var existingRecordID string
	for recordsIter.Next() {
		rec := recordsIter.Current()
		if rec.Name == hostname {
			existingRecordID = rec.ID
			break
		}
	}
	if err := recordsIter.Err(); err != nil {
		return fmt.Errorf("search dns records: %w", err)
	}

	cnameParam := dns.CNAMERecordParam{
		Name:    cloudflare.F(hostname),
		Type:    cloudflare.F(dns.CNAMERecordTypeCNAME),
		Content: cloudflare.F(cnameTarget),
		TTL:     cloudflare.F(dns.TTL1),
		Proxied: cloudflare.F(true),
	}

	if existingRecordID != "" {
		_, err = client.DNS.Records.Update(ctx, existingRecordID, dns.RecordUpdateParams{
			ZoneID: cloudflare.F(zoneID),
			Body:   cnameParam,
		})
		if err != nil {
			return fmt.Errorf("update dns record: %w", err)
		}
	} else {
		_, err = client.DNS.Records.New(ctx, dns.RecordNewParams{
			ZoneID: cloudflare.F(zoneID),
			Body:   cnameParam,
		})
		if err != nil {
			return fmt.Errorf("create dns record: %w", err)
		}
	}

	return nil
}

func runCloudflareAuthLoginFlow(cfg *config) error {
	profile, err := normalizeCloudflareAuthProfile(cfg.AuthProfile)
	if err != nil {
		return err
	}

	fmt.Println(renderSectionTitle(fmt.Sprintf("Cloudflare Login (Profile: %s)", profile), canStyleStdout()))
	fmt.Println("This will open your browser to authorize civa via Cloudflare.")

	clientID := "54d11594-84e4-41aa-b438-e81b8fa78ee7" // Wrangler Client ID
	redirectURI := "http://localhost:8976/oauth/callback"

	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return fmt.Errorf("failed to generate pkce: %w", err)
	}
	verifier := base64.RawURLEncoding.EncodeToString(b)
	h := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(h[:])

	authURL := fmt.Sprintf(
		"https://dash.cloudflare.com/oauth2/auth?client_id=%s&response_type=code&redirect_uri=%s&scope=%s&code_challenge=%s&code_challenge_method=S256",
		clientID, redirectURI, "zone:read,zone:edit,account:read,account:edit,offline_access", challenge,
	)

	fmt.Printf("\nOpening browser to:\n%s\n\n", authURL)
	openBrowser(authURL)
	fmt.Println("Waiting for authorization...")

	type tokenResult struct {
		accessToken string
		err         error
	}

	resultCh := make(chan tokenResult, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/oauth/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth/callback" {
			http.NotFound(w, r)
			return
		}

		if errParam := r.URL.Query().Get("error"); errParam != "" {
			errDesc := r.URL.Query().Get("error_description")
			errMsg := fmt.Sprintf("OAuth error: %s (%s)", errParam, errDesc)
			fmt.Fprintf(w, "<html><body><h2>Authorization Failed</h2><p>%s</p></body></html>", errMsg)
			resultCh <- tokenResult{err: errors.New(errMsg)}
			return
		}

		code := r.URL.Query().Get("code")
		if code == "" {
			fmt.Fprintf(w, "<html><body><h2>Authorization Failed</h2><p>Missing authorization code parameter.</p></body></html>")
			resultCh <- tokenResult{err: fmt.Errorf("authorization failed: missing code parameter in callback")}
			return
		}

		// Exchange code for token synchronously inside the handler.
		data := url.Values{}
		data.Set("client_id", clientID)
		data.Set("grant_type", "authorization_code")
		data.Set("code", code)
		data.Set("redirect_uri", redirectURI)
		data.Set("code_verifier", verifier)

		tokenReq, reqErr := http.NewRequest("POST", "https://dash.cloudflare.com/oauth2/token", strings.NewReader(data.Encode()))
		if reqErr != nil {
			resultCh <- tokenResult{err: fmt.Errorf("failed to build token request: %w", reqErr)}
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, doErr := http.DefaultClient.Do(tokenReq)
		if doErr != nil {
			resultCh <- tokenResult{err: fmt.Errorf("failed to exchange token: %w", doErr)}
			http.Error(w, "token exchange failed", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		bodyBytes, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			resultCh <- tokenResult{err: fmt.Errorf("token exchange failed (status %d): %s", resp.StatusCode, string(bodyBytes))}
			http.Error(w, "token exchange failed", http.StatusBadGateway)
			return
		}

		var tokenResp struct {
			AccessToken string `json:"access_token"`
		}
		if jsonErr := json.Unmarshal(bodyBytes, &tokenResp); jsonErr != nil {
			resultCh <- tokenResult{err: fmt.Errorf("failed to decode token response: %w", jsonErr)}
			http.Error(w, "invalid token response", http.StatusInternalServerError)
			return
		}
		if tokenResp.AccessToken == "" {
			resultCh <- tokenResult{err: fmt.Errorf("cloudflare returned empty access token")}
			http.Error(w, "empty token", http.StatusInternalServerError)
			return
		}

		fmt.Fprintf(w, "<html><body><h2>Authorization successful!</h2><p>You can close this window and return to your terminal.</p><script>window.close()</script></body></html>")
		resultCh <- tokenResult{accessToken: tokenResp.AccessToken}
	})

	srv := &http.Server{Addr: ":8976", Handler: mux}

	go func() {
		if listenErr := srv.ListenAndServe(); listenErr != nil && listenErr != http.ErrServerClosed {
			resultCh <- tokenResult{err: fmt.Errorf("local server failed: %w", listenErr)}
		}
	}()

	// Wait for the callback result with a 5-minute timeout.
	var result tokenResult
	select {
	case result = <-resultCh:
	case <-time.After(5 * time.Minute):
		result = tokenResult{err: fmt.Errorf("login timed out after 5 minutes")}
	}

	_ = srv.Shutdown(context.Background())

	if result.err != nil {
		return result.err
	}

	secretName := cloudflareAuthSecretName(profile)
	if err := writeSecretValue(secretName, result.accessToken); err != nil {
		return fmt.Errorf("failed to save token: %w", err)
	}

	fmt.Printf("\n✅ Token saved to profile %q\n", profile)
	return nil
}

func openBrowser(rawURL string) {
	cmd := "xdg-open"
	args := []string{rawURL}

	if env := os.Getenv("BROWSER"); env != "" {
		cmd = env
	} else if _, err := exec.LookPath("xdg-open"); err != nil {
		if _, err := exec.LookPath("open"); err == nil {
			cmd = "open"
		} else if _, err := exec.LookPath("rundll32"); err == nil {
			cmd = "rundll32"
			args = []string{"url.dll,FileProtocolHandler", rawURL}
		}
	}
	_ = exec.Command(cmd, args...).Start()
}
