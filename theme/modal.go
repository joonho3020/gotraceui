package theme

import (
	"context"
	rtrace "runtime/trace"

	"github.com/joonho3020/gotraceui/color"
	"github.com/joonho3020/gotraceui/layout"

	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/op/clip"
)

type ModalStyle struct {
	Background color.Oklch
	Cancelled  *bool
}

func Modal(cancelled *bool) ModalStyle {
	return ModalStyle{
		Cancelled: cancelled,
	}
}

func (m ModalStyle) Layout(win *Window, gtx layout.Context, w Widget) layout.Dimensions {
	defer rtrace.StartRegion(context.Background(), "theme.Modal.Layout").End()

	// FIXME(dh): the modal doesn't cover the whole window if an offset or transform is active
	defer clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops).Pop()
	Fill(win, gtx.Ops, m.Background)

	for _, ev := range gtx.Events(m) {
		switch ev := ev.(type) {
		case pointer.Event:
			if (ev.Priority == pointer.Foremost || ev.Priority == pointer.Grabbed) && ev.Kind == pointer.Press {
				*m.Cancelled = true
			}

		case key.Event:
			if ev.Name == "⎋" {
				*m.Cancelled = true
			}
		}
	}

	// TODO(dh): the tags should be pointers
	pointer.InputOp{Tag: m, Kinds: 0xFF}.Add(gtx.Ops)
	// TODO(dh): prevent all keyboard input from bubbling up
	// OPT(dh): using m as the tag allocates, because m is of type ModalStyle.
	key.InputOp{Tag: m, Keys: "A|B|C|D|E|F|G|H|J|K|L|M|N|O|P|Q|R|S|T|U|V|W|X|Y|Z|⎋"}.Add(gtx.Ops)
	w(win, gtx)
	return layout.Dimensions{Size: gtx.Constraints.Max}
}
