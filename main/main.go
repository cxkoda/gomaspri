package main

import (
	"log"

	"github.com/cxkoda/gomaspri"
)

func main() {
	config := gomaspri.ReadConfig("./config.toml")
	log.Printf("Config: %v\n", config)

	r := config.GetMail()

	// fmt.Println(r)

	config.PlainForward(r)

}
