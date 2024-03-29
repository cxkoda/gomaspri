package gomaspri

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/BurntSushi/toml"
)

type MailConfig struct {
	ImapHost string `toml:"imapHost"`
	ImapPort uint32 `toml:"imapPort"`
	SmtpHost string `toml:"smtpHost"`
	SmtpPort uint32 `toml:"smtpPort"`
	Address  string `toml:"address"`
	User     string `toml:"user"`
	Pass     string `toml:"pass"`
}

func (mc *MailConfig) ImapHostPort() string {
	return mc.ImapHost + ":" + fmt.Sprint(mc.ImapPort)
}

func (mc *MailConfig) SmtpHostPort() string {
	return mc.SmtpHost + ":" + fmt.Sprint(mc.SmtpPort)
}

type ListConfig struct {
	Interval   uint32   `toml:"interval"`
	Recipients []string `toml:"recipients"`
	Admins     []string `toml:"admins"`
}

type Config struct {
	Mail     MailConfig `toml:"mail"`
	List     ListConfig `toml:"list"`
	filepath string
}

func ReadConfig(filepath string) Config {
	var config Config
	if _, err := toml.DecodeFile(filepath, &config); err != nil {
		log.Fatalln(err)
	}

	config.filepath = filepath
	return config
}

func (config *Config) IsRecipient(address string) bool {
	_, err := config.GetRecipient(address)
	if err == nil {
		return true
	} else {
		return false
	}
}

func (config *Config) GetRecipient(address string) (int, error) {
	for index, listAddress := range config.List.Recipients {
		if address == listAddress {
			return index, nil
		}
	}
	return -1, errors.New("Recipient not in list")
}

func (config *Config) IsAdmin(address string) bool {
	for _, listAddress := range config.List.Admins {
		if address == listAddress {
			return true
		}
	}
	return false
}

func (config *Config) AddRecipient(address string) error {
	ok := config.IsRecipient(address)
	if !ok {
		return errors.New("Recipient already in list")
	}
	config.List.Recipients = append(config.List.Recipients, address)
	return config.UpdateFile()
}

func (config *Config) DelRecipient(address string) error {
	index, err := config.GetRecipient(address)
	if err != nil {
		return err
	}
	config.List.Recipients = append(config.List.Recipients[:index], config.List.Recipients[index+1:]...)
	return config.UpdateFile()

}

func (config *Config) GetRecipientString() string {
	var buf bytes.Buffer

	for _, address := range config.List.Recipients {
		buf.WriteString(fmt.Sprintf("%v\n", address))
	}

	return buf.String()
}

func (config *Config) UpdateFile() error {
	buf := new(bytes.Buffer)
	if err := toml.NewEncoder(buf).Encode(config); err != nil {
		return err
	}

	f, err := os.Create(config.filepath)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.Write(buf.Bytes()); err != nil {
		return err
	}

	return nil
}
