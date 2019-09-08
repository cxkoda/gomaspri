package main

import (
	"fmt"
	"os"

	"github.com/cxkoda/gomaspri"
)

func main() {
	config := gomaspri.ReadConfig(os.Args[1])
	daemon := gomaspri.NewDaemon(config)
	daemon.Connect()
	defer daemon.Close()

	daemon.OnUpdate(nil, func() {
		messages, err := daemon.GetUnseenMail()
		if err != nil {
			fmt.Println(err)
		} else {
			if len(messages) > 0 {
				fmt.Println("Found New Mail", len(messages))
			}
			daemon.ProcessMails(messages)
		}
	})
}
