package gomaspri

import (
	"io"
	"io/ioutil"
	"log"
	"strconv"

	"github.com/BurntSushi/toml"
	"gopkg.in/gomail.v2"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message/mail"
	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
)

type MailConfig struct {
	ImapHost     string `toml:"imapHost"`
	ImapPort     int    `toml:"imapPort"`
	SmtpHost     string `toml:"smtpHost"`
	SmtpPort     int    `toml:"smtpPort"`
	Address      string `toml:"address"`
	User         string `toml:"user"`
	Pass         string `toml:"pass"`
	ImapHostPort string
	SmtpHostPort string
}

type ListConfig struct {
	Recipients []string `toml:"recipients"`
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

	config.Mail.ImapHostPort = config.Mail.ImapHost + ":" + strconv.Itoa(config.Mail.ImapPort)
	config.Mail.SmtpHostPort = config.Mail.SmtpHost + ":" + strconv.Itoa(config.Mail.SmtpPort)

	return config
}

func GetUnseenMessageSeq(client *client.Client, mbox *imap.MailboxStatus) (*imap.SeqSet, uint32, error) {
	from := mbox.UnseenSeqNum
	to := mbox.Messages

	if mbox.UnseenSeqNum == 0 {
		return new(imap.SeqSet), 0, nil
	}

	seqset := new(imap.SeqSet)
	seqset.AddRange(from, to)
	seqsetSize := to - from + 1

	// return seqset, seqsetSize, nil
	// section := &imap.BodySectionName{}

	// items := []imap.FetchItem{section.FetchItem(), imap.FetchEnvelope}
	// items := imap.FetchFlags.Expand()
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

func (config *Config) GetUnseenMail() []imap.Message {
	log.Println("Connecting to server...")

	// Connect to server
	c, err := client.DialTLS(config.Mail.ImapHostPort, nil)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Connected")

	// Don't forget to logout
	defer c.Logout()

	// Login
	if err := c.Login(config.Mail.User, config.Mail.Pass); err != nil {
		log.Fatal(err)
	}
	log.Println("Logged in")

	// Select INBOX
	mbox, err := c.Select("INBOX", false)
	if err != nil {
		log.Fatal(err)
	}
	// log.Println("Flags for INBOX:", mbox.Flags)

	// Get the last message
	if mbox.Messages == 0 {
		log.Println("The mailbox is empty")
	}

	seqset, unseen, err := GetUnseenMessageSeq(c, mbox)
	if err != nil {
		log.Fatal(err)
	}

	if unseen == 0 {
		return []imap.Message{}
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
		log.Printf("%v: %v <%v@%v>: %v\n", msg.Envelope.Date, msg.Envelope.From[0].PersonalName, msg.Envelope.From[0].MailboxName, msg.Envelope.From[0].HostName, msg.Envelope.Subject)
		messages = append(messages, *msg)
	}

	if err := <-done; err != nil {
		log.Fatal(err)
	}

	return messages
}

func (config *Config) SendMail(r imap.Literal) {
	m := gomail.NewMessage()
	m.SetHeader("From", config.Mail.Address)
	recipients := config.List.Recipients
	m.SetHeader("To", recipients...)

	// Create a new mail reader
	mr, err := mail.CreateReader(r)
	if err != nil {
		log.Fatal(err)
	}

	subject, err := mr.Header.Subject()
	if err != nil {
		log.Fatal(err)
	}

	m.SetHeader("Subject", subject)

	// Read each mail's part
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		} else if err != nil {
			log.Fatal(err)
		}

		switch h := p.Header.(type) {
		case *mail.InlineHeader:
			b, _ := ioutil.ReadAll(p.Body)
			log.Printf("type: %v\n", p.Header.Get("Content-Type"))
			log.Printf("Got text: %v\n", string(b))

			m.SetBody(p.Header.Get("Content-Type"), string(b))
		case *mail.AttachmentHeader:
			filename, _ := h.Filename()

			b, _ := ioutil.ReadAll(p.Body)
			m.SetBody(p.Header.Get("Content-Type"), string(b))
			log.Printf("Got attachment: %v\n", filename)
		}
	}

	// Connect to the smtp and send the email
	d := gomail.NewDialer(config.Mail.SmtpHost, config.Mail.SmtpPort, config.Mail.User, config.Mail.Pass)

	if err := d.DialAndSend(m); err != nil {
		panic(err)
	}
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
