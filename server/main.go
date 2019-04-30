package main

import (
	"github.com/mattermost/mattermost-server/plugin"
)

const (
	BOT_USER_KEY = "RentaCarlosBot"
)

func main() {
	plugin.ClientMain(&Plugin{})
}
