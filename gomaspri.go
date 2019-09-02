package gomaspri

import (
	"fmt"
	"log"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
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

func (config *Config) GetUnseenMail() ([]imap.Message, error) {
	log.Println("Connecting to server...")

	// Connect to server
	c, err := client.DialTLS(config.Mail.ImapHostPort, nil)
	if err != nil {
		return nil, err
	}
	log.Println("Connected")

	// Don't forget to logout
	defer c.Logout()

	// Login
	if err := c.Login(config.Mail.User, config.Mail.Pass); err != nil {
		return nil, err
	}
	log.Println("Logged in")

	// Select INBOX
	mbox, err := c.Select("INBOX", false)
	if err != nil {
		return nil, err
	}
	// log.Println("Flags for INBOX:", mbox.Flags)

	// Get the last message
	if mbox.Messages == 0 {
		log.Println("The mailbox is empty")
	}

	seqset, unseen, err := getUnseenMessageSeq(c, mbox)
	if err != nil {
		return nil, err
	}

	if unseen == 0 {
		return []imap.Message{}, nil
	}

	// Get the whole message body
	section := &imap.BodySectionName{}
	items := []imap.FetchItem{section.FetchItem(), imap.FetchEnvelope}

	messageChannels := make(chan *imap.Message, unseen)
	done := make(chan error, unseen)
	go func() {
		done <- c.Fetch(seqset, items, messageChannels)
	}()

	// Convert channel to slice
	messages := make([]imap.Message, 0)
	for msg := range messageChannels {
		senderAddress := fmt.Sprintf("%v@%v", msg.Envelope.From[0].MailboxName, msg.Envelope.From[0].HostName)
		log.Printf("%v: %v <%v>: %v\n", msg.Envelope.Date, msg.Envelope.From[0].PersonalName, senderAddress, msg.Envelope.Subject)
		if config.ContainsAddress(senderAddress) {
			messages = append(messages, *msg)
		} else {
			log.Println("Rejected: Sender not in list")
		}
	}

	if err := <-done; err != nil {
		return nil, err
	}

	return messages, nil
}

func getUnseenMessageSeq(client *client.Client, mbox *imap.MailboxStatus) (*imap.SeqSet, uint32, error) {
	from := mbox.UnseenSeqNum
	to := mbox.Messages

	if mbox.UnseenSeqNum == 0 {
		return new(imap.SeqSet), 0, nil
	}

	seqset := new(imap.SeqSet)
	seqset.AddRange(from, to)
	seqsetSize := to - from + 1

	items := []imap.FetchItem{imap.FetchFlags}

	messages := make(chan *imap.Message, seqsetSize)
	done := make(chan error, seqsetSize)
	go func() {
		done <- client.Fetch(seqset, items, messages)
	}()

	seqsetUnseen := new(imap.SeqSet)
	var nUnseen uint32 = 0

	for msg := range messages {
		// log.Println(msg.Flags)
		isNew := true
		for _, flag := range msg.Flags {
			if flag == imap.SeenFlag {
				isNew = false
				break
			}
		}
		if isNew {
			// log.Println("Found an unseen one", msg.SeqNum)
			seqsetUnseen.AddNum(msg.SeqNum)
			nUnseen++
		}
	}

	if err := <-done; err != nil {
		return nil, 0, err
	}

	return seqsetUnseen, nUnseen, nil
}

func (config *Config) ForwardMessages(messages []imap.Message) {
	for _, msg := range messages {
		config.forward(msg)
	}
}

func (config *Config) forward(msg imap.Message) {
	log.Printf("Forwarding: %v - %v <%v@%v>: %v\n", msg.Envelope.Date, msg.Envelope.From[0].PersonalName, msg.Envelope.From[0].MailboxName, msg.Envelope.From[0].HostName, msg.Envelope.Subject)

	// Getting body
	section := &imap.BodySectionName{}
	msgBody := msg.GetBody(section)
	if msgBody == nil {
		log.Println("Server didn't returned message body")
		return
	}

	log.Printf(" -> to: %v\n", config.List.Recipients)

	// Set up authentication information.
	auth := sasl.NewPlainClient("", config.Mail.User, config.Mail.Pass)

	// Connect to the server, authenticate, set the sender and recipient,
	// and send the email all in one step.
	from := config.Mail.Address
	to := config.List.Recipients
	err := smtp.SendMail(config.Mail.SmtpHostPort, auth, from, to, msgBody)
	if err != nil {
		log.Fatal(err)
	}
}

func (config *Config) Repeat(fun func()) {
	for {
		fun()
		time.Sleep(time.Duration(config.List.Interval) * time.Second)
	}
}
