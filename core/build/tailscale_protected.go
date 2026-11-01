package build

import (
	"errors"
	"net"
	"regexp"
	"strings"

	"singbox-launcher/core/state"
)

func protectedList(raw string) []string {
	// Match the shared template text_list format exactly: one item per line.
	var values []string
	for _, line := range strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n") {
		if value := strings.TrimSpace(line); value != "" {
			values = append(values, value)
		}
	}
	return values
}

// Validate before template expansion: losing one guard fragment must never
// turn a selected protected network into a fallback route.
func validateTailscaleProtected(ctx PresetMergeContext) error {
	if strings.EqualFold(strings.TrimSpace(ctx.GlobalVars["desktop_command_api"]), "true") {
		secret := strings.TrimSpace(ctx.GlobalVars["clash_secret"])
		if secret == "" || secret == "CHANGE_THIS_TO_YOUR_SECRET_TOKEN" {
			return errors.New("Set a private Clash API secret before enabling the local core API")
		}
	}
	for _, r := range ctx.Rules {
		if !r.Enabled || r.Kind != state.RuleKindPreset || r.Ref != "tailscale-protected" {
			continue
		}
		target := strings.TrimSpace(r.Vars["out"])
		found := false
		for _, n := range ctx.PresetNodes {
			if n.Tag == target && n.Body["type"] == "tailscale" {
				found = true
				break
			}
		}
		if target == "" || !found {
			return errors.New("Protected Tailscale networks require an available Tailscale endpoint; the configuration was not applied")
		}
		cidrs := protectedList(r.Vars["protected_cidrs"])
		suffixes := protectedList(r.Vars["private_dns_suffixes"])
		if len(cidrs) == 0 && len(suffixes) == 0 {
			return errors.New("Protected Tailscale networks require at least one CIDR or private DNS suffix")
		}
		for _, cidr := range cidrs {
			if _, _, err := net.ParseCIDR(strings.TrimSpace(cidr)); err != nil {
				return errors.New("Invalid protected Tailscale CIDR; the configuration was not applied")
			}
		}
		pattern := regexp.MustCompile(`^[A-Za-z0-9_-]+(?:\.[A-Za-z0-9_-]+)*\.?$`)
		for _, suffix := range suffixes {
			if !pattern.MatchString(strings.TrimPrefix(strings.TrimSpace(suffix), ".")) {
				return errors.New("Invalid private DNS suffix; the configuration was not applied")
			}
		}
	}
	return nil
}
