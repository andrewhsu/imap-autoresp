package main

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"log"
	"net/mail"
	"net/smtp"
	"os"
	"time"

	"github.com/mxk/go-imap/imap"
)

func main() {

	var (
		c   *imap.Client
		cmd *imap.Command
		rsp *imap.Response
		cfg *tls.Config
	)

	c, err := imap.DialTLS("imap.gmail.com:993", cfg)
	if err != nil {
		log.Fatal(err)
	}

	defer c.Logout(5 * time.Second)

	// Print server greeting (first response in the unilateral server data queue)
	log.Println("Server says hello:", c.Data[0].Info)
	c.Data = nil

	// Authenticate
	if c.State() == imap.Login {
		_, err := c.Login("jsmith@example.com", "password")
		if err != nil {
			log.Fatal(err)
		}
	} else {
		log.Fatal("dunno what state i'm in")
	}

	// Check for new unilateral server data responses
	for _, rsp := range c.Data {
		log.Println("Server data:", rsp)
	}
	c.Data = nil

	// Open a mailbox (synchronous command - no need for imap.Wait)
	_, err = c.Select("INBOX", true)
	log.Print("\nMailbox status:\n", c.Mailbox)

	for {

		cmd, err = c.Idle()
		if err != nil {
			log.Fatal(err)
		}

		for cmd.InProgress() {
			// Wait for the next response (no timeout)
			c.Recv(-1)

			// Process command data
			for _, rsp = range cmd.Data {
				log.Println("Idle response:", rsp)
			}
			cmd.Data = nil

			// Process unilateral server data
			for _, rsp = range c.Data {
				log.Println("Server data:", rsp)
			}
			c.Data = nil

			c.IdleTerm()
		}

		// Fetch the headers of the 5 most recent messages
		set, _ := imap.NewSeqSet("")
		if c.Mailbox.Messages >= 5 {
			set.AddRange(c.Mailbox.Messages-4, c.Mailbox.Messages)
		} else {
			set.Add("1:*")
		}
		cmd, _ = c.Fetch(set, "RFC822.HEADER")

		// Process responses while the command is running
		log.Println("Messages from " + time.Now().String())
		for cmd.InProgress() {
			// Wait for the next response (no timeout)
			c.Recv(-1)

			// Process command data
			for _, rsp = range cmd.Data {
				header := imap.AsBytes(rsp.MessageInfo().Attrs["RFC822.HEADER"])
				if msg, _ := mail.ReadMessage(bytes.NewReader(header)); msg != nil {
					from := msg.Header.Get("From")
					subj := msg.Header.Get("Subject")
					a, err := mail.ParseAddress(from)
					addr := a.Address
					if err != nil {
						log.Fatal(err)
					}
					log.Println("|--", addr, " ", subj)
					if "sender@example.com" == addr {
						msgid := msg.Header.Get("Message-ID")
						body := "Name: John Smith\nChoice #1: Grove Lot 99\nCall me at 408-555-1234 or Email me at jsmith@example.com"
						SendResponse(msgid, subj, addr, body)                       // send email response to originator
						SendResponse(msgid, subj, "4085551234@txt.att.net", "done") // send txt msg to self
						log.Println(msgid, " ", subj, " ", addr)
						os.Exit(0)
					}
				}
			}
			cmd.Data = nil

			// Process unilateral server data
			for _, rsp = range c.Data {
				log.Println("Server data:", rsp)
			}
			c.Data = nil
		}

		// Check command completion status
		if rsp, err := cmd.Result(imap.OK); err != nil {
			if err == imap.ErrAborted {
				log.Println("Fetch command aborted")
			} else {
				log.Println("Fetch error:", rsp.Info)
			}
		}

	} // end infinite loop

}

func SendResponse(msgid string, subj string, to string, body string) {
	header := make(map[string]string)
	header["In-Reply-To"] = msgid
	header["References"] = msgid
	header["Content-Type"] = "text/plain; charset=UTF-8"
	header["Subject"] = "Re: " + subj
	header["From"] = "John Smith<jsmith@example.com>"
	header["To"] = to

	message := ""
	for k, v := range header {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}

	message += "\r\n" + body

	auth := smtp.PlainAuth("", "jsmith@example.com", "password", "smtp.gmail.com")
	log.Println(message)
	err := smtp.SendMail("smtp.gmail.com:587", auth, "jsmith@example.com", []string{to}, []byte(message))
	if err != nil {
		log.Fatal(err)
	}
}
