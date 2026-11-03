package db

import (
	"context"
	"os"
	"time"

	"github.com/shivasymbl/muster/server/logger"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var Client *mongo.Client
var Db *mongo.Database
var EventsCollection *mongo.Collection
var UsersCollection *mongo.Collection
var DailyUserLogCollection *mongo.Collection
var FriendRequestsCollection *mongo.Collection
var EventResponsesCollection *mongo.Collection
var AttendeesCollection *mongo.Collection
var FoldersCollection *mongo.Collection
var FolderEventsCollection *mongo.Collection
var OtpCodesCollection *mongo.Collection
var McpTokensCollection *mongo.Collection
var McpActionsCollection *mongo.Collection

func Init() func() {
	// Establish mongodb connection
	var ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mongoURI := os.Getenv("MONGODB_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://localhost"
	}

	Client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		logger.StdErr.Panicln(err)
	}

	// Define mongodb database + collections
	dbName := os.Getenv("MONGO_DB_NAME")
	if dbName == "" {
		dbName = "muster"
	}
	Db = Client.Database(dbName)
	EventsCollection = Db.Collection("events")
	UsersCollection = Db.Collection("users")
	DailyUserLogCollection = Db.Collection("dailyuserlogs")
	FriendRequestsCollection = Db.Collection("friendrequests")
	EventResponsesCollection = Db.Collection("eventResponses")
	AttendeesCollection = Db.Collection("attendees")
	FoldersCollection = Db.Collection("folders")
	FolderEventsCollection = Db.Collection("folderEvents")
	OtpCodesCollection = Db.Collection("otpCodes")
	McpTokensCollection = Db.Collection("mcpTokens")
	McpActionsCollection = Db.Collection("mcpActions")

	// Create TTL index so expired OTP docs are auto-deleted
	otpIndexModel := mongo.IndexModel{
		Keys:    bson.M{"expiresAt": 1},
		Options: options.Index().SetExpireAfterSeconds(0),
	}
	OtpCodesCollection.Indexes().CreateOne(context.Background(), otpIndexModel)

	// One stored answer per person per muster. If older duplicates exist, this
	// fails and the server still starts. The range write then reports a conflict.
	answerIndex := mongo.IndexModel{
		Keys:    bson.D{{Key: "userId", Value: 1}, {Key: "eventId", Value: 1}},
		Options: options.Index().SetUnique(true).SetName("userId_1_eventId_1"),
	}
	if _, err := EventResponsesCollection.Indexes().CreateOne(context.Background(), answerIndex); err != nil {
		logger.StdErr.Printf("answer unique index not created: %v", err)
	}

	// Return a function to close the connection
	return func() {
		Client.Disconnect(ctx)
	}
}

// MongoDB backup / restore commands

// Backup
// mongodump --uri="mongodb://localhost:27017" --db=muster

// Restore
// mongorestore --uri="mongodb://localhost:27017" --drop --db=muster ./dump
