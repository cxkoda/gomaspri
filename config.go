package gomaspri

import (
	"fmt"
	"log"

	"github.com/BurntSushi/toml"
)

type MailConfig struct {
	ImapHost     string `toml:"imapHost"`
	ImapPort     uint32 `toml:"imapPort"`
	SmtpHost     string `toml:"smtpHost"`
	SmtpPort     uint32 `toml:"smtpPort"`
	Address      string `toml:"address"`
	User         string `toml:"user"`
	Pass         string `toml:"pass"`
	ImapHostPort string
	SmtpHostPort string
}

type ListConfig struct {
	Interval   uint32   `toml:"interval"`
	Recipients []string `toml:"recipients"`
	Admins     []string `toml:"admins"`
}

type Config struct {
	Mail     MailConfig `toml:"mail"`
	List     ListConfig `toml:"list"`
	Filepath string
}

func ReadConfig(Filepath string) Config {
	var config Config
	if _, err := toml.DecodeFile(Filepath, &config); err != nil {
		log.Fatalln(err)
	}

	config.Filepath = Filepath
	config.Mail.ImapHostPort = config.Mail.ImapHost + ":" + fmt.Sprint(config.Mail.ImapPort)
	config.Mail.SmtpHostPort = config.Mail.SmtpHost + ":" + fmt.Sprint(config.Mail.SmtpPort)

	return config
}

func (config *Config) ContainsAddress(address string) bool {
	for _, listAddress := range config.List.Recipients {
		if address == listAddress {
			return true
		}
	}
	return false
}

func (config *Config) IsAdmin(address string) bool {
	for _, listAddress := range config.List.Admins {
		if address == listAddress {
			return true
		}
	}
	return false
}
