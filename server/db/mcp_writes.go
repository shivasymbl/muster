package db

import (
	"context"
	"time"

	"github.com/shivasymbl/muster/server/models"
	"github.com/shivasymbl/muster/server/services/mcp"
	"github.com/shivasymbl/muster/server/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func ReplaceAvailability(user *models.User, eventID primitive.ObjectID, availability, ifNeeded []primitive.DateTime) error {
	ctx := context.Background()
	var existing models.EventResponse
	err := EventResponsesCollection.FindOne(ctx, bson.M{"userId": user.Id.Hex(), "eventId": eventID}).Decode(&existing)
	response := models.Response{
		UserId:       user.Id,
		Availability: availability,
		IfNeeded:     ifNeeded,
	}
	if err == mongo.ErrNoDocuments {
		_, err = EventResponsesCollection.InsertOne(ctx, models.EventResponse{
			UserId:    user.Id.Hex(),
			EventId:   eventID,
			Response:  &response,
			UpdatedAt: primitive.NewDateTimeFromTime(time.Now().UTC()),
		})
		if err != nil {
			return err
		}
		_, err = EventsCollection.UpdateOne(ctx, bson.M{"_id": eventID}, bson.M{"$inc": bson.M{"numResponses": 1}})
	} else if err != nil {
		return err
	} else {
		if existing.Response != nil {
			response.ManualAvailability = existing.Response.ManualAvailability
			response.UseCalendarAvailability = existing.Response.UseCalendarAvailability
			response.EnabledCalendars = existing.Response.EnabledCalendars
			response.CalendarOptions = existing.Response.CalendarOptions
		}
		_, err = EventResponsesCollection.UpdateOne(ctx, bson.M{"_id": existing.Id}, bson.M{"$set": bson.M{"response": &response, "updatedAt": primitive.NewDateTimeFromTime(time.Now().UTC())}})
	}
	if err != nil {
		return err
	}
	if user.Email == "" {
		return nil
	}
	_, err = AttendeesCollection.UpdateOne(ctx, bson.M{"email": user.Email, "eventId": eventID}, bson.M{"$set": bson.M{"declined": false}})
	return err
}

func DeclineInvite(user *models.User, eventID primitive.ObjectID) error {
	res, err := AttendeesCollection.UpdateOne(context.Background(), bson.M{
		"email":   user.Email,
		"eventId": eventID,
	}, bson.M{"$set": bson.M{"declined": true, "updatedAt": primitive.NewDateTimeFromTime(time.Now().UTC())}})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func AddAttendeeEmails(eventID primitive.ObjectID, emails []string) ([]string, error) {
	ctx := context.Background()
	added := make([]string, 0)
	for _, email := range emails {
		n, err := AttendeesCollection.CountDocuments(ctx, bson.M{"email": email, "eventId": eventID})
		if err != nil {
			return nil, err
		}
		if n > 0 {
			continue
		}
		_, err = AttendeesCollection.InsertOne(ctx, models.Attendee{
			Email:     email,
			Declined:  utils.FalsePtr(),
			UpdatedAt: primitive.NewDateTimeFromTime(time.Now().UTC()),
			EventId:   eventID,
		})
		if err != nil {
			return nil, err
		}
		added = append(added, email)
	}
	return added, nil
}

// arrayEquals matches a stored date array against the value that was read.
// An empty list matches a missing, null or empty array.
func arrayEquals(list []primitive.DateTime) any {
	if len(list) == 0 {
		return bson.M{"$in": bson.A{nil, bson.A{}}}
	}
	return list
}

// ReplaceAvailabilityIf is a compare-and-set on the single eventResponses document.
// It writes only if that document still has the availability, ifNeeded and
// updatedAt that snap recorded, and returns false when it does not (or when a
// first answer was created meanwhile). Other response fields are preserved.
// This is not a cross-document transaction: numResponses and the attendee
// declined flag are separate writes after the conditional one.
func ReplaceAvailabilityIf(user *models.User, eventID primitive.ObjectID, snap mcp.AnswerSnapshot, availability, ifNeeded []primitive.DateTime) (bool, error) {
	ctx := context.Background()
	now := primitive.NewDateTimeFromTime(time.Now().UTC())
	if availability == nil {
		availability = []primitive.DateTime{}
	}
	if ifNeeded == nil {
		ifNeeded = []primitive.DateTime{}
	}
	if snap.Exists {
		var updated any = bson.M{"$exists": false}
		if snap.UpdatedAt != 0 {
			updated = snap.UpdatedAt
		}
		res, err := EventResponsesCollection.UpdateOne(ctx, bson.M{
			"userId":                user.Id.Hex(),
			"eventId":               eventID,
			"updatedAt":             updated,
			"response.availability": arrayEquals(snap.Availability),
			"response.ifNeeded":     arrayEquals(snap.IfNeeded),
		}, bson.M{"$set": bson.M{
			"response.availability": availability,
			"response.ifNeeded":     ifNeeded,
			"updatedAt":             now,
		}})
		if err != nil {
			return false, err
		}
		if res.MatchedCount == 0 {
			return false, nil
		}
	} else {
		res, err := EventResponsesCollection.UpdateOne(ctx, bson.M{"userId": user.Id.Hex(), "eventId": eventID},
			bson.M{"$setOnInsert": bson.M{
				"response":  models.Response{UserId: user.Id, Availability: availability, IfNeeded: ifNeeded},
				"updatedAt": now,
			}}, options.Update().SetUpsert(true))
		if err != nil {
			return false, err
		}
		if res.UpsertedCount == 0 {
			return false, nil
		}
		// No unique (userId,eventId) index exists, so two first-answer upserts can both
		// insert. Each inserter counts afterwards and removes its own row if another exists:
		// at most one survives, and a double loss reports a conflict to both (safe to retry).
		n, err := EventResponsesCollection.CountDocuments(ctx, bson.M{"userId": user.Id.Hex(), "eventId": eventID})
		if err != nil {
			return false, err
		}
		if n > 1 {
			_, err = EventResponsesCollection.DeleteOne(ctx, bson.M{"_id": res.UpsertedID})
			return false, err
		}
		if _, err = EventsCollection.UpdateOne(ctx, bson.M{"_id": eventID}, bson.M{"$inc": bson.M{"numResponses": 1}}); err != nil {
			return true, err
		}
	}
	if user.Email == "" {
		return true, nil
	}
	_, err := AttendeesCollection.UpdateOne(ctx, bson.M{"email": user.Email, "eventId": eventID}, bson.M{"$set": bson.M{"declined": false}})
	return true, err
}
