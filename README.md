# Discord Event Receiver

A library for receiving Discord (and custom) events from NATS

## Example

```go
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/chatorbot/receiver"
	"github.com/Postcord/objects"
	"github.com/sirupsen/logrus"
)

// Message represents a Discord message
type Message map[string]interface{}

func main() {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	logger.SetFormatter(&logrus.TextFormatter{
		ForceColors:      true,
		DisableTimestamp: true,
		FullTimestamp:    true,
		TimestampFormat:  "",
	})
	r, err := receiver.New(&receiver.Config{
		NatsAddr: "nats://127.0.0.1:4444",
		Token:    os.Getenv("DISCORD_TOKEN"),
		Logger:   logger,
	})
	if err != nil {
		panic("failed to create receiver")
	}
	defer r.Close()

	r.On("discord.ready", ready)
	r.On("discord.message_create", message)
	r.Start()

	// Wait here until CTRL-C or other term signal is received.
	fmt.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc
}

func ready(s disgord.Session, m *objects.Ready) error {
	fmt.Printf("Bot ready: %s\n", m.User.Username)
	return nil
}

func message(s disgord.Session, m *objects.MessageCreate) error {
	fmt.Printf("%s: %s\n", m.Message.Author.Username, m.Message.Content)
	return nil
}
```
