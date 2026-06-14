package nirilayout

import (
	"bytes"
	"flag"
	"fmt"
	"image/color"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/calico32/kdl-go"
	"github.com/diamondburned/gotk4/pkg/cairo"
	"golang.org/x/sys/unix"
)

const (
	defaultBorderWidth = 2
	defaultFontSize    = 10
	defaultFontFamily  = "monospace"
	defaultFontWeight  = cairo.FontWeightNormal
	defaultFontStyle   = cairo.FontSlantNormal
	defaultLineSpacing = 4
)

var niriConfigDir = flag.String("c", "~/.config/niri", N("niri config directory"))

var (
	langFlag      = flag.String("lang", "", N("interface language code (e.g. \"it\"); overrides the system locale"))
	lowercaseFlag = flag.Bool("lowercase", false, N("render all interface text in lowercase"))
	leftAlignFlag = flag.Bool("leftalign", false, N("left-align the text in the search box (default is centered)"))
)

func GetNiriConfigDir() (configDir string, err error) {
	if strings.HasPrefix(*niriConfigDir, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		configDir = filepath.Join(home, (*niriConfigDir)[1:])
	} else {
		configDir = *niriConfigDir
	}

	configDir, err = filepath.Abs(configDir)
	if err != nil {
		return "", err
	}

	return
}

func GatherLayouts(configDir string) ([]Layout, error) {
	files, err := os.ReadDir(configDir)
	if err != nil {
		return nil, err
	}

	layouts := make([]Layout, 0, len(files))
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		if !strings.HasPrefix(file.Name(), "layout_") || !strings.HasSuffix(file.Name(), ".kdl") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(configDir, file.Name()))
		if err != nil {
			return nil, fmt.Errorf("could not read %s: %w", file.Name(), err)
		}
		layout, err := parseLayoutFromConfig(file.Name(), data)
		if err != nil {
			return nil, fmt.Errorf("could not parse %s: %w", file.Name(), err)
		}
		if layout.Name == "" {
			layout.Name = strings.TrimSuffix(strings.TrimPrefix(file.Name(), "layout_"), ".kdl")
		}

		layout.Path = filepath.Join(configDir, file.Name())
		layouts = append(layouts, layout)
	}

	return layouts, nil
}

func SetCurrentLayout(layout Layout) {
	configDir, err := GetNiriConfigDir()
	if err != nil {
		log.Fatal(err)
		return
	}

	temp := filepath.Join(configDir, fmt.Sprintf("nirilayout-%d.kdl", unix.Getpid()))

	err = os.Symlink(layout.Path, temp)
	if err != nil {
		log.Fatal(err)
		return
	}

	err = os.Rename(temp, filepath.Join(configDir, "nirilayout.kdl"))
	if err != nil {
		log.Fatal(err)
		return
	}
}

type Layout struct {
	Path         string            `kdl:"-"`
	Name         string            `kdl:"name"`
	Shortcuts    []string          `kdl:"shortcut"`
	DefaultStyle outputStyleConfig `kdl:"style"`
	Outputs      []*Output         `kdl:"output,multiple"`
}

type Output struct {
	Name         string            `kdl:",arg"`
	NameOverride string            `kdl:"name"`
	LegacyColor  *int              `kdl:"color"` // backwards compat color index
	Style        outputStyleConfig `kdl:"style"`
	Scale        float64           `kdl:"scale"`
	Transform    string            `kdl:"transform"`
	Position     *Position         `kdl:"position"`
	Mode         string            `kdl:"mode"` // WWWWxHHHH[@RR.RRR]
	Modeline     Modeline          `kdl:"modeline"`
	Off          bool              `kdl:"off,presence"`

	resolvedStyle outputStyle
}

// A Modeline is a VESA CVT mode, in Xorg format.
type Modeline struct {
	DotClock   float64  `kdl:",arg"`
	HDisplay   int      `kdl:",arg"`
	HSyncStart int      `kdl:",arg"`
	HSyncEnd   int      `kdl:",arg"`
	HTotal     int      `kdl:",arg"`
	VDisplay   int      `kdl:",arg"`
	VSyncStart int      `kdl:",arg"`
	VSyncEnd   int      `kdl:",arg"`
	VTotal     int      `kdl:",arg"`
	Flags      []string `kdl:",args"`
}

type Position struct {
	X int `kdl:"x"`
	Y int `kdl:"y"`
}

type outputStyle struct {
	fillColor   color.RGBA
	borderColor color.RGBA
	textColor   color.RGBA
	borderWidth int
	fontFamily  string
	fontSize    float64
	fontWeight  cairo.FontWeight
	fontStyle   cairo.FontSlant
	hideDetails bool
	lineSpacing int
}

type outputStyleConfig struct {
	Fill        kdl.Value `kdl:"fill"`
	Border      kdl.Value `kdl:"border"`
	Text        kdl.Value `kdl:"text"`
	BorderWidth *int      `kdl:"border-width"`
	FontFamily  string    `kdl:"font-family"`
	FontSize    float64   `kdl:"font-size"`
	FontWeight  kdl.Value `kdl:"font-weight"`
	FontStyle   kdl.Value `kdl:"font-style"`
	HideDetails *bool     `kdl:"hide-details,presence"`
	LineSpacing *int      `kdl:"line-spacing"`
}

func (o Output) LogicalRect() (x, y, w, h int) {
	if o.Position != nil {
		x = o.Position.X
		y = o.Position.Y
	} else {
		x = -1
		y = -1
	}
	w, h = o.Resolution()
	if o.Scale != 0 {
		w = int(float64(w) / o.Scale)
		h = int(float64(h) / o.Scale)
	}
	switch o.Transform {
	case "90", "flipped-90", "270", "flipped-270":
		w, h = h, w
	}
	return
}

func (o Output) Resolution() (w int, h int) {
	if o.Modeline.DotClock != 0 {
		w = o.Modeline.HDisplay
		h = o.Modeline.VDisplay
	} else if o.Mode != "" {
		parts := strings.Split(strings.Split(o.Mode, "@")[0], "x")
		w, _ = strconv.Atoi(parts[0])
		h, _ = strconv.Atoi(parts[1])
	}
	return w, h
}

func parseLayoutFromConfig(filename string, niriConfig []byte) (layout Layout, err error) {
	var sb strings.Builder
	for line := range bytes.SplitSeq(niriConfig, []byte("\n")) {
		l := bytes.TrimLeft(line, " \t")
		if bytes.HasPrefix(l, []byte("//!")) {
			sb.Write(l[3:])
		} else {
			sb.Write(line)
		}
		sb.WriteByte('\n')
	}

	err = kdl.Decode(strings.NewReader(sb.String()), &layout, kdl.WithSourceName(filename))
	if err != nil {
		return
	}

	if len(layout.Outputs) == 0 {
		err = fmt.Errorf("%s: no outputs defined in layout", filename)
		return
	}

	outputs := make([]*Output, 0, len(layout.Outputs))
	for _, output := range layout.Outputs {
		if output.Off {
			continue
		}
		if output.Modeline.DotClock == 0 && output.Mode == "" {
			return Layout{}, fmt.Errorf("output %s: no mode or modeline defined, can't determine size (use //! mode to set a mode for this output)", output.Name)
		}
		if output.Mode != "" {
			if !strings.Contains(output.Mode, "x") {
				return Layout{}, fmt.Errorf("output %s: mode %q is not in the format WWWxHHH[@RR.RRR]", output.Name, output.Mode)
			}
		}

		name := output.Name
		if output.NameOverride != "" {
			name = output.NameOverride
		}
		output.resolvedStyle, err = resolveOutputStyle(output.Style, output.LegacyColor, layout.DefaultStyle, name)
		if err != nil {
			err = fmt.Errorf("%s: output %s: %w", filename, name, err)
			return
		}

		outputs = append(outputs, output)
	}
	slices.SortFunc(outputs, func(a, b *Output) int {
		return strings.Compare(a.Name, b.Name)
	})
	layout.Outputs = outputs

	return
}

func resolveOutputStyle(s outputStyleConfig, legacyColor *int, defaultStyle outputStyleConfig, outputName string) (style outputStyle, err error) {
	if legacyColor != nil {
		style.fillColor = fillColors[*legacyColor%len(fillColors)]
		style.borderColor = borderColors[*legacyColor%len(borderColors)]
		style.textColor = pickTextColor(style.fillColor)
		style.borderWidth = defaultBorderWidth
		style.fontFamily = defaultFontFamily
		style.fontSize = defaultFontSize
		style.fontWeight = defaultFontWeight
		style.fontStyle = defaultFontStyle
		style.lineSpacing = defaultLineSpacing
		return
	}

	// merge s with defaultStyle first
	if !s.Fill.IsValid() {
		s.Fill = defaultStyle.Fill
	}
	if !s.Border.IsValid() {
		s.Border = defaultStyle.Border
	}
	if !s.Text.IsValid() {
		s.Text = defaultStyle.Text
	}
	if s.BorderWidth == nil {
		s.BorderWidth = defaultStyle.BorderWidth
	}
	if s.FontFamily == "" {
		s.FontFamily = defaultStyle.FontFamily
	}
	if s.FontSize == 0 {
		s.FontSize = defaultStyle.FontSize
	}
	if !s.FontWeight.IsValid() {
		s.FontWeight = defaultStyle.FontWeight
	}
	if !s.FontStyle.IsValid() {
		s.FontStyle = defaultStyle.FontStyle
	}
	if s.HideDetails == nil {
		s.HideDetails = defaultStyle.HideDetails
	}
	if s.LineSpacing == nil {
		s.LineSpacing = defaultStyle.LineSpacing
	}

	// now parse and validate each field
	switch s.Fill.Kind() {
	case kdl.String:
		style.fillColor, err = parseColor(s.Fill.String())
		if err != nil {
			err = fmt.Errorf("invalid fill color: %w", err)
		}
	case kdl.Int:
		style.fillColor = fillColors[s.Fill.Int()%len(fillColors)]
	case kdl.Invalid:
		style.fillColor, _ = pickWindowColors(outputName)
	default:
		err = fmt.Errorf("expected string/int for fill color, got %s", s.Fill.Kind())
		return
	}

	switch s.Border.Kind() {
	case kdl.String:
		style.borderColor, err = parseColor(s.Border.String())
		if err != nil {
			err = fmt.Errorf("invalid border color: %w", err)
		}
	case kdl.Int:
		style.borderColor = borderColors[s.Border.Int()%len(borderColors)]
	case kdl.Invalid:
		if !s.Fill.IsValid() {
			// fill color was also automatic
			_, style.borderColor = pickWindowColors(outputName)
		} else {
			// automatically pick a border color based on the fill color
			style.borderColor = pickBorderColor(style.fillColor)
		}
	default:
		err = fmt.Errorf("expected string/int for border color, got %s", s.Border.Kind())
		return
	}

	switch s.Text.Kind() {
	case kdl.String:
		style.textColor, err = parseColor(s.Text.String())
		if err != nil {
			err = fmt.Errorf("invalid text color: %w", err)
		}
	case kdl.Invalid:
		style.textColor = pickTextColor(style.fillColor)
	default:
		err = fmt.Errorf("expected string/int for text color, got %s", s.Text.Kind())
		return
	}

	style.borderWidth = defaultBorderWidth
	if s.BorderWidth != nil {
		style.borderWidth = *s.BorderWidth
	}

	style.fontFamily = defaultFontFamily
	if s.FontFamily != "" {
		style.fontFamily = s.FontFamily
	}

	style.fontSize = defaultFontSize
	if s.FontSize != 0 {
		style.fontSize = s.FontSize
	}

	style.fontWeight = defaultFontWeight
	switch s.FontWeight.Kind() {
	case kdl.String:
		switch s.FontWeight.String() {
		case "normal":
			style.fontWeight = cairo.FontWeightNormal
		case "bold":
			style.fontWeight = cairo.FontWeightBold
		default:
			err = fmt.Errorf("invalid font weight: %q (expected \"normal\" or \"bold\")", s.FontWeight.String())
			return
		}
	case kdl.Int:
		switch s.FontWeight.Int() {
		case 400:
			style.fontWeight = cairo.FontWeightNormal
		case 700:
			style.fontWeight = cairo.FontWeightBold
		default:
			err = fmt.Errorf("invalid font weight: %d (only 400 and 700 supported)", s.FontWeight.Int())
			return
		}
	case kdl.Invalid:
	default:
		err = fmt.Errorf("expected string/int for font weight, got %s", s.FontWeight.Kind())
		return
	}

	style.fontStyle = defaultFontStyle
	switch s.FontStyle.Kind() {
	case kdl.String:
		switch s.FontStyle.String() {
		case "normal":
			style.fontStyle = cairo.FontSlantNormal
		case "italic":
			style.fontStyle = cairo.FontSlantItalic
		case "oblique":
			style.fontStyle = cairo.FontSlantOblique
		default:
			err = fmt.Errorf("invalid font style: %q", s.FontStyle.String())
			return
		}
	case kdl.Invalid:
	default:
		err = fmt.Errorf("expected string for font style, got %s", s.FontStyle.Kind())
		return
	}

	style.hideDetails = false
	if s.HideDetails != nil {
		style.hideDetails = *s.HideDetails
	}

	style.lineSpacing = defaultLineSpacing
	if s.LineSpacing != nil {
		style.lineSpacing = *s.LineSpacing
	}

	return
}
