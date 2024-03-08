package theme

import (
	"context"
	"image"
	rtrace "runtime/trace"
	"strings"

	"joonho3020/go/gotraceui/color"
	"joonho3020/go/gotraceui/gesture"
	"joonho3020/go/gotraceui/layout"
	"joonho3020/go/gotraceui/widget"

	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/x/eventx"
	"golang.org/x/exp/slices"
)

// XXX split into style and state
type CommandPalette struct {
	Prompt string

	editor widget.Editor
	list   widget.List

	filtered []int
	active   int
	cmds     CommandProvider
	tags     []byte
	gestures []gesture.Click
}

type Command interface {
	Layout(win *Window, gtx layout.Context, current bool) layout.Dimensions
	Filter(input string) bool
	Link() Action
}

type CommandProvider interface {
	At(idx int) Command
	Len() int
}

type CommandSlice []Command

func (cs CommandSlice) Len() int {
	return len(cs)
}

func (cs CommandSlice) At(idx int) Command {
	return cs[idx]
}

type MultiCommandProvider struct {
	Providers []CommandProvider
}

func (mcp MultiCommandProvider) Len() int {
	n := 0
	for _, cp := range mcp.Providers {
		n += cp.Len()
	}
	return n
}

func (mcp MultiCommandProvider) At(idx int) Command {
	providers := mcp.Providers
	for idx >= providers[0].Len() {
		idx -= providers[0].Len()
		providers = providers[1:]
	}
	return providers[0].At(idx)
}

type NormalCommand struct {
	PrimaryLabel   string
	SecondaryLabel string
	Category       string
	Color          color.Oklch
	Shortcut       string
	Aliases        []string
	Fn             func() Action
}

func (cmd NormalCommand) Link() Action {
	return cmd.Fn()
}

func (cmd NormalCommand) Filter(input string) bool {
	for _, f := range strings.Fields(input) {
		// XXX calling ToLower every time Filter gets called is a bit stupid
		lf := strings.ToLower(f)
		if !(strings.Contains(strings.ToLower(cmd.PrimaryLabel), lf) ||
			strings.Contains(strings.ToLower(cmd.SecondaryLabel), lf) ||
			strings.Contains(strings.ToLower(cmd.Category), lf)) {

			any := false
			for _, alias := range cmd.Aliases {
				if strings.Contains(strings.ToLower(alias), lf) {
					any = true
					break
				}
			}

			if !any {
				return false
			}

		}
	}
	return true
}

func (cmd NormalCommand) Layout(win *Window, gtx layout.Context, current bool) layout.Dimensions {
	defer rtrace.StartRegion(context.Background(), "theme.NormalCommand.Layout").End()

	activeColor := color.Oklch{L: 0.9394, C: 0.22094984386637648, H: 119.08, A: 1}

	bg := cmd.Color
	if current {
		bg = activeColor
	}

	const padding unit.Dp = 2
	const indicatorWidth unit.Dp = 3

	right := Record(win, gtx, func(win *Window, gtx layout.Context) layout.Dimensions {
		if current {
			gtx.Constraints.Max.X -= gtx.Dp(indicatorWidth)
		}

		var dims layout.Dimensions
		return layout.UniformInset(5).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Rigids(gtx, layout.Horizontal,
				func(gtx layout.Context) layout.Dimensions {
					dims = layout.Rigids(gtx, layout.Vertical,
						func(gtx layout.Context) layout.Dimensions {
							f := font.Font{Weight: font.Bold}
							return widget.Label{MaxLines: 1}.Layout(gtx, win.Theme.Shaper, f, 14, cmd.PrimaryLabel, win.ColorMaterial(gtx, win.Theme.Palette.Foreground))
						},
						func(gtx layout.Context) layout.Dimensions {
							if cmd.SecondaryLabel == "" {
								return layout.Dimensions{}
							}
							f := font.Font{}
							return widget.Label{MaxLines: 0}.Layout(gtx, win.Theme.Shaper, f, 14, cmd.SecondaryLabel, win.ColorMaterial(gtx, win.Theme.Palette.Foreground))
						},
						func(gtx layout.Context) layout.Dimensions {
							if cmd.Category == "" {
								return layout.Dimensions{}
							}
							f := font.Font{Style: font.Italic}
							// XXX avoid the allocation
							return widget.Label{MaxLines: 1}.Layout(gtx, win.Theme.Shaper, f, 12, "Category: "+cmd.Category, win.ColorMaterial(gtx, win.Theme.Palette.Foreground))
						},
					)

					return dims
				},

				func(gtx layout.Context) layout.Dimensions {
					if cmd.Shortcut == "" {
						return layout.Dimensions{}
					}

					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					gtx.Constraints.Min.Y = dims.Size.Y
					return layout.MiddleAligned(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.RightAligned(gtx, func(gtx layout.Context) layout.Dimensions {
							return Bordered{Width: 1, Color: oklch(0, 0, 0)}.Layout(win, gtx, func(win *Window, gtx layout.Context) layout.Dimensions {
								return Background{Color: oklch(100, 0, 0)}.Layout(win, gtx, func(win *Window, gtx layout.Context) layout.Dimensions {
									return layout.UniformInset(padding).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										f := font.Font{
											Weight: font.Bold,
										}
										return widget.Label{MaxLines: 1}.Layout(gtx, win.Theme.Shaper, f, 14, cmd.Shortcut, win.ColorMaterial(gtx, win.Theme.Palette.Foreground))
									})
								})
							})
						})
					})
				},
			)
		})
	})

	return Background{
		Color: bg,
	}.Layout(win, gtx, func(win *Window, gtx layout.Context) layout.Dimensions {
		return layout.Rigids(gtx, layout.Horizontal,
			func(gtx layout.Context) layout.Dimensions {
				if !current {
					return layout.Dimensions{}
				}

				FillShape(win, gtx.Ops, oklch(0, 0, 0), clip.Rect{Max: image.Pt(gtx.Dp(indicatorWidth), right.Dimensions.Size.Y)}.Op())
				return layout.Dimensions{Size: image.Pt(gtx.Dp(indicatorWidth), right.Dimensions.Size.Y)}
			},
			func(gtx layout.Context) layout.Dimensions {
				return right.Layout(win, gtx)
			},
		)
	})
}

func (pl *CommandPalette) Set(cmds CommandProvider) {
	pl.cmds = cmds
	pl.filter(pl.editor.Text())
}

func (pl *CommandPalette) filter(input string) {
	if input == "" {
		if cap(pl.filtered) < pl.cmds.Len() {
			pl.filtered = make([]int, pl.cmds.Len())
		} else {
			pl.filtered = pl.filtered[:pl.cmds.Len()]
		}
		for i := range pl.filtered {
			pl.filtered[i] = i
		}
	} else {
		pl.filtered = pl.filtered[:0]

		for i := 0; i < pl.cmds.Len(); i++ {
			if pl.cmds.At(i).Filter(input) {
				pl.filtered = append(pl.filtered, i)
			}
		}
	}
}

func (pl *CommandPalette) Layout(win *Window, gtx layout.Context) layout.Dimensions {
	defer rtrace.StartRegion(context.Background(), "theme.CommandPalette.Layout").End()
	defer clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops).Pop()

	// XXX the combination of this layout function and modals causes the palette to always be centered, which causes the
	// top of the palette to move when the palette's height shrinks. that will happen during filtering, and it's rather
	// disorienting. we want the palette's top border to be fixed instead.
	pl.editor.Submit = true
	pl.list.Axis = layout.Vertical

	const (
		desiredWidth     unit.Dp = 600
		desiredMaxHeight unit.Dp = 300
		outerPadding     unit.Dp = 5
		fontSize         unit.Sp = 14
		separatorHeight  unit.Dp = 2
		outerBorder      unit.Dp = 1
	)
	var (
		background  = oklch(98.29, 0.0286, 145.35)
		borderColor = oklch(0, 0, 0)
	)

	width := min(gtx.Dp(desiredWidth), gtx.Constraints.Max.X)
	maxHeight := min(gtx.Dp(desiredMaxHeight), gtx.Constraints.Max.Y)

	pl.editor.SingleLine = true
	pl.editor.Focus()

	if pl.active > len(pl.filtered)-1 {
		pl.active = max(0, len(pl.filtered)-1)
	}

	m := op.Record(gtx.Ops)
	var spy *eventx.Spy
	dims := Bordered{Color: borderColor, Width: outerBorder}.Layout(win, gtx, func(win *Window, gtx layout.Context) layout.Dimensions {
		return Background{
			Color: background,
		}.Layout(win, gtx, func(win *Window, gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = width
			gtx.Constraints.Max.X = width
			gtx.Constraints.Min.Y = 0
			gtx.Constraints.Max.Y = maxHeight

			return layout.Rigids(gtx, layout.Vertical,
				func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(outerPadding).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return Background{
							Color: oklch(100, 0, 0),
						}.Layout(win, gtx, func(win *Window, gtx layout.Context) layout.Dimensions {
							prompt := pl.Prompt
							if prompt == "" {
								prompt = "Type your command here"
							}
							editor := Editor(win.Theme, &pl.editor, prompt)
							editor.TextSize = fontSize
							var ngtx layout.Context
							spy, ngtx = eventx.Enspy(gtx)
							return layout.UniformInset(outerPadding).Layout(ngtx, Dumb(win, editor.Layout))
						})
					})
				},
				func(gtx layout.Context) layout.Dimensions {
					size := image.Pt(gtx.Constraints.Min.X, gtx.Dp(separatorHeight))
					defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
					Fill(win, gtx.Ops, borderColor)
					return layout.Dimensions{
						Size: size,
					}
				},
				func(gtx layout.Context) layout.Dimensions {
					cnt := 0
					return List(win.Theme, &pl.list).Layout(win, gtx, len(pl.filtered), func(gtx layout.Context, index int) layout.Dimensions {
						if len(pl.tags) <= cnt {
							pl.tags = slices.Grow(pl.tags, 1+cnt-len(pl.tags))[:cnt+1]
						}
						if len(pl.gestures) <= cnt {
							pl.gestures = slices.Grow(pl.gestures, 1+cnt-len(pl.gestures))[:cnt+1]
						}
						tag := &pl.tags[cnt]
						ges := &pl.gestures[cnt]
						cnt++
						return layout.Rigids(gtx, layout.Vertical,
							func(gtx layout.Context) layout.Dimensions {
								for _, ev := range gtx.Events(tag) {
									if ev, ok := ev.(pointer.Event); ok && ev.Kind == pointer.Move {
										pl.active = index
									}
								}
								for _, ev := range ges.Update(gtx.Queue) {
									if ev.Kind == gesture.KindClick {
										win.EmitAction(pl.cmds.At(pl.filtered[index]).Link())
										win.CloseModal()
									}
								}
								dims := pl.cmds.At(pl.filtered[index]).Layout(win, gtx, pl.active == index)
								defer clip.Rect{Max: dims.Size}.Push(gtx.Ops).Pop()
								pointer.CursorPointer.Add(gtx.Ops)
								pointer.InputOp{Kinds: pointer.Move, Tag: tag}.Add(gtx.Ops)
								ges.Add(gtx.Ops)
								return dims
							},
							func(gtx layout.Context) layout.Dimensions {
								if index < len(pl.filtered)-1 {
									size := image.Pt(gtx.Constraints.Min.X, gtx.Dp(separatorHeight)/2)
									defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
									Fill(win, gtx.Ops, oklch(53.82, 0, 0))
									return layout.Dimensions{
										Size: size,
									}
								} else {
									return layout.Dimensions{}
								}
							},
						)
					})
				},
			)
		})
	})

	dims.Size = gtx.Constraints.Constrain(dims.Size)
	c := m.Stop()

	defer clip.Rect{Max: dims.Size}.Push(gtx.Ops).Pop()
	key.InputOp{Tag: pl, Keys: "↑|↓"}.Add(gtx.Ops)
	c.Add(gtx.Ops)

	for _, ev := range pl.editor.Events() {
		switch ev.(type) {
		case widget.SubmitEvent:
			if pl.active < 0 || pl.active >= len(pl.filtered) {
				// Do nothing, there is no active element
				continue
			}
			win.EmitAction(pl.cmds.At(pl.filtered[pl.active]).Link())
			win.CloseModal()
		case widget.ChangeEvent:
			pl.filter(pl.editor.Text())
		}
	}

	handleKey := func(ev key.Event) {
		if ev.State == key.Press && ev.Modifiers == 0 {
			switch ev.Name {
			case "↑":
				pl.active = pl.active - 1
				if pl.active < 0 {
					pl.active = len(pl.filtered) - 1
				}
				firstVisible := pl.list.Position.First
				if pl.list.Position.Offset != 0 {
					firstVisible++
				}
				if !(pl.active >= firstVisible && pl.active < firstVisible+pl.list.Position.Count) {
					pl.list.ScrollTo(pl.active)
				}
			case "↓":
				pl.active++
				if pl.active >= len(pl.filtered) {
					pl.active = 0
					pl.list.ScrollTo(0)
				} else {
					end := pl.list.Position.First + pl.list.Position.Count - 1
					if pl.list.Position.OffsetLast != 0 {
						end--
					}
					if pl.active > end {
						pl.list.Position.ForceEndAligned = true
						if pl.list.Position.OffsetLast == 0 {
							pl.list.Position.Offset++
						}
					}
				}
			}
		}
	}

	// The editor widget selectively handles the up and down arrow keys, depending on the contents of the text field and
	// the position of the cursor. This means that our own InputOp won't always be getting all events. But due to the
	// selectiveness of the editor's InputOp, we can't fully rely on it, either. We need to combine the events of the
	// two.
	//
	// To be consistent, we handle all events after layout of the nested widgets, to have the same frame latency for all
	// events.
	for _, ev := range gtx.Events(pl) {
		if ev, ok := ev.(key.Event); ok {
			handleKey(ev)
		}
	}

	for _, evs := range spy.AllEvents() {
		for _, ev := range evs.Items {
			if ev, ok := ev.(key.Event); ok {
				handleKey(ev)
			}
		}
	}

	return dims
}
