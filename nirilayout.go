package nirilayout

import (
	_ "embed"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/diamondburned/gotk4-layer-shell/pkg/gtk4layershell"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

const version = `nirilayout v0.4.0`

//go:embed style.css
var appStylesheet string

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

func Run(app *gtk.Application, layouts []Layout, startIndex int, err error) {
	// load default stylesheet
	gtk.StyleContextAddProviderForDisplay(
		gdk.DisplayGetDefault(), loadStylesheet(appStylesheet),
		gtk.STYLE_PROVIDER_PRIORITY_APPLICATION,
	)

	// load ~/.config/niri/nirilayout.css if it exists, to allow user overrides of the default stylesheet
	configDir, configErr := GetNiriConfigDir()
	if configErr == nil {
		userStylesheetPath := filepath.Join(configDir, "nirilayout.css")
		if _, err := os.Stat(userStylesheetPath); err == nil {
			content, err := os.ReadFile(userStylesheetPath)
			if err != nil {
				log.Printf("Error reading user stylesheet: %v", err)
			} else {
				gtk.StyleContextAddProviderForDisplay(
					gdk.DisplayGetDefault(), loadStylesheet(string(content)),
					gtk.STYLE_PROVIDER_PRIORITY_USER,
				)
			}
		}
	}

	win := gtk.NewApplicationWindow(app)
	win.SetTitle("nirilayout")

	gtk4layershell.InitForWindow(&win.Window)
	gtk4layershell.SetLayer(&win.Window, gtk4layershell.LayerShellLayerOverlay)
	gtk4layershell.SetKeyboardMode(&win.Window, gtk4layershell.LayerShellKeyboardModeExclusive)
	gtk4layershell.SetMargin(&win.Window, gtk4layershell.LayerShellEdgeTop, 10)
	gtk4layershell.SetNamespace(&win.Window, "nirilayout")

	root := gtk.NewBox(gtk.OrientationVertical, 16)
	win.SetChild(root)

	quit := func() {
		win.RemoveCSSClass("visible")
		glib.TimeoutAdd(75, func() bool {
			app.Quit()
			return false
		})
	}

	var selector *gtk.FlowBox

	if err != nil {
		label := gtk.NewLabel(Tf("Error loading layouts: %v", err))
		label.SetHAlign(gtk.AlignCenter)
		root.Append(label)
	} else if len(layouts) == 0 {
		label := gtk.NewLabel(T("No layouts found. Please create layout_<name>.kdl files in ~/.config/niri to use nirilayout."))
		label.SetHAlign(gtk.AlignCenter)
		root.Append(label)
	} else {
		selector = gtk.NewFlowBox()
		selector.SetColumnSpacing(8)
		selector.SetRowSpacing(8)
		selector.SetMaxChildrenPerLine(5)
		selector.SetOrientation(gtk.OrientationHorizontal)
		selector.SetSelectionMode(gtk.SelectionNone)
		selector.SetActivateOnSingleClick(true)
		root.Append(selector)
	}

	for i, layout := range layouts {
		button := gtk.NewButton()
		b := gtk.NewBox(gtk.OrientationVertical, 8)
		container := gtk.NewCenterBox()
		container.SetSizeRequest(drawingSize, drawingSize)
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
			SetCurrentLayout(layout)
			app.Quit()
		})

		button.SetCursorFromName("pointer")

		selector.Insert(button, -1)
		if i == startIndex {
			button.AddCSSClass("selected")
			button.AddCSSClass("current")
		}
	}

	inputBox := gtk.NewCenterBox()

	input := gtk.NewEntry()
	input.SetSizeRequest(400, 0)
	if *leftAlignFlag {
		input.SetAlignment(0)
	} else {
		input.SetAlignment(0.5)
	}
	input.SetPlaceholderText(T("Name or shortcut…"))
	input.ConnectChanged(func() {
		text := input.Text()
		for _, layout := range layouts {
			for _, shortcut := range layout.Shortcuts {
				if text == shortcut {
					SetCurrentLayout(layout)
					app.Quit()
				}
			}
			if text == layout.Name {
				SetCurrentLayout(layout)
				quit()
			}
		}
	})
	label := gtk.NewLabel(version)
	label.SetSensitive(false)
	label.SetMarginEnd(16)
	inputBox.SetStartWidget(label)
	label = gtk.NewLabel(T("Esc to quit"))
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
		for j := 0; selector.ChildAtIndex(j) != nil; j++ {
			button := selector.ChildAtIndex(j).Child().(*gtk.Button)
			if j == i {
				button.AddCSSClass("selected")
			} else {
				button.RemoveCSSClass("selected")
			}
		}
		if len(layouts) == 0 {
			return
		}
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
		case gdk.KEY_Up:
			skip := int(selector.MaxChildrenPerLine())
			if index-skip >= 0 {
				index -= skip
			}
			setActiveLayout(index)
			return true
		case gdk.KEY_Down:
			skip := int(selector.MaxChildrenPerLine())
			if index+skip < len(layouts) {
				index += skip
			}
			setActiveLayout(index)
			return true
		case gdk.KEY_Return:
			if len(layouts) != 0 {
				SetCurrentLayout(layouts[index])
			}
			quit()
			return true
		}
		return false
	})
	input.AddController(k)

	win.SetVisible(true)
}
