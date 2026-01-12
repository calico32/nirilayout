package main

import (
	"bytes"
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"hash/fnv"
	"image/color"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/diamondburned/gotk4-layer-shell/pkg/gtk4layershell"
	"github.com/diamondburned/gotk4/pkg/cairo"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/calico32/kdl-go"
)

const version = `nirilayout v0.1.0`

type Layout struct {
	path      string
	Name      string    `kdl:"name"`
	Shortcuts []string  `kdl:"shortcut"`
	Displays  []Display `kdl:"display,multiple"`
}

type Display struct {
	Name   string `kdl:",arg"`
	X      int    `kdl:"x"`
	Y      int    `kdl:"y"`
	Width  int    `kdl:"w"`
	Height int    `kdl:"h"`
}

func parseLayoutFromConfig(filename string, niriConfig []byte) (layout Layout, err error) {
	var sb strings.Builder
	for line := range bytes.SplitSeq(niriConfig, []byte("\n")) {
		if bytes.HasPrefix(line, []byte("//!")) {
			sb.Write(line[3:])
			sb.WriteByte('\n')
		}
	}

	err = kdl.DecodeNamed(filename, strings.NewReader(sb.String()), &layout)
	return
}

func gatherLayouts(configDir string) ([]Layout, error) {
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
			return nil, err
		}
		if layout.Name == "" {
			layout.Name = strings.TrimSuffix(strings.TrimPrefix(file.Name(), "layout_"), ".kdl")
		}
		layout.path = filepath.Join(configDir, file.Name())
		layouts = append(layouts, layout)
	}

	return layouts, nil
}

func setCurrentLayout(layout Layout) {
	configDir, err := getNiriConfigDir()
	if err != nil {
		log.Fatal(err)
		return
	}

	temp := filepath.Join(configDir, fmt.Sprintf("nirilayout-%d.kdl", unix.Getpid()))

	err = os.Symlink(layout.path, temp)
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

var niriConfigDir = flag.String("c", "~/.config/niri", "niri config directory")

func getNiriConfigDir() (configDir string, err error) {
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

func main() {
	flag.Parse()

	var layouts []Layout
	var current string

	configDir, err := getNiriConfigDir()

	if err == nil {
		layouts, err = gatherLayouts(configDir)
	}

	if err == nil {
		current, err = os.Readlink(filepath.Join(configDir, "nirilayout.kdl"))
		if errors.Is(err, fs.ErrNotExist) {
			err = nil
		}
	}

	index := slices.IndexFunc(layouts, func(layout Layout) bool {
		return layout.path == current
	})
	if index == -1 {
		index = 0
	}

	app := gtk.NewApplication("co.calebc.nirilayout", gio.ApplicationDefaultFlags)
	app.ConnectActivate(func() {
		activate(app, layouts, index, err)
	})

	if code := app.Run(nil); code > 0 {
		os.Exit(code)
	}
}

//go:embed style.css
var stylesheet string

func activate(app *gtk.Application, layouts []Layout, startIndex int, err error) {
	gtk.StyleContextAddProviderForDisplay(
		gdk.DisplayGetDefault(), loadStylesheet(stylesheet),
		gtk.STYLE_PROVIDER_PRIORITY_APPLICATION,
	)

	win := gtk.NewApplicationWindow(app)
	win.SetTitle("nirilayout")

	gtk4layershell.InitForWindow(&win.Window)
	gtk4layershell.SetLayer(&win.Window, gtk4layershell.LayerShellLayerOverlay)
	gtk4layershell.SetKeyboardMode(&win.Window, gtk4layershell.LayerShellKeyboardModeExclusive)
	gtk4layershell.SetMargin(&win.Window, gtk4layershell.LayerShellEdgeTop, 10)

	root := gtk.NewBox(gtk.OrientationVertical, 16)
	win.SetChild(root)

	var selector *gtk.Box

	if err != nil {
		label := gtk.NewLabel(fmt.Sprintf("Error loading layouts: %v", err))
		label.SetHAlign(gtk.AlignCenter)
		root.Append(label)
	} else if len(layouts) == 0 {
		label := gtk.NewLabel("No layouts found. Please create layout_<name>.kdl files ~/.config/niri to use nirilayout.")
		label.SetHAlign(gtk.AlignCenter)
		root.Append(label)
	} else {
		selector = gtk.NewBox(gtk.OrientationHorizontal, 16)
		root.Append(selector)
	}

	for i, layout := range layouts {
		button := gtk.NewButton()
		b := gtk.NewBox(gtk.OrientationVertical, 8)
		container := gtk.NewCenterBox()
		container.SetSizeRequest(200, 200)
		preview := drawLayout(layout)
		preview.SetHAlign(gtk.AlignCenter)
		preview.SetVAlign(gtk.AlignCenter)
		container.SetCenterWidget(preview)
		b.Append(container)

		if len(layout.Shortcuts) == 0 {
			b.Append(gtk.NewLabel(layout.Name))
		} else {
			b.Append(gtk.NewLabel(fmt.Sprintf("%v %s", layout.Shortcuts, layout.Name)))
		}

		button.SetChild(b)
		button.ConnectClicked(func() {
			setCurrentLayout(layout)
			app.Quit()
		})
		if i == startIndex {
			button.AddCSSClass("selected")
			button.AddCSSClass("current")
		}

		button.SetCursorFromName("pointer")

		selector.Append(button)
	}

	inputBox := gtk.NewCenterBox()

	input := gtk.NewEntry()
	input.SetSizeRequest(400, 0)
	input.SetAlignment(0.5)
	input.SetPlaceholderText("name or shortcut...")
	input.ConnectChanged(func() {
		text := input.Text()
		for _, layout := range layouts {
			for _, shortcut := range layout.Shortcuts {
				if text == shortcut {
					setCurrentLayout(layout)
					app.Quit()
				}
			}
			if text == layout.Name {
				setCurrentLayout(layout)
				app.Quit()
			}
		}
	})
	label := gtk.NewLabel(version)
	label.SetSensitive(false)
	label.SetMarginEnd(16)
	inputBox.SetStartWidget(label)
	label = gtk.NewLabel("esc to quit")
	label.SetSensitive(false)
	label.SetMarginStart(16)
	inputBox.SetEndWidget(label)

	inputBox.SetCenterWidget(input)

	root.Append(inputBox)

	win.ConnectShow(func() {
		input.GrabFocus()
		win.AddCSSClass("visible")
	})

	index := startIndex

	setActiveLayout := func(i int) {
		if len(layouts) == 0 {
			return
		}
		j := 0
		child := selector.FirstChild()
		for child != nil {
			button := child.(*gtk.Button)
			if j == i {
				button.AddCSSClass("selected")
			} else {
				button.RemoveCSSClass("selected")
			}
			child = button.NextSibling()
			j++
		}
	}

	quit := func() {
		win.RemoveCSSClass("visible")
		glib.TimeoutAdd(75, func() bool {
			app.Quit()
			return false
		})
	}

	k := gtk.NewEventControllerKey()
	k.SetPropagationPhase(gtk.PhaseCapture)
	k.ConnectKeyPressed(func(keyval uint, keycode uint, state gdk.ModifierType) bool {
		switch keyval {
		case gdk.KEY_Escape:
			quit()
			return true
		case gdk.KEY_Right:
			index++
			if index >= len(layouts) {
				index = 0
			}
			setActiveLayout(index)
			return true
		case gdk.KEY_Left:
			index--
			if index < 0 {
				index = len(layouts) - 1
			}
			setActiveLayout(index)
			return true
		case gdk.KEY_Return:
			if len(layouts) != 0 {
				setCurrentLayout(layouts[index])
			}
			quit()
			return true
		}
		return false
	})
	input.AddController(k)

	win.SetVisible(true)
}

func drawLayout(layout Layout) *gtk.DrawingArea {
	const targetSize = 200
	const borderWidth = 2

	da := gtk.NewDrawingArea()
	layoutWidth := float64(0)
	layoutHeight := float64(0)
	for _, layout := range layout.Displays {
		layoutWidth = max(layoutWidth, float64(layout.X)+float64(layout.Width))
		layoutHeight = max(layoutHeight, float64(layout.Y)+float64(layout.Height))
	}
	scale := min(targetSize/layoutWidth, targetSize/layoutHeight)
	da.SetSizeRequest(int(layoutWidth*scale)+borderWidth, int(layoutHeight*scale)+borderWidth)

	da.SetDrawFunc(func(drawingArea *gtk.DrawingArea, cr *cairo.Context, width, height int) {
		cr.SetSourceRGBA(0, 0, 0, 0)
		cr.Paint()
		cr.SelectFontFace("monospace", cairo.FontSlantNormal, cairo.FontWeightNormal)
		cr.SetFontSize(10)
		cr.MoveTo(0, 0)
		for _, layout := range layout.Displays {
			x, y, w, h := float64(layout.X)*scale, float64(layout.Y)*scale,
				float64(layout.Width)*scale, float64(layout.Height)*scale

			name := layout.Name
			i := -1
			if index := strings.IndexByte(layout.Name, ':'); index != -1 {
				name = layout.Name[:index]
				c, err := strconv.Atoi(layout.Name[index+1:])
				if err != nil {
					log.Fatal(err)
				}
				i = c
			}
			windowColor, borderColor := pickWindowColors(name, i)

			cr.Rectangle(x, y, w, h)
			cr.SetSourceRGBA(rgba(windowColor))
			cr.Fill()

			cr.Rectangle(x+borderWidth/2, y+borderWidth/2, w-borderWidth, h-borderWidth)
			cr.SetLineWidth(borderWidth)
			cr.SetSourceRGBA(rgba(borderColor))
			cr.Stroke()

			extents := cr.TextExtents(name)
			cr.MoveTo(x+w/2-extents.Width/2-extents.XBearing, y+h/2-extents.Height/2-extents.YBearing)
			cr.SetSourceRGBA(rgba(gray100))
			cr.ShowText(name)
		}
	})

	return da
}

var fillColors = []color.Color{
	gray600,
	red700, orange700, amber700, yellow700,
	lime700, green700, emerald700, teal700,
	cyan700, sky700, blue700, indigo700,
	violet700, purple700, fuchsia700, pink700,
	rose700,
}

var borderColors = []color.Color{
	gray400,
	red500, orange500, amber500, yellow500,
	lime500, green500, emerald500, teal500,
	cyan500, sky500, blue500, indigo500,
	violet500, purple500, fuchsia500, pink500,
	rose500,
}

func pickWindowColors(name string, i int) (fill, border color.Color) {
	if i >= 0 {
		return fillColors[i%len(fillColors)], borderColors[i%len(borderColors)]
	}
	h := fnv.New32a()
	h.Write([]byte(name))
	i = int(h.Sum32()) % len(fillColors)
	return fillColors[i], borderColors[i]
}

func rgba(color color.Color) (r, g, b, a float64) {
	rw, gw, bw, aw := color.RGBA()
	r = float64(rw) / 0xffff
	g = float64(gw) / 0xffff
	b = float64(bw) / 0xffff
	a = float64(aw) / 0xffff
	return
}

func loadStylesheet(content string) *gtk.CSSProvider {
	prov := gtk.NewCSSProvider()
	prov.ConnectParsingError(func(sec *gtk.CSSSection, err error) {
		loc := sec.StartLocation()
		lines := strings.Split(content, "\n")
		log.Printf("CSS error (%v) at line: %q", err, lines[loc.Lines()])
	})
	prov.LoadFromString(content)
	return prov
}
