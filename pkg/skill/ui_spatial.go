//go:build darwin

package skill

import (
	"fmt"
	"image"
	"strings"

	"github.com/LocalKinAI/kinax-go"
)

// spatial_find — locate an AX element by its position RELATIVE TO
// another anchor element. Solves the very common UI shape "the
// 'Submit' button is below the 'Email' label" — where the matcher
// for the button alone is too ambiguous (multiple AXButtons on
// screen), but anchoring spatially makes it precise.
//
// Conceptually: find the anchor first by role/title/identifier,
// take its bbox, then scan all elements matching the candidate
// matcher and pick the one whose center is in the requested
// half-plane (above / below / left_of / right_of) AND closest by
// Euclidean distance to the anchor center, optionally clamped by
// max_distance_px.
//
// Why a separate verb instead of extending `find`: `find` returns
// many elements; spatial_find ALWAYS returns exactly one (or none).
// Different return contract = different verb.

type spatialDirection int

const (
	dirNone spatialDirection = iota
	dirAbove
	dirBelow
	dirLeftOf
	dirRightOf
	dirNear // Euclidean distance only, ignore direction
)

func (s *uiSkill) spatialFind(params map[string]string) (string, error) {
	app, err := s.openTarget(params)
	if err != nil {
		return "", err
	}
	defer app.Close()

	// Anchor: required. Identified by role/title/identifier with
	// `anchor_` prefix to keep the candidate matcher params clear.
	anchorParams := map[string]string{
		"role":           params["anchor_role"],
		"title":          params["anchor_title"],
		"title_contains": params["anchor_title_contains"],
		"identifier":     params["anchor_identifier"],
	}
	if !hasMatcherParam(anchorParams) {
		return "", fmt.Errorf("spatial_find: anchor required (anchor_role / anchor_title / anchor_title_contains / anchor_identifier)")
	}
	anchorMatcher, anchorDesc, err := s.matcher(anchorParams)
	if err != nil {
		return "", fmt.Errorf("spatial_find: anchor: %w", err)
	}
	depth := atoiDefault(params["depth"], 25)
	anchor, ok := app.FindFirst(anchorMatcher, depth)
	if !ok {
		return "", fmt.Errorf("spatial_find: anchor not found (%s)", anchorDesc)
	}
	defer anchor.Close()
	anchorBox, err := elementBounds(anchor)
	if err != nil {
		return "", fmt.Errorf("spatial_find: anchor has no geometry: %w", err)
	}

	// Candidate matcher: same fields as `find`. Required.
	candidateParams := map[string]string{
		"role":           params["role"],
		"title":          params["title"],
		"title_contains": params["title_contains"],
		"identifier":     params["identifier"],
		"description":    params["description"],
	}
	if !hasMatcherParam(candidateParams) {
		return "", fmt.Errorf("spatial_find: candidate matcher required (role / title / title_contains / identifier / description)")
	}
	candMatcher, candDesc, err := s.matcher(candidateParams)
	if err != nil {
		return "", fmt.Errorf("spatial_find: candidate: %w", err)
	}

	dir, dirLabel := parseSpatialDirection(params["direction"])
	maxDist := atoiDefault(params["max_distance_px"], 0) // 0 = unbounded

	// Get all candidates, score by spatial filter + distance.
	hits := app.FindAll(candMatcher, depth)
	defer closeAll(hits)
	if len(hits) == 0 {
		return "", fmt.Errorf("spatial_find: no candidates matching %s", candDesc)
	}

	type scored struct {
		idx  int
		dist float64
	}
	var ranked []scored
	anchorCx, anchorCy := centerOf(anchorBox)
	for i, h := range hits {
		hb, err := elementBounds(h)
		if err != nil {
			continue
		}
		// Direction filter: candidate's CENTER must be in requested
		// half-plane relative to anchor's CENTER.
		hcx, hcy := centerOf(hb)
		if !directionMatches(dir, anchorCx, anchorCy, hcx, hcy) {
			continue
		}
		dist := euclidean(anchorCx, anchorCy, hcx, hcy)
		if maxDist > 0 && dist > float64(maxDist) {
			continue
		}
		ranked = append(ranked, scored{idx: i, dist: dist})
	}
	if len(ranked) == 0 {
		return "", fmt.Errorf(
			"spatial_find: %d candidate(s) matched %s but none satisfy %s relative to anchor %s",
			len(hits), candDesc, dirLabel, anchorDesc,
		)
	}
	// Pick smallest distance.
	best := ranked[0]
	for _, r := range ranked[1:] {
		if r.dist < best.dist {
			best = r
		}
	}
	winner := hits[best.idx]
	role, _ := winner.Role()
	title, _ := winner.Title()
	id, _ := winner.Identifier()
	wb, _ := elementBounds(winner)
	wcx, wcy := centerOf(wb)
	return fmt.Sprintf(
		"spatial_find: matched %s %q [%s] %s anchor %s — center=(%d,%d) bbox=(%d,%d %dx%d) distance=%.0fpx (best of %d)",
		role, title, id, dirLabel, anchorDesc,
		wcx, wcy, wb.Min.X, wb.Min.Y, wb.Dx(), wb.Dy(),
		best.dist, len(ranked),
	), nil
}

// elementBounds reads AXFrame if available; falls back to
// AXPosition + AXSize. Returns image.Rectangle in display-local
// coordinates (top-left origin).
func elementBounds(el *kinax.Element) (image.Rectangle, error) {
	pos, err := el.Position()
	if err != nil {
		return image.Rectangle{}, fmt.Errorf("position: %w", err)
	}
	sz, err := el.Size()
	if err != nil {
		return image.Rectangle{}, fmt.Errorf("size: %w", err)
	}
	return image.Rect(pos.X, pos.Y, pos.X+sz.X, pos.Y+sz.Y), nil
}

func centerOf(r image.Rectangle) (int, int) {
	return r.Min.X + r.Dx()/2, r.Min.Y + r.Dy()/2
}

func euclidean(x1, y1, x2, y2 int) float64 {
	dx := float64(x1 - x2)
	dy := float64(y1 - y2)
	return sqrtFloat(dx*dx + dy*dy)
}

// sqrtFloat — pull math/sqrt without importing the package twice.
// (We already pull math via imageDistance in screen_extras.go.)
func sqrtFloat(x float64) float64 {
	if x <= 0 {
		return 0
	}
	// Newton-Raphson, 4 iterations — enough precision for px-scale
	// distance comparisons. Avoids the math package import for
	// this single call site.
	z := x
	for i := 0; i < 4; i++ {
		z = (z + x/z) / 2
	}
	return z
}

func parseSpatialDirection(s string) (spatialDirection, string) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "above", "up":
		return dirAbove, "above"
	case "below", "down":
		return dirBelow, "below"
	case "left_of", "left":
		return dirLeftOf, "left_of"
	case "right_of", "right":
		return dirRightOf, "right_of"
	case "near", "":
		return dirNear, "near"
	default:
		return dirNone, "none"
	}
}

func directionMatches(d spatialDirection, ax, ay, cx, cy int) bool {
	switch d {
	case dirAbove:
		return cy < ay
	case dirBelow:
		return cy > ay
	case dirLeftOf:
		return cx < ax
	case dirRightOf:
		return cx > ax
	case dirNear:
		return true
	default:
		return false
	}
}
