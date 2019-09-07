package gomaspri

import (
	"bytes"
	"errors"
	"fmt"
	"io"
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
		messages = append(messages, *msg)
	}

	if err := <-done; err != nil {
		return nil, err
	}

	return messages, nil
}

func getUnseenMessageSeq(client *client.Client, mbox *imap.MailboxStatus) (*imap.SeqSet, uint32, error) {

	criteria := imap.NewSearchCriteria()
	criteria.WithoutFlags = []string{imap.SeenFlag}
	ids, err := client.Search(criteria)
	if err != nil {
		return nil, 0, err
	}
	log.Println("IDs found:", ids)

	if len(ids) > 0 {
		seqset := new(imap.SeqSet)
		seqset.AddNum(ids...)
		return seqset, uint32(len(ids)), nil
	} else {
		return new(imap.SeqSet), 0, nil
	}
}

func (config *Config) ProcessMails(messages []imap.Message) {
	for _, msg := range messages {
		senderAddress := fmt.Sprintf("%v@%v", msg.Envelope.From[0].MailboxName, msg.Envelope.From[0].HostName)
		subject := msg.Envelope.Subject

		if subject == "*show" && config.ContainsAddress(senderAddress) {
			config.SendList(senderAddress)
		} else if config.ContainsAddress(senderAddress) {
			config.ForwardMessage(msg)
		} else {
			log.Println("Rejected: Sender not in list")
		}
	}
}

func (config *Config) sendMail(to []string, msg io.Reader) error {
	// Set up authentication information.
	auth := sasl.NewPlainClient("", config.Mail.User, config.Mail.Pass)

	// Connect to the server, authenticate, set the sender and recipient,
	// and send the email all in one step.
	from := config.Mail.Address
	err := smtp.SendMail(config.Mail.SmtpHostPort, auth, from, to, msg)
	return err
}

func (config *Config) SendList(recipient string) error {
	log.Println("Sending list to: %v", recipient)
	msg := fmt.Sprintf("To: %v\nSubject: Mailing list\n", recipient)
	for _, address := range config.List.Recipients {
		msg = msg + "\n" + address
	}
	reader := bytes.NewReader([]byte(msg))
	return config.sendMail([]string{recipient}, reader)
}

func (config *Config) AddRecipients(senderAddress string, msg imap.Message) error {
	log.Println("Adding rec")

	// Getting body
	section := &imap.BodySectionName{}
	// section, err := imap.ParseBodySectionName(imap.FetchBody)
	// if err != nil {
	// 	return err
	// }
	msgBody := msg.GetBody(section)
	if msgBody == nil {
		return errors.New("Server didn't returned message body")
	}

	log.Println("BODY:%v", msgBody)

	// return config.SendList(senderAddress)
	return nil
}

func (config *Config) ForwardMessages(messages []imap.Message) {
	for _, msg := range messages {
		err := config.ForwardMessage(msg)
		if err != nil {
			log.Println(err)
		}
	}
}

func (config *Config) ForwardMessage(msg imap.Message) error {
	log.Printf("Forwarding: %v - %v <%v@%v>: %v\n", msg.Envelope.Date, msg.Envelope.From[0].PersonalName, msg.Envelope.From[0].MailboxName, msg.Envelope.From[0].HostName, msg.Envelope.Subject)

	// Getting body
	section := &imap.BodySectionName{}
	msgBody := msg.GetBody(section)
	if msgBody == nil {
		return errors.New("Server didn't returned message body")
	}

	log.Printf(" -> to: %v\n", config.List.Recipients)

	to := config.List.Recipients
	err := config.sendMail(to, msgBody)
	return err
}

func (config *Config) Repeat(fun func()) {
	for {
		fun()
		time.Sleep(time.Duration(config.List.Interval) * time.Second)
	}
}
