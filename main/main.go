package main

import (
	"log"

	"github.com/cxkoda/gomaspri"
)

func main() {
	config := gomaspri.ReadConfig("./config.toml")

	config.Repeat(func() {
		messages, err := config.GetUnseenMail()
		if err != nil {
			log.Println(err)
		} else {
			log.Println("Found New Mail", len(messages))
			config.ForwardMessages(messages)
		}
	})
}
