package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

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

	slackClient := slack.New(os.Getenv("SLACK_API_TOKEN"))
	airtableClient := airtable.NewClient(os.Getenv("AIRTABLE_API_KEY"))

	table := airtableClient.GetTable(os.Getenv("AIRTABLE_BASE_ID"), os.Getenv("AIRTABLE_TABLE_ID"))

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
