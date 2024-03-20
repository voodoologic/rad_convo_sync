package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	"github.com/mehanizm/airtable"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

func chunkRecords(records []*airtable.Record, chunkSize int) (chunks [][]*airtable.Record) {
	for i := 0; i < len(records); i += chunkSize {
		end := i + chunkSize

		if end > len(records) {
			end = len(records)
		}

		chunks = append(chunks, records[i:end])
	}

	return chunks
}

func syncSlackMessageToAirtable(airtableClient *airtable.Client, msg slackevents.MessageAction) {

}

func sync_historical(slackClient *slack.Client, airtable_client *airtable.Client, table *airtable.Table) {
	blankParams := &slack.GetConversationsParameters{}

	channels, _, _ := slackClient.GetConversations(blankParams)
	for _, channel := range channels {
		fmt.Printf("ID: %s, Name: %s\n", channel.ID, channel.Name)
		lol := &slack.GetConversationHistoryParameters{
			ChannelID: channel.ID,
			Inclusive: true,
		}
		response, _ := slackClient.GetConversationHistory(lol)
		// fmt.Println(response)
		if channel.Name != "mcdaniel-sewer-and-drain" {
			continue
		}
		records := airtable.Records{}
		for _, slackMessage := range response.Messages {
			msgID := slackMessage.Msg.ClientMsgID
			existigRecords, err := table.GetRecords().WithFilterFormula(fmt.Sprintf("{Message ID} = '%s'", msgID)).Do()
			if msgID == "" {
				fmt.Println("Don't want a blank record")
				continue
			}
			if len(existigRecords.Records) > 1 {
				fmt.Println("already have that record")
				continue
			}
			user, err := slackClient.GetUserInfo(slackMessage.User)
			if err != nil {

				fmt.Printf("Could not retrieve user: %s\n", err)
			} else {
				err = nil
			}
			floatTime, err := strconv.ParseFloat(slackMessage.Timestamp, 64)
			if err != nil {
				panic(err)
			}
			intTime := int64(floatTime)
			tm := time.Unix(intTime, 0)
			pst, err := time.LoadLocation("America/Los_Angeles")
			tm.In(pst)
			airTableRow := airtable.Record{
				Fields: map[string]interface{}{
					"Message ID": slackMessage.Msg.ClientMsgID,
					"Username":   user.Name,
					"Message":    slackMessage.Text,
					"Channel":    channel.Name,
					"Timestamp":  tm.Format("January 2, 2006 - 03:04 PM"),
				},
			}
			records.Records = append(records.Records, &airTableRow)
		}
		var chunkSize = 10
		allRecordsChunks := chunkRecords(records.Records, chunkSize)
		for _, recordsChunk := range allRecordsChunks {
			records.Records = recordsChunk
			res, err := table.AddRecords(&records)
			if err != nil {
				fmt.Printf("Could not create record(s): %s\n", err)
				return
			}
			fmt.Printf("Created record(s), received %+v\n", *res)
		}
	}
}

func main() {
	_ = godotenv.Load(".envrc")

	slackClient := slack.New(
		os.Getenv("SLACK_BOT_TOKEN"),
		slack.OptionDebug(true),
		slack.OptionLog(log.New(os.Stdout, "api: ", log.Lshortfile|log.LstdFlags)),
		slack.OptionAppLevelToken(os.Getenv("SLACK_APP_TOKEN")),
	)
	socketClient := socketmode.New(
		slackClient,
		socketmode.OptionDebug(true),
		socketmode.OptionLog(log.New(os.Stdout, "socketmode: ", log.Lshortfile|log.LstdFlags)),
	)
	airtableClient := airtable.NewClient(os.Getenv("AIRTABLE_API_KEY"))

	table := airtableClient.GetTable(os.Getenv("AIRTABLE_BASE_ID"), os.Getenv("AIRTABLE_TABLE_ID"))
	// sync_historical(slackClient, airtableClient, table)

	records, err := table.GetRecords().Do()
	if err != nil {
		panic(err)
	}
	for _, record := range records.Records {
		fmt.Print(record.Fields)
	}
	fmt.Println(records)
	sync_conversations(socketClient, airtableClient)
	fmt.Println("ready")
}

func sync_conversations(socketClient *socketmode.Client, airtableClient *airtable.Client) {

	blankParams := &slack.GetConversationsParameters{}

	channels, _, _ := socketClient.GetConversations(blankParams)
	fmt.Println(channels)

	go func() {
		for evt := range socketClient.Events {
			switch evt.Type {
			case socketmode.EventTypeConnecting:
				fmt.Println("Connecting to Slack with Socket Mode...")
			case socketmode.EventTypeConnectionError:
				fmt.Println("Connection failed. Retrying later...")
			case socketmode.EventTypeConnected:
				fmt.Println("Connected to Slack with Socket Mode.")
			case socketmode.EventTypeEventsAPI:
				eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
				if !ok {
					fmt.Printf("Ignored %+v\n", evt)

					continue
				}

				fmt.Printf("Event received: %+v\n", eventsAPIEvent)

				socketClient.Ack(*evt.Request)

				switch eventsAPIEvent.Type {
				case slackevents.CallbackEvent:
					innerEvent := eventsAPIEvent.InnerEvent
					switch ev := innerEvent.Data.(type) {
					case *slackevents.AppMentionEvent:
						_, _, err := socketClient.PostMessage(ev.Channel, slack.MsgOptionText("Yes, hello.", false))
						if err != nil {
							fmt.Printf("failed posting message: %v", err)
						}
					case *slackevents.MemberJoinedChannelEvent:
						fmt.Printf("user %q joined to channel %q", ev.User, ev.Channel)
					}
				default:
					socketClient.Debugf("unsupported Events API event received")
				}
			case socketmode.EventTypeInteractive:
				callback, ok := evt.Data.(slack.InteractionCallback)
				if !ok {
					fmt.Printf("Ignored %+v\n", evt)

					continue
				}

				fmt.Printf("Interaction received: %+v\n", callback)

				var payload interface{}

				switch callback.Type {
				case slack.InteractionTypeBlockActions:
					// See https://api.slack.com/apis/connections/socket-implement#button

					socketClient.Debugf("button clicked!")
				case slack.InteractionTypeShortcut:
				case slack.InteractionTypeViewSubmission:
					// See https://api.slack.com/apis/connections/socket-implement#modal
				case slack.InteractionTypeDialogSubmission:
				default:

				}

				socketClient.Ack(*evt.Request, payload)
			case socketmode.EventTypeSlashCommand:
				cmd, ok := evt.Data.(slack.SlashCommand)
				if !ok {
					fmt.Printf("Ignored %+v\n", evt)

					continue
				}

				socketClient.Debugf("Slash command received: %+v", cmd)

				payload := map[string]interface{}{
					"blocks": []slack.Block{
						slack.NewSectionBlock(
							&slack.TextBlockObject{
								Type: slack.MarkdownType,
								Text: "foo",
							},
							nil,
							slack.NewAccessory(
								slack.NewButtonBlockElement(
									"",
									"somevalue",
									&slack.TextBlockObject{
										Type: slack.PlainTextType,
										Text: "bar",
									},
								),
							),
						),
					},
				}

				socketClient.Ack(*evt.Request, payload)
			default:
				fmt.Fprintf(os.Stderr, "Unexpected event type received: %s\n", evt.Type)
			}
		}
	}()
	socketClient.Run()
}
