package main

import (
	"fmt"
	"strings"

	"github.com/mattermost/mattermost-server/model"
	"github.com/mattermost/mattermost-server/plugin"
)

func getCommand() *model.Command {
	return &model.Command{
		Trigger:          "rent-a-carlos",
		DisplayName:      "Rent a Carlos",
		Description:      "Rent a Carlo Bot",
		AutoComplete:     true,
		AutoCompleteDesc: "Available commands: request, help",
		AutoCompleteHint: "[command]",
	}
}

func (p *Plugin) ExecuteCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {

	user, uErr := p.API.GetUser(args.UserId)
	if uErr != nil {
		return &model.CommandResponse{}, uErr
	}

	split := strings.Fields(args.Command)
	command := split[0]
	action := ""
	if len(split) > 1 {
		action = strings.TrimSpace(split[1])
	}

	if command != "/rent-a-carlos" {
		return &model.CommandResponse{}, nil
	}

	if strings.Trim(args.Command, " ") == "/rent-a-carlos" {
		p.InteractiveSchedule(args.TriggerId, user)
		return &model.CommandResponse{}, nil
	}

	helpMsg := `run:
	/rent-a-carlos request <@User> <Description of what you need> - to request a help from a specified user
	/rent-a-carlos help - to show the help
	`

	if action == "help" {
		return getCommandResponse(model.COMMAND_RESPONSE_TYPE_EPHEMERAL, helpMsg), nil
	}

	switch action {
	case "request":
		resp, err := p.handleRequestHelp(args)
		return resp, err
	case "help":
	default:
		return getCommandResponse(model.COMMAND_RESPONSE_TYPE_EPHEMERAL, helpMsg), nil
	}

	return &model.CommandResponse{}, nil
}

func getCommandResponse(responseType, text string) *model.CommandResponse {
	return &model.CommandResponse{
		ResponseType: responseType,
		Text:         text,
		Type:         model.POST_DEFAULT,
	}
}

func (p *Plugin) sendEphemeralMessage(msg, channelId, userId string) {
	ephemeralPost := &model.Post{
		Message:   msg,
		ChannelId: channelId,
		UserId:    p.botUserID,
		Props: model.StringInterface{
			"from_webhook": "true",
		},
	}

	p.API.LogDebug("Will send an ephemeralPost", "msg", msg)

	p.API.SendEphemeralPost(userId, ephemeralPost)
}

func (p *Plugin) handleRequestHelp(args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	split := strings.Fields(args.Command)

	parameters := []string{}
	if len(split) > 2 {
		parameters = split[2:]
	}

	if len(parameters) < 2 {
		return getCommandResponse(model.COMMAND_RESPONSE_TYPE_EPHEMERAL, "Missing user and/or description"), nil
	}

	helpMsgArray := parameters[1:]
	helpMsg := strings.Join(helpMsgArray, " ")

	userReporter, err := p.API.GetUser(args.UserId)
	if err != nil {
		return getCommandResponse(model.COMMAND_RESPONSE_TYPE_EPHEMERAL, "Error getting your details"), nil
	}

	userNameRequested := strings.TrimPrefix(parameters[0], "@")
	userRequested, err := p.API.GetUserByUsername(userNameRequested)
	if err != nil {
		return getCommandResponse(model.COMMAND_RESPONSE_TYPE_EPHEMERAL, "Did not found the requested user"), nil
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
				"reporter_user_id": args.UserId,
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
				"reporter_user_id": args.UserId,
			},
			URL: fmt.Sprintf("http://localhost%v/plugins/%v/api/reject?token=%s", siteURLPort, manifest.Id, p.configuration.Token),
		},
	}

	attachment := &model.SlackAttachment{
		Title:  "Request for help",
		Fields: fields,
		Text:   helpMsg,
		Actions: []*model.PostAction{
			completeAction,
			rejectAction,
		},
	}

	post := &model.Post{}

	model.ParseSlackAttachment(post, []*model.SlackAttachment{attachment})
	if _, appErr := p.CreateBotDMPost(userRequested.Id, post); appErr != nil {
		return getCommandResponse(model.COMMAND_RESPONSE_TYPE_EPHEMERAL, "Error creating the Rent-a-Carlos request post"), nil
	}

	msg := fmt.Sprintf("Your request was successfully submitted.\nRequest:\n%s", helpMsg)
	postReporter := &model.Post{
		Message: msg,
	}

	model.ParseSlackAttachment(post, []*model.SlackAttachment{attachment})
	if _, appErr := p.CreateBotDMPost(args.UserId, postReporter); appErr != nil {
		return getCommandResponse(model.COMMAND_RESPONSE_TYPE_EPHEMERAL, "Error creating the Rent-a-Carlos request post"), nil
	}

	return &model.CommandResponse{}, nil
}

func addFields(fields []*model.SlackAttachmentField, title, msg string, short bool) []*model.SlackAttachmentField {
	return append(fields, &model.SlackAttachmentField{
		Title: title,
		Value: msg,
		Short: model.SlackCompatibleBool(short),
	})
}

func (p *Plugin) CreateBotDMPost(userID string, post *model.Post) (*model.Post, *model.AppError) {
	channel, err := p.API.GetDirectChannel(userID, p.botUserID)
	if err != nil {
		p.API.LogError("Couldn't get bot's DM channel", "user_id", userID, "err", err)
		return nil, err
	}

	post.UserId = p.botUserID
	post.ChannelId = channel.Id

	created, err := p.API.CreatePost(post)
	if err != nil {
		p.API.LogError("Couldn't send bot DM", "user_id", userID, "err", err)
		return nil, err
	}

	return created, nil
}

func (p *Plugin) InteractiveSchedule(triggerId string, user *model.User) {

	config := p.API.GetConfig()
	siteURLPort := *config.ServiceSettings.ListenAddress
	dialogRequest := model.OpenDialogRequest{
		TriggerId: triggerId,
		URL:       fmt.Sprintf("http://localhost%v/plugins/%v/api/dialog?token=%s", siteURLPort, manifest.Id, p.configuration.Token),
		Dialog: model.Dialog{
			Title:       "Rent a Carlos - Request for help",
			CallbackId:  model.NewId(),
			SubmitLabel: "Request help!",
			Elements: []model.DialogElement{
				{
					DisplayName: "Request",
					Name:        "message",
					Type:        "text",
					SubType:     "text",
					HelpText:    "describe your request",
					Optional:    false,
				},
				{
					DisplayName: "Assignee",
					Name:        "target",
					Type:        "select",
					DataSource:  "users",
					HelpText:    "the @user that you need help",
				},
			},
		},
	}
	if pErr := p.API.OpenInteractiveDialog(dialogRequest); pErr != nil {
		p.API.LogError("Failed opening interactive dialog " + pErr.Error())
	}
}
