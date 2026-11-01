package backup

import (
	"encoding/json"
	"errors"
)

type BackupRequirements struct {
	MinContract   string   `json:"min_contract"`
	CoreBuildTags []string `json:"core_build_tags"`
}

func atomicTransportPresent(value interface{}) (awg, guard bool) {
	switch v := value.(type) {
	case map[string]interface{}:
		if v["type"] == "tailscale" && (v["awg"] != nil || v["underlay_mode"] != nil) {
			awg = true
		}
		if v["ref"] == "tailscale-protected" {
			guard = true
		}
		for _, child := range v {
			a, g := atomicTransportPresent(child)
			awg = awg || a
			guard = guard || g
		}
	case []interface{}:
		for _, child := range v {
			a, g := atomicTransportPresent(child)
			awg = awg || a
			guard = guard || g
		}
	}
	return
}

func markAtomicBackup(b *Backup10) error {
	data, err := json.Marshal(b)
	if err != nil {
		return err
	}
	var value interface{}
	if err = json.Unmarshal(data, &value); err != nil {
		return err
	}
	awg, guard := atomicTransportPresent(value)
	if awg || guard {
		tag := "with_tailscale"
		if awg {
			tag = "with_tailscale_awg"
		}
		b.LxBackup = FormatVersionAtomic
		b.Requires = &BackupRequirements{MinContract: "1.2.0", CoreBuildTags: []string{tag}}
	}
	return nil
}

func validateAtomicBackup(b *Backup10) error {
	if b.Requires == nil || b.Requires.MinContract != "1.2.0" || len(b.Requires.CoreBuildTags) == 0 {
		return errors.New("backup requires a newer or unsupported transport contract")
	}
	for _, tag := range b.Requires.CoreBuildTags {
		if tag != "with_tailscale" && tag != "with_tailscale_awg" {
			return errors.New("backup requires an unsupported core capability")
		}
	}
	return nil
}
