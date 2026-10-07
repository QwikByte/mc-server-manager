package server

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// formatting matches the escape sequences of terminals, with the parameters and final byte of
// those that set colours (SGR, ending in m), and Minecraft's codes: a colour in hexadecimal
// (§x§r§r§g§g§b§b) or a single code (§a, §l, ...).
var formatting = regexp.MustCompile(`\x1b\[([0-9;?]*)[ -/]*([@-~])|§[xX]((?:§[0-9a-fA-F]){6})|§([0-9a-fk-orxA-FK-ORX])|\r`)

// plain removes colours and formatting so that console output reads as plain text, drops the
// other control characters but line breaks and tabs, so that a server can't control the
// terminal of the local CLI, and replaces invalid UTF-8, which gRPC can't send in a string,
// as strings.Map does.
func plain(text string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, formatting.ReplaceAllString(text, ""))
}

// formatted turns the colours and formatting of console output into Minecraft's codes, as
// StreamLogsResponse describes them, or returns "" if the text has none.
func formatted(text string) string {
	text = strings.ToValidUTF8(text, "�")
	var out strings.Builder
	var current, shown style
	coded := false
	write := func(s string) {
		if s == "" {
			return
		}
		codes := current.since(shown)
		coded = coded || codes != ""
		out.WriteString(codes + s)
		shown = current
	}
	last := 0
	for _, m := range formatting.FindAllStringSubmatchIndex(text, -1) {
		write(text[last:m[0]])
		last = m[1]
		group := func(i int) string {
			if m[2*i] < 0 {
				return ""
			}
			return text[m[2*i]:m[2*i+1]]
		}
		switch {
		case group(2) == "m":
			current = current.sgr(group(1))
		case group(3) != "":
			rgb, _ := strconv.ParseUint(strings.ReplaceAll(group(3), "§", ""), 16, 32)
			current = style{color: nearest(int(rgb>>16), int(rgb>>8&0xff), int(rgb&0xff))}
		case group(4) != "":
			current = current.code(strings.ToLower(group(4))[0])
		}
	}
	write(text[last:])
	if !coded {
		return ""
	}
	return out.String()
}

// style is the colour and formatting of text, as Minecraft knows them.
type style struct {
	color byte // Minecraft's code of the colour, from 0 to f, or 0 for the default
	// basic is set for the 8 colours of terminals, which are bright while bright is set, as
	// Paper, Velocity and BungeeCord write Minecraft's light colours.
	basic, bright bool
	formats       uint8 // bits of formatCodes
}

// formatCodes are Minecraft's codes of bold, strikethrough, underlined and italic text.
const formatCodes = "lmno"

// terminalColors are Minecraft's codes of the 8 colours of terminals: black, red, green,
// yellow, blue, magenta, cyan and white. Each one's light colour is 8 codes further.
const terminalColors = "04261537"

// palette holds the colours of Minecraft's codes 0 to f.
var palette = [16][3]int{
	{0, 0, 0}, {0, 0, 170}, {0, 170, 0}, {0, 170, 170}, {170, 0, 0}, {170, 0, 170}, {255, 170, 0}, {170, 170, 170},
	{85, 85, 85}, {85, 85, 255}, {85, 255, 85}, {85, 255, 255}, {255, 85, 85}, {255, 85, 255}, {255, 255, 85}, {255, 255, 255},
}

const hexDigits = "0123456789abcdef"

// light returns the light colour of one of Minecraft's codes 0 to 7.
func light(code byte) byte { return hexDigits[strings.IndexByte(hexDigits, code)+8] }

// nearest returns the code of the colour of Minecraft that is nearest to the given one.
func nearest(r, g, b int) byte {
	best, distance := 0, 1<<30
	for i, c := range palette {
		if d := (r-c[0])*(r-c[0]) + (g-c[1])*(g-c[1]) + (b-c[2])*(b-c[2]); d < distance {
			best, distance = i, d
		}
	}
	return hexDigits[best]
}

// shown returns the colour that text of the style shows in.
func (s style) shown() byte {
	if s.basic && s.bright {
		return light(s.color)
	}
	return s.color
}

// since returns the codes that change text from the style prev to s.
func (s style) since(prev style) string {
	var b strings.Builder
	formats := prev.formats
	if s.shown() != prev.shown() || formats&^s.formats != 0 {
		// A colour resets the formatting, and only §r removes it without a colour.
		if s.shown() == 0 {
			b.WriteString("§r")
		} else {
			b.WriteString("§" + string(s.shown()))
		}
		formats = 0
	}
	for i := range len(formatCodes) {
		if s.formats&^formats&(1<<i) != 0 {
			b.WriteString("§" + formatCodes[i:i+1])
		}
	}
	return b.String()
}

// code applies one of Minecraft's codes; obfuscated text (k) is shown as it is.
func (s style) code(c byte) style {
	switch {
	case c == 'r':
		return style{}
	case strings.IndexByte(hexDigits, c) >= 0:
		return style{color: c}
	case strings.IndexByte(formatCodes, c) >= 0:
		s.formats |= 1 << strings.IndexByte(formatCodes, c)
	}
	return s
}

// sgr applies the parameters of a terminal's sequence that sets colours, e.g. "0;32;1".
// Backgrounds are left out, and so is blinking, which Minecraft's obfuscated text becomes.
func (s style) sgr(params string) style {
	p := strings.Split(params, ";")
	for i := 0; i < len(p); i++ {
		n, _ := strconv.Atoi(p[i]) // empty means 0
		switch {
		case n == 0:
			s = style{}
		case n == 1:
			s.bright = true
		case n == 22:
			s.bright = false
		case n == 21: // Minecraft's bold, as Paper, Velocity and BungeeCord write it
			s.formats |= 1
		case n == 9 || n == 29:
			s.formats = toggle(s.formats, 2, n == 9)
		case n == 4 || n == 24:
			s.formats = toggle(s.formats, 4, n == 4)
		case n == 3 || n == 23:
			s.formats = toggle(s.formats, 8, n == 3)
		case n >= 30 && n <= 37:
			s.color, s.basic = terminalColors[n-30], true
		case n >= 90 && n <= 97:
			s.color, s.basic = light(terminalColors[n-90]), false
		case n == 39:
			s.color, s.basic = 0, false
		case n == 38 || n == 48: // extended colours: 5;index or 2;r;g;b
			color, used := extended(p[i+1:])
			if n == 38 && color != 0 {
				s.color, s.basic = color, false
			}
			i += used
		}
	}
	return s
}

func toggle(formats, bit uint8, on bool) uint8 {
	if on {
		return formats | bit
	}
	return formats &^ bit
}

// extended returns the code of the colour that the parameters after 38 or 48 name, and how
// many of them it used; 0 if they name none.
func extended(p []string) (byte, int) {
	n := make([]int, min(len(p), 4))
	for i := range n {
		n[i], _ = strconv.Atoi(p[i])
	}
	switch {
	case len(n) >= 2 && n[0] == 5:
		return color256(n[1]), 2
	case len(n) >= 4 && n[0] == 2:
		return nearest(n[1], n[2], n[3]), 4
	}
	return 0, len(n)
}

// color256 returns the code nearest to a colour of 256-colour terminals.
func color256(i int) byte {
	switch {
	case i < 0 || i > 255:
		return 0
	case i < 8:
		return terminalColors[i]
	case i < 16:
		return light(terminalColors[i-8])
	case i < 232: // a cube of 6×6×6 colours
		level := func(v int) int { return min(v, 1)*55 + v*40 }
		i -= 16
		return nearest(level(i/36), level(i/6%6), level(i%6))
	}
	grey := 8 + (i-232)*10
	return nearest(grey, grey, grey)
}
