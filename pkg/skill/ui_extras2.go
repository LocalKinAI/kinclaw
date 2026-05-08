//go:build darwin

package skill

import (
	"fmt"
	"strings"

	"github.com/LocalKinAI/kinax-go"
)

// This file groups the v1.14 "ui claw 100%" verbs — generic AX
// attribute read/write + scroll-into-view + force-focus. These are
// the kinkit-equivalent of macOS Accessibility Inspector's "set any
// attribute / call any action" surface.
//
// Why expose them generically: there are dozens of AX attributes
// (AXSelectedTextRange, AXScrollPosition, AXSliderValue, etc.) and
// a hundred AX actions. Carving each into its own verb would bloat
// the surface; a generic read/write keeps the common case cheap and
// gives the soul protocol an escape hatch for niche attributes.
//
// Naming compromise: the generic `ui attribute` verb is called for
// READ-ANY; the generic `ui set_attribute` is called for WRITE-ANY.
// Element selection uses the same role/title/identifier matcher as
// every other ui verb.

// ---------------------------------------------------------------------
// scroll_to — scroll a matched element into view.
//
// Calls AXShowMenu's sibling AXScrollToVisible action when the element
// supports it. Falls back to setting AXShowingMenu (not all elements)
// or just AXSelected (which often triggers scroll). Useful for "the
// row I want to click is scrolled out of view" before `click`.
// ---------------------------------------------------------------------

func (s *uiSkill) scrollTo(params map[string]string) (string, error) {
	app, err := s.openTarget(params)
	if err != nil {
		return "", err
	}
	defer app.Close()

	m, desc, err := s.matcher(params)
	if err != nil {
		return "", err
	}
	depth := atoiDefault(params["depth"], 20)
	el, ok := app.FindFirst(m, depth)
	if !ok {
		return "", fmt.Errorf("scroll_to: no element matching %s", desc)
	}
	defer el.Close()

	// Try AXScrollToVisible first (AXScrollArea + various list
	// containers honor this); fall back to AXSelected if not.
	if err := el.Perform(kinax.ActionScrollToVisible); err == nil {
		return fmt.Sprintf("scroll_to: AXScrollToVisible succeeded on %s", desc), nil
	}
	// Fallback: try setting AXSelected = true. Tables and outline
	// views typically scroll the selected row into view.
	if err := el.SetBool(kinax.AttrSelected, true); err == nil {
		return fmt.Sprintf("scroll_to: SetSelected=true succeeded on %s (table/outline fallback)", desc), nil
	}
	return "", fmt.Errorf("scroll_to: neither AXScrollToVisible nor AXSelected=true worked on %s — element does not support scroll-into-view", desc)
}

// ---------------------------------------------------------------------
// focus — set keyboard focus to a matched element.
//
// Setting AXFocused = true is the canonical AX way to redirect
// keyboard input to a text field / button without clicking. Some
// elements ignore this attribute (read-only or focus-constrained
// containers); they error cleanly with "AXFocused not settable".
// ---------------------------------------------------------------------

func (s *uiSkill) focus(params map[string]string) (string, error) {
	app, err := s.openTarget(params)
	if err != nil {
		return "", err
	}
	defer app.Close()

	m, desc, err := s.matcher(params)
	if err != nil {
		return "", err
	}
	depth := atoiDefault(params["depth"], 20)
	el, ok := app.FindFirst(m, depth)
	if !ok {
		return "", fmt.Errorf("focus: no element matching %s", desc)
	}
	defer el.Close()
	if err := el.SetBool(kinax.AttrFocused, true); err != nil {
		return "", fmt.Errorf("focus: SetFocused on %s: %w", desc, err)
	}
	return fmt.Sprintf("focus: set on %s", desc), nil
}

// ---------------------------------------------------------------------
// attribute — generic AX attribute read.
//
// Lets the soul query any AX attribute name without us hard-coding a
// dedicated verb for each. Returns the raw value as a string (booleans
// stringified, numbers formatted compact). Element-typed attributes
// (e.g. AXChildren) are not supported by this verb — they need their
// own AttributeElement-aware reader; for those use `ui tree` or
// `ui find` with the right matcher.
// ---------------------------------------------------------------------

func (s *uiSkill) attribute(params map[string]string) (string, error) {
	attr := strings.TrimSpace(params["attribute"])
	if attr == "" {
		return "", fmt.Errorf("attribute: `attribute` param is required (e.g. AXEnabled, AXValue, AXSelectedTextRange)")
	}
	app, err := s.openTarget(params)
	if err != nil {
		return "", err
	}
	defer app.Close()

	m, desc, err := s.matcher(params)
	if err != nil {
		return "", err
	}
	depth := atoiDefault(params["depth"], 20)
	el, ok := app.FindFirst(m, depth)
	if !ok {
		return "", fmt.Errorf("attribute: no element matching %s", desc)
	}
	defer el.Close()
	v, err := el.Attribute(attr)
	if err != nil {
		// Try bool / int paths before giving up — AX attribute values
		// can be typed CFBoolean / CFNumber that the string getter
		// can't decode cleanly.
		if bv, berr := el.AttributeBool(attr); berr == nil {
			return fmt.Sprintf("%s on %s = %v (bool)", attr, desc, bv), nil
		}
		if iv, ierr := el.AttributeInt(attr); ierr == nil {
			return fmt.Sprintf("%s on %s = %d (int)", attr, desc, iv), nil
		}
		return "", fmt.Errorf("attribute %q on %s: %w", attr, desc, err)
	}
	return fmt.Sprintf("%s on %s = %q", attr, desc, v), nil
}

// ---------------------------------------------------------------------
// set_attribute — generic AX attribute write.
//
// The corollary of attribute: write any settable AX attribute. We
// dispatch on the value's parseability (bool first if value=true/false,
// else string) and call the right kinax setter. For elements that
// don't allow setting (read-only attributes), AX returns
// kAXErrorAttributeUnsupported which surfaces as a clean error.
// ---------------------------------------------------------------------

func (s *uiSkill) setAttribute(params map[string]string) (string, error) {
	attr := strings.TrimSpace(params["attribute"])
	if attr == "" {
		return "", fmt.Errorf("set_attribute: `attribute` param is required")
	}
	value, hasValue := params["value"]
	if !hasValue {
		return "", fmt.Errorf("set_attribute: `value` param is required")
	}
	app, err := s.openTarget(params)
	if err != nil {
		return "", err
	}
	defer app.Close()

	m, desc, err := s.matcher(params)
	if err != nil {
		return "", err
	}
	depth := atoiDefault(params["depth"], 20)
	el, ok := app.FindFirst(m, depth)
	if !ok {
		return "", fmt.Errorf("set_attribute: no element matching %s", desc)
	}
	defer el.Close()

	// Type detection: explicit type=bool/string param overrides
	// auto-detection. Auto: if value is exactly "true" / "false",
	// treat as bool; else string.
	typeHint := strings.ToLower(strings.TrimSpace(params["type"]))
	if typeHint == "" {
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "true", "false":
			typeHint = "bool"
		default:
			typeHint = "string"
		}
	}

	switch typeHint {
	case "bool":
		b := strings.ToLower(strings.TrimSpace(value)) == "true"
		if err := el.SetBool(attr, b); err != nil {
			return "", fmt.Errorf("set_attribute %s=%v on %s: %w", attr, b, desc, err)
		}
		return fmt.Sprintf("set_attribute %s = %v (bool) on %s", attr, b, desc), nil
	case "string":
		if err := el.SetString(attr, value); err != nil {
			return "", fmt.Errorf("set_attribute %s=%q on %s: %w", attr, value, desc, err)
		}
		return fmt.Sprintf("set_attribute %s = %q (string) on %s", attr, value, desc), nil
	default:
		return "", fmt.Errorf("set_attribute: type must be bool|string (got %q)", typeHint)
	}
}
