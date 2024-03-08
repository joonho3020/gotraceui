package theme

import (
	"context"
	rtrace "runtime/trace"

	"joonho3020/go/gotraceui/color"
	"joonho3020/go/gotraceui/layout"
	"joonho3020/go/gotraceui/widget"
)

type ComponentButtons struct {
	close  widget.PrimaryClickable
	back   widget.PrimaryClickable
	detach widget.PrimaryClickable
	attach widget.PrimaryClickable

	state ComponentState
}

func (pb *ComponentButtons) Transition(state ComponentState) {
	pb.state = state
}

func (pb *ComponentButtons) WantsTransition(gtx layout.Context) ComponentState {
	if pb.detach.Clicked(gtx) {
		return ComponentStateTab
	} else if pb.attach.Clicked(gtx) {
		return ComponentStatePanel
	} else if pb.close.Clicked(gtx) {
		return ComponentStateClosed
	} else {
		return ComponentStateNone
	}
}

func (pb *ComponentButtons) Backed(gtx layout.Context) bool {
	return pb.back.Clicked(gtx)
}

func (pb *ComponentButtons) Layout(win *Window, gtx layout.Context) layout.Dimensions {
	defer rtrace.StartRegion(context.Background(), "theme.ComponentButtons.Layout").End()

	type button struct {
		w     *widget.PrimaryClickable
		label string
		cmd   NormalCommand
	}

	var buttons []button
	switch pb.state {
	case ComponentStatePanel:
		buttons = []button{
			{
				&pb.back,
				"Back",
				NormalCommand{
					PrimaryLabel: "Go to previous panel",
					Aliases:      []string{"back"},
				},
			},

			{
				&pb.detach,
				"Tabify",
				NormalCommand{
					PrimaryLabel: "Turn panel into tab",
				},
			},
		}
	case ComponentStateTab:
	case ComponentStateWindow:
		buttons = []button{
			{
				&pb.attach,
				"Attach",
				NormalCommand{
					PrimaryLabel: "Attach panel",
				},
			},

			{
				&pb.close,
				"Close",
				NormalCommand{
					PrimaryLabel: "Close panel",
				},
			},
		}
	}

	var cmds CommandSlice
	children := make([]layout.Widget, 0, 3)
	for _, btn := range buttons {
		btn := btn
		children = append(children,
			func(gtx layout.Context) layout.Dimensions {
				return Button(win.Theme, &btn.w.Clickable, btn.label).Layout(win, gtx)
			},
			layout.Spacer{Width: 5}.Layout,
		)

		cmd := btn.cmd
		cmd.Category = "Panel"
		cmd.Color = color.Oklch{L: 0.7862, C: 0.104, H: 140, A: 1}
		cmd.Fn = func() Action {
			return ExecuteAction(func(gtx layout.Context) {
				btn.w.Click()
			})
		}
		cmds = append(cmds, cmd)
	}

	win.AddCommandProvider(CommandSlice(cmds))
	return layout.Rigids(gtx, layout.Horizontal, children...)
}

// WidgetComponent turns any widget into a Component. You are responsible for handling component button events.
type WidgetComponent struct {
	w Widget
	ComponentButtons
}

func (wp *WidgetComponent) Layout(win *Window, gtx layout.Context) layout.Dimensions {
	defer rtrace.StartRegion(context.Background(), "theme.WidgetComponent.Layout").End()

	return layout.Rigids(gtx, layout.Horizontal, Dumb(win, wp.ComponentButtons.Layout), Dumb(win, wp.w))
}
