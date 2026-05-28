package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

// Match Recording
// input: channel
// output: table of current scores
// 1. Scan channel for posts since last scan
// 2. For each post
//   2.1 test post is a "report post"
// 2. For each report post
//   2.1 Extract players
//   2.2 Extract scores
//   2.3 Extract match type e.g. bounty, non-bounty first, non-bounty repeated
//   2.4 Return "Report" with above info
// 3. For each player
//   3.1 Calculate new points for player based on Reports
// 4. Update Discord scoreboard post with new and total points
// 5. Update website scoreboard with new and total points

// Bot Data Structures & Functions. To store in memory rather than read/write json continually.
type BotData struct {
	Metadata map[string]interface{}
	Players  map[string]interface{}
	Matches  map[string]interface{}
	Season   map[string]interface{}

	Mutex sync.RWMutex
}

var botData BotData

// functions for loading .json data at bot startup
func loadMetadata() error {
	data, err := os.ReadFile("site/data/metadata.json")
	if err != nil {
		return err
	}

	err = json.Unmarshal(data, &botData.Metadata)
	if err != nil {
		return err
	}
	log.Println("metadata.json loaded")
	return nil
}
func loadPlayers() error {
	data, err := os.ReadFile("site/data/players.json")
	if err != nil {
		return err
	}

	err = json.Unmarshal(data, &botData.Players)
	if err != nil {
		return err
	}
	log.Println("players.json loaded")
	return nil
}
func loadMatches() error {
	data, err := os.ReadFile("site/data/matches.json")
	if err != nil {
		return err
	}

	err = json.Unmarshal(data, &botData.Matches)
	if err != nil {
		return err
	}
	log.Println("matches.json loaded")
	return nil
}
func loadSeason() error {
	data, err := os.ReadFile("site/data/season.json")
	if err != nil {
		return err
	}

	err = json.Unmarshal(data, &botData.Season)
	if err != nil {
		return err
	}
	log.Println("season.json loaded")
	return nil
}

// functions for saving/writing json data back
func saveMetadata() error {
	data, err := json.MarshalIndent(
		botData.Metadata,
		"",
		"    ",
	)

	if err != nil {
		log.Printf("Error saving metadata.json: %v", err)
		return err
	}

	return os.WriteFile(
		"site/data/metadata.json",
		data,
		0644,
	)
}
func savePlayers() error {
	//update last_updated metadata
	metadata := botData.Players["metadata"].(map[string]interface{})
	metadata["last_updated"] = time.Now().UTC().Format(time.RFC3339)

	data, err := json.MarshalIndent(
		botData.Players,
		"",
		"    ",
	)

	if err != nil {
		log.Printf("Error saving players.json: %v", err)
		return err
	}

	return os.WriteFile(
		"site/data/players.json",
		data,
		0644,
	)
}
func saveMatches() error {
	//update last_updated metadata
	metadata := botData.Matches["metadata"].(map[string]interface{})
	metadata["last_updated"] = time.Now().UTC().Format(time.RFC3339)

	data, err := json.MarshalIndent(
		botData.Matches,
		"",
		"    ",
	)

	if err != nil {
		log.Printf("Error saving matches.json: %v", err)
		return err
	}

	return os.WriteFile(
		"site/data/matches.json",
		data,
		0644,
	)
}
func saveSeason() error {
	data, err := json.MarshalIndent(
		botData.Season,
		"",
		"    ",
	)

	if err != nil {
		log.Printf("Error saving season.json: %v", err)
		return err
	}

	return os.WriteFile(
		"site/data/season.json",
		data,
		0644,
	)
}

// functionsto load & save all data at once
func loadAllData() error {
	if err := loadMetadata(); err != nil {
		return err
	}
	if err := loadPlayers(); err != nil {
		return err
	}
	if err := loadMatches(); err != nil {
		return err
	}
	if err := loadSeason(); err != nil {
		return err
	}
	return nil
}
func saveAllData() error {
	if err := saveMetadata(); err != nil {
		return err
	}
	if err := savePlayers(); err != nil {
		return err
	}
	if err := saveMatches(); err != nil {
		return err
	}
	if err := saveSeason(); err != nil {
		return err
	}
	log.Println("All data saved to .json")
	return nil
}

//-------------------------------------------------------------------

type MatchResult struct { // this structure with capital letters apparently helps JSON parse. IDK...
	Winner string `json:"winner"`
	Loser  string `json:"loser"`
	Result string `json:"result"`
	Bounty bool   `json:"bounty"`
}

type League_Player struct {
	discord_userid string
	discord_name   string
	player_type    string
	decklist       string
}

const prefix string = "!skbot"

// slash command global variable
var commands = []*discordgo.ApplicationCommand{

	//Result command for reporting matches
	{Name: "result",
		Description: "Record a match result",

		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionUser,
				Name:        "winner",
				Description: "Winning Player",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionUser,
				Name:        "loser",
				Description: "Losing Player",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "result",
				Description: "Match Result",
				Required:    true,
				Choices: []*discordgo.ApplicationCommandOptionChoice{
					{
						Name:  "3-0",
						Value: "3-0",
					},
					{
						Name:  "2-1",
						Value: "2-1",
					},
					{
						Name:  "Concession",
						Value: "0-0",
					},
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionBoolean,
				Name:        "bounty",
				Description: "Was this a bounty match?",
				Required:    true,
			},
		},
	},

	//Signup (battler, jammer, decklist)
	{Name: "signup",
		Description: "League signup commands",

		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "battler",
				Description: "Sign up as a Battler ⚔️",
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "jammer",
				Description: "Sign up as a Jammer 👊",
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "decklist",
				Description: "Submit or update your decklist as a Battler ⚔️",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionString,
						Name:        "url",
						Description: "Decklist URL (Moxfield or similar)",
						Required:    true,
					},
					{
						Type:        discordgo.ApplicationCommandOptionString,
						Name:        "name",
						Description: "Deck Name (Optional)",
						Required:    false,
					},
				},
			},
		},
	},

	//Drop command for player self-elected drops
	{Name: "drop",
		Description: "Drop yourself from the current league season",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "reason",
				Description: "Reason for leaving (Optional)",
				Required:    false,
			},
		},
	},

	//League commands (open-signups, close-signups, new-season)
	{Name: "league",
		Description: "Admin League Commands. Open/Close Signups. Start new season.",

		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "open-signups",
				Description: "Opens league registration for battlers.",
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "close-signups",
				Description: "Closes league registration for battlers.",
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "new-season",
				Description: "Initializes a new season.",
			},
		},
	},

	//Round commands (new, post, close, reminder)
	{Name: "round",
		Description: "Admin Round Commands. Make new round. Post Reminders.",

		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "new",
				Description: "Generates pairings for new league round.",
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "post",
				Description: "Posts current pairings to weekly-matches channel.",
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "close",
				Description: "Closes/Ends the current round.",
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "reminder",
				Description: "Posts a reminder for unreported matches to weekly-matches channel.",
			},
		},
	},

	//Match commands (edit, delete)
	{Name: "match",
		Description: "Admin Match Commands. Edit/delete match data.",

		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "edit",
				Description: "Revise the result of a match using its matchID.",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionString,
						Name:        "matchid",
						Description: "MatchID",
						Required:    true,
					},
					{
						Type:        discordgo.ApplicationCommandOptionUser,
						Name:        "winner",
						Description: "Winning Player",
						Required:    false,
					},
					{
						Type:        discordgo.ApplicationCommandOptionUser,
						Name:        "loser",
						Description: "Losing Player",
						Required:    false,
					},
					{
						Type:        discordgo.ApplicationCommandOptionString,
						Name:        "result",
						Description: "Match Result",
						Required:    false,
						Choices: []*discordgo.ApplicationCommandOptionChoice{
							{
								Name:  "3-0",
								Value: "3-0",
							},
							{
								Name:  "2-1",
								Value: "2-1",
							},
							{
								Name:  "Concession",
								Value: "0-0",
							},
						},
					},
					{
						Type:        discordgo.ApplicationCommandOptionBoolean,
						Name:        "bounty",
						Description: "Was this a bounty match?",
						Required:    false,
					},
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "delete",
				Description: "Posts current pairings to weekly-matches channel.",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionString,
						Name:        "matchid",
						Description: "MatchID",
						Required:    true,
					},
				},
			},
		},
	},

	//Admin Player Commands (signup, drop, decklist-review, points-modify, info)
	{Name: "admin-player",
		Description: "Admin Player Commands",

		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "signup",
				Description: "Admin signup for a specified player.",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionUser,
						Name:        "player",
						Description: "Player",
						Required:    true,
					},
					{
						Type:        discordgo.ApplicationCommandOptionString,
						Name:        "role",
						Description: "Role Assigned",
						Required:    true,
						Choices: []*discordgo.ApplicationCommandOptionChoice{
							{
								Name:  "battler",
								Value: "Battler ⚔️",
							},
							{
								Name:  "jammer",
								Value: "Jammer 👊",
							},
						},
					},
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "drop",
				Description: "Drops specified player from the league.",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionUser,
						Name:        "player",
						Description: "Player",
						Required:    true,
					},
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "decklist-review",
				Description: "Decklist review submission for specified player.",

				//THIS WILL LIKELY NEED MORE OPTIONS
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionUser,
						Name:        "player",
						Description: "Player",
						Required:    true,
					},
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "points-modify",
				Description: "Changes the league points of a specified player",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionUser,
						Name:        "player",
						Description: "Player",
						Required:    true,
					},
					{
						Type:        discordgo.ApplicationCommandOptionString,
						Name:        "type",
						Description: "Add, Subtract, or Set?",
						Required:    true,
						Choices: []*discordgo.ApplicationCommandOptionChoice{
							{
								Name:  "add",
								Value: "Add",
							},
							{
								Name:  "subtract",
								Value: "Subtract",
							},
							{
								Name:  "set",
								Value: "Set",
							},
						},
					},
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "info",
				Description: "Posts current league info for specified player.",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionUser,
						Name:        "player",
						Description: "Player",
						Required:    true,
					},
				},
			},
		},
	},
}

// function to register slash commands, done as essentially last step.
func registerCommands(s *discordgo.Session) {

	err := godotenv.Load()
	if err != nil {
		log.Fatalln("Error loading enironment variables:", err)
	}

	for _, cmd := range commands {
		_, err := s.ApplicationCommandCreate(
			s.State.User.ID,
			os.Getenv("GUILD_ID"), // <- empty to create a global command (GPT)
			cmd,
		)

		if err != nil {
			log.Printf("Cannot create comand %s: %v", cmd.Name, err)
		}
	}
}

// function for role check for slash commands.
// returns true if role is met, false if not.
func memberHasRole(
	member *discordgo.Member,
	allowedRoles []string,
) bool {
	for _, memberRole := range member.Roles {

		for _, allowedRole := range allowedRoles {

			if memberRole == allowedRole {
				return true
			}
		}
	}
	return false
}

func main() {
	// start by loading things like API tokens from the .env file
	err := godotenv.Load()
	if err != nil {
		log.Fatalln("Error loading enironment variables:", err)
	} else {
		log.Println("Tokens loaded.")
	}

	// then set up the connection to Discord as the bot
	discord, err := discordgo.New("Bot " + os.Getenv("DISCORD_TOKEN"))
	if err != nil {
		log.Fatalln("Error connecting to Discord:", err)
	} else {
		log.Println("Discord sucessfully connected.")
	}

	err = loadAllData()
	if err != nil {
		log.Fatalln("Error loading bot data:", err)
	} else {
		log.Println("Bot data loaded")
	}

	//regex to parse only the numbers from a string (for userIDs)
	userIDRegex := regexp.MustCompile(`[^0-9]+`)

	//---------------------------------------------------------------------//
	//SLASH COMMAND TESTING GROUNDS

	//Slash command handler.
	discord.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		//Ensures its a slash command
		if i.Type != discordgo.InteractionApplicationCommand {
			return
		}

		switch i.ApplicationCommandData().Name {
		case "result":

			//check if user is allowed to complete command
			allowedRoles := []string{
				os.Getenv("BATTLER_ID"),
				os.Getenv("JAMMER_ID"),
			}

			if !memberHasRole(i.Member, allowedRoles) {
				s.InteractionRespond(
					i.Interaction,
					&discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Content: "Only active league members can execute this command. Please contact a league organizer if you are missing the correct role.",
							Flags:   discordgo.MessageFlagsEphemeral,
						},
					},
				)
				return
			}

			//gathers the options
			options := i.ApplicationCommandData().Options

			//establish the variables from the command
			var winner *discordgo.User
			var loser *discordgo.User
			var result string
			var bounty bool

			for _, opt := range options {
				switch opt.Name {
				case "winner":
					winner = opt.UserValue(s)
				case "loser":
					loser = opt.UserValue(s)
				case "result":
					result = string(opt.StringValue())
				case "bounty":
					bounty = opt.BoolValue()
				}
			}

			matchResultReport := MatchResult{
				Winner: winner.ID,
				Loser:  loser.ID,
				Result: result,
				Bounty: bounty,
			}

			//Mutex lock the botData to protect from multiple commands mess
			botData.Mutex.Lock()

			//Get season number & make prefix
			currentSeason := int(botData.Metadata["current_season"].(map[string]interface{})["season"].(float64))
			seasonPrefix := fmt.Sprintf("S%02d", currentSeason)

			//Read current season matches & metadata
			currentSeasonMatches := botData.Matches["current_season"].(map[string]interface{})["matches"].(map[string]interface{})
			currentSeasonMetadata := botData.Matches["current_season"].(map[string]interface{})["metadata"].(map[string]interface{})

			//construct the next match id of form S06-001
			nextMatchId := currentSeasonMetadata["next_match_id"].(float64)
			newMatchId := fmt.Sprintf("%s-%03d",
				seasonPrefix,
				int(nextMatchId),
			)

			//add the new match result to the json data
			currentSeasonMatches[newMatchId] = map[string]interface{}{
				"winner": matchResultReport.Winner,
				"loser":  matchResultReport.Loser,
				"result": matchResultReport.Result,
				"bounty": matchResultReport.Bounty,
			}

			//increment next_match_id
			currentSeasonMetadata["next_match_id"] = nextMatchId + 1

			//Update the last update time
			botData.Matches["metadata"].(map[string]interface{})["last_updated"] =
				time.Now().UTC().Format(time.RFC3339)

			//Save matches.json
			err := saveMatches()
			if err != nil {
				//Unlocks before kicking out due to error
				botData.Mutex.Unlock()
				log.Printf("Error saving matches.json: %v", err)

				//ephemeral reply stating there was an error
				s.InteractionRespond(
					i.Interaction,
					&discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Content: "Failed to record match result.",
							Flags:   discordgo.MessageFlagsEphemeral,
						},
					},
				)
				return
			}

			//Unlock the botData
			botData.Mutex.Unlock()

			//construct the embedded message from the match result.
			embed := &discordgo.MessageEmbed{
				Title: "Match Result Recorded",
				Fields: []*discordgo.MessageEmbedField{
					{
						Name:   matchResultReport.Result,
						Value:  fmt.Sprintf("<@%v> WON vs <@%v>", matchResultReport.Winner, matchResultReport.Loser),
						Inline: true,
					},
				},
				Footer: &discordgo.MessageEmbedFooter{
					Text: fmt.Sprintf("Bounty: %v | MatchID: %v", matchResultReport.Bounty, newMatchId),
				},
				Color: 0xD80621, // Canadian Flag Red 🍁
			}

			//Send message in Bounty Board channel
			msg, errAnnounce := s.ChannelMessageSendEmbed(
				os.Getenv("BOUNTY_CHNL_ID"),
				embed,
			)
			if errAnnounce != nil {
				log.Printf("Error making match announcement: %v\n", errAnnounce)
				s.InteractionRespond(
					i.Interaction,
					&discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Content: "Match recorded successfully, but announcement failed.",
							Flags:   discordgo.MessageFlagsEphemeral,
						},
					},
				)
				return
			}

			//Construct link to message for reply
			msgURL := fmt.Sprintf(
				"https://discord.com/channels/%s/%s/%s",
				i.GuildID,
				msg.ChannelID,
				msg.ID,
			)

			s.InteractionRespond(
				i.Interaction,
				&discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: fmt.Sprintf("Match Recorded Successfully.\n%s", msgURL),
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				},
			)
		case "signup":
			sub := i.ApplicationCommandData().Options[0].Name
			switch sub {
			// /signup battler
			case "battler":
				//Read metadata for if league signups are open
				signupStatus := botData.Metadata["current_season"].(map[string]interface{})["signups"].(bool)

				//if signupStatus is false, let the user know and return
				if !signupStatus {
					s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Content: "Signups are currently closed for this season. Please use `/signup jammer` if you are interested in joining as a Jammer 👊.",
							Flags:   discordgo.MessageFlagsEphemeral,
						},
					})
					return
				}

				//upkeep initialization
				guildMember, _ := s.GuildMember(i.GuildID, i.Member.User.ID)

				//check current roles. If already a battler, let them know they are signed up. If they are a jammer, remove the jammer role
				for _, r := range guildMember.Roles {
					if r == os.Getenv("BATTLER_ID") {
						//Already signed up!
						s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
							Type: discordgo.InteractionResponseChannelMessageWithSource,
							Data: &discordgo.InteractionResponseData{
								Content: "You are already signed up as a Battler ⚔️ for this season.\n*If you would like to change roles to a Jammer 👊 you can use the `/signup jammer` command.*",
								Flags:   discordgo.MessageFlagsEphemeral,
							},
						})
						return
					}
					if r == os.Getenv("JAMMER_ID") {
						s.GuildMemberRoleRemove(i.GuildID, i.Member.User.ID, os.Getenv("JAMMER_ID"))
					}
				}

				//Revise the player's data in season.json
				//lock the data and unlock once returned/finished
				botData.Mutex.Lock()

				seasonPlayers := botData.Season["season_players"].(map[string]interface{})

				//Logic check if player already in season data
				existingSeasonPlayer, exists := seasonPlayers[i.Member.User.ID]

				if exists {

					//Player already exists -> update their role
					playerData := existingSeasonPlayer.(map[string]interface{})
					playerData["role"] = "battler"
					playerData["active"] = true
					playerData["dropped"] = false

				} else {

					//New player to season -> create fresh entry
					seasonPlayers[i.Member.User.ID] = map[string]interface{}{
						"active": true,
						"decklist": map[string]interface{}{
							"url":  "",
							"name": "",
						},
						"dropped":      false,
						"opponents":    []interface{}{},
						"received_bye": false,
						"role":         "battler",
						"standings": map[string]interface{}{
							"points":      0,
							"wins":        0,
							"losses":      0,
							"game_wins":   0,
							"game_losses": 0,
						},
					}

				}

				//Revise the player's data in players.json
				playersHistory := botData.Players["players"].(map[string]interface{})

				//Set player nickname
				nickname := guildMember.Nick
				if nickname == "" {
					nickname = guildMember.User.Username
				}

				//Logic check if player exists in players.json
				existingHistoricalPlayer, exists := playersHistory[i.Member.User.ID]

				if exists {

					//Player already exists -> update their discord nickname or username
					playerData := existingHistoricalPlayer.(map[string]interface{})
					playerData["discord_nickname"] = nickname

				} else {

					//New player to league overall -> create fresh entry
					playersHistory[i.Member.User.ID] = map[string]interface{}{
						"discord_nickname": nickname,
						"historical_record": map[string]interface{}{
							"game_losses": 0,
							"game_wins":   0,
							"losses":      0,
							"wins":        0,
						},
						"last_decklist": map[string]interface{}{
							"name": "",
							"url":  "",
						},
						"seasons_played": []interface{}{},
					}

				}

				//save the season and players jsons
				err_season := saveSeason()
				err_players := savePlayers()
				//unlock botdata
				botData.Mutex.Unlock()

				//Print errors if any (AFTER UNLOCKING)
				if err_season != nil {
					return
				}
				if err_players != nil {
					return
				}

				//Add battler role
				s.GuildMemberRoleAdd(i.GuildID, i.Member.User.ID, os.Getenv("BATTLER_ID"))

				//Respond with an ephemeral message
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "Thanks for signing up as a Battler ⚔️ for this season!\n**Please use the `/signup decklist` command to provide your decklist before the season starts.**",
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
			// /signup jammer
			case "jammer":

				//upkeep initialization
				guildMember, _ := s.GuildMember(i.GuildID, i.Member.User.ID)

				//check current roles. If already a jammer, let them know they are signed up. If they are a battler, remove the battler role
				for _, r := range guildMember.Roles {
					if r == os.Getenv("JAMMER_ID") {
						//Already signed up!
						s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
							Type: discordgo.InteractionResponseChannelMessageWithSource,
							Data: &discordgo.InteractionResponseData{
								Content: "You are already signed up as a Jammer 👊 for this season.\n*If you would like to change roles to a Battler ⚔️ and signups are currently open you can use the `/signup battler` command.*",
								Flags:   discordgo.MessageFlagsEphemeral,
							},
						})
						return
					}
					if r == os.Getenv("BATTLER_ID") {
						s.GuildMemberRoleRemove(i.GuildID, i.Member.User.ID, os.Getenv("BATTLER_ID"))
					}
				}

				//Revise the player's data in season.json
				//lock the data and unlock once returned/finished
				botData.Mutex.Lock()

				seasonPlayers := botData.Season["season_players"].(map[string]interface{})

				//Logic check if player already in season data
				existingPlayer, exists := seasonPlayers[i.Member.User.ID]

				if exists {

					//Player already exists -> update their role
					playerData := existingPlayer.(map[string]interface{})
					playerData["role"] = "jammer"
					playerData["active"] = true
					playerData["dropped"] = false

				} else {

					//New player to season -> create fresh entry
					seasonPlayers[i.Member.User.ID] = map[string]interface{}{
						"active": true,
						"decklist": map[string]interface{}{
							"url":  "",
							"name": "",
						},
						"dropped":      false,
						"opponents":    []interface{}{},
						"received_bye": false,
						"role":         "jammer",
						"standings": map[string]interface{}{
							"points":      0,
							"wins":        0,
							"losses":      0,
							"game_wins":   0,
							"game_losses": 0,
						},
					}

				}

				//Revise the player's data in players.json
				playersHistory := botData.Players["players"].(map[string]interface{})

				//Set player nickname
				nickname := guildMember.Nick
				if nickname == "" {
					nickname = guildMember.User.Username
				}

				//Logic check if player exists in players.json
				existingHistoricalPlayer, exists := playersHistory[i.Member.User.ID]

				if exists {

					//Player already exists -> update their discord nickname or username
					playerData := existingHistoricalPlayer.(map[string]interface{})
					playerData["discord_nickname"] = nickname

				} else {

					//New player to league overall -> create fresh entry
					playersHistory[i.Member.User.ID] = map[string]interface{}{
						"discord_nickname": nickname,
						"historical_record": map[string]interface{}{
							"game_losses": 0,
							"game_wins":   0,
							"losses":      0,
							"wins":        0,
						},
						"last_decklist": map[string]interface{}{
							"name": "",
							"url":  "",
						},
						"seasons_played": []interface{}{},
					}

				}

				//save the season and players jsons
				err_season := saveSeason()
				err_players := savePlayers()
				//unlock botdata
				botData.Mutex.Unlock()

				//Print errors if any (AFTER UNLOCKING)
				if err_season != nil {
					return
				}
				if err_players != nil {
					return
				}

				//Add jammer role
				s.GuildMemberRoleAdd(i.GuildID, i.Member.User.ID, os.Getenv("JAMMER_ID"))

				//Respond with an ephemeral message
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "Thanks for signing up as a Jammer 👊 for this season!",
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
			}
		case "drop":

			//check if user is currently signed up
			allowedRoles := []string{
				os.Getenv("BATTLER_ID"),
				os.Getenv("JAMMER_ID"),
			}
			// if they dont have the role, reply and return
			if !memberHasRole(i.Member, allowedRoles) {
				s.InteractionRespond(
					i.Interaction,
					&discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Content: "You are already not an active participant in the current season. Carry on 🍁",
							Flags:   discordgo.MessageFlagsEphemeral,
						},
					},
				)
				return
			}

			//upkeep initializations
			guildMember, _ := s.GuildMember(i.GuildID, i.Member.User.ID)

			//create drop reason from optional field
			dropReason := "No Reason Provided"
			if len(i.ApplicationCommandData().Options) > 0 {
				dropReason = i.ApplicationCommandData().Options[0].StringValue()
			}

			for _, r := range guildMember.Roles {
				roleName := ""

				if r == os.Getenv("BATTLER_ID") {
					roleName = "Battler ⚔️"
				}

				if r == os.Getenv("JAMMER_ID") {
					roleName = "Jammer 👊"
				}

				//If roleName has not been updated (Not battler or jammer), skip
				if roleName == "" {
					continue
				}

				//Remove the current role and add the Past League Player role
				s.GuildMemberRoleRemove(i.GuildID, i.Member.User.ID, r)
				s.GuildMemberRoleAdd(i.GuildID, i.Member.User.ID, os.Getenv("INACTIVE_ID"))

				//Revise the player's data in season.json
				//lock the data and unlock once returned/finished
				botData.Mutex.Lock()
				defer botData.Mutex.Unlock()
				//change dropped to true and active to false
				playerData := botData.Season["season_players"].(map[string]interface{})[i.Member.User.ID].(map[string]interface{})
				playerData["dropped"] = true
				playerData["active"] = false
				//save the season
				err := saveSeason()
				if err != nil {
					return
				}

				//Tell the player they have been dropped
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: fmt.Sprintf("You have been dropped as a %v for the current season. Hope to see you again in the future!", roleName),
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
				//Make drop announcement in organizer channel
				s.ChannelMessageSend(
					os.Getenv("ADMIN_CHNL_ID"),
					fmt.Sprintf("<@%v> has self-dropped as a %v for the current season.\n**Reason:** *%v*", i.Member.User.ID, roleName, dropReason),
				)

				return
			}

		case "league":
			sub := i.ApplicationCommandData().Options[0].Name
			switch sub {
			case "open-signups":

				//Read metadata for if league signups are open
				metadataJson, err := os.ReadFile("site/data/metadata.json")
				if err != nil {
					log.Println(err)
					return
				}

				var metadata map[string]interface{}

				err = json.Unmarshal(metadataJson, &metadata)
				if err != nil {
					log.Println(err)
					return
				}

				signupStatus := metadata["current_season"].(map[string]interface{})["signups"].(bool)

				if !signupStatus {
					//If false open them

					//Update the signup status
					metadata["current_season"].(map[string]interface{})["signups"] = true

					//Write back to the JSON data
					updated_metadata, err := json.MarshalIndent(
						metadata,
						"",
						"    ",
					)
					if err != nil {
						log.Printf("Error Marshalling Updated Metadata: %v\n", err)
						return
					}

					err = os.WriteFile(
						"site/data/metadata.json",
						updated_metadata,
						0644,
					)

					if err != nil {
						log.Printf("Error writing metadata.json: %v\n", err)
						return
					}

					//Reply with a hidden message that the league is now open
					s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Content: "Signups for the current league have been opened!",
						},
					})
				} else {
					//Reply and say that they are already open
					s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Content: "The league is already open! Carry on 🍁",
							Flags:   discordgo.MessageFlagsEphemeral,
						},
					})
				}
			case "close-signups":
				//Read metadata for if league signups are open
				metadataJson, err := os.ReadFile("site/data/metadata.json")
				if err != nil {
					log.Println(err)
					return
				}

				var metadata map[string]interface{}

				err = json.Unmarshal(metadataJson, &metadata)
				if err != nil {
					log.Println(err)
					return
				}

				signupStatus := metadata["current_season"].(map[string]interface{})["signups"].(bool)

				if signupStatus {
					//If true close them

					//Update the signup status
					metadata["current_season"].(map[string]interface{})["signups"] = false

					//Write back to the JSON data
					updated_metadata, err := json.MarshalIndent(
						metadata,
						"",
						"    ",
					)
					if err != nil {
						log.Printf("Error Marshalling Updated Metadata: %v\n", err)
						return
					}

					err = os.WriteFile(
						"site/data/metadata.json",
						updated_metadata,
						0644,
					)

					if err != nil {
						log.Printf("Error writing metadata.json: %v\n", err)
						return
					}

					//Reply with a hidden message that the league is now closed
					s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Content: "Signups for the current league have been closed!",
							Flags:   discordgo.MessageFlagsEphemeral,
						},
					})
				} else {
					//Reply and say that they are already closed
					s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Content: "The league is already closed! Carry on 🍁",
							Flags:   discordgo.MessageFlagsEphemeral,
						},
					})
				}
			case "new-season":

				//Read metadata for if league signups are open
				metadataJson, err := os.ReadFile("site/data/metadata.json")
				if err != nil {
					log.Println(err)
					return
				}

				var metadata map[string]interface{}

				err = json.Unmarshal(metadataJson, &metadata)
				if err != nil {
					log.Println(err)
					return
				}

				signupStatus := metadata["current_season"].(map[string]interface{})["signups"].(bool)

				if !signupStatus {
					//If false open them

					//Update the signup status
					metadata["current_season"].(map[string]interface{})["signups"] = true

					//Write back to the JSON data
					updated_metadata, err := json.MarshalIndent(
						metadata,
						"",
						"    ",
					)
					if err != nil {
						log.Printf("Error Marshalling Updated Metadata: %v\n", err)
						return
					}

					err = os.WriteFile(
						"site/data/metadata.json",
						updated_metadata,
						0644,
					)

					if err != nil {
						log.Printf("Error writing metadata.json: %v\n", err)
						return
					}

					//Reply with a hidden message that the league is now open
					s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Content: fmt.Sprintf("The league is now open! An announcement will be posted in <#%v>", os.Getenv("SIGNUP_CHNL_ID")),
						},
					})

					//Make league opening announcement
					embed := &discordgo.MessageEmbed{
						Title:       "🍁⚔️ Olympia Canadian Highlander League Signups Are Now OPEN! 👊🍁",
						Description: "Season 7",
						Fields: []*discordgo.MessageEmbedField{
							{
								Value: "Welcome to Olympia Canlander Season 7.\n" +
									"The league will be running from 05/21/2026 until 06/21/2026. \n\n" +
									"📝 | Signup using `/signup battler` or `/signup jammer`. Battlers must submit their decklist before the season begins.\n\n" +
									"📖 | [RULES](https://docs.google.com/document/d/1RZqrqEkHq-7VvKPMwbnqLxN6dfciJkXXuS5MKVr-KNI/edit?usp=sharing) | You can find the full rules for this season here or by using the `/rules` command.\n",
								Inline: true,
							},
						},
						Color: 0xD80621, // Canadian Flag Red 🍁
					}

					_, err_announce := s.ChannelMessageSendComplex(
						os.Getenv("SIGNUP_CHNL_ID"),

						&discordgo.MessageSend{
							Content: "@everyone",

							Embeds: []*discordgo.MessageEmbed{
								embed,
							},

							AllowedMentions: &discordgo.MessageAllowedMentions{
								Parse: []discordgo.AllowedMentionType{
									discordgo.AllowedMentionTypeEveryone,
								},
							},
						},
					)

					if err_announce != nil {
						log.Printf("Error making League Opening Announcement: %v\n", err_announce)
						return
					}
				} else {
					//Reply and say that they are already open
					s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Content: "The current league is already open! Carry on 🍁",
							Flags:   discordgo.MessageFlagsEphemeral,
						},
					})
				}
			}
		case "round":
			sub := i.ApplicationCommandData().Options[0].Name
			switch sub {
			case "new":
				//Generate pairings using current standings. Assign matchups. Assign byes. Constructs round structure to seasons.json
			case "post":
				//Post in weekly-matches the current round structure
			case "close":
				//ENDs the round.
				//How do we handle unreported matches?
			case "reminder":
				//Posts reminder for unreported matchest in weekly-matches
			}
		case "match":
			sub := i.ApplicationCommandData().Options[0].Name
			switch sub {
			case "edit":
				//Revise a match using its matchID
			case "delete":
				//Remove a match from database using its matchID
			}
		case "admin-player":
			sub := i.ApplicationCommandData().Options[0].Name
			switch sub {
			case "signup":
				//Admin version of signup for selected player
			case "drop":
				//Admin version of drop for selected player
			case "decklist-review":
				//Decklist review for current season
			case "points-modify":
				//Manually modifies points of a specified player
			case "info":
				//Generates player info for a specified player
			}
		}
	})

	//---------------------------------------------------------------------//
	// OLD Schema (non slash command)
	//---------------------------------------------------------------------//
	//List of commands and descriptions. Calls to this string array when interpreting a message, so add it to here first then use logic from the array (see others)
	prefix_commands := [][]string{
		{"Help", "Provides a list of available commands and their descriptions"},
		{"Hello", "Responds to a 'Hello' message and reacts with a 🍑 emoji"},
		{"Points", "Provides a link to the current canlander points list"},
		{"Scoreboard", "Provides a link to the current website scoreboard and league standings"},
	}

	//In chat command functions for the bot using the !skbot prefix.
	discord.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		// Handler relates to automated functions of the bot. It intakes a session (generated by our discord.go), and a trigger (in this case it is a message being sent anywhere)
		// now "m." refers to the message that triggered this handler

		// This checks if the author of a message is the bot itself. Ensuring the bot doesnt get caught in a loop
		if m.Author.ID == s.State.User.ID {
			return
		}

		//break the message into arguments to parse the prefix command "!skbot"
		msg_args := strings.Split(m.Content, " ")

		//does nothing if the prefix command is not the first part of the message
		if msg_args[0] != prefix {
			return
		}

		//constructs the message content after the prefix for command use
		command_content := strings.Join(msg_args[1:], " ")

		//determines if the command is valid by checking if command_content matches the first column of the commands array
		//dont like using "i" here, but idk....
		valid_command := false
		for cmd := range prefix_commands {
			if command_content == prefix_commands[cmd][0] {
				valid_command = true
			}
		}

		//If the command is not valid through the loop it just replies with a this message instead prompting them to fix it.
		if valid_command == false {
			s.ChannelMessageSendReply(m.ChannelID, fmt.Sprintf("Sorry <@%v>, I don't recognize that command. Please try again or use the \"%v Help\" command to view a list of available commands.", m.Author.ID, prefix), m.Reference())
			return
		}

		//Responds to a "help" message with an embedded message listing the available commands and their descriptions
		if command_content == prefix_commands[0][0] { //help

			//construct the embedded message from the commands list.
			embed := &discordgo.MessageEmbed{
				Title: fmt.Sprintf("Available %v Commands", prefix),
				Color: 0xD80621, // Canadian Flag Red 🍁
			}
			for _, cmd := range prefix_commands {
				embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
					Name:   cmd[0],
					Value:  cmd[1],
					Inline: false,
				})
			}
			s.ChannelMessageSendEmbed(m.ChannelID, embed)
			return
		}

		// Responds to a "hello" message with a "Hello <username>!" message and reacts with a 🍑 emoji
		if command_content == prefix_commands[1][0] { //hello
			s.ChannelMessageSendReply(m.ChannelID, fmt.Sprintf("Hello <@%v>!", m.Author.ID), m.Reference())
			s.MessageReactionAdd(m.ChannelID, m.ID, "🍑")
			return
		}

		// Responds to the "points" command with link to the current canlander points list
		if command_content == prefix_commands[2][0] { //points
			s.ChannelMessageSendReply(m.ChannelID, "Here is the current canlander points list: <https://canadianhighlander.ca/points-list/>. Happy brewing!", m.Reference())
			return
		}

		// Responds to the "scoreboard" command with link to the current website scoreboard and league standings
		if command_content == prefix_commands[3][0] { //scoreboard
			s.ChannelMessageSendReply(m.ChannelID, "You can view the current scoreboard and league standings on the bot website: <https://bot.olycanlan.org/>", m.Reference())
			return
		}

	})

	//Handler related to adding reactions. Using as a method to sign up for the league.
	//Note to add functionality in the future where it instead DMs the user and requests a decklist.
	discord.AddHandler(func(s *discordgo.Session, r *discordgo.MessageReactionAdd) {

		//Prevents the bot from taking action from messages outside the signup channel.
		if r.ChannelID != os.Getenv("SIGNUP_CHNL_ID") {
			return
		}

		//This prevents the bot from making any action if the message has an ❌ emoji reaction.
		//The intent is that I can add an ❌ when signups end, stopping all role changes for the league phase.
		msg, _ := s.ChannelMessage(r.ChannelID, r.MessageID)
		for _, reaction := range msg.Reactions {
			if reaction.Emoji.Name == "❌" {
				return
			}
		}

		if r.Emoji.Name == "⚔️" {

			//When someone signs up as a "battler"...
			// First we have to check if that player has participated in previous leagues (likely)
			// IF THEY ARE NEW. Gather info about them for players.json (League_Player struct)
			// IF THEY HAVE PARTICIPATED PREVIOUSLY. Update their discord_name. Grab their last decklist link.
			// Add them to the "battler" section of the players.json, copying their relevant info if they participated previously
			// Add the current battler role
			// DM the player with a notice they have signed up and request they DM the bot their decklist. If participated previously we can include their last known deck link.

			s.GuildMemberRoleAdd(r.GuildID, r.UserID, "1505974853658874050")
			s.ChannelMessageSend(r.ChannelID, fmt.Sprintf(" <@%v> has been signed up as a Battler ⚔️ for this season!", r.UserID))

			new_signup := League_Player{
				discord_userid: r.UserID,
				discord_name:   r.Member.DisplayName(),
				player_type:    "Battler",
				decklist:       "",
			}

			fmt.Println(new_signup)

			channel, err := s.UserChannelCreate(r.UserID)
			if err != nil {
				log.Printf("Error creating DM channel: %v\n", err)
				return
			}
			s.ChannelMessageSend(channel.ID, "Thanks for signing up as a Battler ⚔️ for this season of the Olympia Canadian Highlander league!")
			s.ChannelMessageSend(channel.ID, "Please message me a link to your decklist on Moxfield or anoter deck hosting site.")

			// Something here to capture responses. I think that might have to be above too, since its triggered by a message...
		}
		if r.Emoji.Name == "👊" {
			s.GuildMemberRoleAdd(r.GuildID, r.UserID, "1505977716543848570")
			s.ChannelMessageSend(r.ChannelID, fmt.Sprintf(" <@%v> has been signed up as a Jammer 👊 for this season!", r.UserID))
		}
	})

	//Partner handler for removing reactions to remove/change roles
	discord.AddHandler(func(s *discordgo.Session, r *discordgo.MessageReactionRemove) {

		//Prevents the bot from taking action from messages outside the signup channel.
		if r.ChannelID != os.Getenv("SIGNUP_CHNL_ID") {
			return
		}

		//This prevents the bot from making any action if the message has an ❌ emoji reaction.
		//The intent is that I can add an ❌ when signups end, stopping all role changes for the league phase.
		msg, _ := s.ChannelMessage(r.ChannelID, r.MessageID)
		for _, reaction := range msg.Reactions {
			if reaction.Emoji.Name == "❌" {
				return
			}
		}

		if r.Emoji.Name == "⚔️" {
			s.GuildMemberRoleRemove(r.GuildID, r.UserID, "1505974853658874050")
			s.ChannelMessageSend(r.ChannelID, fmt.Sprintf(" <@%v> has been removed as a Battler ⚔️ for this season!", r.UserID))
		}
		if r.Emoji.Name == "👊" {
			s.GuildMemberRoleRemove(r.GuildID, r.UserID, "1505977716543848570")
			s.ChannelMessageSend(r.ChannelID, fmt.Sprintf(" <@%v> has been removed as a Jammer 👊 for this season!", r.UserID))
		}
	})

	//Handler for recording match results in the bounty-board channel.
	discord.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {

		//Standard check to prevent loops
		if m.Author.ID == s.State.User.ID {
			return
		}

		//Check if message is within the bounty board channel. ID stored in .env
		if m.ChannelID != os.Getenv("BOUNTY_CHNL_ID") {
			return
		}

		//Parses message
		msg_args := strings.Split(m.Content, " ")
		if msg_args[0] == "!result" {

			//Only error check currently, though could be built out to intercept other errors.
			//Possible other errors: not tagging/userID, match result > 3 games total, Bounty/Non-Bounty not specified or mispelled, etc.
			if len(msg_args) != 5 {
				s.ChannelMessageSendReply(m.ChannelID, "Invalid command format. Please use: !result <player1> <score> <player2> <match_type>. Deleting original message", m.Reference())
				s.ChannelMessageDelete(m.ChannelID, m.ID)
				return
			}

			//Construct the match_result struct.
			match_result := MatchResult{
				Winner: "",
				Loser:  "",
				Result: "",
				Bounty: false,
			}

			//Logic to parse the winner and loser based on the game results of the match.
			result_split := strings.Split(msg_args[2], "-")
			if result_split[0] > result_split[1] {
				//Using regex to extract only the numbers from the userID tags. Typical format is <@12345>, so this removes the <@> for better storage.
				match_result.Winner = userIDRegex.ReplaceAllString(msg_args[1], "")
				match_result.Loser = userIDRegex.ReplaceAllString(msg_args[3], "")
				match_result.Result = fmt.Sprintf("%v-%v", result_split[0], result_split[1])
			} else {
				match_result.Winner = userIDRegex.ReplaceAllString(msg_args[3], "")
				match_result.Loser = userIDRegex.ReplaceAllString(msg_args[1], "")
				match_result.Result = fmt.Sprintf("%v-%v", result_split[1], result_split[0])
			}

			//Logic to determine if the match was bounty. Default is false.
			if msg_args[4] == "Bounty" {
				match_result.Bounty = true
			}

			//Read matches.json
			matches_json, err := os.ReadFile("site/data/matches.json")
			if err != nil {
				log.Printf("Error reading matches.json: %v\n", err)
				return
			}

			var matches_data map[string]interface{}

			err = json.Unmarshal(matches_json, &matches_data)
			if err != nil {
				log.Printf("Error unmarshalling matches.json: %v\n", err)
				return
			}

			//Read metadata.json
			metadata_json, err := os.ReadFile("site/data/metadata.json")
			if err != nil {
				log.Printf("Error reading metadata.json: %v\n", err)
				return
			}

			var metadata map[string]interface{}

			err = json.Unmarshal(metadata_json, &metadata)
			if err != nil {
				log.Printf("Error unmarshalling metadata.json: %v\n", err)
				return
			}

			//Get season number & make prefix
			current_season := int(metadata["current_season"].(map[string]interface{})["season"].(float64))
			season_prefix := fmt.Sprintf("S%02d", current_season)

			//Read current season matches & metadata
			current_season_matches := matches_data["current_season"].(map[string]interface{})["matches"].(map[string]interface{})

			current_season_metadata := matches_data["current_season"].(map[string]interface{})["metadata"].(map[string]interface{})

			//construct the next match id of form S06-001
			next_match_id := int(current_season_metadata["next_match_id"].(float64))
			new_match_id := fmt.Sprintf("%s-%03d",
				season_prefix,
				next_match_id)

			//add the new match result to the json data
			current_season_matches[new_match_id] = map[string]interface{}{
				"winner": match_result.Winner,
				"loser":  match_result.Loser,
				"result": match_result.Result,
				"bounty": match_result.Bounty,
			}

			//increment next_match_id
			current_season_metadata["next_match_id"] = next_match_id + 1

			//Update the last update time
			matches_data["metadata"].(map[string]interface{})["last_updated"] =
				time.Now().UTC().Format(time.RFC3339)

			//Write back to the JSON data
			updated_matches_json, err := json.MarshalIndent(
				matches_data,
				"",
				"    ",
			)
			if err != nil {
				log.Printf("Error Marshalling Updated Matches Data: %v\n", err)
				return
			}

			err = os.WriteFile(
				"site/data/matches.json",
				updated_matches_json,
				0644,
			)

			if err != nil {
				log.Printf("Error writing matches.json: %v\n", err)
				return
			}

			//construct the embedded message from the match result.
			embed := &discordgo.MessageEmbed{
				Title: "Match Result Recorded",
				Fields: []*discordgo.MessageEmbedField{
					{
						Name:   match_result.Result,
						Value:  fmt.Sprintf("<@%v> WON vs <@%v>", match_result.Winner, match_result.Loser),
						Inline: true,
					},
				},
				Footer: &discordgo.MessageEmbedFooter{
					Text: fmt.Sprintf("Bounty: %v", match_result.Bounty),
				},
				Color: 0xD80621, // Canadian Flag Red 🍁
			}
			//Send embedded message
			s.ChannelMessageSendEmbed(m.ChannelID, embed)

			//Currently just prints the result to the terminal.
			//Maybe this is where it will feed into the website?
			fmt.Println(match_result)
		}
	})

	//---------------------------------------------------------------------//

	// This aligns the intents of the bot with the privileged intents.
	// Not 100% confident what this is needed for
	discord.Identify.Intents = discordgo.IntentsAllWithoutPrivileged

	err = discord.Open()
	if err != nil {
		log.Fatalln(err)
	}
	defer discord.Close()

	registerCommands(discord)

	//Terminal print indicating the bot is running
	fmt.Println("Bot is running")

	// "Listens" for CNTRL-C in the terminal to interrupt/close the bot once running.
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
	fmt.Println("Bot is shutting down...")

	//Save all data at shutdown
	err = saveAllData()
	if err != nil {
		log.Println("Error saving bot data:", err)
	}

	log.Println("Bot data saved.")
}
