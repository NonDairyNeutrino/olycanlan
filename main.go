package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand/v2"
	neturl "net/url"
	"os"
	"os/signal"
	"sort"
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

type RoundPlayer struct {
	ID          string
	Wins        int
	Pairings    []string
	ReceivedBye bool
}

type RoundPairing struct {
	Table    int    `json:"table"`
	Player1  string `json:"player1"`
	Player2  string `json:"player2"`
	Bye      bool   `json:"bye"`
	Reported bool   `json:"reported"`
	MatchID  string `json:"match_id"`
	Result   string `json:"result"`
	Winner   string `json:"winner"`
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
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionString,
						Name:        "start-date",
						Description: "Starting Date in MM-DD-YYYY Format",
						Required:    true,
					},
				},
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

	//Admin Player Commands (signup, drop, decklist-review, points-modify, info)
	{Name: "admin",
		Description: "Admin Player Commands",

		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "player-signup",
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
								Name:  "Battler ⚔️",
								Value: "battler",
							},
							{
								Name:  "Jammer 👊",
								Value: "jammer",
							},
						},
					},
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "player-drop",
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
				Name:        "player-points",
				Description: "Changes the league points of a specified player",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionUser,
						Name:        "player",
						Description: "Player",
						Required:    true,
					},
					{
						Type:        discordgo.ApplicationCommandOptionInteger,
						Name:        "points",
						Description: "Points",
						Required:    true,
					},
					{
						Type:        discordgo.ApplicationCommandOptionString,
						Name:        "type",
						Description: "Add, Subtract, or Set?",
						Required:    true,
						Choices: []*discordgo.ApplicationCommandOptionChoice{
							{
								Name:  "Add",
								Value: "add",
							},
							{
								Name:  "Subtract",
								Value: "subtract",
							},
							{
								Name:  "Set",
								Value: "set",
							},
						},
					},
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "player-info",
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
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "match-edit",
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
				Name:        "match-delete",
				Description: "Deletes a match from dataset using its matchID.",
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
}

// function to register slash commands, done as essentially last step.
func registerCommands(s *discordgo.Session) {

	err := godotenv.Load()
	if err != nil {
		log.Fatalln("Error loading environment variables:", err)
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
func memberHasRole(member *discordgo.Member, allowedRoles []string) bool {
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

	//---------------------------------------------------------------------//
	//SLASH COMMAND DEVELOPMENT

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
				"msg_id": "",
				"status": "active",
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

			//Edit bot data again to add msgID
			botData.Mutex.Lock()

			currentSeasonMatches[newMatchId].(map[string]interface{})["msg_id"] = msg.ID

			err = saveMatches()
			botData.Mutex.Unlock()
			if err != nil {
				log.Printf("Error saving message ID to json: %v", err)
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
					playerData["decklist"].(map[string]interface{})["url"] = "Not Submitted"

				} else {

					//New player to season -> create fresh entry
					seasonPlayers[i.Member.User.ID] = map[string]interface{}{
						"active": true,
						"decklist": map[string]interface{}{
							"url":      "Not Submitted",
							"name":     "",
							"approved": false,
						},
						"dropped":      false,
						"pairings":     []interface{}{},
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
							"url":      "Not Submitted",
							"name":     "",
							"approved": false,
						},
						"dropped":      false,
						"pairings":     []interface{}{},
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
			// /signup decklist
			case "decklist":
				//Update season.json -> "season_players" -> userID -> "decklist" -> "name" and "url"

				//check if user is allowed to complete command
				allowedRoles := []string{
					os.Getenv("BATTLER_ID"),
				}

				if !memberHasRole(i.Member, allowedRoles) {
					s.InteractionRespond(
						i.Interaction,
						&discordgo.InteractionResponse{
							Type: discordgo.InteractionResponseChannelMessageWithSource,
							Data: &discordgo.InteractionResponseData{
								Content: "Only Battlers ⚔️ need to submit a decklist. Please contact a league organizer if you are missing the correct role.",
								Flags:   discordgo.MessageFlagsEphemeral,
							},
						},
					)
					return
				}

				//Logic check if player already in season data
				existingSeasonPlayer, exists := botData.Season["season_players"].(map[string]interface{})[i.Member.User.ID]

				//If exists = false, then there is something here. They dont have player data in season.json but are holding the battler role.
				if !exists {
					s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Content: fmt.Sprintf("No seasonal player data found for <@%v>. An organizer will correct the issue shortly", i.Member.User.ID),
							Flags:   discordgo.MessageFlagsEphemeral,
						},
					})

					//Make error announcement in organizer channel
					s.ChannelMessageSend(
						os.Getenv("ADMIN_CHNL_ID"),
						fmt.Sprintf("<@%v> - Bot failed to retrieve <@%v>'s player data for decklist submission.\nThey either incorrectly have the Battler ⚔️ role, or their data has been corrupted.", os.Getenv("ORGANIZER_ID"), i.Member.User.ID),
					)
					return
				}

				//Get player decklist data
				playerDecklistData := existingSeasonPlayer.(map[string]interface{})["decklist"]
				currentDecklist := playerDecklistData.(map[string]interface{})["url"]

				//Read metadata for if league signups are open
				signupStatus := botData.Metadata["current_season"].(map[string]interface{})["signups"].(bool)

				//if signupStatus is false and their currentDecklist was not submitted, let the user know.
				if !signupStatus && currentDecklist == "Not Submitted" {
					s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Content: "Signups are currently closed for this season. It appears you do not have a submitted decklist. Please contact an organizer.",
							Flags:   discordgo.MessageFlagsEphemeral,
						},
					})
					return
				}
				//if signupStatus is false, let the user know and return including their submitted decklist.
				if !signupStatus {
					s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Content: fmt.Sprintf("Signups are currently closed for this season. Please use the original decklist submitted:\n%v", currentDecklist),
							Flags:   discordgo.MessageFlagsEphemeral,
						},
					})
					return
				}

				//gathers the options
				options := i.ApplicationCommandData().Options

				//establish the variables from the command
				var url string
				var name string

				//Goes one layer deeper here because this is a subcommand
				for _, opt := range options {
					if opt.Name == "decklist" {
						for _, subOpt := range opt.Options {
							switch subOpt.Name {
							case "url":
								url = string(subOpt.StringValue())
							case "name":
								name = string(subOpt.StringValue())
							}
						}
					}

				}

				//Normalize the URL formatting
				if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
					url = "https://" + url
				}
				_, err := neturl.ParseRequestURI(url)
				if err != nil {
					s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Content: fmt.Sprintf("The URL provided is invalid. Please resubmit `/signup decklist` with a valid URL.\n%v", url),
							Flags:   discordgo.MessageFlagsEphemeral,
						},
					})
					return
				}

				//Mutex Lock the botData for writing
				botData.Mutex.Lock()

				//Assign the playerDecklistData
				playerDecklistData.(map[string]interface{})["url"] = url
				playerDecklistData.(map[string]interface{})["name"] = name

				//Save season.json
				err_season := saveSeason()
				//Unlock botdata
				botData.Mutex.Unlock()
				//Print errors AFTER unlock
				if err_season != nil {
					return
				}

				//Reply with ephemeral reply confirming submission
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: fmt.Sprintf("Your decklist has been submitted.\n[%v](%v)", name, url),
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})

				//Send message that the decklist is ready for review in admin channel
				embed := &discordgo.MessageEmbed{
					Title:       "Decklist Review Needed",
					Description: fmt.Sprintf("<@%v> submitted a decklist for review", i.Member.User.ID),
					Fields: []*discordgo.MessageEmbedField{
						{
							Name:   "Deck",
							Value:  fmt.Sprintf("[%s](%s)", name, url),
							Inline: true,
						},
					},
					Footer: &discordgo.MessageEmbedFooter{
						Text: i.Member.User.ID,
					},
					Color: 0xD80621, // Canadian Flag Red 🍁
				}

				//Send message in Bounty Board channel
				msg, err := s.ChannelMessageSendComplex(
					os.Getenv("ADMIN_CHNL_ID"),
					&discordgo.MessageSend{
						Content: fmt.Sprintf("<@&%v>", os.Getenv("ORGANIZER_ID")),
						Embed:   embed,
					},
				)
				if err != nil {
					log.Printf("Error sending message in admin channel: %v", err)
					return
				}
				_ = s.MessageReactionAdd(os.Getenv("ADMIN_CHNL_ID"), msg.ID, "🔍")
				_ = s.MessageReactionAdd(os.Getenv("ADMIN_CHNL_ID"), msg.ID, "❌")

				return
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
					fmt.Sprintf("<@&%v>\n<@%v> has self-dropped as a %v for the current season.\n**Reason:** *%v*", os.Getenv("ORGANIZER_ID"), i.Member.User.ID, roleName, dropReason),
				)

				return
			}

		case "league":
			sub := i.ApplicationCommandData().Options[0].Name

			//role check here for ADMINS only
			allowedRoles := []string{
				os.Getenv("ORGANIZER_ID"),
			}

			if !memberHasRole(i.Member, allowedRoles) {
				s.InteractionRespond(
					i.Interaction,
					&discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Content: "Only admins/organizers may complete this command. Carry on 🍁!",
							Flags:   discordgo.MessageFlagsEphemeral,
						},
					},
				)
				return
			}

			switch sub {
			case "open-signups":

				//Read metadata for if league signups are open
				metaData := botData.Metadata["current_season"].(map[string]interface{})
				signupStatus := metaData["signups"].(bool)

				if !signupStatus {
					//If false open them

					//Lock the botData
					botData.Mutex.Lock()

					//Update the signup status
					metaData["signups"] = true

					//Write back to the JSON data
					err := saveMetadata()
					botData.Mutex.Unlock()
					if err != nil {
						return
					}

					//Reply with a hidden message that the league is now open
					s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Content: "Signups for the current league have been opened!",
							Flags:   discordgo.MessageFlagsEphemeral,
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
				metaData := botData.Metadata["current_season"].(map[string]interface{})
				signupStatus := metaData["signups"].(bool)

				if signupStatus {
					//If true close them
					//Lock the botData
					botData.Mutex.Lock()

					//Update the signup status
					metaData["signups"] = false

					//Update the current_players metadata by counting the "active" players in season data
					seasonPlayers := botData.Season["season_players"].(map[string]interface{})
					activePlayers := 0

					for _, player := range seasonPlayers {
						active, _ := player.(map[string]interface{})["active"].(bool)
						if active {
							activePlayers++
						}
					}

					metaData["active_players"] = activePlayers

					//Write back to the JSON data
					err := saveMetadata()
					botData.Mutex.Unlock()
					if err != nil {
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
				metaData := botData.Metadata["current_season"].(map[string]interface{})
				signupStatus := metaData["signups"].(bool)
				oldSeasonNum := metaData["season"].(float64)

				if !signupStatus {
					//If false, then new season can be opened

					//Lock the botData
					botData.Mutex.Lock()

					//Update start date
					//Get input date
					subOptions := i.ApplicationCommandData().Options[0].Options
					inputDate := subOptions[0].StringValue()

					//Check date formatting
					startDate, err := time.Parse("01-02-2006", inputDate)
					if err != nil {
						//Send hidden command to resend with correct date formatting
						s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
							Type: discordgo.InteractionResponseChannelMessageWithSource,
							Data: &discordgo.InteractionResponseData{
								Content: fmt.Sprintf("Your submitted date `%v` was not in the correct MM-DD-YYYY format", inputDate),
								Flags:   discordgo.MessageFlagsEphemeral,
							},
						})

						//unlock data before evacuating
						botData.Mutex.Unlock()
						return
					}

					//Reformat date for consistency
					storedDate := startDate.Format(time.RFC3339)

					//Write new start date to league
					metaData["start_date"] = storedDate

					//Update the signup status
					metaData["signups"] = true

					//Update the current season
					newSeasonNum := oldSeasonNum + 1
					metaData["season"] = newSeasonNum

					//Reset the round counter
					metaData["current_round"] = 0

					//Clean matches.json data, moving matches to archive and resetting metadata
					seasonMatchesData := botData.Matches["current_season"].(map[string]interface{})
					currentMatches := seasonMatchesData["matches"].(map[string]interface{})

					archiveRoot := botData.Matches["archive"].(map[string]interface{})
					archiveMatches := archiveRoot["matches"].(map[string]interface{})

					//change status to archived and move to archive
					for id, match := range currentMatches {
						matchData := match.(map[string]interface{})
						matchData["status"] = "archived"
						archiveMatches[id] = matchData
					}

					//Clean season.json data. Saving current to archive and making fresh season data
					//clear current_season
					seasonMatchesData["matches"] = map[string]interface{}{}
					//reset matchID counter
					seasonMetaData := seasonMatchesData["metadata"].(map[string]interface{})
					seasonMetaData["next_match_id"] = 1

					//Add player data from season.json to historical players.json
					leagueDataHist := botData.Players["players"].(map[string]interface{})
					leagueDataSeason := botData.Season["season_players"].(map[string]interface{})

					for id := range leagueDataSeason {
						//Gather player specific data
						playerDataSeason := leagueDataSeason[id].(map[string]interface{})
						playerDataHist := leagueDataHist[id].(map[string]interface{})

						//Establish subsets
						playerSeasonStandings := playerDataSeason["standings"].(map[string]interface{})
						playerSeasonDeck := playerDataSeason["decklist"].(map[string]interface{})
						playerHistRecord := playerDataHist["historical_record"].(map[string]interface{})
						playerLastDeck := playerDataHist["last_decklist"].(map[string]interface{})

						//Update historical_record
						playerHistRecord["wins"] =
							playerHistRecord["wins"].(float64) +
								playerSeasonStandings["wins"].(float64)
						playerHistRecord["losses"] =
							playerHistRecord["losses"].(float64) +
								playerSeasonStandings["losses"].(float64)
						playerHistRecord["game_wins"] =
							playerHistRecord["game_wins"].(float64) +
								playerSeasonStandings["game_wins"].(float64)
						playerHistRecord["game_losses"] =
							playerHistRecord["game_losses"].(float64) +
								playerSeasonStandings["game_losses"].(float64)

						//Update last_decklist only if they submitted one (as a battler)
						if playerSeasonDeck["url"] != "" {
							playerLastDeck["name"] = playerSeasonDeck["name"]
							playerLastDeck["url"] = playerSeasonDeck["url"]
						}

						//Update seasons_played
						seasonsPlayed := playerDataHist["seasons_played"].([]interface{})
						seasonsPlayed = append(seasonsPlayed, oldSeasonNum)
						playerDataHist["seasons_played"] = seasonsPlayed

					}

					//save current season data to archive
					data, err := json.MarshalIndent(
						botData.Season,
						"",
						"    ",
					)
					if err != nil {
						log.Printf("Error marshalling season.json for archive: %v", err)
						botData.Mutex.Unlock()
						return
					}
					err = os.WriteFile(
						fmt.Sprintf("site/data/archive/season-%v.json", oldSeasonNum),
						data,
						0644,
					)
					if err != nil {
						log.Printf("Error saving season.json to archive: %v", err)
						botData.Mutex.Unlock()
						return
					}
					log.Println("season.json archived")

					//reset season data
					botData.Season = map[string]interface{}{
						"rounds":         map[string]interface{}{},
						"season_players": map[string]interface{}{},
					}

					//Write back to the JSON data
					err_saveMeta := saveMetadata()
					err_saveMatches := saveMatches()
					err_saveSeason := saveSeason()
					err_savePlayers := savePlayers()
					botData.Mutex.Unlock()
					if err_saveMeta != nil {
						return
					}
					if err_saveMatches != nil {
						return
					}
					if err_saveSeason != nil {
						return
					}
					if err_savePlayers != nil {
						return
					}

					//Reply with a hidden message that the league is now open
					s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Content: fmt.Sprintf("Olympia Canlander Season %v is now open!\nAn announcement will be posted in <#%v>", newSeasonNum, os.Getenv("SIGNUP_CHNL_ID")),
							Flags:   discordgo.MessageFlagsEphemeral,
						},
					})

					//format a display date
					//NOT NECESSARY BUT KEEPING FOR NOW -> location, _ := time.LoadLocation("America/Los_Angeles")
					displayDate := startDate.Format("January 2, 2006")

					//Make league opening announcement embed msg
					embed := &discordgo.MessageEmbed{
						Title:       "🍁⚔️ Olympia Canadian Highlander League Signups Are Now OPEN! 👊🍁",
						Description: fmt.Sprintf("Season %v", newSeasonNum),
						Fields: []*discordgo.MessageEmbedField{
							{
								Value: fmt.Sprintf(
									"Welcome to Olympia Canlander Season %v.\n"+
										"The league will begin on %v. \n\n"+
										"📝 | Signup using `/signup battler` or `/signup jammer`. Battlers must submit their decklist before the season begins.\n\n"+
										"📖 | [RULES](https://docs.google.com/document/d/1RZqrqEkHq-7VvKPMwbnqLxN6dfciJkXXuS5MKVr-KNI/edit?usp=sharing) | You can find the full rules for this season here or by typing the `!rules`.\n",
									newSeasonNum,
									displayDate,
								),
								Inline: true,
							},
						},
						Color: 0xD80621, // Canadian Flag Red 🍁
					}

					//Post announcement
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
							Content: "The current league is already open.\nThe current league season must be closed (`/league close-signups`) before a new season can be began.\nCarry on 🍁",
							Flags:   discordgo.MessageFlagsEphemeral,
						},
					})
				}
			}
		case "round":
			sub := i.ApplicationCommandData().Options[0].Name

			//role check here for ADMINS only
			allowedRoles := []string{
				os.Getenv("ORGANIZER_ID"),
			}

			if !memberHasRole(i.Member, allowedRoles) {
				s.InteractionRespond(
					i.Interaction,
					&discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Content: "Only admins/organizers may complete this command. Carry on 🍁!",
							Flags:   discordgo.MessageFlagsEphemeral,
						},
					},
				)
				return
			}

			switch sub {
			case "new":
				//Generate pairings using current standings. Assign matchups. Assign byes. Constructs round structure to seasons.json

				//Initialize a roundplayers list of RoundPlayer structs
				var roundPlayers []RoundPlayer
				playerMap := make(map[string]RoundPlayer)

				//Lock bot data
				botData.Mutex.Lock()

				//Read in the active season players and their current tournament wins
				seasonPlayers := botData.Season["season_players"].(map[string]interface{})

				for playerID, player := range seasonPlayers { // for loop across season players
					playerData := player.(map[string]interface{})

					active := playerData["active"].(bool)

					if !active { // if active == FALSE skip player
						continue
					}

					//Parse previous pairings into a string list
					pairingsRaw := playerData["pairings"].([]interface{})
					pairings := make([]string, 0, len(pairingsRaw))
					for _, opp := range pairingsRaw {
						pairings = append(pairings, opp.(string))
					}

					//Add player to roundPlayers list
					roundPlayers = append(roundPlayers, RoundPlayer{
						ID:          playerID,
						Wins:        int(playerData["standings"].(map[string]interface{})["wins"].(float64)),
						Pairings:    pairings,
						ReceivedBye: playerData["received_bye"].(bool),
					})
				}

				//Sort roundPlayers by wins
				sort.Slice(roundPlayers, func(i, j int) bool {
					return roundPlayers[i].Wins > roundPlayers[j].Wins
				})

				//Populate the playerMap of roundPlayers for quick lookup
				for _, p := range roundPlayers {
					playerMap[p.ID] = p
				}

				//Create the set of pairings
				var roundPairings []RoundPairing
				valid := false
				tries := 0

			CreatePairings:
				for !valid {
					valid = true
					tries++
					//If at 10 tries, just kick out and notify admin (see below)
					if tries == 10 {
						break CreatePairings
					}

					//Randomize each bracket of wins before assigning pairings
					for start := 0; start < len(roundPlayers); {
						end := start + 1

						//Increment end if still in matching win bracket
						for end < len(roundPlayers) && roundPlayers[end].Wins == roundPlayers[start].Wins {
							end++
						}

						//Once we have a slice with matching wins, shuffle them
						rand.Shuffle(end-start, func(i, j int) {
							roundPlayers[start+i], roundPlayers[start+j] =
								roundPlayers[start+j], roundPlayers[start+i]
						})

						start = end //start the next win bracket where the last left off
					}

					//Assign pairings by table
					table := 1
					for i := 0; i < len(roundPlayers); i += 2 {

						//Iterated table number and player
						pairing := RoundPairing{
							Table:   table,
							Player1: roundPlayers[i].ID,
						}

						if i+1 < len(roundPlayers) {
							pairing.Player2 = roundPlayers[i+1].ID
							pairing.Bye = false
						} else {
							pairing.Player2 = ""
							pairing.Bye = true
						}

						//Create standard everything else
						pairing.Reported = false
						pairing.MatchID = ""
						pairing.Result = ""
						pairing.Winner = ""

						roundPairings = append(roundPairings, pairing)

						table++
					}

					//Check if its valid
					for _, table := range roundPairings {

						p1Id := table.Player1
						p2Id := table.Player2

						//Get player 1's round data
						var p1ReceivedBye bool
						var p1Pairings []string

						for _, p := range roundPlayers {
							if p.ID == p1Id {
								p1Pairings = p.Pairings
								p1ReceivedBye = p.ReceivedBye
							}
						}

						if table.Bye { //table bye -> check if player1 previously had the bye
							if p1ReceivedBye {
								valid = false
								break
							}
						} else {
							//get player 1's previous pairings
							for _, pairID := range p1Pairings {
								if pairID == p2Id {
									valid = false
									break
								}
							}
						}
					}
				}

				if !valid {
					//Unable to generate pairings after 10 tries.... notify admin
					s.InteractionRespond(
						i.Interaction,
						&discordgo.InteractionResponse{
							Type: discordgo.InteractionResponseChannelMessageWithSource,
							Data: &discordgo.InteractionResponseData{
								Content: "Round pairing generation failed after 10 attempts.\nPlease contact the bot manager and review the seasonal player data.",
								Flags:   discordgo.MessageFlagsEphemeral,
							},
						},
					)
					botData.Mutex.Unlock()
					return
				}

				//Valid pairing found. Proceed with posting/notifying admin

				//Save as "pending" round for review

				roundsData := botData.Season["rounds"].(map[string]interface{}) // Pull rounds data

				roundsData["pending"].(map[string]interface{})["pairings"] = roundPairings // apply pairings to "pending"
				roundsData["pending"].(map[string]interface{})["status"] = "pending"       // set status to "pending"

				err_save := saveSeason()
				botData.Mutex.Unlock()
				if err_save != nil {
					log.Printf("Error saving pairing to json: %v", err_save)
					return
				}

				//Reply to admin with pairings & data

				//Construct embed
				//var fields []*discordgo.MessageEmbedField

			case "post":
				//Post in weekly-matches the current round structure
			case "close":
				//ENDs the round.
				//How do we handle unreported matches?
			case "reminder":
				//Posts reminder for unreported matchest in weekly-matches
			}
		case "admin": // Admin commands. Edit player data and match data
			sub := i.ApplicationCommandData().Options[0].Name

			//role check here for ADMINS only
			allowedRoles := []string{
				os.Getenv("ORGANIZER_ID"),
			}

			if !memberHasRole(i.Member, allowedRoles) {
				s.InteractionRespond(
					i.Interaction,
					&discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Content: "Only admins/organizers may complete this command. Carry on 🍁!",
							Flags:   discordgo.MessageFlagsEphemeral,
						},
					},
				)
				return
			}

			switch sub {
			case "player-signup": //Admin version of signup for selected player

				//Gather info submitted in command
				subOptions := i.ApplicationCommandData().Options[0].Options

				//Player
				player := subOptions[0].UserValue(s)

				//Role
				role := subOptions[1].StringValue()

				switch role {
				case "battler":

					//Read metadata for if league signups are open
					signupStatus := botData.Metadata["current_season"].(map[string]interface{})["signups"].(bool)

					//if signupStatus is false, let the user know and return
					if !signupStatus {
						s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
							Type: discordgo.InteractionResponseChannelMessageWithSource,
							Data: &discordgo.InteractionResponseData{
								Content: "Signups are currently closed for this season. Please use `/league open-signups` to open signups and then resubmit.",
								Flags:   discordgo.MessageFlagsEphemeral,
							},
						})
						return
					}

					//upkeep initialization
					guildMember, _ := s.GuildMember(i.GuildID, player.ID)

					//check current roles. If already a battler, let them know they are signed up. If they are a jammer, remove the jammer role
					for _, r := range guildMember.Roles {
						if r == os.Getenv("BATTLER_ID") {
							//Already signed up!
							s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
								Type: discordgo.InteractionResponseChannelMessageWithSource,
								Data: &discordgo.InteractionResponseData{
									Content: fmt.Sprintf("<@%v> is already signed up as a Battler ⚔️ for this season.", player.ID),
									Flags:   discordgo.MessageFlagsEphemeral,
								},
							})
							return
						}
						if r == os.Getenv("JAMMER_ID") {
							s.GuildMemberRoleRemove(i.GuildID, player.ID, os.Getenv("JAMMER_ID"))
						}
					}

					//Revise the player's data in season.json
					//lock the data and unlock once returned/finished
					botData.Mutex.Lock()

					seasonPlayers := botData.Season["season_players"].(map[string]interface{})

					//Logic check if player already in season data
					existingSeasonPlayer, exists := seasonPlayers[player.ID]

					if exists {

						//Player already exists -> update their role
						playerData := existingSeasonPlayer.(map[string]interface{})
						playerData["role"] = "battler"
						playerData["active"] = true
						playerData["dropped"] = false
						playerData["decklist"].(map[string]interface{})["url"] = "Not Submitted"

					} else {

						//New player to season -> create fresh entry
						seasonPlayers[player.ID] = map[string]interface{}{
							"active": true,
							"decklist": map[string]interface{}{
								"url":      "Not Submitted",
								"name":     "",
								"approved": false,
							},
							"dropped":      false,
							"pairings":     []interface{}{},
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
					existingHistoricalPlayer, exists := playersHistory[player.ID]

					if exists {

						//Player already exists -> update their discord nickname or username
						playerData := existingHistoricalPlayer.(map[string]interface{})
						playerData["discord_nickname"] = nickname

					} else {

						//New player to league overall -> create fresh entry
						playersHistory[player.ID] = map[string]interface{}{
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
					s.GuildMemberRoleAdd(i.GuildID, player.ID, os.Getenv("BATTLER_ID"))

					//Respond with an ephemeral message
					s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Content: fmt.Sprintf("<@%v> has been signed up as a Battler ⚔️ for this season!\n**Please direct them to use the `/signup decklist` command to provide their decklist before the season starts.**", player.ID),
							Flags:   discordgo.MessageFlagsEphemeral,
						},
					})

				case "jammer":

					//upkeep initialization
					guildMember, _ := s.GuildMember(i.GuildID, player.ID)

					//check current roles. If already a jammer, let them know they are signed up. If they are a battler, remove the battler role

					for _, r := range guildMember.Roles {
						if r == os.Getenv("JAMMER_ID") {
							//Already signed up!
							s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
								Type: discordgo.InteractionResponseChannelMessageWithSource,
								Data: &discordgo.InteractionResponseData{
									Content: fmt.Sprintf("<@%v> is already signed up as a Jammer 👊 for this season.", player.ID),
									Flags:   discordgo.MessageFlagsEphemeral,
								},
							})
							return
						}
						if r == os.Getenv("BATTLER_ID") {
							s.GuildMemberRoleRemove(i.GuildID, player.ID, os.Getenv("BATTLER_ID"))
						}
					}

					//Revise the player's data in season.json
					//lock the data and unlock once returned/finished
					botData.Mutex.Lock()

					seasonPlayers := botData.Season["season_players"].(map[string]interface{})

					//Logic check if player already in season data
					existingPlayer, exists := seasonPlayers[player.ID]

					if exists {

						//Player already exists -> update their role
						playerData := existingPlayer.(map[string]interface{})
						playerData["role"] = "jammer"
						playerData["active"] = true
						playerData["dropped"] = false

					} else {

						//New player to season -> create fresh entry
						seasonPlayers[player.ID] = map[string]interface{}{
							"active": true,
							"decklist": map[string]interface{}{
								"url":      "Not Submitted",
								"name":     "",
								"approved": false,
							},
							"dropped":      false,
							"pairings":     []interface{}{},
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
					existingHistoricalPlayer, exists := playersHistory[player.ID]

					if exists {

						//Player already exists -> update their discord nickname or username
						playerData := existingHistoricalPlayer.(map[string]interface{})
						playerData["discord_nickname"] = nickname

					} else {

						//New player to league overall -> create fresh entry
						playersHistory[player.ID] = map[string]interface{}{
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
					s.GuildMemberRoleAdd(i.GuildID, player.ID, os.Getenv("JAMMER_ID"))

					//Respond with an ephemeral message
					s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Content: fmt.Sprintf("<@%v> has been signed up as a Jammer 👊 for this season!", player.ID),
							Flags:   discordgo.MessageFlagsEphemeral,
						},
					})
				}

			case "player-drop": //Admin version of drop for selected player

				//Gather info submitted in command
				subOptions := i.ApplicationCommandData().Options[0].Options

				//Player
				player := subOptions[0].UserValue(s)

				//upkeep initializations
				guildMember, _ := s.GuildMember(i.GuildID, player.ID)

				//check if user is currently signed up
				allowedRoles := []string{
					os.Getenv("BATTLER_ID"),
					os.Getenv("JAMMER_ID"),
				}

				// if they dont have the role, reply and return
				if !memberHasRole(guildMember, allowedRoles) {
					s.InteractionRespond(
						i.Interaction,
						&discordgo.InteractionResponse{
							Type: discordgo.InteractionResponseChannelMessageWithSource,
							Data: &discordgo.InteractionResponseData{
								Content: fmt.Sprintf("<@%v> is already not an active participant in the current season. Carry on 🍁", player.ID),
								Flags:   discordgo.MessageFlagsEphemeral,
							},
						},
					)
					return
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
					s.GuildMemberRoleRemove(i.GuildID, player.ID, r)
					s.GuildMemberRoleAdd(i.GuildID, player.ID, os.Getenv("INACTIVE_ID"))

					//Revise the player's data in season.json
					//lock the data and unlock once returned/finished
					botData.Mutex.Lock()
					defer botData.Mutex.Unlock()
					//change dropped to true and active to false
					playerData := botData.Season["season_players"].(map[string]interface{})[player.ID].(map[string]interface{})
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
							Content: fmt.Sprintf("<@%v> has been dropped as a %v for the current season.", player.ID, roleName),
							Flags:   discordgo.MessageFlagsEphemeral,
						},
					})
					//Make drop announcement in organizer channel
					s.ChannelMessageSend(
						os.Getenv("ADMIN_CHNL_ID"),
						fmt.Sprintf("<@&%v>\n<@%v> has been admin-dropped as a %v for the current season.", os.Getenv("ORGANIZER_ID"), player.ID, roleName),
					)

					return
				}
			case "player-points": //Manually modifies points of a specified player

				//Gather info submitted in command
				subOptions := i.ApplicationCommandData().Options[0].Options

				//Player
				player := subOptions[0].UserValue(s)

				//Points
				newPoints := float64(subOptions[1].IntValue())

				//Action
				action := subOptions[2].StringValue()

				//Get Player data
				botData.Mutex.Lock()

				seasonPlayers := botData.Season["season_players"].(map[string]interface{})

				//Logic check if player already in season data
				existingSeasonPlayer, exists := seasonPlayers[player.ID]
				if !exists {
					//player does not exist in seasonal data.
					s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Content: fmt.Sprintf("No seasonal player data found for <@%v>", player.ID),
							Flags:   discordgo.MessageFlagsEphemeral,
						},
					})
					botData.Mutex.Unlock()
					return
				}

				playerSeasonData := existingSeasonPlayer.(map[string]interface{})
				playerSeasonStandings := playerSeasonData["standings"].(map[string]interface{})

				//Adjust points
				originalPoints := playerSeasonStandings["points"].(float64)
				adjustedPoints := float64(0)
				switch action {
				case "add": // add submitted points
					adjustedPoints = originalPoints + newPoints
					playerSeasonStandings["points"] = adjustedPoints
				case "subtract": // subtract submitted points
					adjustedPoints = originalPoints - newPoints
					//correct for negative points
					if adjustedPoints < 0 {
						adjustedPoints = 0
					}
					playerSeasonStandings["points"] = adjustedPoints
				case "set": // set points to submitted value
					adjustedPoints = newPoints
					//correct for negative points
					if adjustedPoints < 0 {
						adjustedPoints = 0
					}
					playerSeasonStandings["points"] = adjustedPoints
				}

				//save the season
				err := saveSeason()
				botData.Mutex.Unlock()
				if err != nil {
					return
				}

				//Reply to admin
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: fmt.Sprintf("<@%v> has had their league points adjusted to `%v` from `%v`.\nA message will be posted in <#%s>", player.ID, adjustedPoints, originalPoints, os.Getenv("ADMIN_CHNL_ID")),
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})

				//Make post in admin channel
				s.ChannelMessageSend(
					os.Getenv("ADMIN_CHNL_ID"),
					fmt.Sprintf("<@&%v>\n<@%v> points adjusted by <@%v>:\n   *`%v` -> `%v` (%v %v)*",
						os.Getenv("ORGANIZER_ID"),
						player.ID,
						i.Member.User.ID,
						originalPoints,
						adjustedPoints,
						action,
						newPoints),
				)

			case "player-info": //Generates player info for a specified player

				//Gather info submitted in command
				subOptions := i.ApplicationCommandData().Options[0].Options

				//Player
				player := subOptions[0].UserValue(s)

				//Get Player data
				botData.Mutex.Lock()

				seasonPlayers := botData.Season["season_players"].(map[string]interface{})
				histPlayers := botData.Players["players"].(map[string]interface{})

				//Logic check if player already in season data
				existingSeasonPlayer, exists_season := seasonPlayers[player.ID]

				//Construct season part of msg
				seasonMsg := fmt.Sprintf("No seasonal data found for <@%v>", player.ID)
				//If player exists revise seasonMsg to include data
				if exists_season {
					playerSeasonData := existingSeasonPlayer.(map[string]interface{})
					playerStandings := playerSeasonData["standings"].(map[string]interface{})

					//Seasonal Data
					role := playerSeasonData["role"]
					//decklist logic. Only get decklist for battlers
					decklistMsg := "N/A"
					if role == "battler" {
						decklist_url := playerSeasonData["decklist"].(map[string]interface{})["url"].(string)
						decklist_name := playerSeasonData["decklist"].(map[string]interface{})["name"].(string)
						decklistMsg = fmt.Sprintf("[%s](%s)", decklist_name, decklist_url)
					}

					points := playerStandings["points"]
					mWins := playerStandings["wins"]
					mLosses := playerStandings["losses"]
					gWins := playerStandings["game_wins"]
					gLosses := playerStandings["game_losses"]

					active := playerSeasonData["active"].(bool)
					dropped := playerSeasonData["dropped"].(bool)
					received_bye := playerSeasonData["received_bye"].(bool)

					//Construct a list of opponents for message that links
					opponents := playerSeasonData["opponents"].([]interface{})
					var oppMentions []string
					for _, opp := range opponents {
						oppMentions = append(oppMentions, fmt.Sprintf("<@%s>", opp.(string)))
					}
					opponentsMsg := strings.Join(oppMentions, ", ")

					//Construct season data formatted string
					seasonMsg = fmt.Sprintf("Role: %v | Decklist: %v\nPoints: %v | Record: %v-%v (Games: %v-%v)\nActive: %v | Dropped: %v | Bye?: %v\nOpponents: %v",
						role, decklistMsg,
						points, mWins, mLosses, gWins, gLosses,
						active, dropped, received_bye,
						opponentsMsg)
				}

				//Logic check if player already in season data
				existingHistoricalPlayer, exists_hist := histPlayers[player.ID]

				//Construct season part of msg
				histMsg := fmt.Sprintf("No historical data found for <@%v>", player.ID)
				//If player exists revise histMsg to include data
				if exists_hist {
					playerHistData := existingHistoricalPlayer.(map[string]interface{})
					playerHistRecord := playerHistData["historical_record"].(map[string]interface{})

					//Hist Data
					mWinsHist := playerHistRecord["wins"]
					mLossesHist := playerHistRecord["losses"]
					gWinsHist := playerHistRecord["game_wins"]
					gLossesHist := playerHistRecord["game_losses"]

					//Construct a list of seasons played
					playedSeasons := playerHistData["seasons_played"].([]interface{})
					var seasonsStrings []string
					for _, seas := range playedSeasons {
						seasonsStrings = append(seasonsStrings, fmt.Sprintf("S%.0f", seas.(float64)))
					}
					playedSeasonsMsg := strings.Join(seasonsStrings, ", ")

					//Construct season data formatted string
					histMsg = fmt.Sprintf("Record: %v-%v (Games: %v-%v)\nSeasons Played: %v\n",
						mWinsHist, mLossesHist, gWinsHist, gLossesHist,
						playedSeasonsMsg)
				}

				//Relock data
				botData.Mutex.Unlock()

				//Construct embed with player data
				embed := &discordgo.MessageEmbed{
					Title:       "PLAYER INFO",
					Description: fmt.Sprintf("Below is a summary of <@%v>'s player data", player.ID),
					Fields: []*discordgo.MessageEmbedField{
						{
							Name:   "SEASON DATA",
							Value:  seasonMsg,
							Inline: false,
						},
						{
							Name:   "HISORICAL DATA",
							Value:  histMsg,
							Inline: false,
						},
					},
				}

				//Send ephemeral reply
				s.InteractionRespond(
					i.Interaction,
					&discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Embeds: []*discordgo.MessageEmbed{
								embed,
							},
							Flags: discordgo.MessageFlagsEphemeral,
						},
					},
				)

			case "match-edit": //Allows revision of a match's data using its matchID.
				//collect options data submitted by command
				subOptions := i.ApplicationCommandData().Options[0].Options

				matchID := subOptions[0].StringValue()

				//Fetch match data
				matchesData := botData.Matches["current_season"].(map[string]interface{})["matches"].(map[string]interface{})
				matchData, exists := matchesData[matchID].(map[string]interface{})

				if !exists {
					//Match does not exist in database
					s.InteractionRespond(
						i.Interaction,
						&discordgo.InteractionResponse{
							Type: discordgo.InteractionResponseChannelMessageWithSource,
							Data: &discordgo.InteractionResponseData{
								Content: fmt.Sprintf("The matchID you submitted was not found: `%v`", matchID),
								Flags:   discordgo.MessageFlagsEphemeral,
							},
						},
					)
					return
				}

				if matchData["status"] == "voided" {
					//Match already voided
					s.InteractionRespond(
						i.Interaction,
						&discordgo.InteractionResponse{
							Type: discordgo.InteractionResponseChannelMessageWithSource,
							Data: &discordgo.InteractionResponseData{
								Content: fmt.Sprintf("The matchID you submitted has already been voided: `%v`", matchID),
								Flags:   discordgo.MessageFlagsEphemeral,
							},
						},
					)
					return
				}

				//Current match data
				currentWinner := matchData["winner"].(string)
				currentLoser := matchData["loser"].(string)
				currentResult := matchData["result"].(string)
				currentBounty := matchData["bounty"].(bool)
				msgID := matchData["msg_id"].(string)

				//Gather input data, using current value if not provided
				winnerRev := currentWinner
				loserRev := currentLoser
				resultRev := currentResult
				bountyRev := currentBounty
				//Booleans are weird so have to do this way. Pointers?
				var bounty *bool

				for _, opt := range subOptions {
					switch opt.Name {
					case "winner":
						winnerRev = opt.UserValue(s).ID
					case "loser":
						loserRev = opt.UserValue(s).ID
					case "result":
						resultRev = opt.StringValue()
					case "bounty":
						b := opt.BoolValue()
						bounty = &b
					}
				}

				if bounty != nil {
					bountyRev = *bounty
				}

				//check if match details changed at all
				if winnerRev == currentWinner && loserRev == currentLoser &&
					resultRev == currentResult && bountyRev == currentBounty {
					s.InteractionRespond(
						i.Interaction,
						&discordgo.InteractionResponse{
							Type: discordgo.InteractionResponseChannelMessageWithSource,
							Data: &discordgo.InteractionResponseData{
								Content: fmt.Sprintf("The match details submitted matches the log for `%v`.\nEither the correction was already made, or review your submission and resubmit.", matchID),
								Flags:   discordgo.MessageFlagsEphemeral,
							},
						},
					)
					return
				}

				//check if winner == loser
				if winnerRev == loserRev {
					s.InteractionRespond(
						i.Interaction,
						&discordgo.InteractionResponse{
							Type: discordgo.InteractionResponseChannelMessageWithSource,
							Data: &discordgo.InteractionResponseData{
								Content: fmt.Sprintf("The winner and loser cannot match. Please resubmit.\nMatch ID: `%v`.", matchID),
								Flags:   discordgo.MessageFlagsEphemeral,
							},
						},
					)
					return
				}

				//if winnerRev == loserRev, either...
				//new winner = old loser, meaning that new loser = old winner
				if winnerRev == loserRev && winnerRev == currentLoser {
					loserRev = currentWinner
				}
				//new loser = old winner, meaning that new winner = old loser
				if winnerRev == loserRev && loserRev == currentWinner {
					winnerRev = currentLoser
				}

				//lock bot data
				botData.Mutex.Lock()

				//reassign values in matchesData
				matchData["winner"] = winnerRev
				matchData["loser"] = loserRev
				matchData["result"] = resultRev
				matchData["bounty"] = bountyRev
				matchData["status"] = "edited"

				//Save matches
				err := saveMatches()
				botData.Mutex.Unlock()
				if err != nil {
					return
				}

				//edit original message
				messageLink := fmt.Sprintf(
					"https://discord.com/channels/%s/%s/%s",
					i.GuildID,
					os.Getenv("BOUNTY_CHNL_ID"),
					msgID,
				)

				//reconstruct embed
				embed := &discordgo.MessageEmbed{
					Title: "Match Result Recorded (⚠️Edited)",
					Fields: []*discordgo.MessageEmbedField{
						{
							Name:   resultRev,
							Value:  fmt.Sprintf("<@%v> WON vs <@%v>", winnerRev, loserRev),
							Inline: true,
						},
					},
					Footer: &discordgo.MessageEmbedFooter{
						Text: fmt.Sprintf("Bounty: %v | MatchID: %v", bountyRev, matchID),
					},
					Color: 0xD80621, // Canadian Flag Red 🍁
				}

				//Edit message
				_, err_edit := s.ChannelMessageEditComplex(
					&discordgo.MessageEdit{
						ID:      msgID,
						Channel: os.Getenv("BOUNTY_CHNL_ID"),
						Embeds:  &[]*discordgo.MessageEmbed{embed},
					},
				)

				if err_edit != nil {
					log.Printf("Error editing match message: %v", err_edit)
					return
				}

				//reply to the user
				s.InteractionRespond(
					i.Interaction,
					&discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Content: fmt.Sprintf("Match (ID:`%v`) Edited.\nWinner: <@%v> | Loser: <@%v> | Result: %v | Bounty: %v\nOriginal message edited: %s", matchID, winnerRev, loserRev, resultRev, bountyRev, messageLink),
							Flags:   discordgo.MessageFlagsEphemeral,
						},
					},
				)

			case "match-delete": //Removes a match from the matches data using its matchID.
				//collect options data submitted by command
				subOptions := i.ApplicationCommandData().Options[0].Options

				matchID := subOptions[0].StringValue()

				//Fetch match data
				matchesData := botData.Matches["current_season"].(map[string]interface{})["matches"].(map[string]interface{})
				matchData, exists := matchesData[matchID].(map[string]interface{})

				if !exists {
					//Match does not exist in database
					s.InteractionRespond(
						i.Interaction,
						&discordgo.InteractionResponse{
							Type: discordgo.InteractionResponseChannelMessageWithSource,
							Data: &discordgo.InteractionResponseData{
								Content: fmt.Sprintf("The matchID you submitted was not found: `%v`", matchID),
								Flags:   discordgo.MessageFlagsEphemeral,
							},
						},
					)
					return
				}

				//lock bot data
				botData.Mutex.Lock()

				//change match status to voided
				matchData["status"] = "voided"

				//Save matches
				err := saveMatches()
				botData.Mutex.Unlock()
				if err != nil {
					return
				}

				//Edit original posting
				msgID := matchData["msg_id"].(string)
				messageLink := fmt.Sprintf(
					"https://discord.com/channels/%s/%s/%s",
					i.GuildID,
					os.Getenv("BOUNTY_CHNL_ID"),
					msgID,
				)

				//Construct new embed
				embed := &discordgo.MessageEmbed{
					Title:       "⛔ Match Voided ⛔",
					Description: "This match has been removed",
					Color:       0xD80621, // Canadian Flag Red 🍁,
					Footer: &discordgo.MessageEmbedFooter{
						Text: fmt.Sprintf("MatchID: %v", matchID),
					},
				}

				//Edit message
				_, err_edit := s.ChannelMessageEditComplex(
					&discordgo.MessageEdit{
						ID:      msgID,
						Channel: os.Getenv("BOUNTY_CHNL_ID"),
						Embeds:  &[]*discordgo.MessageEmbed{embed},
					},
				)

				if err_edit != nil {
					log.Printf("Error editing match message: %v", err_edit)
					return
				}

				//Reply with ephemeral msg
				s.InteractionRespond(
					i.Interaction,
					&discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Content: fmt.Sprintf("Match `%v` has been voided\nOriginal Post Edited:%s", matchID, messageLink),
							Flags:   discordgo.MessageFlagsEphemeral,
						},
					},
				)
			}
		}
	})

	//Reaction handler. Used for decklist review, .....
	discord.AddHandler(func(s *discordgo.Session, r *discordgo.MessageReactionAdd) {

		//Prevents the bot from taking action from messages it reacts to or those outside the admin channel.
		if r.UserID == s.State.User.ID {
			return
		}
		if r.ChannelID != os.Getenv("ADMIN_CHNL_ID") {
			return
		}

		//if reaction is on the decklist review message

		switch r.Emoji.Name {
		case "🔍":
			//Retrieve message data
			msg, err := s.ChannelMessage(r.ChannelID, r.MessageID)
			if err != nil {
				return
			}

			//Sanity checks for message to ensure its the right type
			//Ensure it has an embed
			if len(msg.Embeds) == 0 {
				return
			}
			//Ensure it was written by the bot
			if msg.Author.ID != s.State.User.ID {
				return
			}
			//Ensure it is a "Decklist Review" msg
			if msg.Embeds[0].Title != "Decklist Review Needed" {
				return
			}
			//Ensure it has a footer
			if msg.Embeds[0].Footer == nil {
				return
			}

			//Grab player's userID from the footer of the embeded decklist review msg
			embed := msg.Embeds[0]
			playerID := embed.Footer.Text

			//Update the botData and json data
			//Lock data
			botData.Mutex.Lock()

			playerDecklistData := botData.Season["season_players"].(map[string]interface{})[playerID].(map[string]interface{})["decklist"].(map[string]interface{})

			playerDecklistData["approved"] = true

			err = saveSeason()
			//Unlock data BEFORE error
			botData.Mutex.Unlock()
			if err != nil {
				return
			}

			//DM the player letting them know its been approved.
			channel, err := s.UserChannelCreate(playerID)
			if err == nil {
				s.ChannelMessageSend(
					channel.ID,
					"Your decklist for the current season of the Olympia Canadian Highlander League has been approved. Be on the lookout for the first round pairings in the `#weekly-matches` channel!",
				)
			}

			embed.Title = "Decklist Approved 🔍"
			embed.Color = 0x00FF00

			_, _ = s.ChannelMessageEditEmbed(
				r.ChannelID,
				r.MessageID,
				embed,
			)

		case "❌":
			//Retrieve message data
			msg, err := s.ChannelMessage(r.ChannelID, r.MessageID)
			if err != nil {
				return
			}

			//Sanity checks for message to ensure its the right type
			//Ensure it has an embed
			if len(msg.Embeds) == 0 {
				return
			}
			//Ensure it was written by the bot
			if msg.Author.ID != s.State.User.ID {
				return
			}
			//Ensure it is a "Decklist Review" msg
			if msg.Embeds[0].Title != "Decklist Review Needed" {
				return
			}
			//Ensure it has a footer
			if msg.Embeds[0].Footer == nil {
				return
			}

			//Grab player's userID from the footer of the embeded decklist review msg
			embed := msg.Embeds[0]
			playerID := embed.Footer.Text

			//DM the player letting them know its been denied.
			channel, err := s.UserChannelCreate(playerID)
			if err == nil {
				s.ChannelMessageSend(
					channel.ID,
					"Your decklist for the current season of the Olympia Canadian Highlander League has been denied. Please use `/signup decklist` to resubmit, or contact an organizer.",
				)
			}

			embed.Title = "Decklist Denied ❌"
			embed.Color = 0xFF0000

			_, _ = s.ChannelMessageEditEmbed(
				r.ChannelID,
				r.MessageID,
				embed,
			)
		}
	})

	//---------------------------------------------------------------------//
	//CHAT COMMANDS

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
