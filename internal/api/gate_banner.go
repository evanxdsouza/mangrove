package api

import (
	"fmt"
	"strings"
)

// bannerScene is one tide level's palette, ported from the dashboard's
// TideBanner.svelte (web/src/lib/components/TideBanner.svelte) so the
// standalone gate pages carry the same mangrove-at-the-waterline art as the
// dashboard's own sign-in card. Only the scenes those pages use are ported.
type bannerScene struct {
	skyTop, skyBottom     string
	skyStop               float64
	waterTop, waterBottom string
	waterY                int
	ridge                 string
	ridgeOpacity          float64
	haloColor             string
	haloStrength          float64
	haloR, haloCY         int
	orbCY, orbR           int
	orbFill               string
	glintFill             string
	glintOpacity          float64
	glintRows             int
	stars                 float64
	roots                 string
	vignette              string
	vignetteOpacity       float64
}

var (
	bannerHighDay = bannerScene{
		skyTop: "#2E6B66", skyBottom: "#E7B77A", skyStop: 0.75, waterTop: "#3C7F78", waterBottom: "#15423A", waterY: 92,
		ridge: "#2B6B5F", ridgeOpacity: 0.5, haloColor: "#FFE9B8", haloStrength: 0.9, haloR: 60, haloCY: 76,
		orbCY: 80, orbR: 13, orbFill: "#FFF1CC", glintFill: "#FFF1CC", glintOpacity: 0.5, glintRows: 3,
		roots: "#0F2B24", vignette: "#0A1F1A", vignetteOpacity: 0.38,
	}
	bannerHighNight = bannerScene{
		skyTop: "#06202A", skyBottom: "#1F4A4A", skyStop: 0.75, waterTop: "#1E5B58", waterBottom: "#061D1A", waterY: 92,
		ridge: "#0E3A38", ridgeOpacity: 0.7, haloColor: "#BFE3DC", haloStrength: 0.55, haloR: 60, haloCY: 62,
		orbCY: 66, orbR: 12, orbFill: "#EAF2E1", glintFill: "#BFE3DC", glintOpacity: 0.4, glintRows: 3,
		stars: 1, roots: "#04100C", vignette: "#000000", vignetteOpacity: 0.5,
	}
	// "Turning" -- the in-progress scene, for a deployment waking back up.
	bannerMidDay = bannerScene{
		skyTop: "#5C7FA3", skyBottom: "#F2C98B", skyStop: 0.8, waterTop: "#6A93A8", waterBottom: "#2A4F5E", waterY: 94,
		ridge: "#3E6A6A", ridgeOpacity: 0.5, haloColor: "#FFE9B8", haloStrength: 0.9, haloR: 70, haloCY: 90,
		orbCY: 94, orbR: 14, orbFill: "#FFF1CC", glintFill: "#FFF1CC", glintOpacity: 0.55, glintRows: 3,
		roots: "#0F2B24", vignette: "#0A1F1A", vignetteOpacity: 0.38,
	}
	bannerMidNight = bannerScene{
		skyTop: "#182640", skyBottom: "#8A6340", skyStop: 0.8, waterTop: "#3B6378", waterBottom: "#0B1D2A", waterY: 94,
		ridge: "#1F3B4A", ridgeOpacity: 0.7, haloColor: "#F2C98B", haloStrength: 0.6, haloR: 66, haloCY: 92,
		orbCY: 94, orbR: 13, orbFill: "#F2C98B", glintFill: "#F2C98B", glintOpacity: 0.5, glintRows: 2,
		stars: 0.6, roots: "#04100C", vignette: "#000000", vignetteOpacity: 0.5,
	}
)

var bannerStars = [][4]float64{
	{120, 22, 1.2, 0.8}, {210, 40, 1, 0.6}, {300, 16, 1.4, 0.85}, {380, 34, 1, 0.55},
	{560, 20, 1.3, 0.8}, {640, 42, 1, 0.6}, {720, 18, 1.2, 0.75}, {60, 46, 1, 0.5},
}

// tideBannerSVG renders a scene as inline SVG. id prefixes the gradient
// IDs, which must be unique per page since a page carries a day and a
// night banner side by side (CSS shows one per color scheme).
func tideBannerSVG(id, class string, s bannerScene) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<svg class="tide-banner %s" viewBox="0 0 800 132" preserveAspectRatio="xMidYMid slice" aria-hidden="true">`, class)
	fmt.Fprintf(&b, `<defs><linearGradient id="%ss" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="%s"/><stop offset="%g" stop-color="%s"/></linearGradient>`, id, s.skyTop, s.skyStop, s.skyBottom)
	fmt.Fprintf(&b, `<linearGradient id="%sw" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="%s"/><stop offset="1" stop-color="%s"/></linearGradient>`, id, s.waterTop, s.waterBottom)
	fmt.Fprintf(&b, `<radialGradient id="%sh"><stop offset="0" stop-color="%s" stop-opacity="%g"/><stop offset="1" stop-color="%s" stop-opacity="0"/></radialGradient>`, id, s.haloColor, s.haloStrength, s.haloColor)
	fmt.Fprintf(&b, `<linearGradient id="%sv" x1="0" y1="0" x2="1" y2="0"><stop offset="0" stop-color="%[2]s" stop-opacity="%[3]g"/><stop offset=".25" stop-color="%[2]s" stop-opacity="0"/><stop offset=".75" stop-color="%[2]s" stop-opacity="0"/><stop offset="1" stop-color="%[2]s" stop-opacity="%[3]g"/></linearGradient></defs>`, id, s.vignette, s.vignetteOpacity)
	fmt.Fprintf(&b, `<rect width="800" height="132" fill="url(#%ss)"/>`, id)
	if s.stars > 0 {
		fmt.Fprintf(&b, `<g fill="#EAF2E1" opacity="%g">`, s.stars)
		for _, st := range bannerStars {
			fmt.Fprintf(&b, `<circle cx="%g" cy="%g" r="%g" opacity="%g"/>`, st[0], st[1], st[2], st[3])
		}
		b.WriteString(`</g>`)
	}
	fmt.Fprintf(&b, `<circle cx="470" cy="%d" r="%d" fill="url(#%sh)"/>`, s.haloCY, s.haloR, id)
	fmt.Fprintf(&b, `<circle cx="470" cy="%d" r="%d" fill="%s"/>`, s.orbCY, s.orbR, s.orbFill)
	fmt.Fprintf(&b, `<path d="M0 88Q90 70 180 82T360 79T540 84T800 76V100H0z" fill="%s" opacity="%g"/>`, s.ridge, s.ridgeOpacity)
	fmt.Fprintf(&b, `<rect y="%d" width="800" height="%d" fill="url(#%sw)"/>`, s.waterY, 132-s.waterY, id)
	fmt.Fprintf(&b, `<g fill="%s" opacity="%g"><rect x="448" y="%d" width="44" height="2.5" rx="1"/><rect x="456" y="%d" width="30" height="2.5" rx="1"/>`, s.glintFill, s.glintOpacity, s.waterY+6, s.waterY+13)
	if s.glintRows == 3 {
		fmt.Fprintf(&b, `<rect x="462" y="%d" width="18" height="2.5" rx="1"/>`, s.waterY+20)
	}
	b.WriteString(`</g>`)
	fmt.Fprintf(&b, `<g fill="%[1]s"><ellipse cx="86" cy="60" rx="64" ry="24" opacity=".92"/><ellipse cx="140" cy="68" rx="38" ry="16" opacity=".85"/><ellipse cx="716" cy="64" rx="70" ry="24" opacity=".92"/><ellipse cx="660" cy="72" rx="34" ry="14" opacity=".85"/>`+
		`<g fill="none" stroke="%[1]s" stroke-width="4" stroke-linecap="round"><path d="M30 132C34 106 52 88 82 74"/><path d="M62 132C64 112 78 96 96 82"/><path d="M104 132C104 114 112 100 124 88"/><path d="M780 132C774 104 752 88 720 76"/><path d="M746 132C744 112 730 96 712 84"/><path d="M700 132C702 114 694 100 684 90"/></g></g>`, s.roots)
	fmt.Fprintf(&b, `<rect width="800" height="132" fill="url(#%sv)"/></svg>`, id)
	return b.String()
}

// gateBanner is the day/night pair for one level; gateStyles shows the one
// matching the visitor's color scheme.
func gateBanner(day, night bannerScene) string {
	return `<div class="banner">` + tideBannerSVG("bd", "day", day) + tideBannerSVG("bn", "night", night) + `</div>`
}
