package infra

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	cloudflare "github.com/cloudflare/cloudflare-go/v6"
	"github.com/cloudflare/cloudflare-go/v6/option"
	"github.com/cloudflare/cloudflare-go/v6/zones"
)

type cloudflareZone struct {
	ID   string
	Name string
}

type cloudflareZoneService interface {
	ListZones(ctx context.Context, apiToken string) ([]cloudflareZone, error)
}

type cloudflareSDKZoneService struct{}

var cloudflareZonesClient cloudflareZoneService = cloudflareSDKZoneService{}
var cloudflareRequestTimeout = 20 * time.Second

func runToolsFlow(cfg *config) error {
	provider, err := resolveToolsProvider(cfg)
	if err != nil {
		return err
	}
	if provider == "" {
		return nil
	}

	switch provider {
	case toolsProviderCloudflare:
		cfg.ToolsProvider = toolsProviderCloudflare
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
		return runCloudflareZoneListFlow(cfg)
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

func runCloudflareZoneListFlow(cfg *config) error {
	apiToken, err := resolveCloudflareAPIToken(cfg)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), cloudflareRequestTimeout)
	defer cancel()

	zones, err := cloudflareZonesClient.ListZones(ctx, apiToken)
	if err != nil {
		return fmt.Errorf("list cloudflare zones: %w", err)
	}

	sort.Slice(zones, func(i, j int) bool {
		return zones[i].Name < zones[j].Name
	})

	printSection("Cloudflare Zones")
	if len(zones) == 0 {
		fmt.Fprintln(os.Stderr, "No zones found for this API token.")
		return nil
	}

	for _, zone := range zones {
		fmt.Fprintf(os.Stderr, "- %s (%s)\n", zone.Name, zone.ID)
	}

	return nil
}

func resolveCloudflareAPIToken(cfg *config) (string, error) {
	token := strings.TrimSpace(cfg.CloudflareToken)
	if cfg.Provided.CloudflareToken {
		if token == "" {
			return "", fmt.Errorf("--token must not be empty")
		}
		return token, nil
	}

	if token == "" {
		token = strings.TrimSpace(os.Getenv("CLOUDFLARE_API_TOKEN"))
	}

	if token == "" {
		if !shouldPrompt(cfg) {
			return "", fmt.Errorf("cloudflare API token is required; use --token or set CLOUDFLARE_API_TOKEN")
		}

		value, err := promptSecretValue("Cloudflare API token")
		if err != nil {
			return "", err
		}
		token = strings.TrimSpace(value)
	}

	if token == "" {
		return "", fmt.Errorf("cloudflare API token must not be empty")
	}

	return token, nil
}

func promptToolsProvider(defaultValue string) (string, error) {
	value := defaultValue
	field := huh.NewSelect[string]().
		Title("Tools provider").
		Options(
			huh.NewOption("Cloudflare", toolsProviderCloudflare),
		).
		Value(&value)
	if err := field.Run(); err != nil {
		return "", normalizePromptError(err)
	}
	return value, nil
}

func promptCloudflareToolsAction(defaultValue string) (string, error) {
	value := defaultValue
	field := huh.NewSelect[string]().
		Title("Cloudflare action").
		Options(
			huh.NewOption("List zones", toolsActionCloudflareZone),
		).
		Value(&value)
	if err := field.Run(); err != nil {
		return "", normalizePromptError(err)
	}
	return value, nil
}

func (cloudflareSDKZoneService) ListZones(ctx context.Context, apiToken string) ([]cloudflareZone, error) {
	client := cloudflare.NewClient(option.WithAPIToken(apiToken))
	response, err := client.Zones.List(ctx, zones.ZoneListParams{})
	if err != nil {
		return nil, err
	}

	zones := make([]cloudflareZone, 0, len(response.Result))
	for _, zone := range response.Result {
		zones = append(zones, cloudflareZone{ID: zone.ID, Name: zone.Name})
	}

	return zones, nil
}
