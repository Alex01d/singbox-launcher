package tailscalewidgets

import (
	"encoding/json"
	"errors"
	"io"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"

	"singbox-launcher/core/config"
	uidialogs "singbox-launcher/internal/dialogs"
	"singbox-launcher/internal/locale"
)

// ProfileEditor edits only the portable profile; it never accesses node state.
type ProfileEditor struct {
	Box        *fyne.Container
	Mode       *widget.Select
	Label      *widget.Entry
	Parameters *widget.Entry
	fields     *fyne.Container
	Parent     fyne.Window
	onChange   func()
	syncing    bool
}

func NewProfileEditor(onChange func()) *ProfileEditor {
	e := &ProfileEditor{onChange: onChange}
	changed := func() {
		if !e.syncing && e.onChange != nil {
			e.onChange()
		}
	}
	e.Label = widget.NewEntry()
	e.Label.SetPlaceHolder(locale.T("Optional profile label"))
	e.Label.OnChanged = func(string) { changed() }
	e.Parameters = widget.NewMultiLineEntry()
	e.Parameters.SetMinRowsVisible(8)
	e.Parameters.SetText("{}")
	e.Parameters.OnChanged = func(string) { changed() }
	e.Mode = widget.NewSelect([]string{"WireGuard", "AWG2", "AWG3"}, func(string) { e.refresh(); changed() })
	note := widget.NewLabel(locale.T("Both peers must use a compatible profile. Exported profiles may contain a shared key; they never include this device's identity."))
	note.Wrapping = fyne.TextWrapWord
	imp := widget.NewButton(locale.T("Import profile"), func() {
		if e.Parent == nil {
			return
		}
		d := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
			if err != nil {
				uidialogs.ShowError(e.Parent, err)
				return
			}
			if r == nil {
				return
			}
			defer r.Close()
			data, err := io.ReadAll(io.LimitReader(r, 1<<20+1))
			if err != nil {
				uidialogs.ShowError(e.Parent, err)
				return
			}
			p, err := config.ParseTailscaleAWGProfile(data)
			if err != nil {
				uidialogs.ShowError(e.Parent, err)
				return
			}
			e.Load(p)
			changed()
		}, e.Parent)
		d.SetFilter(storage.NewExtensionFileFilter([]string{".json"}))
		d.Show()
	})
	exp := widget.NewButton(locale.T("Export profile"), func() {
		if e.Parent == nil {
			return
		}
		p, err := e.Value()
		if err != nil {
			uidialogs.ShowError(e.Parent, err)
			return
		}
		if p == nil {
			uidialogs.ShowError(e.Parent, errors.New("Select an AWG mode before exporting"))
			return
		}
		data, err := json.MarshalIndent(p, "", "  ")
		if err != nil {
			uidialogs.ShowError(e.Parent, err)
			return
		}
		d := dialog.NewFileSave(func(w fyne.URIWriteCloser, err error) {
			if err != nil {
				uidialogs.ShowError(e.Parent, err)
				return
			}
			if w == nil {
				return
			}
			_, writeErr := w.Write(append(data, '\n'))
			closeErr := w.Close()
			if writeErr != nil {
				uidialogs.ShowError(e.Parent, writeErr)
			} else if closeErr != nil {
				uidialogs.ShowError(e.Parent, closeErr)
			}
		}, e.Parent)
		d.SetFileName("awg-profile.json")
		d.Show()
	})
	e.fields = container.NewVBox(widget.NewLabel(locale.T("Profile label")), e.Label, widget.NewLabel(locale.T("AWG parameters (JSON)")), e.Parameters, note)
	e.Box = container.NewVBox(widget.NewLabel(locale.T("Tailscale peer transport")), e.Mode, container.NewHBox(imp, exp), e.fields)
	e.Mode.SetSelected("WireGuard")
	return e
}

func (e *ProfileEditor) refresh() {
	if e.fields == nil {
		return
	}
	if e.Mode.Selected == "WireGuard" {
		e.fields.Hide()
	} else {
		e.fields.Show()
	}
}

func (e *ProfileEditor) Value() (*config.TailscaleAWGProfile, error) {
	if e.Mode.Selected == "WireGuard" {
		return nil, nil
	}
	version := 0
	if e.Mode.Selected == "AWG2" {
		version = 2
	}
	if e.Mode.Selected == "AWG3" {
		version = 3
	}
	var parameters json.RawMessage = json.RawMessage(strings.TrimSpace(e.Parameters.Text))
	value := struct {
		Version    int             `json:"version"`
		ProfileID  string          `json:"profile_id,omitempty"`
		Parameters json.RawMessage `json:"parameters"`
	}{version, strings.TrimSpace(e.Label.Text), parameters}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, errors.New("Invalid AWG parameters JSON")
	}
	return config.ParseTailscaleAWGProfile(raw)
}

func (e *ProfileEditor) Load(value interface{}) {
	e.syncing = true
	defer func() { e.syncing = false; e.refresh() }()
	if value == nil {
		e.Mode.SetSelected("WireGuard")
		return
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return
	}
	var p struct {
		Version    int             `json:"version"`
		ProfileID  string          `json:"profile_id"`
		Parameters json.RawMessage `json:"parameters"`
	}
	if json.Unmarshal(raw, &p) != nil {
		e.Mode.ClearSelected()
		e.Parameters.SetText(string(raw))
		return
	}
	switch p.Version {
	case 2:
		e.Mode.SetSelected("AWG2")
	case 3:
		e.Mode.SetSelected("AWG3")
	default:
		e.Mode.ClearSelected()
	}
	e.Label.SetText(p.ProfileID)
	var object interface{}
	if json.Unmarshal(p.Parameters, &object) == nil {
		data, err := json.MarshalIndent(object, "", "  ")
		if err == nil {
			e.Parameters.SetText(string(data))
			return
		}
	}
	e.Parameters.SetText(string(p.Parameters))
}
