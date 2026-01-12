# nirilayout

nirilayout is a simple tool to quickly switch your
[niri](https://github.com/YaLTeR/niri) output configuration between different
layouts. Especially useful for laptop users who move between different setups
frequently.

![nirilayout screenshot](screenshot.png)

## Usage

### Install nirilayout

Install Go 1.25+, GTK 4.16+ and [GTK4 Layer
Shell](https://github.com/wmww/gtk4-layer-shell). Clone this repo, then run `go
install` to install nirilayout to $GOBIN. Alternatively, run `make` to build
nirilayout in the current directory.

### Configure layouts

_(This assumes you're using the default config directory `~/.config/niri`. If
your configuration is elsewhere, pass the `-c` flag to nirilayout each time you
run it.)_

In `~/.config/niri`, create layouts for your different setups in files
named `layout_<name>.kdl`.

Each layout file should contain the `output` blocks that niri normally uses (see
[niri's docs](https://yalter.github.io/niri/Configuration%3A-Outputs.html)).
Then, add special nirilayout-specific comments throughout the file to configure
the nirilayout switcher. nirilayout will uncomment any line that starts with
`//!` and parse them as KDL alongside the regular niri configuration.

Global configuration, outside of any `output` block:

- `//! name`: The name of the layout. Wrap in quotes if it contains spaces.
- `//! shortcut`: The shortcut(s) to use for this layout. You can specify multiple
  shortcuts by separating them with spaces.

Per-output configuration:

- `//! name`: Specifies a custom name for this output. If not specified, the
  name at the top (`output "..."`) will be used.
- `//! color`: Specifies a custom color for this output. nirilayout will pick a
  color for each display based on its name, but you can pick a custom color
  yourself by setting this option to a number between 0 and 17. 0 is gray and
  1-17 are the first 17 colors of the [Tailwind CSS color
  palette](https://tailwindcss.com/docs/colors).

  | Index | Color  |     | Index | Color   |     | Index | Color   |
  | ----- | ------ | --- | ----- | ------- | --- | ----- | ------- |
  | 0     | gray   |     | 6     | green   |     | 12    | indigo  |
  | 1     | red    |     | 7     | emerald |     | 13    | violet  |
  | 2     | orange |     | 8     | teal    |     | 14    | purple  |
  | 3     | amber  |     | 9     | cyan    |     | 15    | fuchsia |
  | 4     | yellow |     | 10    | sky     |     | 16    | pink    |
  | 5     | lime   |     | 11    | blue    |     | 17    | rose    |

- `//! mode`: If you don't want to specify a mode to niri, you'll need to
  explicitly set a nirilayout-only size so that the switcher can draw a preview.
  Do this by using a `//! mode` comment with the desired mode in `"WWWxHHH"`
  format.

For example, `layout_vertical.kdl` might look like this:

```kdl
//! name Vertical
//! shortcut v

output "Lenovo Group Limited E27q-20 V5HDD696" {
    //! name "external"
    mode "2560x1440@74.780"
    scale 1
    position x=0 y=0
}

output "BOE 0x0AC1 Unknown" {
    //! name "laptop"
    //! color 9
    mode "2560x1600@120.001"
    scale 1.5
    position x=427 y=1440
}
```

## Configure niri

Now, run `nirilayout` once to select an initial layout. This creates
`~/.config/niri/nirilayout.kdl`, a symlink to the layout you selected.

Finally, remove any `output` blocks in `~/.config/niri/config.kdl` and add an
include to load `nirilayout.kdl`.

```kdl
// config.kdl
// ...
include "nirilayout.kdl"
// ...
```

All done!

## Use nirilayout

Run `nirilayout` again to switch between your layouts. You can bind a shortcut
to `nirilayout` in your `~/.config/niri/config.kdl` to make it easier to switch.

In the switcher, you can select a layout with `←`/`→`/`Return` or the mouse. You
can also type the name of a layout or its shortcut to select it. Shortcuts and
names are case-sensitive.

As soon as you finish typing a name/shortcut, the switcher will immediately
change to the selected layout; no need to press return/enter. Note that this
means that shortcuts cannot be prefixes of other shortcuts (e.g. if you have
both "a" and "ab" as shortcuts, typing "a" will immediately select the first
layout and you won't be able to type "ab").

Layouts are presented in lexicographical order by name. If you want to change
the order, you can rename the files in the config directory.

# Contributing

Contributions are welcome! If you find a bug or have a feature request, please
open an issue or a pull request.

# License

nirilayout is licensed under the MIT license. See [LICENSE](LICENSE) for more
information.
