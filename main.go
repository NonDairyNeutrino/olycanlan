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
		return err
	}

	return os.WriteFile(
		"site/data/metadata.json",
		data,
		0644,
	)
}

func savePlayers() error {
	data, err := json.MarshalIndent(
		botData.Players,
		"",
		"    ",
	)

	if err != nil {
		return err
	}

	return os.WriteFile(
		"site/data/players.json",
		data,
		0644,
	)
}

func saveMatches() error {
	data, err := json.MarshalIndent(
		botData.Matches,
		"",
		"    ",
	)

	if err != nil {
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
	//Tester little ping pong command
	{Name: "ping",
		Description: "Replies with Pong!",
	},

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

	//Signup (battler, jammer, open, close)
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
				Name:        "open",
				Description: "Open the current season for signups",
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "close",
				Description: "Closes the current season for signups",
			},
		},
	},

	//Season commands (drop, new, rules)
	{Name: "season",
		Description: "Season commands",

		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "drop",
				Description: "Drop from the current league season",
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "new",
				Description: "Begins a new season of the league",
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
	// Add new slash commands as new case: "command"
	discord.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		//Ensures its a slash command
		if i.Type != discordgo.InteractionApplicationCommand {
			return
		}

		switch i.ApplicationCommandData().Name {
		case "ping":

			err := s.InteractionRespond(
				i.Interaction,
				&discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "Pong!",
					},
				},
			)

			if err != nil {
				log.Println(err)
			}
		case "result":

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
			nextMatchId := int(currentSeasonMetadata["next_match_id"].(float64))
			newMatchId := fmt.Sprintf("%s-%03d",
				seasonPrefix,
				nextMatchId)

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
					s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Content: "Signups are currently closed for this season. Please use `/signup jammer` if you are interested in joining as a Jammer 👊.",
							Flags:   discordgo.MessageFlagsEphemeral,
						},
					})
					return
				}

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
			case "open":

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
			case "close":
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

					//Reply with a hidden message that the league is now open
					s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Content: "Signups for the current league have been closed!",
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
			}
		case "season":
			sub := i.ApplicationCommandData().Options[0].Name
			switch sub {
			//season drop
			case "drop":

				guildMember, _ := s.GuildMember(i.GuildID, i.Member.User.ID)

				//check current roles. If not already a jammer or battler do nothing.
				for _, r := range guildMember.Roles {
					if r == os.Getenv("JAMMER_ID") || r == os.Getenv("BATTLER_ID") {
						//Currently signed up
						//Remove role
						s.GuildMemberRoleRemove(i.GuildID, i.Member.User.ID, r)
						s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
							Type: discordgo.InteractionResponseChannelMessageWithSource,
							Data: &discordgo.InteractionResponseData{
								Content: "You have been dropped from the current league season. Hope you can join us in the future!",
								Flags:   discordgo.MessageFlagsEphemeral,
							},
						})
					}
				}
			//season new
			case "new":

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
		}
	})

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
