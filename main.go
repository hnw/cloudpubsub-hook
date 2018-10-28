package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"regexp"
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
	Pattern          map[string]Pattern
}

type Pattern struct {
	Command   string
	PassArgs  bool `toml:"pass_args"`
	PassStdin bool `toml:"pass_stdin"`
	ExpandEnv bool `toml:"expand_env"`
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
		log.Printf("Got message: %q\n", string(msg.Data))
		cmdOut, err := execCommand(conf, string(msg.Data))
		if err != nil {
			log.Println("Command not found")
			msg.Ack()
			return
		}
		log.Printf("Command output: %q\n", cmdOut)
		resultMsg := pubsub.Message{
			Data:       []byte(cmdOut),
			Attributes: msg.Attributes,
		}
		res := topic.Publish(cctx, &resultMsg)
		res.Get(cctx)
		msg.Ack()
	})
	if err != nil {
		log.Fatalf("Failed to receiving messages: %v", err)
		return
	}
}

func matchedKeyFromDict(dict map[string]Pattern, args []string) (string, error) {
	hitKey := ""
	for i := range args {
		key := strings.Join(args[:i+1], " ")
		if _, ok := dict[key]; ok {
			hitKey = key
		}
	}
	if hitKey == "" {
		if _, ok := dict[hitKey]; !ok {
			return "", errors.New("Input text is not in dictionary")
		}
	}
	return hitKey, nil
}

func execCommand(conf Config, cmdText string) (string, error) {
	re := regexp.MustCompile("[\t\n\v\f\r ]+")
	args := re.Split(cmdText, -1)
	matched, err := matchedKeyFromDict(conf.Pattern, args)
	if err != nil {
		return "", err
	}
	pt := conf.Pattern[matched]
	nMatched := len(re.Split(matched, -1))
	cmdInput := ""
	if pt.PassArgs {
		args = append(re.Split(pt.Command, -1), args[nMatched:]...)
	} else {
		args = re.Split(pt.Command, -1)
		if pt.PassStdin {
			arr := re.Split(cmdText, nMatched+1)
			if len(arr) == nMatched+1 {
				cmdInput = arr[nMatched]
			}
		}
	}

	if len(args) <= 0 {
		return "", errors.New("command to execute is not specified")
	}
	cmd := exec.Command(args[0], args[1:]...)
	if len(cmdInput) > 0 {
		r, w := io.Pipe()
		cmd.Stdin = r
		go func() {
			fmt.Fprint(w, cmdInput)
			w.Close()
		}()
	}
	cmdOutput, err := c.Output()
	if err != nil {
		return "", err
	}
	return string(cmdOutput), nil
}
