package main

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/mehanizm/airtable"
	"github.com/slack-go/slack"
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

func main() {
	_ = godotenv.Load(".envrc")
	// Create a new Slack Client
	slackClient := slack.New(os.Getenv("SLACK_API_TOKEN"))

	// Create a new Airtable Client
	airtableClient := airtable.NewClient(os.Getenv("AIRTABLE_API_KEY"))

	// table := airtableClient.Table(os.Getenv("AIRTABLE_TABLE_NAME"))

	// schema, err := airtableClient.GetBaseSchema("your_database_ID").Do()
	bases, err := airtableClient.GetBases().WithOffset("").Do()
	for _, base := range bases.Bases {
		fmt.Println(base)
	}
	// Pull all Slack channel messages
	// historyParams := slack.NewHistoryParameters()
	table := airtableClient.GetTable(os.Getenv("AIRTABLE_BASE_ID"), os.Getenv("AIRTABLE_TABLE_ID"))
	// slackClient.GetConversationHistory(params *slack.GetConversationHistoryParameters)

	type SlackMessage struct {
		MessageID   string
		Username    string
		Text        string
		ChannelID   string
		ChannelName string
		Timestamp   string
		UserID      string
		Reactions   string
		Attachments string
		ThreadTS    string
	}
	records, err := table.GetRecords().Do()
	if err != nil {
		panic(err)
	}
	for _, record := range records.Records {
		fmt.Print(record.Fields)
	}
	fmt.Println(records)
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
			airTableRow := airtable.Record{
				Fields: map[string]interface{}{
					"Message ID": slackMessage.Msg.ClientMsgID,
					"Username":   slackMessage.Username,
					"Message":    slackMessage.Text,
					"Channel":    channel.Name,
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
