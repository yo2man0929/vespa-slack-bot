package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/anaskhan96/soup"
	"github.com/joho/godotenv"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

func contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}
	return false
}

func runCmd(action string, args ...string) ([]byte, error) {
	var cmd *exec.Cmd
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel() // The cancel should be deferred so resources are cleaned up
	var fname string
	fname, err := exec.LookPath(action)
	if err == nil {
		fname, err = filepath.Abs(fname)
	}
	if err != nil {
		return []byte("No " + action + " found!"), err
	}

	cmd = exec.CommandContext(ctx, fname, args...)

	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		err = ctx.Err()
	}
	//fmt.Println(string(out))
	return out, err
}

// fetch google search result
func fetch(url string) soup.Root {
	fmt.Println("Fetch Url", url)
	soup.Headers = map[string]string{
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/94.0.4606.81 Safari/537.36",
	}

	source, err := soup.Get(string(url))
	if err != nil {
		log.Fatal(err)
	}
	doc := soup.HTMLParse(source)
	return doc
}

// get one search url from a bunch of urls
func parseUrls(url string) string {
	var urls []string
	doc := fetch(url)
	for _, root := range doc.FindAll("img", "data-ils", "4") {
		src := root.Attrs()["data-src"]
		urls = append(urls, src)

	}
	time.Sleep(2 * time.Second)
	//ch <- true
	if len(urls) == 0 {
		return "Please use other words to search"
	}
	return urls[rand.Intn(len(urls))]
}

// handleEventMessage will take an event and handle it properly based on the type of event
func handleEventMessage(event slackevents.EventsAPIEvent, client *slack.Client) error {
	switch event.Type {
	// First we check if this is an CallbackEvent
	case slackevents.CallbackEvent:

		innerEvent := event.InnerEvent
		// Yet Another Type switch on the actual Data to see if its an AppMentionEvent
		switch ev := innerEvent.Data.(type) {
		case *slackevents.AppMentionEvent:
			// The application has been mentioned since this Event is a Mention event
			err := handleAppMentionEvent(ev, client)
			if err != nil {
				return err
			}
		}
	default:
		return errors.New("unsupported event type")
	}
	return nil
}

// handleAppMentionEvent is used to take care of the AppMentionEvent when the bot is mentioned
func handleAppMentionEvent(event *slackevents.AppMentionEvent, client *slack.Client) error {

	// Grab the user name based on the ID of the one who mentioned the bot
	user, err := client.GetUserInfo(event.User)

	if err != nil {
		return err
	}
	// Whitelist devops members to access the bot
	userSlice := []string{"joe_yang", "emin_hsieh", "egan_wu", "yo2man0929", "tony_luo"}
	text := strings.ToLower(event.Text)

	// Create the attachment and assigned based on the message
	attachment := slack.Attachment{}

	if strings.Contains(text, "help") {
		// Greet the user
		info := `
		EX:
			img test
			restart_gm (待側)
			shell /home/egame/restart-gameservice.sh (待側)
			shell /home/egame/test.sh
		`
		attachment.Text = info
		attachment.Color = "#4af030"

	} else if strings.Contains(text, "img") {
		searchText := strings.Split(text, " ")[2:]
		url := "http://www.google.com/images?q=" + strings.Join(searchText, "")
		attachment.Text = parseUrls(url)
		attachment.Pretext = "MEME image search"
		attachment.Color = "#4af030"

	} else if strings.Contains(text, "restart_gm") {
		if contains(userSlice, user.Name) {
			godotenv.Load(".env")

			juser := os.Getenv("JENKINS_USER")
			jtoken := os.Getenv("JENKINS_TOKEN")
			auth_args := fmt.Sprintf("-u %s:%s", juser, jtoken)

			out, err := runCmd("/usr/bin/curl",
				"https://jenkins.paradise-soft.com.tw/job/Egame/job/UAT/job/restart-gamemaster/buildWithParameters?token=restart-gamemaster",
				auth_args, ">", "/dev/null")
			if err == context.DeadlineExceeded {
				attachment.Pretext = "Timeout!"
				attachment.Color = "#4af030"
			}
			attachment.Text = string(out)
			attachment.Pretext = "Use bash Command"
			attachment.Color = "#4af030"
		} else {
			attachment.Pretext = "No Permission!"
			attachment.Color = "#4af030"
		}
	} else if strings.Contains(text, "shell") {
		if contains(userSlice, user.Name) {
			cutCmd := strings.Join(strings.Split(text, " ")[2:], " ")
			out, err := runCmd("bash", "-c", cutCmd)
			if err == context.DeadlineExceeded {
				attachment.Pretext = "Timeout!"
				attachment.Color = "#4af030"
			}
			attachment.Text = string(out)
			attachment.Pretext = "Use bash Command"
			attachment.Color = "#4af030"
		} else {
			attachment.Pretext = "No Permission!"
			attachment.Color = "#4af030"
		}
	} else {
		// Send a message to the user
		info := `
			Please use this command to get more information!
			@vespa help
		`
		attachment.Text = fmt.Sprintf("Hi %s %s", user.Name, info)
		attachment.Pretext = "Only commands below are supported!"
		attachment.Color = "#3d3d3d"
	}
	// Send the message to the channel
	// The Channel is available in the event message
	_, _, err = client.PostMessage(event.Channel, slack.MsgOptionAttachments(attachment))
	if err != nil {
		return fmt.Errorf("failed to post message: %w", err)
	}
	return nil
}

// handleSlashCommand will take a slash command and route to the appropriate function
func handleSlashCommand(command slack.SlashCommand, client *slack.Client) (interface{}, error) {
	// We need to switch depending on the command
	switch command.Command {
	case "/hello":
		// This was a hello command, so pass it along to the proper function
		return nil, handleHelloCommand(command, client)
	case "/was-this-article-useful":
		return handleIsArticleGood(command, client)
	}

	return nil, nil
}

// handleHelloCommand will take care of /hello submissions
func handleHelloCommand(command slack.SlashCommand, client *slack.Client) error {
	// The Input is found in the text field so
	// Create the attachment and assigned based on the message
	attachment := slack.Attachment{}
	// Add Some default context like user who mentioned the bot
	attachment.Fields = []slack.AttachmentField{
		{
			Title: "Date",
			Value: time.Now().String(),
		}, {
			Title: "Initializer",
			Value: command.UserName,
		},
	}

	// Greet the user
	attachment.Text = fmt.Sprintf("Hello %s", command.Text)
	attachment.Color = "#4af030"

	// Send the message to the channel
	// The Channel is available in the command.ChannelID
	_, _, err := client.PostMessage(command.ChannelID, slack.MsgOptionAttachments(attachment))
	if err != nil {
		return fmt.Errorf("failed to post message: %w", err)
	}
	return nil
}

// handleIsArticleGood will trigger a Yes or No question to the initializer
func handleIsArticleGood(command slack.SlashCommand, client *slack.Client) (interface{}, error) {
	// Create the attachment and assigned based on the message
	attachment := slack.Attachment{}

	// Create the checkbox element
	checkbox := slack.NewCheckboxGroupsBlockElement("answer",
		slack.NewOptionBlockObject("yes", &slack.TextBlockObject{Text: "Yes", Type: slack.MarkdownType}, &slack.TextBlockObject{Text: "Did you Enjoy it?", Type: slack.MarkdownType}),
		slack.NewOptionBlockObject("no", &slack.TextBlockObject{Text: "No", Type: slack.MarkdownType}, &slack.TextBlockObject{Text: "Did you Dislike it?", Type: slack.MarkdownType}),
	)
	// Create the Accessory that will be included in the Block and add the checkbox to it
	accessory := slack.NewAccessory(checkbox)
	// Add Blocks to the attachment
	attachment.Blocks = slack.Blocks{
		BlockSet: []slack.Block{
			// Create a new section block element and add some text and the accessory to it
			slack.NewSectionBlock(
				&slack.TextBlockObject{
					Type: slack.MarkdownType,
					Text: "Did you think this article was helpful?",
				},
				nil,
				accessory,
			),
		},
	}

	attachment.Text = "Rate the tutorial"
	attachment.Color = "#4af030"
	return attachment, nil
}

func main() {

	// Load Env variables from .dot file
	godotenv.Load(".env")

	token := os.Getenv("SLACK_AUTH_TOKEN")
	appToken := os.Getenv("SLACK_APP_TOKEN")
	// Create a new client to slack by giving token
	// Set debug to true while developing
	// Also add a ApplicationToken option to the client
	client := slack.New(token, slack.OptionDebug(true), slack.OptionAppLevelToken(appToken))
	// go-slack comes with a SocketMode package that we need to use that accepts a Slack client and outputs a Socket mode client instead
	socketClient := socketmode.New(
		client,
		socketmode.OptionDebug(true),
		// Option to set a custom logger
		socketmode.OptionLog(log.New(os.Stdout, "socketmode: ", log.Lshortfile|log.LstdFlags)),
	)

	// Create a context that can be used to cancel goroutine
	ctx, cancel := context.WithCancel(context.Background())
	// Make this cancel called properly in a real program , graceful shutdown etc
	defer cancel()

	go func(ctx context.Context, client *slack.Client, socketClient *socketmode.Client) {
		// Create a for loop that selects either the context cancellation or the events incomming
		for {
			select {
			// inscase context cancel is called exit the goroutine
			case <-ctx.Done():
				log.Println("Shutting down socketmode listener")
				return
			case event := <-socketClient.Events:
				// We have a new Events, let's type switch the event
				// Add more use cases here if you want to listen to other events.
				switch event.Type {
				// handle EventAPI events
				case socketmode.EventTypeEventsAPI:
					// The Event sent on the channel is not the same as the EventAPI events so we need to type cast it
					eventsAPIEvent, ok := event.Data.(slackevents.EventsAPIEvent)
					if !ok {
						log.Printf("Could not type cast the event to the EventsAPIEvent: %v\n", event)
						continue
					}
					// We need to send an Acknowledge to the slack server
					socketClient.Ack(*event.Request)
					// Now we have an Events API event, but this event type can in turn be many types, so we actually need another type switch
					err := handleEventMessage(eventsAPIEvent, client)
					if err != nil {
						// Replace with actual err handeling
						log.Fatal(err)
					}
				// Handle Slash Commands
				case socketmode.EventTypeSlashCommand:
					// Just like before, type cast to the correct event type, this time a SlashEvent
					command, ok := event.Data.(slack.SlashCommand)
					if !ok {
						log.Printf("Could not type cast the message to a SlashCommand: %v\n", command)
						continue
					}
					// Dont forget to acknowledge the request
					socketClient.Ack(*event.Request)
					// handleSlashCommand will take care of the command
					_, err := handleSlashCommand(command, client)
					if err != nil {
						log.Fatal(err)
					}

				}
			}

		}
	}(ctx, client, socketClient)

	socketClient.Run()
}
