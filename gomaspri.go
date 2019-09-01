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

func (config *Config) GetMail() []imap.Literal {
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
		log.Fatal("No message in mailbox")
	}
	from := uint32(1)
	to := mbox.Messages
	seqset := new(imap.SeqSet)
	seqset.AddRange(from, to)

	// Get the whole message body
	section := &imap.BodySectionName{}
	// items := []imap.FetchItem{imap.FetchAll}
	// items := imap.FetchFull.Expand()
	// items = append(items, section.FetchItem())
	items := []imap.FetchItem{section.FetchItem(), imap.FetchEnvelope}
	log.Println(items)

	messages := make(chan *imap.Message, mbox.Messages)
	done := make(chan error, mbox.Messages)
	go func() {
		done <- c.Fetch(seqset, items, messages)
	}()

	var messageBodies []imap.Literal

	for msg := range messages {
		isNew := false
		for _, flag := range msg.Flags {
			if flag == imap.SeenFlag {
				isNew = true
			}
		}
		if isNew {
			log.Println(msg.Envelope.Subject)

			r := msg.GetBody(section)

			if r == nil {
				log.Fatal("Server didn't returned message body")
			}

			messageBodies = append(messageBodies, r)
		}
	}

	if err := <-done; err != nil {
		log.Fatal(err)
	}

	return messageBodies
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

func (config *Config) PlainForward(messages []imap.Literal) {
	for _, msg := range messages {
		config.plainForwardSingle(msg)
	}
}

func (config *Config) plainForwardSingle(msg imap.Literal) {
	log.Printf("Forwarding to: %v\n", config.List.Recipients)
	// Set up authentication information.
	auth := sasl.NewPlainClient("", config.Mail.User, config.Mail.Pass)

	// Connect to the server, authenticate, set the sender and recipient,
	// and send the email all in one step.
	from := config.Mail.Address
	to := config.List.Recipients
	err := smtp.SendMail(config.Mail.SmtpHostPort, auth, from, to, msg)
	if err != nil {
		log.Fatal(err)
	}
}
