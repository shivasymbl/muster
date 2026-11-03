package db

import (
	"context"
	"time"

	"github.com/shivasymbl/muster/server/services/mcp"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type mongoActions struct{}

func (mongoActions) Create(rec *mcp.ActionRecord) error {
	_, err := McpActionsCollection.InsertOne(context.Background(), rec)
	return err
}

func (mongoActions) Claim(userID, id primitive.ObjectID, now time.Time) (*mcp.ActionRecord, error) {
	var rec mcp.ActionRecord
	err := McpActionsCollection.FindOneAndUpdate(context.Background(), bson.M{
		"_id":       id,
		"userId":    userID,
		"status":    "prepared",
		"expiresAt": bson.M{"$gt": now},
	}, bson.M{"$set": bson.M{"status": "claimed"}}, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&rec)
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

func (mongoActions) Get(userID, id primitive.ObjectID) (*mcp.ActionRecord, error) {
	var rec mcp.ActionRecord
	err := McpActionsCollection.FindOne(context.Background(), bson.M{"_id": id, "userId": userID}).Decode(&rec)
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

func (mongoActions) Finish(userID, id primitive.ObjectID, status string, result map[string]any, errCode, errMessage string) error {
	_, err := McpActionsCollection.UpdateOne(context.Background(), bson.M{"_id": id, "userId": userID}, bson.M{"$set": bson.M{
		"status": status, "result": result, "errCode": errCode, "errMessage": errMessage,
	}})
	return err
}
