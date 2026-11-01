package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"

	"singbox-launcher/contract"
)

// TailscaleAWGProfile is shareable transport configuration, never node identity.
type TailscaleAWGProfile struct {
	Version    int                    `json:"version"`
	ProfileID  string                 `json:"profile_id,omitempty"`
	Parameters map[string]interface{} `json:"parameters"`
}

type awgParameterSchema struct {
	Type    string               `json:"type"`
	Pattern string               `json:"pattern"`
	Maximum *float64             `json:"maximum"`
	OneOf   []awgParameterSchema `json:"oneOf"`
}

func validAWGParameter(v interface{}, s awgParameterSchema) bool {
	if len(s.OneOf) > 0 {
		for _, candidate := range s.OneOf {
			if validAWGParameter(v, candidate) {
				return true
			}
		}
		return false
	}
	switch s.Type {
	case "integer":
		n, ok := v.(json.Number)
		if !ok {
			return false
		}
		i, err := n.Int64()
		return err == nil && i >= 0 && (s.Maximum == nil || float64(i) <= *s.Maximum)
	case "boolean":
		_, ok := v.(bool)
		return ok
	case "string":
		value, ok := v.(string)
		if !ok {
			return false
		}
		if s.Pattern != "" {
			p, err := regexp.Compile(s.Pattern)
			return err == nil && p.MatchString(value)
		}
		return true
	}
	return false
}

func ParseTailscaleAWGProfile(data []byte) (*TailscaleAWGProfile, error) {
	if len(data) > 1<<20 {
		return nil, errors.New("AWG profile is too large")
	}
	var p TailscaleAWGProfile
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return nil, errors.New("Invalid AWG profile JSON or unknown profile field")
	}
	var extra interface{}
	if err := dec.Decode(&extra); err != io.EOF {
		return nil, errors.New("AWG profile must contain one JSON object")
	}
	if p.Version != 2 && p.Version != 3 {
		return nil, errors.New("AWG profile version must be 2 or 3")
	}
	if p.ProfileID != "" && !regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`).MatchString(p.ProfileID) {
		return nil, errors.New("Invalid AWG profile label")
	}
	if len(p.Parameters) == 0 {
		return nil, errors.New("AWG parameters must not be empty")
	}
	var schema struct {
		Properties struct {
			Parameters struct {
				Properties map[string]awgParameterSchema `json:"properties"`
			} `json:"parameters"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(contract.TailscaleAWGSchema, &schema); err != nil {
		return nil, errors.New("AWG profile schema is unavailable")
	}
	for name, value := range p.Parameters {
		rule, known := schema.Properties.Parameters.Properties[name]
		if !known {
			return nil, errors.New("Unknown AWG parameter; the profile was not changed")
		}
		if !validAWGParameter(value, rule) {
			return nil, fmt.Errorf("Invalid AWG parameter: %s", name)
		}
	}
	// Cross-field constraints and wire compatibility belong to core validation.
	return &p, nil
}
