package main

import (
	"context"
	"log"

	"github.com/shivasymbl/muster/server/db"
	"github.com/shivasymbl/muster/server/models"
	"go.mongodb.org/mongo-driver/bson"
)

func main() {
	closeConnection := db.Init()
	defer closeConnection()

	_, err := db.EventsCollection.UpdateMany(context.Background(), bson.M{"type": nil}, bson.M{
		"$set": bson.M{"type": models.SPECIFIC_DATES},
	})
	if err != nil {
		log.Fatal(err)
	}

}
