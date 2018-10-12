package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"

	"cloud.google.com/go/pubsub"
	"github.com/BurntSushi/toml"
	"golang.org/x/net/context"
)

type Config struct {
	ProjectID        string `toml:"project_id"`
	SubscriptionName string `toml:"subscription_name"`
	TopicName        string `toml:"topic_name"`
	Credentials      string
	Debug            bool
}

func main() {
	var conf Config
	if _, err := toml.DecodeFile("config/config.toml", &conf); err != nil {
		log.Fatalf("Failed to decode TOML: %v", err)
		return
	}
	os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "config/"+conf.Credentials)

	ctx := context.Background()

	// Creates a client.
	client, err := pubsub.NewClient(ctx, conf.ProjectID)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	var mu sync.Mutex
	received := 0

	sub := client.Subscription(conf.SubscriptionName)
	topic := client.Topic(conf.TopicName)

	cctx, cancel := context.WithCancel(ctx)
	err = sub.Receive(cctx, func(ctx context.Context, msg *pubsub.Message) {
		mu.Lock()
		defer mu.Unlock()
		received++
		if received >= 10 {
			cancel()
			msg.Nack()
			return
		}
		fmt.Printf("Got message: %q\n", string(msg.Data))
		cmdOut, err := execCommand(string(msg.Data))
		if err == nil {
			resultMsg := pubsub.Message{
				Data:       []byte(cmdOut),
				Attributes: msg.Attributes,
			}
			res := topic.Publish(cctx, &resultMsg)
			res.Get(cctx)
		}
		msg.Ack()
	})
	if err != nil {
		log.Fatalf("Failed to receiving messages: %v", err)
		return
	}
}

func execCommand(cmd string) (string, error) {
	args := strings.Split(cmd, " ")
	if len(args) >= 2 {
		out, err := exec.Command(args[1], args[2:]...).Output()
		if err != nil {
			return "", err
		}
		return string(out), nil
	}
	return "", nil
}
