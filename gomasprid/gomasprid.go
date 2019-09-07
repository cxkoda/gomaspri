package main

import (
	"log"
	"os"

	"github.com/cxkoda/gomaspri"
)

func main() {
	config := gomaspri.ReadConfig(os.Args[1])
	daemon := gomaspri.NewDaemon(config)
	daemon.Connect()
	defer daemon.Close()

	daemon.Repeat(nil, func() {
		messages, err := daemon.GetUnseenMail()
		if err != nil {
			log.Println(err)
		} else {
			log.Println("Found New Mail", len(messages))
			daemon.ProcessMails(messages)
		}
	})
}
