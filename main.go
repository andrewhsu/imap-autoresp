/*
Copyright (c) 2015, Andrew Hsu <andrewhsu@acm.org>
All rights reserved.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:

1. Redistributions of source code must retain the above copyright notice, this
   list of conditions and the following disclaimer.
2. Redistributions in binary form must reproduce the above copyright notice,
   this list of conditions and the following disclaimer in the documentation
   and/or other materials provided with the distribution.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND
ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT OWNER OR CONTRIBUTORS BE LIABLE FOR
ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES
(INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES;
LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND
ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

The views and conclusions contained in the software and documentation are those
of the authors and should not be interpreted as representing official policies,
either expressed or implied, of the FreeBSD Project.
*/
package main

import "fmt"
import "log"
import "time"
import "net/mail"
import "net/smtp"
import "crypto/tls"
import "bytes"
import "os"
import "github.com/mxk/go-imap/imap"

func main() {

var (
  c   *imap.Client
  cmd *imap.Command
  rsp *imap.Response
  cfg *tls.Config
)

c, err := imap.DialTLS("imap.gmail.com:993",cfg)
if err != nil {
  log.Fatal(err)
}

defer c.Logout(5 * time.Second)

// Print server greeting (first response in the unilateral server data queue)
fmt.Println("Server says hello:", c.Data[0].Info)
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
  fmt.Println("Server data:", rsp)
}
c.Data = nil

// Open a mailbox (synchronous command - no need for imap.Wait)
_, err = c.Select("INBOX", true)
fmt.Print("\nMailbox status:\n", c.Mailbox)

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
      fmt.Println("Idle response:", rsp)
    }
    cmd.Data = nil

    // Process unilateral server data
    for _, rsp = range c.Data {
      fmt.Println("Server data:", rsp)
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
  fmt.Println("\nMessages from " + time.Now().String())
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
        fmt.Println("|--", addr, " ", subj)
        if "sender@example.com" == addr {
          msgid := msg.Header.Get("Message-ID")
          body := "Name: John Smith\nChoice #1: Grove Lot 99\nCall me at 408-555-1234 or Email me at jsmith@example.com"
          SendResponse(msgid, subj, addr, body) // send email response to originator
          SendResponse(msgid, subj, "4085551234@txt.att.net", "done") // send txt msg to self
          fmt.Println(msgid, " ", subj, " ", addr)
          os.Exit(0);
        }
      }
    }
    cmd.Data = nil

    // Process unilateral server data
    for _, rsp = range c.Data {
      fmt.Println("Server data:", rsp)
    }
    c.Data = nil
  }

  // Check command completion status
  if rsp, err := cmd.Result(imap.OK); err != nil {
    if err == imap.ErrAborted {
      fmt.Println("Fetch command aborted")
    } else {
      fmt.Println("Fetch error:", rsp.Info)
    }
  }

  } // end infinite loop

}

func Max(a int, b int) int {
  if a > b {
    return a
  }
  return b
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

  auth := smtp.PlainAuth("", "jsmith@example.com","password","smtp.gmail.com")
  fmt.Println(message)
  err := smtp.SendMail("smtp.gmail.com:587",auth,"jsmith@example.com", []string{to}, []byte(message))
  if err != nil {
    log.Fatal(err)
  }
}
