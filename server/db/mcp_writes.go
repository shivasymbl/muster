package db

import (
	"context"
	"time"

	"github.com/shivasymbl/muster/server/models"
	"github.com/shivasymbl/muster/server/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
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
