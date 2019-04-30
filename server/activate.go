package main

import (
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-server/model"
)

func (p *Plugin) OnActivate() error {
	p.API.LogDebug("Activating Rent-a-Carlos plugin")

	if err := p.ensureBotExists(); err != nil {
		return errors.Wrap(err, "failed to ensure bot user exists")
	}

	p.API.RegisterCommand(getCommand())

	p.API.LogDebug("Rent-a-Carlos plugin activated")

	return nil
}

func (p *Plugin) ensureBotExists() error {
	// Attempt to find an existing bot
	botUserIdBytes, err := p.API.KVGet(BOT_USER_KEY)
	if err != nil {
		return err
	}

	if botUserIdBytes == nil {
		// Create a bot since one doesn't exist
		p.API.LogDebug("Creating bot for Rent-a-Carlos plugin")

		bot, err := p.API.CreateBot(&model.Bot{
			Username:    "rent-a-carlos",
			DisplayName: "Rent a Carlos",
			Description: "Created by the Rent-a-Carlos plugin.",
		})
		if err != nil {
			return err
		}

		// Give it a profile picture
		err = p.API.SetProfileImage(bot.UserId, profileImage)
		if err != nil {
			p.API.LogError("Failed to set profile image for bot", "err", err)
		}

		p.API.LogDebug("Bot created for Rent-a-Carlos plugin")

		// Save the bot ID
		err = p.API.KVSet(BOT_USER_KEY, []byte(bot.UserId))
		if err != nil {
			return err
		}

		p.botUserID = bot.UserId
	} else {
		p.botUserID = string(botUserIdBytes)
	}

	return nil
}
