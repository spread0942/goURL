package ui

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"time"

	"gourl/internal/core"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var methods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}

type completion struct {
	response core.Response
	err      error
}

type UI struct {
	app        *tview.Application
	pages      *tview.Pages
	views      *tview.Pages
	list       *tview.List
	editor     *tview.Form
	response   *tview.TextView
	status     *tview.TextView
	body       *tview.Flex
	screen     tcell.Screen
	dir        string
	state      core.State
	baseline   core.Request
	client     *http.Client
	results    chan completion
	cancel     context.CancelFunc
	popup      bool
	view       string
	sidebar    bool
	width      int
	height     int
	refreshing bool
}

func New(dir string, state core.State, timeout time.Duration) *UI {
	ui := &UI{app: tview.NewApplication(), dir: dir, state: state, client: &http.Client{Timeout: timeout}, results: make(chan completion, 1), view: "request"}
	ui.pages = tview.NewPages()
	ui.views = tview.NewPages()
	ui.list = tview.NewList().ShowSecondaryText(false).SetHighlightFullLine(true)
	ui.list.SetBorder(true).SetTitle(" Saved requests ")
	ui.response = tview.NewTextView().SetDynamicColors(false).SetScrollable(true).SetWrap(true)
	ui.response.SetBorder(true).SetTitle(" Response ")
	ui.response.SetText("No response")
	ui.status = tview.NewTextView().SetDynamicColors(false)
	ui.body = tview.NewFlex()
	ui.views.AddPage("response", ui.response, true, false)
	ui.rebuildList()
	initial := core.NewRequest()
	for _, request := range state.Requests {
		if request.ID == state.ActiveRequest {
			initial = request
			break
		}
	}
	ui.load(initial)
	navigation := tview.NewForm().SetHorizontal(true).
		AddButton("Request", func() { ui.show("request") }).
		AddButton("Response", func() { ui.show("response") }).
		AddButton("Environments", ui.environments)
	navigation.SetBorderPadding(0, 0, 0, 0)
	header := tview.NewTextView().SetText(" goURL  /  API workspace").SetTextColor(tcell.ColorAqua)
	footer := tview.NewTextView().SetText(" ^N New  ^S Save  ^R Send  ^X Cancel  ^E Env  ^Q Quit  F2/F3/F4 Views")
	root := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(header, 1, 0, false).
		AddItem(navigation, 3, 0, false).
		AddItem(ui.body, 0, 1, true).
		AddItem(ui.status, 1, 0, false).
		AddItem(footer, 1, 0, false)
	ui.pages.AddPage("main", root, true, true)
	ui.app.SetRoot(ui.pages, true).EnableMouse(true).EnablePaste(true).SetFocus(ui.editor)
	ui.app.SetInputCapture(ui.input)
	ui.app.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		ui.screen = screen
		ui.receive()
		width, height := screen.Size()
		if width != ui.width || height != ui.height {
			ui.width, ui.height = width, height
			ui.layout()
		}
		return false
	})
	ui.message("Ready")
	return ui
}

func (ui *UI) Run() error {
	defer func() {
		if ui.cancel != nil {
			ui.cancel()
		}
		ui.client.CloseIdleConnections()
	}()
	return ui.app.Run()
}

func (ui *UI) layout() {
	ui.body.Clear()
	if ui.width >= 80 {
		ui.body.AddItem(ui.list, 24, 0, ui.sidebar).AddItem(ui.views, 0, 1, !ui.sidebar)
	} else if ui.sidebar {
		ui.body.AddItem(ui.list, 0, 1, true)
	} else {
		ui.body.AddItem(ui.views, 0, 1, true)
	}
}

func (ui *UI) input(event *tcell.EventKey) *tcell.EventKey {
	if event.Key() == tcell.KeyF24 {
		ui.receive()
		return event
	}
	if ui.popup {
		if event.Key() == tcell.KeyCtrlC || event.Key() == tcell.KeyCtrlQ {
			return tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone)
		}
		return event
	}
	switch event.Key() {
	case tcell.KeyCtrlQ, tcell.KeyCtrlC:
		ui.guard(func() {
			if ui.cancel != nil {
				ui.cancel()
			}
			ui.app.Stop()
		})
	case tcell.KeyCtrlN:
		ui.guard(func() { ui.load(core.NewRequest()); ui.show("request") })
	case tcell.KeyCtrlS:
		ui.save()
	case tcell.KeyCtrlD:
		ui.duplicate()
	case tcell.KeyCtrlE:
		ui.environments()
	case tcell.KeyCtrlR, tcell.KeyF5:
		ui.send()
	case tcell.KeyCtrlX:
		if ui.cancel != nil {
			ui.cancel()
			ui.message("Cancelling request...")
		}
	case tcell.KeyF2:
		ui.sidebar = true
		ui.layout()
		ui.app.SetFocus(ui.list)
	case tcell.KeyF3:
		ui.show("request")
	case tcell.KeyF4:
		ui.show("response")
	default:
		return event
	}
	return nil
}

func (ui *UI) message(message string) {
	ui.status.SetText(" " + core.SafeText(message))
}

func (ui *UI) show(view string) {
	ui.view, ui.sidebar = view, false
	ui.views.SwitchToPage(view)
	ui.layout()
	if view == "request" {
		ui.app.SetFocus(ui.editor)
	} else {
		ui.app.SetFocus(ui.response)
	}
}

func (ui *UI) persist(next core.State) bool {
	if err := core.Save(ui.dir, next); err != nil {
		ui.message("Save failed: " + err.Error())
		return false
	}
	ui.state = next
	return true
}

func (ui *UI) rebuildList() {
	ui.list.Clear()
	for _, request := range ui.state.Requests {
		ui.list.AddItem(tview.Escape(core.SafeText(request.Name)), "", 0, func() {
			ui.guard(func() {
				next := ui.state
				next.ActiveRequest = request.ID
				if ui.persist(next) {
					ui.load(request)
					ui.show("request")
				}
			})
		})
		if request.ID == ui.state.ActiveRequest {
			ui.list.SetCurrentItem(ui.list.GetItemCount() - 1)
		}
	}
}

func (ui *UI) dirty() bool {
	request, err := ui.draft()
	return err != nil || !reflect.DeepEqual(request, ui.baseline)
}

func (ui *UI) guard(action func()) {
	if !ui.dirty() {
		action()
		return
	}
	ui.confirm("Discard unsaved request changes?", action)
}

func (ui *UI) confirm(text string, action func()) {
	previous := ui.app.GetFocus()
	modal := tview.NewModal().SetText(text).AddButtons([]string{"Keep editing", "Discard"})
	modal.SetDoneFunc(func(index int, _ string) {
		ui.closeDialog(previous)
		if index == 1 {
			action()
		}
	})
	ui.popup = true
	ui.pages.AddPage("dialog", modal, true, true)
	ui.app.SetFocus(modal)
}

func (ui *UI) dialog(primitive tview.Primitive, width, height int) {
	frame := tview.NewGrid().SetRows(0, height, 0).SetColumns(0, width, 0).
		AddItem(primitive, 1, 1, 1, 1, 0, 0, true)
	ui.popup = true
	ui.pages.AddPage("dialog", frame, true, true)
	ui.app.SetFocus(primitive)
}

func (ui *UI) closeDialog(focus tview.Primitive) {
	ui.pages.RemovePage("dialog")
	ui.popup = false
	ui.app.SetFocus(focus)
}

func (ui *UI) send() {
	if ui.cancel != nil {
		ui.message("A request is already running")
		return
	}
	source, err := ui.draft()
	if err != nil {
		ui.message(err.Error())
		return
	}
	var variables map[string]string
	for _, env := range ui.state.Environments {
		if env.ID == ui.state.ActiveEnvironment {
			variables = env.Variables
		}
	}
	source, err = core.Resolve(source, variables)
	if err != nil {
		ui.message(err.Error())
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	ui.cancel = cancel
	ui.message("Sending " + source.Method + "...")
	ui.response.SetText("Waiting for response...")
	ui.show("response")
	screen := ui.screen
	go func() {
		response, err := core.Execute(ctx, ui.client, source)
		ui.results <- completion{response, err}
		if screen != nil {
			_ = screen.PostEvent(tcell.NewEventKey(tcell.KeyF24, 0, tcell.ModNone))
		}
	}()
}

func (ui *UI) receive() {
	select {
	case result := <-ui.results:
		if ui.cancel != nil {
			ui.cancel()
			ui.cancel = nil
		}
		if result.err != nil {
			ui.message("Request failed: " + result.err.Error())
			ui.response.SetText(core.SafeText(result.err.Error()))
			return
		}
		response := result.response
		summary := fmt.Sprintf("%s | %s | %d bytes", response.Status, response.Duration.Round(time.Millisecond), len(response.Body))
		if response.Truncated {
			summary += " | TRUNCATED at 5 MiB"
		}
		var output strings.Builder
		output.WriteString(summary + "\n\n")
		names := make([]string, 0, len(response.Headers))
		for name := range response.Headers {
			names = append(names, name)
		}
		slices.Sort(names)
		for _, name := range names {
			for _, value := range response.Headers[name] {
				fmt.Fprintf(&output, "%s: %s\n", name, value)
			}
		}
		output.WriteString("\n" + core.DisplayBody(response.Body))
		ui.response.SetText(core.SafeText(output.String())).ScrollToBeginning()
		ui.message(summary)
	default:
	}
}
