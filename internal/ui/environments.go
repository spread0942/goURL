package ui

import (
	"crypto/rand"
	"slices"
	"strings"

	"gourl/internal/core"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (ui *UI) environments() {
	list := tview.NewList().ShowSecondaryText(false)
	list.SetBorder(true).SetTitle(" Environments ")
	list.AddItem("+ New environment", "", 'n', func() { ui.editEnvironment(core.Environment{ID: rand.Text(), Variables: map[string]string{}}) })
	for _, env := range ui.state.Environments {
		label := env.Name
		if env.ID == ui.state.ActiveEnvironment {
			label += " (active)"
		}
		list.AddItem(tview.Escape(core.SafeText(label)), "", 0, func() { ui.editEnvironment(env) })
	}
	list.AddItem("Close", "", 'q', func() { ui.closeDialog(ui.editor); ui.show("request") })
	list.SetDoneFunc(func() { ui.closeDialog(ui.editor); ui.show("request") })
	ui.dialog(list, min(70, max(20, ui.width-2)), min(20, max(6, ui.height-2)))
}

func (ui *UI) editEnvironment(env core.Environment) {
	form := tview.NewForm()
	form.SetBorder(true).SetTitle(" Environment ")
	names := make([]string, 0, len(env.Variables))
	for name := range env.Variables {
		names = append(names, name)
	}
	slices.Sort(names)
	pairs := make([]core.Pair, 0, len(names))
	for _, name := range names {
		pairs = append(pairs, core.Pair{Key: name, Value: env.Variables[name]})
	}
	initial := core.FormatPairs(pairs, "=")
	form.AddInputField("Name", env.Name, 0, nil, nil).
		AddTextArea("Variables (key=value)", initial, 0, 8, 0, nil)
	nameField := form.GetFormItem(0).(*tview.InputField)
	variablesField := form.GetFormItem(1).(*tview.TextArea)
	errorView := tview.NewTextView().SetTextColor(tcell.ColorRed)
	container := tview.NewFlex().SetDirection(tview.FlexRow).AddItem(form, 0, 1, true).AddItem(errorView, 2, 0, false)
	open := func() {
		ui.dialog(container, min(76, max(20, ui.width-2)), min(23, max(8, ui.height-2)))
		ui.app.SetFocus(form)
	}
	back := func() {
		if nameField.GetText() == env.Name && variablesField.GetText() == initial {
			ui.environments()
			return
		}
		modal := tview.NewModal().SetText("Discard unsaved environment changes?").AddButtons([]string{"Keep editing", "Discard"})
		modal.SetDoneFunc(func(index int, _ string) {
			if index == 1 {
				ui.environments()
			} else {
				open()
			}
		})
		ui.pages.AddPage("dialog", modal, true, true)
		ui.app.SetFocus(modal)
	}
	save := func() {
		name := strings.TrimSpace(nameField.GetText())
		if name == "" {
			errorView.SetText("Name is required")
			return
		}
		for _, existing := range ui.state.Environments {
			if existing.ID != env.ID && existing.Name == name {
				errorView.SetText("Environment name already exists")
				return
			}
		}
		pairs, err := core.ParsePairs(variablesField.GetText(), "=", true)
		if err != nil {
			errorView.SetText(core.SafeText(err.Error()))
			return
		}
		updated := core.Environment{ID: env.ID, Name: name, Variables: map[string]string{}}
		for _, pair := range pairs {
			updated.Variables[pair.Key] = pair.Value
		}
		next := ui.state
		next.Environments = slices.Clone(next.Environments)
		index := slices.IndexFunc(next.Environments, func(item core.Environment) bool { return item.ID == env.ID })
		if index < 0 {
			next.Environments = append(next.Environments, updated)
		} else {
			next.Environments[index] = updated
		}
		next.ActiveEnvironment = env.ID
		if ui.persist(next) {
			ui.updateEnvironments()
			ui.closeDialog(ui.editor)
			ui.show("request")
			ui.message("Environment saved and selected")
		} else {
			errorView.SetText("Could not save; see status line")
		}
	}
	form.AddButton("Save & use", save).AddButton("Delete", func() {
		index := slices.IndexFunc(ui.state.Environments, func(item core.Environment) bool { return item.ID == env.ID })
		if index < 0 {
			errorView.SetText("Environment has not been saved")
			return
		}
		modal := tview.NewModal().SetText("Delete this environment?").AddButtons([]string{"Cancel", "Delete"})
		modal.SetDoneFunc(func(button int, _ string) {
			if button != 1 {
				open()
				return
			}
			next := ui.state
			next.Environments = slices.Delete(slices.Clone(next.Environments), index, index+1)
			if next.ActiveEnvironment == env.ID {
				next.ActiveEnvironment = ""
			}
			if ui.persist(next) {
				ui.updateEnvironments()
				ui.environments()
			} else {
				open()
				errorView.SetText("Could not save; see status line")
			}
		})
		ui.pages.AddPage("dialog", modal, true, true)
		ui.app.SetFocus(modal)
	}).AddButton("Back", back).SetCancelFunc(back)
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlS {
			save()
			return nil
		}
		return event
	})
	open()
}
