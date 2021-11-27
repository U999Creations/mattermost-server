// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/web"
)

// channel is an internal graphQL wrapper struct to add resolver methods.
type channel struct {
	model.Channel
	PrettyDisplayName string
}

// match with api4.getTeam
func (ch *channel) Team(ctx context.Context) (*model.Team, error) {
	if ch.TeamId == "" {
		return nil, nil
	}

	return getGraphQLTeam(ctx, ch.TeamId)
}

func postProcessChannels(c *web.Context, channels []*model.Channel) ([]*channel, error) {
	// This approach becomes effectively similar to a dataloader if the displayName computation
	// were to be done at the field level per channel.

	// Get DM/GM channelIDs
	var channelIDs []string
	for _, ch := range channels {
		if ch.IsGroupOrDirect() {
			channelIDs = append(channelIDs, ch.Id)
		}
	}

	var pref *model.Preference
	var userInfo map[string][]*model.User
	var err error
	var appErr *model.AppError

	// Avoiding unnecessary queries unless necessary.
	if len(channelIDs) > 0 {
		userInfo, err = c.App.Srv().Store.Channel().GetMembersInfoByChannelIds(channelIDs)
		if err != nil {
			return nil, err
		}

		pref, appErr = c.App.GetPreferenceByCategoryAndNameForUser(c.AppContext.Session().UserId, "display_settings", "name_format")
		if appErr != nil {
			return nil, err
		}
	}

	// Convert to the wrapper format.
	res := make([]*channel, 0, len(channels))
	for _, ch := range channels {
		prettyName := ch.DisplayName

		if ch.IsGroupOrDirect() {
			// get users slice for channel id
			users := userInfo[ch.Id]
			if users == nil {
				return nil, fmt.Errorf("user info not found for channel id: %s", ch.Id)
			}
			prettyName = getPrettyDNForUsers(pref.Value, users)
		}

		res = append(res, &channel{Channel: *ch, PrettyDisplayName: prettyName})
	}

	return res, nil
}

func getPrettyDNForUsers(displaySetting string, users []*model.User) string {
	displayNames := make([]string, 0, len(users))
	// TODO: optimize this logic.
	// Name computation happens repeatedly for the same user from
	// multiple channels.
	for _, u := range users {
		displayNames = append(displayNames, getPrettyDNForUser(displaySetting, u))
	}

	sort.Strings(displayNames)
	return strings.Join(displayNames, ", ")
}

func getPrettyDNForUser(displaySetting string, user *model.User) string {
	var displayName string
	switch displaySetting {
	case "nickname_full_name":
		displayName = user.Nickname
		if displayName == "" {
			displayName = user.GetFullName()
		}
		if displayName == "" {
			displayName = user.Username
		}
	case "full_name":
		displayName = user.GetFullName()
		if displayName == "" {
			displayName = user.Username
		}
	case "username":
		displayName = user.Username
	default:
		displayName = user.Username
	}

	return displayName
}
