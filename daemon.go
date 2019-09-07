package gomaspri

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/mail"
	"strings"
	"time"

	"github.com/badoux/checkmail"
	"github.com/emersion/go-imap"
	idle "github.com/emersion/go-imap-idle"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
)

type ListDaemon struct {
	config Config
	client *client.Client
}

func NewDaemon(config Config) ListDaemon {
	var daemon ListDaemon
	daemon.config = config
	return daemon
}

func (daemon *ListDaemon) Connect() error {
	log.Println("Connecting to server...")

	// Connect to server
	c, err := client.DialTLS(daemon.config.Mail.ImapHostPort(), nil)
	if err != nil {
		return err
	}
	daemon.client = c
	log.Println("Connected")

	// Login
	if err := daemon.client.Login(daemon.config.Mail.User, daemon.config.Mail.Pass); err != nil {
		return err
	}
	log.Println("Logged in")
	return nil
}

func (daemon *ListDaemon) Close() {
	daemon.client.Logout()
}

func (daemon *ListDaemon) GetUnseenMail() ([]imap.Message, error) {

	// Select INBOX
	mbox, err := daemon.client.Select("INBOX", false)
	if err != nil {
		return nil, err
	}
	// log.Println("Flags for INBOX:", mbox.Flags)

	// Get the last message
	if mbox.Messages == 0 {
		log.Println("The mailbox is empty")
	}

	seqset, unseen, err := daemon.getUnseenMessageSeq()
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
		done <- daemon.client.Fetch(seqset, items, messageChannels)
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

func (daemon *ListDaemon) getUnseenMessageSeq() (*imap.SeqSet, uint32, error) {

	criteria := imap.NewSearchCriteria()
	criteria.WithoutFlags = []string{imap.SeenFlag}
	ids, err := daemon.client.Search(criteria)
	if err != nil {
		return nil, 0, err
	}

	if len(ids) > 0 {
		seqset := new(imap.SeqSet)
		seqset.AddNum(ids...)
		return seqset, uint32(len(ids)), nil
	} else {
		return new(imap.SeqSet), 0, nil
	}
}

func (daemon *ListDaemon) ProcessMails(messages []imap.Message) {
	for _, msg := range messages {
		senderAddress := fmt.Sprintf("%v@%v", msg.Envelope.From[0].MailboxName, msg.Envelope.From[0].HostName)
		subject := msg.Envelope.Subject

		if subject == "*show" && daemon.config.ContainsAddress(senderAddress) {
			daemon.SendList(senderAddress)
		} else if subject == "*add" && daemon.config.IsAdmin(senderAddress) {
			daemon.AddRecipients(senderAddress, msg)
		} else if daemon.config.ContainsAddress(senderAddress) {
			daemon.ForwardMessage(msg)
		} else {
			log.Println("Rejected: Sender not in list")
		}
	}
}

func (daemon *ListDaemon) sendMail(to []string, msg io.Reader) error {
	// Set up authentication information.
	auth := sasl.NewPlainClient("", daemon.config.Mail.User, daemon.config.Mail.Pass)

	// Connect to the server, authenticate, set the sender and recipient,
	// and send the email all in one step.
	from := daemon.config.Mail.Address
	err := smtp.SendMail(daemon.config.Mail.SmtpHostPort(), auth, from, to, msg)
	return err
}

func (daemon *ListDaemon) SendList(recipient string) error {
	log.Printf("Sending list to: %v\n", recipient)
	msg := fmt.Sprintf("To: %v\nSubject: Mailing list\n", recipient)
	for _, address := range daemon.config.List.Recipients {
		msg = msg + "\n" + address
	}
	reader := bytes.NewReader([]byte(msg))
	return daemon.sendMail([]string{recipient}, reader)
}

func (daemon *ListDaemon) AddRecipients(senderAddress string, msg imap.Message) error {
	log.Println("Adding rec")

	// Getting body
	section := &imap.BodySectionName{}
	// section, err := imap.ParseBodySectionName(imap.FetchBody)
	// if err != nil {
	// 	return err
	// }
	response := msg.GetBody(section)
	if response == nil {
		return errors.New("Server didn't returned message body")
	}

	m, err := mail.ReadMessage(response)
	if err != nil {
		log.Fatal(err)
	}

	// header := m.Header
	// log.Println("Date:", header.Get("Date"))
	// log.Println("From:", header.Get("From"))
	// log.Println("To:", header.Get("To"))
	// log.Println("Subject:", header.Get("Subject"))

	body, err := ioutil.ReadAll(m.Body)
	if err != nil {
		return err
	}
	bodymessage := string(body)
	for _, addressRaw := range strings.Split(bodymessage, "\r\n") {
		if len(addressRaw) == 0 {
			continue
		}

		address := strings.TrimSpace(addressRaw)
		mailErr := checkmail.ValidateFormat(address)
		if mailErr == nil {
			fmt.Printf("Adding new recipient: %v\n", address)
			if err := daemon.config.AddRecipient(address); err != nil {
				fmt.Println(err)
			}
		} else {
			fmt.Printf("Rejecting new recipient: %v\n", address)
		}
	}

	daemon.SendList(senderAddress)
	return err
}

func (daemon *ListDaemon) ForwardMessages(messages []imap.Message) {
	for _, msg := range messages {
		err := daemon.ForwardMessage(msg)
		if err != nil {
			log.Println(err)
		}
	}
}

func (daemon *ListDaemon) ForwardMessage(msg imap.Message) error {
	log.Printf("Forwarding: %v - %v <%v@%v>: %v\n", msg.Envelope.Date, msg.Envelope.From[0].PersonalName, msg.Envelope.From[0].MailboxName, msg.Envelope.From[0].HostName, msg.Envelope.Subject)

	// Getting body
	section := &imap.BodySectionName{}
	msgBody := msg.GetBody(section)
	if msgBody == nil {
		return errors.New("Server didn't returned message body")
	}

	log.Printf(" -> to: %v\n", daemon.config.List.Recipients)

	to := daemon.config.List.Recipients
	err := daemon.sendMail(to, msgBody)
	return err
}

func (daemon *ListDaemon) Repeat(stop <-chan struct{}, fun func()) error {
	pollInterval := time.Duration(daemon.config.List.Interval) * time.Second
	t := time.NewTicker(pollInterval)
	defer t.Stop()

	for {
		select {
		case <-t.C:
			if err := daemon.client.Noop(); err != nil {
				return err
			}
			fun()
		case <-stop:
			return nil
		case <-daemon.client.LoggedOut():
			return errors.New("disconnected while idling")
		}
	}
	return nil
}

func (daemon *ListDaemon) OnUpdate(stop <-chan struct{}, fun func()) {

	idleClient := idle.NewClient(daemon.client)

	// Create a channel to receive mailbox updates
	updates := make(chan client.Update)
	daemon.client.Updates = updates

	// Start idling
	done := make(chan error, 1)
	go func() {
		done <- idleClient.IdleWithFallback(stop, time.Duration(daemon.config.List.Interval)*time.Second)
	}()

	// Listen for updates
	for {
		select {
		case update := <-updates:
			log.Println("New update:", update)
		case err := <-done:
			if err != nil {
				log.Fatal(err)
			}
			log.Println("Not idling anymore")
			return
		}
	}

}
