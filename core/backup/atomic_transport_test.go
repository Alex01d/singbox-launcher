package backup

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"singbox-launcher/core/state"
)

func TestAtomicTransportBackupBoundary(t *testing.T) {
	plain := state.New()
	b, _, err := Export10(plain, ExportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if b.LxBackup != 2 || b.Requires != nil {
		t.Fatal("ordinary backup compatibility changed")
	}
	st := state.New()
	st.Sources = []state.Source{state.NewServerSource("private", json.RawMessage(`{"type":"tailscale","awg":{"version":2,"profile_id":"lab","parameters":{"s1":64}}}`))}
	st.Sources[0].SkipTailscalePresets = true
	b, _, err = Export10(st, ExportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if b.LxBackup != 3 || b.Requires == nil || b.Requires.MinContract != "1.2.0" {
		t.Fatal("atomic profile exported to an old format")
	}
	data, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	file, warnings, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range warnings {
		if strings.Contains(w.Code, "unknown") {
			t.Fatalf("own backup has unknown fields: %+v", w)
		}
	}
	if file.V10.LxBackup != 3 || !strings.Contains(string(data), `"profile_id":"lab"`) {
		t.Fatal("profile or marker lost")
	}
	// The complete import path must merge into the existing local identity.
	target := state.New()
	target.Sources = []state.Source{state.NewServerSource("local-private", append(json.RawMessage(nil), st.Sources[0].Body...))}
	target.Rules = []state.Rule{state.NewPresetRule("tailscale-protected", map[string]string{"out": "local-private", "protected_cidrs": "192.0.2.0/24"})}
	localID, localTag := target.Sources[0].ID, target.Sources[0].Tag
	localBody := append(json.RawMessage(nil), target.Sources[0].Body...)
	localRules, _ := json.Marshal(target.Rules)
	result, err := ImportFile(target, file, ImportOptions{KnownPresets: []string{"tailscale-protected"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(target.Sources) != 1 || target.Sources[0].ID != localID || target.Sources[0].Tag != localTag || !bytes.Equal(target.Sources[0].Body, localBody) || !target.Sources[0].SkipTailscalePresets {
		t.Fatal("import changed the local identity, profile, or record flag")
	}
	importedRules, _ := json.Marshal(target.Rules)
	if !bytes.Equal(importedRules, localRules) || result.RulesReplaced {
		t.Fatal("partial import changed the protected rules")
	}
	for _, tc := range []struct {
		name     string
		version  int
		requires *BackupRequirements
	}{
		{"missing", FormatVersionAtomic, nil},
		{"contract", FormatVersionAtomic, &BackupRequirements{MinContract: "9.0.0", CoreBuildTags: []string{"with_tailscale_awg"}}},
		{"empty capabilities", FormatVersionAtomic, &BackupRequirements{MinContract: "1.2.0"}},
		{"unsupported capability", FormatVersionAtomic, &BackupRequirements{MinContract: "1.2.0", CoreBuildTags: []string{"unknown"}}},
		{"future marker", FormatVersionAtomic + 1, b.Requires},
	} {
		t.Run(tc.name, func(t *testing.T) {
			invalid := *b
			invalid.LxBackup, invalid.Requires = tc.version, tc.requires
			before, _ := json.Marshal(target)
			invalidData, err := json.Marshal(&invalid)
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err := Parse(invalidData); err == nil {
				t.Fatal("invalid backup parsed")
			}
			if _, err := Import10(target, &invalid, ImportOptions{}); err == nil {
				t.Fatal("direct import accepted invalid requirements or version")
			}
			after, _ := json.Marshal(target)
			if !bytes.Equal(before, after) {
				t.Fatal("rejected backup mutated state")
			}
		})
	}
	b.Requires = nil
	data, _ = json.Marshal(b)
	if _, _, err := Parse(data); err == nil {
		t.Fatal("missing requirements accepted")
	}
	underlay := state.New()
	underlay.Sources = []state.Source{state.NewServerSource("carrier", json.RawMessage(`{"type":"tailscale","underlay_mode":"derp"}`))}
	b, _, err = Export10(underlay, ExportOptions{})
	if err != nil || b.LxBackup != 3 {
		t.Fatal("carrier-only backup downgraded")
	}
	guards := state.New()
	guards.Rules = []state.Rule{state.NewPresetRule("tailscale-protected", map[string]string{"out": "private", "protected_cidrs": "10.253.144.0/24"})}
	b, _, err = Export10(guards, ExportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if b.LxBackup != 3 {
		t.Fatal("guard-only backup downgraded")
	}
}
