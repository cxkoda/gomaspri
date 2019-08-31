package gomaspri

import (
	"fmt"
	"github.com/BurntSushi/toml"
	"log"
)

var log = log.New(os.Stderr, "", log.LstdFlags | log.Lshortfile)

type MailConfig struct {
	Host string `toml:"host"`
	User string `toml:"user"`
	Pass string `toml:"pass"`
}

type ListConfig struct {
}

type Config struct {
	Mail MailConfig `toml:"mail"`
	List ListConfig `toml:"list"`
}

func ReadConfig(Filepath string) Config {
	var config Config
	if _, err := toml.DecodeFile(Filepath, &config); err != nil {
		log.Fatalln(err)
	}

	return config
}