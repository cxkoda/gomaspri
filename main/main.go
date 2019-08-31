package main

import (
	"fmt"

	"github.com/cxkoda/gomaspri"
)

func main() {
	config := gomaspri.ReadConfig("./config.toml")
	fmt.Println(config)
}
