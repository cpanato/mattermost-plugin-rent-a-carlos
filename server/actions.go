package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/mattermost/mattermost-server/model"
)

func (p *Plugin) handleRejectTask(w http.ResponseWriter, r *http.Request) {
	p.API.LogInfo("Received reject action")

	var action *Action
	json.NewDecoder(r.Body).Decode(&action)

	if action == nil {
		encodeEphermalMessage(w, "We could not decode the action")
		return
	}

	if action.Context.ReporterUserID == "" {
		encodeEphermalMessage(w, "Reporter UserID cannot be empty")
		return
	}

	actionPost, errPost := p.API.GetPost(action.PostID)
	if errPost != nil {
		p.API.LogError("Rent-a-Carlos failed to get post", "err=", errPost.Error())
		p.API.DeletePost(action.PostID)
	} else {
		p.API.DeletePost(actionPost.Id)

		var helpMsgFromAttachment []string
		helpMsgFromAttachment = append(helpMsgFromAttachment, "Your Request was rejected.\nRequest:")
		for _, attachment := range actionPost.Attachments() {
			helpMsgFromAttachment = append(helpMsgFromAttachment, attachment.Text)
		}
		reporterPost := &model.Post{
			Message: strings.Join(helpMsgFromAttachment, "\n"),
		}
		if _, appErr := p.CreateBotDMPost(action.Context.ReporterUserID, reporterPost); appErr != nil {
			encodeEphermalMessage(w, "Error creating the Rent-a-Carlos request post")
		}
	}
	encodeEphermalMessage(w, "Task deleted.")
	return
}

func (p *Plugin) handleCompleteTask(w http.ResponseWriter, r *http.Request) {
	p.API.LogInfo("Received complete action")

	var action *Action
	json.NewDecoder(r.Body).Decode(&action)

	if action == nil {
		encodeEphermalMessage(w, "We could not decode the action")
		return
	}

	if action.Context.ReporterUserID == "" {
		encodeEphermalMessage(w, "Reporter UserID cannot be empty")
		return
	}

	updatePost := &model.Post{}
	updateAttachment := &model.SlackAttachment{}
	attachments := []*model.SlackAttachment{}
	actionPost, errPost := p.API.GetPost(action.PostID)
	if errPost != nil {
		p.API.LogError("Rent-a-carlos get Post Error", "err=", errPost.Error())
	} else {
		for _, attachment := range actionPost.Attachments() {
			if attachment.Actions == nil {
				attachments = append(attachments, attachment)
				continue
			}
			updateAttachment = attachment
			updateAttachment.Actions = nil
			field := &model.SlackAttachmentField{
				Title: "Task Completed",
				Short: false,
			}
			updateAttachment.Fields = append(updateAttachment.Fields, field)
			attachments = append(attachments, updateAttachment)
		}
		retainedProps := []string{"override_username", "override_icon_url"}
		updatePost.AddProp("from_webhook", "true")

		for _, prop := range retainedProps {
			if value, ok := actionPost.Props[prop]; ok {
				updatePost.AddProp(prop, value)
			}
		}

		model.ParseSlackAttachment(updatePost, attachments)
		updatePost.Id = actionPost.Id
		updatePost.ChannelId = actionPost.ChannelId
		updatePost.UserId = actionPost.UserId
		if _, err := p.API.UpdatePost(updatePost); err != nil {
			p.API.LogError("Rent-a-carlos Update Post Error", "err=", errPost.Error())
		}

		var helpMsgFromAttachment []string
		helpMsgFromAttachment = append(helpMsgFromAttachment, "Your Request was completed.\nRequest:")
		for _, attachment := range actionPost.Attachments() {
			helpMsgFromAttachment = append(helpMsgFromAttachment, attachment.Text)
		}
		reporterPost := &model.Post{
			Message: strings.Join(helpMsgFromAttachment, "\n"),
		}
		if _, appErr := p.CreateBotDMPost(action.Context.ReporterUserID, reporterPost); appErr != nil {
			encodeEphermalMessage(w, "Error creating the Rent-a-Carlos request post")
		}
	}
	encodeEphermalMessage(w, "Task completed.")
	return
}

func (p *Plugin) handleDialog(w http.ResponseWriter, r *http.Request) {
	p.API.LogInfo("Received dialog action")

	request := model.SubmitDialogRequestFromJson(r.Body)

	userReporter, uErr := p.API.GetUser(request.UserId)
	if uErr != nil {
		p.API.LogError(uErr.Error())
		return
	}

	message := request.Submission["message"]
	target := request.Submission["target"]

	userRequested, err := p.API.GetUser(target.(string))
	if err != nil {
		p.API.LogError(err.Error())
		return
	}

	var fields []*model.SlackAttachmentField
	fields = addFields(fields, "Reporter", userReporter.Username, true)

	config := p.API.GetConfig()
	siteURLPort := *config.ServiceSettings.ListenAddress
	completeAction := &model.PostAction{
		Name: "Task Completed",
		Type: model.POST_ACTION_TYPE_BUTTON,
		Integration: &model.PostActionIntegration{
			Context: map[string]interface{}{
				"action":           "complete",
				"reporter_user_id": userReporter.Id,
			},
			URL: fmt.Sprintf("http://localhost%v/plugins/%v/api/complete?token=%s", siteURLPort, manifest.Id, p.configuration.Token),
		},
	}
	rejectAction := &model.PostAction{
		Name: "Reject Task",
		Type: model.POST_ACTION_TYPE_BUTTON,
		Integration: &model.PostActionIntegration{
			Context: map[string]interface{}{
				"action":           "reject",
				"reporter_user_id": userReporter.Id,
			},
			URL: fmt.Sprintf("http://localhost%v/plugins/%v/api/reject?token=%s", siteURLPort, manifest.Id, p.configuration.Token),
		},
	}

	attachment := &model.SlackAttachment{
		Title:  "Request for help",
		Fields: fields,
		Text:   message.(string),
		Actions: []*model.PostAction{
			completeAction,
			rejectAction,
		},
	}

	post := &model.Post{}

	model.ParseSlackAttachment(post, []*model.SlackAttachment{attachment})
	if _, appErr := p.CreateBotDMPost(userRequested.Id, post); appErr != nil {
		p.API.LogError(appErr.Error())
		return
	}

	msg := fmt.Sprintf("Your request was successfully submitted.\nRequest:\n%s", message.(string))
	postReporter := &model.Post{
		Message: msg,
	}

	model.ParseSlackAttachment(post, []*model.SlackAttachment{attachment})
	if _, appErr := p.CreateBotDMPost(userReporter.Id, postReporter); appErr != nil {
		p.API.LogError(appErr.Error())
		return
	}

	return

}

// func (p *Plugin) handleDialog(w http.ResponseWriter, req *http.Request) {

// 	request := model.SubmitDialogRequestFromJson(req.Body)

// 	user, uErr := p.API.GetUser(request.UserId)
// 	if uErr != nil {
// 		p.API.LogError(uErr.Error())
// 		return
// 	}

// 	T, _ := p.translation(user)
// 	location := p.location(user)

// 	message := request.Submission["message"]
// 	target := request.Submission["target"]
// 	ttime := request.Submission["time"]

// 	if target == nil {
// 		target = T("me")
// 	}

// 	when := T("in") + " " + T("button.snooze."+ttime.(string))
// 	switch ttime.(string) {
// 	case "tomorrow":
// 		when = T("tomorrow")
// 	case "nextweek":
// 		when = T("monday")
// 	}

// 	r := &ReminderRequest{
// 		TeamId:   request.TeamId,
// 		Username: user.Username,
// 		Payload:  message.(string),
// 		Reminder: Reminder{
// 			Id:        model.NewId(),
// 			TeamId:    request.TeamId,
// 			Username:  user.Username,
// 			Message:   message.(string),
// 			Completed: p.emptyTime,
// 			Target:    target.(string),
// 			When:      when,
// 		},
// 	}

// 	if cErr := p.CreateOccurrences(r); cErr != nil {
// 		p.API.LogError(cErr.Error())
// 		return
// 	}

// 	if rErr := p.UpsertReminder(r); rErr != nil {
// 		p.API.LogError(rErr.Error())
// 		return
// 	}

// 	if r.Reminder.Target == T("me") {
// 		r.Reminder.Target = T("you")
// 	}

// 	useTo := strings.HasPrefix(r.Reminder.Message, T("to"))
// 	var useToString string
// 	if useTo {
// 		useToString = " " + T("to")
// 	} else {
// 		useToString = ""
// 	}

// 	var responseParameters = map[string]interface{}{
// 		"Target":  r.Reminder.Target,
// 		"UseTo":   useToString,
// 		"Message": r.Reminder.Message,
// 		"When": p.formatWhen(
// 			r.Username,
// 			r.Reminder.When,
// 			r.Reminder.Occurrences[0].Occurrence.In(location).Format(time.RFC3339),
// 			false,
// 		),
// 	}

// 	siteURL := fmt.Sprintf("%s", *p.ServerConfig.ServiceSettings.SiteURL)

// 	reminder := &model.Post{
// 		ChannelId: request.ChannelId,
// 		UserId:    p.remindUserId,
// 		Props: model.StringInterface{
// 			"attachments": []*model.SlackAttachment{
// 				{
// 					Text: T("schedule.response", responseParameters),
// 					Actions: []*model.PostAction{
// 						{
// 							Id: model.NewId(),
// 							Integration: &model.PostActionIntegration{
// 								Context: model.StringInterface{
// 									"reminder_id":   r.Reminder.Id,
// 									"occurrence_id": r.Reminder.Occurrences[0].Id,
// 									"action":        "delete/ephemeral",
// 								},
// 								URL: fmt.Sprintf("%s/plugins/%s/delete/ephemeral", siteURL, manifest.Id),
// 							},
// 							Type: model.POST_ACTION_TYPE_BUTTON,
// 							Name: T("button.delete"),
// 						},
// 						{
// 							Id: model.NewId(),
// 							Integration: &model.PostActionIntegration{
// 								Context: model.StringInterface{
// 									"reminder_id":   r.Reminder.Id,
// 									"occurrence_id": r.Reminder.Occurrences[0].Id,
// 									"action":        "view/ephemeral",
// 								},
// 								URL: fmt.Sprintf("%s/plugins/%s/delete/ephemeral", siteURL, manifest.Id),
// 							},
// 							Type: model.POST_ACTION_TYPE_BUTTON,
// 							Name: T("button.view.reminders"),
// 						},
// 					},
// 				},
// 			},
// 		},
// 	}
// 	p.API.SendEphemeralPost(user.Id, reminder)

// }
