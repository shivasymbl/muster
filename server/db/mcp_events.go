package db

import (
	"context"
	"regexp"

	"github.com/shivasymbl/muster/server/models"
	"github.com/shivasymbl/muster/server/services/mcp"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ListEventsForUser returns the caller's musters, including declined invites and archived musters.
// Deleted musters are excluded. The name filter is applied in this query.
func ListEventsForUser(user *models.User, name string, limit int, cursor string) ([]mcp.ListedEvent, string, error) {
	ctx := context.Background()
	ids := map[primitive.ObjectID]struct{}{}

	responseCursor, err := EventResponsesCollection.Find(ctx, bson.M{"userId": user.Id.Hex()}, options.Find().SetProjection(bson.M{"eventId": 1}))
	if err != nil {
		return nil, "", err
	}
	defer responseCursor.Close(ctx)
	for responseCursor.Next(ctx) {
		var row struct {
			EventID primitive.ObjectID `bson:"eventId"`
		}
		if err := responseCursor.Decode(&row); err != nil {
			return nil, "", err
		}
		ids[row.EventID] = struct{}{}
	}
	if err := responseCursor.Err(); err != nil {
		return nil, "", err
	}

	if user.Email != "" {
		attendeeCursor, err := AttendeesCollection.Find(ctx, bson.M{"email": user.Email}, options.Find().SetProjection(bson.M{"eventId": 1}))
		if err != nil {
			return nil, "", err
		}
		defer attendeeCursor.Close(ctx)
		for attendeeCursor.Next(ctx) {
			var row struct {
				EventID primitive.ObjectID `bson:"eventId"`
			}
			if err := attendeeCursor.Decode(&row); err != nil {
				return nil, "", err
			}
			ids[row.EventID] = struct{}{}
		}
		if err := attendeeCursor.Err(); err != nil {
			return nil, "", err
		}
	}

	union := make([]primitive.ObjectID, 0, len(ids))
	for id := range ids {
		union = append(union, id)
	}

	and := bson.A{
		bson.M{"$or": bson.A{
			bson.M{"isDeleted": bson.M{"$exists": false}},
			bson.M{"isDeleted": false},
		}},
		bson.M{"$or": bson.A{
			bson.M{"ownerId": user.Id},
			bson.M{"_id": bson.M{"$in": union}},
		}},
	}
	if cursor != "" {
		oid, err := primitive.ObjectIDFromHex(cursor)
		if err != nil {
			return nil, "", err
		}
		and = append(and, bson.M{"_id": bson.M{"$lt": oid}})
	}
	if name != "" {
		and = append(and, bson.M{"name": bson.M{"$regex": regexp.QuoteMeta(name), "$options": "i"}})
	}

	find, err := EventsCollection.Find(ctx, bson.M{"$and": and}, options.Find().
		SetSort(bson.D{{Key: "_id", Value: -1}}).
		SetLimit(int64(limit+1)).
		SetProjection(bson.M{
			"_id": 1, "shortId": 1, "name": 1, "type": 1, "isArchived": 1,
			"blindAvailabilityEnabled": 1, "isSignUpForm": 1, "ownerId": 1,
		}))
	if err != nil {
		return nil, "", err
	}
	defer find.Close(ctx)
	events := make([]models.Event, 0)
	if err := find.All(ctx, &events); err != nil {
		return nil, "", err
	}

	next := ""
	if len(events) > limit {
		next = events[limit-1].Id.Hex()
		events = events[:limit]
	}

	pageIDs := make([]primitive.ObjectID, len(events))
	for i, event := range events {
		pageIDs[i] = event.Id
	}
	flags := ResponseFlags(user.Id.Hex(), pageIDs)
	declined := map[string]*bool{}
	if user.Email != "" && len(pageIDs) > 0 {
		attendeeCursor, err := AttendeesCollection.Find(ctx, bson.M{"email": user.Email, "eventId": bson.M{"$in": pageIDs}})
		if err != nil {
			return nil, "", err
		}
		defer attendeeCursor.Close(ctx)
		for attendeeCursor.Next(ctx) {
			var row models.Attendee
			if err := attendeeCursor.Decode(&row); err != nil {
				return nil, "", err
			}
			declined[row.EventId.Hex()] = row.Declined
		}
		if err := attendeeCursor.Err(); err != nil {
			return nil, "", err
		}
	}

	out := make([]mcp.ListedEvent, 0, len(events))
	for _, event := range events {
		flag := flags[event.Id.Hex()]
		_, isAttendee := declined[event.Id.Hex()]
		out = append(out, mcp.ListedEvent{
			Event:       event,
			IsAttendee:  isAttendee,
			Declined:    declined[event.Id.Hex()],
			HasResponse: flag.Exists,
			NonEmpty:    flag.NonEmpty,
		})
	}
	return out, next, nil
}

// ResponseFlags reports whether the caller answered, without returning timestamps.
func ResponseFlags(userHex string, ids []primitive.ObjectID) map[string]mcp.ResponseFlags {
	out := map[string]mcp.ResponseFlags{}
	if len(ids) == 0 {
		return out
	}
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.M{"userId": userHex, "eventId": bson.M{"$in": ids}}}},
		bson.D{{Key: "$project", Value: bson.M{
			"eventId": 1,
			"avail":   bson.M{"$size": bson.M{"$ifNull": bson.A{"$response.availability", bson.A{}}}},
			"need":    bson.M{"$size": bson.M{"$ifNull": bson.A{"$response.ifNeeded", bson.A{}}}},
		}}},
	}
	cursor, err := EventResponsesCollection.Aggregate(context.Background(), pipeline)
	if err != nil {
		panic(err)
	}
	defer cursor.Close(context.Background())
	for cursor.Next(context.Background()) {
		var row struct {
			EventID primitive.ObjectID `bson:"eventId"`
			Avail   int                `bson:"avail"`
			Need    int                `bson:"need"`
		}
		if err := cursor.Decode(&row); err != nil {
			panic(err)
		}
		out[row.EventID.Hex()] = mcp.ResponseFlags{Exists: true, NonEmpty: row.Avail > 0 || row.Need > 0}
	}
	if err := cursor.Err(); err != nil {
		panic(err)
	}
	return out
}
