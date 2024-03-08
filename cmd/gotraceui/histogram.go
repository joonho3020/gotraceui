package main

import (
	"context"
	"fmt"
	"image"
	rtrace "runtime/trace"
	"strconv"
	"time"

	"github.com/joonho3020/gotraceui/layout"
	"github.com/joonho3020/gotraceui/theme"
	"github.com/joonho3020/gotraceui/widget"

	"gioui.org/font"
	"gioui.org/io/pointer"
)

type InteractiveHistogram struct {
	XLabel        string
	YLabel        string
	Config        widget.HistogramConfig
	widget        *theme.Future[*widget.Histogram]
	state         theme.HistogramState
	settingsState HistogramSettingsState
	click         widget.Clickable
	changed       bool

	shouldCloseModal bool
}

func (hist *InteractiveHistogram) Set(win *theme.Window, data []time.Duration) {
	hist.widget = theme.NewFuture(win, func(cancelled <-chan struct{}) *widget.Histogram {
		whist := widget.NewHistogram(&hist.Config, data)
		// Call Reset after creating the histogram so that NewHistogram can update the config with default values.
		hist.settingsState.Reset(hist.Config)
		return whist
	})
}

type InteractiveHistogramUpdateResult struct {
}

func (hist *InteractiveHistogram) Update(gtx layout.Context) (changed bool) {
	saved, cancelled := hist.settingsState.Update(gtx)
	if saved {
		hist.Config.Bins = hist.settingsState.NumBins()
		hist.Config.RejectOutliers = hist.settingsState.RejectOutliers()
		changed = true
		hist.shouldCloseModal = true
	}
	if cancelled {
		hist.settingsState.Reset(hist.Config)
		hist.shouldCloseModal = true
	}
	if start, end, ok := hist.state.Update(gtx); ok {
		hist.Config.Start = start
		hist.Config.End = end
		changed = true
	}
	if hist.changed {
		hist.changed = false
		changed = true
	}
	return changed
}

func (hist *InteractiveHistogram) Layout(win *theme.Window, gtx layout.Context) layout.Dimensions {
	defer rtrace.StartRegion(context.Background(), "main.InteractiveHistogram.Layout").End()

	hist.Update(gtx)

	if hist.shouldCloseModal {
		hist.shouldCloseModal = false
		win.CloseModal()
	}

	for {
		click, ok := hist.click.Clicked(gtx)
		if !ok {
			break
		}
		if click.Button != pointer.ButtonSecondary {
			continue
		}

		menu := []*theme.MenuItem{
			{
				Label: PlainLabel("Change settings"),
				Action: func() theme.Action {
					return theme.ExecuteAction(func(gtx layout.Context) {
						win.SetModal(func(win *theme.Window, gtx layout.Context) layout.Dimensions {
							gtx.Constraints.Min = gtx.Constraints.Constrain(image.Pt(1000, 500))
							gtx.Constraints.Max = gtx.Constraints.Min
							return theme.Dialog(win.Theme, "Histogram settings").Layout(win, gtx, HistogramSettings(&hist.settingsState).Layout)
						})
					})
				},
			},

			{
				// TODO disable when there is nothing to zoom out to
				Label: PlainLabel("Zoom out"),
				Action: func() theme.Action {
					return theme.ExecuteAction(func(gtx layout.Context) {
						hist.Config.Start = 0
						hist.Config.End = 0
						hist.changed = true
					})
				},
			},
		}
		win.SetContextMenu(menu)
	}

	whist, ok := hist.widget.Result()
	if ok {
		hist.state.Histogram = whist
		thist := theme.Histogram(win.Theme, &hist.state)
		thist.XLabel = "Duration"
		thist.YLabel = "Count"

		dims := hist.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return thist.Layout(win, gtx)
		})

		return dims
	} else {
		return theme.Label(win.Theme, "Computing histogram…").Layout(win, gtx)
	}
}

type HistogramSettingsState struct {
	numBinsEditor  widget.Editor
	filterOutliers widget.Bool
	save           widget.PrimaryClickable
	cancel         widget.PrimaryClickable
}

func (hs *HistogramSettingsState) Update(gtx layout.Context) (saved, cancelled bool) {
	for hs.save.Clicked(gtx) {
		saved = true
	}
	for hs.cancel.Clicked(gtx) {
		cancelled = true
	}

	return saved, cancelled
}

func (hss *HistogramSettingsState) NumBins() int {
	n, err := strconv.ParseInt(hss.numBinsEditor.Text(), 10, 32)
	if err != nil || n < 1 || n > 9999 {
		return 100
	}
	return int(n)
}

func (hss *HistogramSettingsState) RejectOutliers() bool {
	return hss.filterOutliers.Value
}

type HistogramSettingsStyle struct {
	State *HistogramSettingsState
}

func (s *HistogramSettingsState) Reset(cfg widget.HistogramConfig) {
	s.filterOutliers.Set(cfg.RejectOutliers)
	numBinsStr := fmt.Sprintf("%d", cfg.Bins)
	s.numBinsEditor.SetText(numBinsStr)
	s.numBinsEditor.SingleLine = true
	s.numBinsEditor.Filter = "0123456789"
	s.numBinsEditor.SetCaret(len(numBinsStr), len(numBinsStr))
}

func HistogramSettings(state *HistogramSettingsState) HistogramSettingsStyle {
	return HistogramSettingsStyle{
		State: state,
	}
}

func (hs HistogramSettingsStyle) Layout(win *theme.Window, gtx layout.Context) layout.Dimensions {
	defer rtrace.StartRegion(context.Background(), "main.HistogramSettingsStyle.Layout").End()

	validateNumBins := func(s string) bool {
		return len(s) <= 4
	}

	settingLabel := func(s string) layout.Dimensions {
		gtx := gtx
		gtx.Constraints.Min.Y = 0
		l := theme.LineLabel(win.Theme, s)
		l.Font = font.Font{Weight: font.Bold}
		return l.Layout(win, gtx)
	}

	dims := layout.Rigids(gtx, layout.Vertical,
		func(gtx layout.Context) layout.Dimensions {
			return settingLabel("Number of bins")
		},

		func(gtx layout.Context) layout.Dimensions {
			tb := theme.TextBox(win.Theme, &hs.State.numBinsEditor, "Number of bins")
			tb.Validate = validateNumBins
			return tb.Layout(win, gtx)
		},

		func(gtx layout.Context) layout.Dimensions {
			return layout.Spacer{Height: 5}.Layout(gtx)
		},

		func(gtx layout.Context) layout.Dimensions {
			return settingLabel("Filter outliers")
		},

		func(gtx layout.Context) layout.Dimensions {
			ngtx := gtx
			ngtx.Constraints.Min = image.Point{}
			dims := theme.Switch(&hs.State.filterOutliers, "No", "Yes").Layout(win, ngtx)
			return layout.Dimensions{
				Size:     gtx.Constraints.Constrain(dims.Size),
				Baseline: dims.Baseline,
			}
		},

		func(gtx layout.Context) layout.Dimensions {
			return layout.Spacer{Height: 10}.Layout(gtx)
		},

		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					btn := theme.Button(win.Theme, &hs.State.save.Clickable, "Save settings")
					// We can't put len(t) > 0 in validateNumBins because widget.Editor doesn't support separate
					// colors for text and caret, and we don't want an empty input box to have a red caret.
					if t := hs.State.numBinsEditor.Text(); len(t) > 0 && validateNumBins(t) {
						return btn.Layout(win, gtx)
					} else {
						gtx.Queue = nil
						return btn.Layout(win, gtx)
					}
				}),

				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return layout.Spacer{Width: 5}.Layout(gtx) }),

				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return theme.Button(win.Theme, &hs.State.cancel.Clickable, "Cancel").Layout(win, gtx)
				}),
			)
		},
	)

	return dims
}
