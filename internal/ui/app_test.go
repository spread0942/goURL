package ui

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"gourl/internal/core"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func TestEditorSaveAndEnvironment(t *testing.T) {
	dir := t.TempDir()
	state := core.State{Version: 1, Environments: []core.Environment{{ID: "dev", Name: "Development", Variables: map[string]string{"host": "localhost"}}}}
	ui := New(dir, state, time.Second)
	ui.editor.GetFormItem(0).(*tview.InputField).SetText("Example")
	ui.editor.GetFormItem(2).(*tview.InputField).SetText("http://{{host}}")
	ui.editor.GetFormItem(3).(*tview.DropDown).SetCurrentOption(1)
	if !ui.dirty() {
		t.Fatal("expected dirty editor")
	}
	ui.save()
	if ui.dirty() {
		t.Fatal("save should clear dirty state")
	}
	loaded, err := core.Load(dir)
	if err != nil || len(loaded.Requests) != 1 || loaded.ActiveEnvironment != "dev" {
		t.Fatalf("saved state: %+v %v", loaded, err)
	}
	ui.duplicate()
	if len(ui.state.Requests) != 2 || ui.state.Requests[0].ID == ui.state.Requests[1].ID {
		t.Fatal("duplicate did not create a separate request")
	}
	ui.editor.GetFormItem(6).(*tview.TextArea).SetText("unsaved", true)
	ui.guard(func() { t.Fatal("discarded without confirmation") })
	if !ui.popup {
		t.Fatal("expected unsaved changes dialog")
	}
	if ui.input(tcell.NewEventKey(tcell.KeyCtrlC, 0, tcell.ModNone)).Key() != tcell.KeyEscape {
		t.Fatal("Ctrl+C must not bypass a dialog")
	}
}

func TestRequestLifecycle(t *testing.T) {
	waiting := make(chan struct{})
	cancelled := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/wait" {
			close(waiting)
			<-request.Context().Done()
			close(cancelled)
			return
		}
		if request.Header.Get("Authorization") != "Bearer test" {
			t.Error("environment did not expand")
		}
		writer.WriteHeader(http.StatusCreated)
		io.WriteString(writer, `{"message":"[red]literal"}`)
	}))
	defer server.Close()
	request := core.NewRequest()
	request.URL = "{{base}}"
	request.Headers = []core.Pair{{Key: "Authorization", Value: "Bearer {{token}}"}}
	state := core.State{Version: 1, Requests: []core.Request{request}, ActiveRequest: request.ID, ActiveEnvironment: "dev", Environments: []core.Environment{{ID: "dev", Name: "Development", Variables: map[string]string{"base": server.URL, "token": "test"}}}}
	dir := t.TempDir()
	ui := New(dir, state, time.Second)
	screen := tcell.NewSimulationScreen("UTF-8")
	ui.app.SetScreen(screen)
	ready, received, exited := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	var firstDraw, firstResponse sync.Once
	ui.app.SetAfterDrawFunc(func(tcell.Screen) {
		firstDraw.Do(func() { close(ready) })
		if strings.Contains(ui.response.GetText(false), "201 Created") {
			firstResponse.Do(func() { close(received) })
		}
	})
	go func() { exited <- ui.Run() }()
	t.Cleanup(ui.app.Stop)
	await := func(done <-chan struct{}) {
		t.Helper()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("UI event loop timed out")
		}
	}
	await(ready)
	ui.app.QueueUpdateDraw(func() { ui.send() })
	await(received)
	ui.app.QueueUpdateDraw(func() {
		if !strings.Contains(ui.response.GetText(false), "[red]literal") {
			t.Error("response markup was not preserved literally")
		}
		ui.show("request")
		ui.editor.GetFormItem(2).(*tview.InputField).SetText("{{base}}/wait")
		ui.save()
		ui.send()
	})
	await(waiting)
	ui.app.QueueEvent(tcell.NewEventKey(tcell.KeyCtrlQ, 0, tcell.ModNone))
	select {
	case err := <-exited:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("quit blocked during request")
	}
	await(cancelled)
	loaded, err := core.Load(dir)
	if err != nil || loaded.Requests[0].URL != "{{base}}/wait" || loaded.ActiveEnvironment != "dev" {
		t.Fatalf("restart state: %+v %v", loaded, err)
	}
}

func TestLayout(t *testing.T) {
	for _, size := range [][2]int{{120, 40}, {80, 24}, {50, 18}} {
		screen := tcell.NewSimulationScreen("UTF-8")
		if err := screen.Init(); err != nil {
			t.Fatal(err)
		}
		screen.SetSize(size[0], size[1])
		ui := New(t.TempDir(), core.State{Version: 1}, time.Second)
		ui.width, ui.height = size[0], size[1]
		ui.layout()
		ui.pages.SetRect(0, 0, size[0], size[1])
		ui.pages.Draw(screen)
		screen.Show()
		contents, _, _ := screen.GetContents()
		visible := false
		for _, cell := range contents {
			if len(cell.Runes) > 0 && cell.Runes[0] == 'g' {
				visible = true
				break
			}
		}
		if !visible {
			t.Fatal("blank screen")
		}
		ui.input(tcell.NewEventKey(tcell.KeyF2, 0, tcell.ModNone))
		if ui.app.GetFocus() != ui.list {
			t.Fatal("F2 should focus saved requests")
		}
		ui.input(tcell.NewEventKey(tcell.KeyF4, 0, tcell.ModNone))
		if ui.app.GetFocus() != ui.response {
			t.Fatal("F4 should focus response")
		}
		ui.pages.Draw(screen)
		screen.Fini()
	}
}

func TestEnvironmentEditing(t *testing.T) {
	ui := New(t.TempDir(), core.State{Version: 1}, time.Second)
	ui.width, ui.height = 100, 40
	press := func(key tcell.Key) {
		t.Helper()
		handler := ui.app.GetFocus().InputHandler()
		if handler == nil {
			t.Fatal("focused widget has no input handler")
		}
		handler(tcell.NewEventKey(key, 0, tcell.ModNone), func(primitive tview.Primitive) { ui.app.SetFocus(primitive) })
	}
	ui.editEnvironment(core.Environment{ID: "dev", Variables: map[string]string{}})
	ui.app.GetFocus().(*tview.InputField).SetText("Development")
	press(tcell.KeyTab)
	ui.app.GetFocus().(*tview.TextArea).SetText("base=http://localhost:8080\ntoken=secret", true)
	press(tcell.KeyTab)
	press(tcell.KeyEnter)
	if ui.popup || len(ui.state.Environments) != 1 || ui.state.ActiveEnvironment != "dev" || ui.state.Environments[0].Variables["token"] != "secret" {
		t.Fatalf("environment was not saved: %+v", ui.state)
	}
	ui.editEnvironment(ui.state.Environments[0])
	ui.app.GetFocus().(*tview.InputField).SetText("Renamed")
	press(tcell.KeyTab)
	ui.app.GetFocus().(*tview.TextArea).SetText("token=one\ntoken=two", true)
	press(tcell.KeyTab)
	press(tcell.KeyEnter)
	if !ui.popup || ui.state.Environments[0].Name != "Development" {
		t.Fatal("invalid environment was saved")
	}
	ui.editEnvironment(ui.state.Environments[0])
	ui.app.GetFocus().(*tview.InputField).SetText("Renamed")
	press(tcell.KeyTab)
	press(tcell.KeyTab)
	press(tcell.KeyEnter)
	if ui.state.Environments[0].Name != "Renamed" {
		t.Fatal("environment rename failed")
	}
	ui.editEnvironment(ui.state.Environments[0])
	press(tcell.KeyTab)
	press(tcell.KeyTab)
	press(tcell.KeyTab)
	press(tcell.KeyEnter)
	press(tcell.KeyRight)
	press(tcell.KeyEnter)
	if len(ui.state.Environments) != 0 || ui.state.ActiveEnvironment != "" {
		t.Fatal("active environment was not deleted")
	}
}
