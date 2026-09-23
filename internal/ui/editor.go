package ui

import (
	"fmt"
	"slices"
	"strings"

	"gourl/internal/core"

	"github.com/rivo/tview"
)

func (ui *UI) load(request core.Request) {
	ui.baseline = request
	ui.editor = tview.NewForm()
	ui.editor.SetBorder(true).SetTitle(" Request ")
	ui.editor.AddInputField("Name", request.Name, 0, nil, nil).
		AddDropDown("Method", methods, max(0, slices.Index(methods, request.Method)), nil).
		AddInputField("URL", request.URL, 0, nil, nil).
		AddDropDown("Environment", []string{"None"}, 0, nil).
		AddTextArea("Headers (key: value)", core.FormatPairs(request.Headers, ": "), 0, 3, 0, nil).
		AddTextArea("Query (key=value)", core.FormatPairs(request.Query, "="), 0, 3, 0, nil).
		AddTextArea("Body", request.Body, 0, 6, 0, nil).
		AddButton("Send", ui.send).
		AddButton("Save", ui.save).
		AddButton("New", func() { ui.guard(func() { ui.load(core.NewRequest()); ui.show("request") }) }).
		AddButton("Duplicate", ui.duplicate).
		AddButton("Delete", ui.deleteRequest)
	ui.updateEnvironments()
	ui.views.RemovePage("request").AddPage("request", ui.editor, true, ui.view == "request")
}

func (ui *UI) updateEnvironments() {
	ui.refreshing = true
	defer func() { ui.refreshing = false }()
	options := []string{"None"}
	selected := 0
	for index, env := range ui.state.Environments {
		options = append(options, tview.Escape(core.SafeText(env.Name)))
		if env.ID == ui.state.ActiveEnvironment {
			selected = index + 1
		}
	}
	dropdown := ui.editor.GetFormItem(3).(*tview.DropDown)
	dropdown.SetOptions(options, func(_ string, index int) {
		if ui.refreshing {
			return
		}
		next := ui.state
		next.ActiveEnvironment = ""
		if index > 0 {
			next.ActiveEnvironment = ui.state.Environments[index-1].ID
		}
		if !ui.persist(next) {
			ui.updateEnvironments()
		}
	}).SetCurrentOption(selected)
}

func (ui *UI) draft() (core.Request, error) {
	request := ui.baseline
	request.Name = ui.editor.GetFormItem(0).(*tview.InputField).GetText()
	_, request.Method = ui.editor.GetFormItem(1).(*tview.DropDown).GetCurrentOption()
	request.URL = ui.editor.GetFormItem(2).(*tview.InputField).GetText()
	var err error
	request.Headers, err = core.ParsePairs(ui.editor.GetFormItem(4).(*tview.TextArea).GetText(), ":", false)
	if err != nil {
		return request, fmt.Errorf("headers: %w", err)
	}
	request.Query, err = core.ParsePairs(ui.editor.GetFormItem(5).(*tview.TextArea).GetText(), "=", false)
	if err != nil {
		return request, fmt.Errorf("query: %w", err)
	}
	request.Body = ui.editor.GetFormItem(6).(*tview.TextArea).GetText()
	return request, nil
}

func (ui *UI) save() {
	request, err := ui.draft()
	if err != nil {
		ui.message(err.Error())
		return
	}
	if strings.TrimSpace(request.Name) == "" {
		ui.message("Request name is required")
		return
	}
	next := ui.state
	next.Requests = slices.Clone(next.Requests)
	index := slices.IndexFunc(next.Requests, func(saved core.Request) bool { return saved.ID == request.ID })
	if index < 0 {
		next.Requests = append(next.Requests, request)
	} else {
		next.Requests[index] = request
	}
	next.ActiveRequest = request.ID
	if ui.persist(next) {
		ui.baseline = request
		ui.rebuildList()
		ui.message("Request saved")
	}
}

func (ui *UI) duplicate() {
	request, err := ui.draft()
	if err != nil {
		ui.message(err.Error())
		return
	}
	request.ID = core.NewRequest().ID
	request.Name += " copy"
	ui.guard(func() {
		ui.load(request)
		ui.save()
		ui.show("request")
	})
}

func (ui *UI) deleteRequest() {
	index := slices.IndexFunc(ui.state.Requests, func(request core.Request) bool { return request.ID == ui.baseline.ID })
	if index < 0 {
		ui.message("This request has not been saved")
		return
	}
	ui.confirm("Delete this saved request and discard its edits?", func() {
		next := ui.state
		next.Requests = slices.Delete(slices.Clone(next.Requests), index, index+1)
		next.ActiveRequest = ""
		if ui.persist(next) {
			ui.rebuildList()
			ui.load(core.NewRequest())
			ui.show("request")
			ui.message("Request deleted")
		}
	})
}
