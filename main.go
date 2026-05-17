package main

import (
	"log"
	"os"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

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
