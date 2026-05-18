package main

import (
	"log"
	"os"

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

	// test the connection to Discord by getting information about the e.g. General channel
	gnrl_id := os.Getenv("CHNL_ID")
	chnl, err := discord.Channel(gnrl_id)
	if err != nil {
		log.Fatalln("Error getting channel id\n", err)
	}
	log.Println(chnl)

}
