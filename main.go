package main

import (
	"fmt"
	"log"
	"os"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal(err)
	}

	discord, err := discordgo.New("Bot " + os.Getenv("APP_ID"))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("ready?", discord.DataReady)

}
