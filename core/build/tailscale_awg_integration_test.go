package build

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"singbox-launcher/core/config"
	"singbox-launcher/core/config/nodeflow"
	"singbox-launcher/core/state"
	"singbox-launcher/core/template"
)

func TestTailscaleAWGContractAndProtection(t *testing.T) {
	profile := `{"version":2,"profile_id":"lab","parameters":{"s1":64,"h1":"100000-199999","h2":"200000-299999","h3":"300000-399999","h4":"400000-499999"}}`
	if _, err := config.ParseTailscaleAWGProfile([]byte(profile)); err != nil {
		t.Fatal(err)
	}
	var value interface{}
	if err := json.Unmarshal([]byte(profile), &value); err != nil {
		t.Fatal(err)
	}
	body := map[string]interface{}{"type": "tailscale", "hostname": "lab", "awg": value}
	clean := nodeflow.Sanitize("tailscale", body)
	if clean.Drop != nil {
		t.Fatal(clean.Drop.Code)
	}
	emitted, err := nodeflow.Emit("tailscale", clean.Clean)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(emitted, &got); err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(value)
	after, _ := json.Marshal(got["awg"])
	if string(before) != string(after) {
		t.Fatal("profile changed during sanitize/emission")
	}
	for _, tags := range [][]string{nil, {}, {"with_awg", "with_tailscale"}} {
		refusal := nodeflow.NodeCoreBuildFailure("tailscale", body, nodeflow.CoreInfo{Tags: tags})
		if refusal == nil || refusal.Code != "tailscale_awg_core_unsupported" {
			t.Fatal("unsupported/unknown core did not fail closed")
		}
	}
	if r := nodeflow.NodeCoreBuildFailure("tailscale", body, nodeflow.CoreInfo{Tags: []string{"with_tailscale_awg"}}); r != nil {
		t.Fatal(r.Reason)
	}
	carrierBody := map[string]interface{}{"type": "tailscale", "underlay_mode": "derp"}
	carrierClean := nodeflow.Sanitize("tailscale", carrierBody)
	carrierJSON, err := nodeflow.Emit("tailscale", carrierClean.Clean)
	if err != nil || !strings.Contains(string(carrierJSON), `"underlay_mode":"derp"`) || len(carrierClean.Warnings) > 0 {
		t.Fatal("carrier mode was pruned or warned as unknown")
	}
	if r := nodeflow.NodeCoreBuildFailure("tailscale", carrierBody, nodeflow.CoreInfo{Tags: []string{"with_tailscale"}}); r == nil {
		t.Fatal("carrier-only config bypassed fatal capability gate")
	}
	// An unknown parameter must reach strict core validation intact; pruning
	// it here could turn a requested profile into a different wire format.
	value.(map[string]interface{})["parameters"].(map[string]interface{})["future_wire_parameter"] = 42
	clean = nodeflow.Sanitize("tailscale", body)
	emitted, err = nodeflow.Emit("tailscale", clean.Clean)
	if err != nil || !strings.Contains(string(emitted), "future_wire_parameter") {
		t.Fatal("profile was silently sanitized")
	}
	if _, err := config.ParseTailscaleAWGProfile([]byte(strings.Replace(profile, `"s1":64`, `"private_key":"private-secret"`, 1))); err == nil || strings.Contains(err.Error(), "private-secret") {
		t.Fatal("identity/information boundary failed")
	}
	// Exercise the production parser -> generator boundary as well as the
	// registry primitive: an unsupported AWG node must abort the whole build.
	st := state.New()
	st.Sources = []state.Source{state.NewServerSource("aws", json.RawMessage(`{"type":"tailscale","underlay_mode":"derp","awg":`+profile+`}`)), state.NewServerSource("outer", json.RawMessage(`{"type":"vless","server":"192.0.2.1","server_port":443,"uuid":"3c3f3116-3811-4732-a441-c2f54b2dd037"}`))}
	path := filepath.Join(t.TempDir(), "state.json")
	if err := st.Save(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := state.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	oldTags, oldVersion := config.CoreBuildTagsProbe, config.CoreVersionProbe
	defer func() { config.CoreBuildTagsProbe = oldTags; config.CoreVersionProbe = oldVersion }()
	config.CoreVersionProbe = func() string { return "1.14.4-lx.1" }
	config.CoreBuildTagsProbe = func() ([]string, map[string]string) { return []string{"with_tailscale", "with_awg"}, nil }
	if _, err := config.GenerateOutboundsFromParserConfig(&loaded.ParserConfig, make(map[string]int), nil, config.DirectionBuildOptions{}); err == nil || !strings.Contains(err.Error(), "tailscale_awg_core_unsupported") {
		t.Fatalf("generator did not reject the whole build: %v", err)
	}
	config.CoreBuildTagsProbe = func() ([]string, map[string]string) {
		return []string{"with_tailscale", "with_awg", "with_tailscale_awg"}, nil
	}
	if result, err := config.GenerateOutboundsFromParserConfig(&loaded.ParserConfig, make(map[string]int), nil, config.DirectionBuildOptions{}); err != nil || len(result.EndpointsJSON) != 1 {
		t.Fatalf("supported generation failed: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join("..", "..", "bin", "wizard_template.json"))
	if err != nil {
		t.Fatal(err)
	}
	td, err := template.ParseTemplateData(raw)
	if err != nil {
		t.Fatal(err)
	}
	ctx := PresetMergeContext{Presets: td.Presets, Rules: []state.Rule{{Kind: state.RuleKindPreset, Ref: "tailscale-protected", Enabled: true, Vars: map[string]string{"out": "aws", "protected_cidrs": "10.253.144.0/24", "private_dns_suffixes": "private.awg.lab"}}}, PresetNodes: []template.PresetNode{{Tag: "aws", Body: map[string]interface{}{"type": "tailscale"}}}, Target: template.LocalTarget()}
	route, err := MergePresetsIntoRoute(json.RawMessage(`{"rules":[]}`), ctx)
	if err != nil {
		t.Fatal(err)
	}
	dns, err := MergePresetsIntoDNS(json.RawMessage(`{"servers":[],"rules":[]}`), ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{"10.253.144.0/24", "private.awg.lab", "aws-protected-dns"} {
		if !strings.Contains(string(route), part) {
			t.Fatalf("missing persistent guard %s: %s", part, route)
		}
	}
	var routeDocument struct {
		Rules []map[string]interface{} `json:"rules"`
	}
	if err := json.Unmarshal(route, &routeDocument); err != nil {
		t.Fatal(err)
	}
	hasProtectedRoute := false
	for _, rule := range routeDocument.Rules {
		if rule["outbound"] == "aws" && rule["ip_cidr"] != nil {
			hasProtectedRoute = true
		}
	}
	if !hasProtectedRoute {
		t.Fatal("protected CIDRs are not bound to the selected endpoint")
	}
	if !strings.Contains(string(dns), "aws-protected-dns") || !strings.Contains(string(dns), "private.awg.lab") {
		t.Fatalf("missing private DNS guard: %s", dns)
	}
	// The target exists in config even before login; removing it must be fatal.
	ctx.PresetNodes = nil
	if _, err := MergePresetsIntoRoute(json.RawMessage(`{"rules":[]}`), ctx); err == nil {
		t.Fatal("missing target silently removed protected route")
	}
}
