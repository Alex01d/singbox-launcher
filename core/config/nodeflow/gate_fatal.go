package nodeflow

import "singbox-launcher/core/config/registry"

// NodeCoreBuildFailure runs before either node or field downgrade. Unlike the
// legacy drop_node policy, a required atomic capability must be confirmed.
func NodeCoreBuildFailure(scheme string, body map[string]interface{}, core CoreInfo) *CoreRefusal {
	reg, err := registry.Get()
	if err != nil {
		return &CoreRefusal{Code: "core_capability_registry_unavailable", Reason: "cannot verify required core capabilities"}
	}
	schema, ok := reg.Body(scheme)
	if !ok {
		return nil
	}
	var result *CoreRefusal
	check := func(path, tag, version string, policy *registry.OnCoreUnsupported) {
		if result != nil || policy == nil || policy.Action != registry.CoreUnsupportedFailBuild {
			return
		}
		reason := ""
		if tag != "" && core.Tags == nil {
			reason = "core build capabilities are unknown"
		} else if version != "" && core.Version == "" {
			reason = "core version is unknown"
		} else {
			reason = core.unmet(tag, version, scheme+"."+path)
		}
		if reason != "" {
			result = &CoreRefusal{Code: policy.Code, Path: path, Reason: reason}
		}
	}
	check("", schema.BuildTag, schema.MinCore, schema.OnCoreUnsupported)
	reg.WalkPresent(scheme, body, func(path string, f *registry.Field, v interface{}) {
		if v == nil {
			return
		}
		check(path, f.BuildTag, f.MinCore, f.OnCoreUnsupported)
	})
	return result
}
