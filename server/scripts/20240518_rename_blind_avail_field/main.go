package main

import (
	"context"
	"fmt"

	"github.com/shivasymbl/muster/server/db"
	"go.mongodb.org/mongo-driver/bson"
)

func main() {
	closeConnection := db.Init()
	defer closeConnection()

	_, err := db.EventsCollection.UpdateMany(context.Background(), bson.M{}, bson.M{
		"$rename": bson.M{"blindavailabilityenabled": "blindAvailabilityEnabled"},
	})
	if err != nil {
		panic(err)
	}

	fmt.Println("Done!!")
}
