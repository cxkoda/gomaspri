package main

import (
	"log"
	"os"

	"github.com/cxkoda/gomaspri"
)

func main() {
	config := gomaspri.ReadConfig(os.Args[1])

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
