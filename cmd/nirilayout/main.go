package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"nirilayout"
	"os"
	"path/filepath"
	"slices"

	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func main() {
	// Initialize i18n from the system locale first so that flag.Usage (which
	// flag.Parse may invoke on -h or a parse error) is already localized. It
	// is re-initialized after flag.Parse to honor -lang and -lowercase.
	nirilayout.InitI18n()

	flag.Usage = func() {
		fmt.Print(nirilayout.T("nirilayout is a layout switcher for niri.\n\nCommand-line options:\n"))
		// Translate each flag's usage string in place, then let the flag
		// package handle the formatting (types, defaults, alignment).
		flag.VisitAll(func(f *flag.Flag) {
			f.Usage = nirilayout.T(f.Usage)
		})
		flag.PrintDefaults()
		fmt.Print(nirilayout.T("\nTo use nirilayout, create layouts in files called ~/.config/niri/layout_<name>.kdl and run nirilayout.\nSee the README for more details:\n"))
		fmt.Print("  https://github.com/calico32/nirilayout/blob/main/README.md\n")
	}

	flag.Parse()

	// Apply -lang and -lowercase now that flags are parsed.
	nirilayout.InitI18n()

	var layouts []nirilayout.Layout
	var current string

	configDir, err := nirilayout.GetNiriConfigDir()
	if err == nil {
		layouts, err = nirilayout.GatherLayouts(configDir)
	}

	if err == nil {
		current, err = os.Readlink(filepath.Join(configDir, "nirilayout.kdl"))
		if errors.Is(err, fs.ErrNotExist) {
			err = nil
		}
	}

	index := slices.IndexFunc(layouts, func(l nirilayout.Layout) bool {
		return l.Path == current
	})
	if index == -1 {
		index = 0
	}

	app := gtk.NewApplication("co.calebc.nirilayout", gio.ApplicationDefaultFlags)
	app.ConnectActivate(func() {
		nirilayout.Run(app, layouts, index, err)
	})

	if code := app.Run(nil); code > 0 {
		os.Exit(code)
	}
}
