package main

import (
	"log"

	"github.com/cxkoda/gomaspri"
)

func main() {
	config := gomaspri.ReadConfig("./config.toml")

	messages := config.GetUnseenMail()
	log.Println("Found New Mail", len(messages))
	config.ForwardMessages(messages)
}
