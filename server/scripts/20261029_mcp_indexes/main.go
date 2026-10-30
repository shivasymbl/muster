package main

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/shivasymbl/muster/server/db"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	disconnect := db.Init()
	defer disconnect()

	createIndex(db.EventResponsesCollection, "eventResponses_userId_eventId", false, bson.D{
		{Key: "userId", Value: 1},
		{Key: "eventId", Value: 1},
	})
	createIndex(db.AttendeesCollection, "attendees_email_declined", false, bson.D{
		{Key: "email", Value: 1},
		{Key: "declined", Value: 1},
	})
	createIndex(db.EventsCollection, "events_ownerId", false, bson.D{
		{Key: "ownerId", Value: 1},
	})
	createIndex(db.McpTokensCollection, "mcpTokens_tokenHash", true, bson.D{
		{Key: "tokenHash", Value: 1},
	})
	createIndex(db.McpTokensCollection, "mcpTokens_userId", false, bson.D{
		{Key: "userId", Value: 1},
	})
}

func createIndex(coll *mongo.Collection, name string, unique bool, keys bson.D) {
	opts := options.Index().SetName(name)
	if unique {
		opts.SetUnique(true)
	}
	_, err := coll.Indexes().CreateOne(context.Background(), mongo.IndexModel{Keys: keys, Options: opts})
	if err == nil {
		fmt.Printf("created %s\n", name)
		return
	}
	var cmd mongo.CommandError
	if errors.As(err, &cmd) && (cmd.Code == 85 || cmd.Code == 86 || cmd.Code == 68) {
		fmt.Printf("%s already exists\n", name)
		return
	}
	log.Fatalf("%s: %v", name, err)
}
