package main

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
)

func main() {
	discord, err := discordgo.New("Bot " + "authentication token")

	fmt.Println("discord = ", discord)
	fmt.Println("err = ", err)

	// if err != nil {
	// 	fmt.Println("Error connecting to Discord (Is the auth token correct?)", err)
	// } else {
	// 	fmt.Println("Discord connected:", discord)
	// }

}
