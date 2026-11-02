package db

import (
	"context"
	"time"

	"github.com/shivasymbl/muster/server/models"
	"github.com/shivasymbl/muster/server/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func CreateMuster(user *models.User, event models.Event, emails []string) (*models.Event, error) {
	ctx := context.Background()
	event.Id = primitive.NewObjectID()
	event.OwnerId = user.Id
	shortID := GenerateShortEventId()
	event.ShortId = &shortID
	num := 0
	event.NumResponses = &num
	event.Name = event.Name
	now := primitive.NewDateTimeFromTime(time.Now().UTC())
	if event.Type == models.GROUP {
		attendees := []models.Attendee{{
			Email: user.Email, Declined: utils.FalsePtr(), EventId: event.Id, UpdatedAt: now,
		}}
		for _, email := range emails {
			attendees = append(attendees, models.Attendee{
				Email: email, Declined: utils.FalsePtr(), EventId: event.Id, UpdatedAt: now,
			})
		}
		for _, attendee := range attendees {
			if _, err := AttendeesCollection.InsertOne(ctx, attendee); err != nil {
				return nil, err
			}
		}
	}
	if _, err := EventsCollection.InsertOne(ctx, event); err != nil {
		return nil, err
	}
	_, err := UsersCollection.UpdateOne(ctx, bson.M{"_id": user.Id}, bson.M{"$inc": bson.M{"numEventsCreated": 1}})
	return &event, err
}

func PatchMuster(eventID primitive.ObjectID, set map[string]any) error {
	_, err := EventsCollection.UpdateOne(context.Background(), bson.M{"_id": eventID}, bson.M{"$set": set})
	return err
}

func RemoveAttendeeEmail(eventID primitive.ObjectID, email, responseUserHex string) error {
	ctx := context.Background()
	if _, err := AttendeesCollection.DeleteOne(ctx, bson.M{"eventId": eventID, "email": email}); err != nil {
		return err
	}
	if responseUserHex == "" {
		return nil
	}
	res, err := EventResponsesCollection.DeleteOne(ctx, bson.M{"eventId": eventID, "userId": responseUserHex})
	if err != nil || res.DeletedCount == 0 {
		return err
	}
	_, err = EventsCollection.UpdateOne(ctx, bson.M{"_id": eventID}, bson.M{"$inc": bson.M{"numResponses": -1}})
	return err
}

func ChangesSince(eventID primitive.ObjectID, since time.Time) ([]models.EventResponse, []models.Attendee, error) {
	ctx := context.Background()
	mark := primitive.NewDateTimeFromTime(since.UTC())
	filter := bson.M{"eventId": eventID, "updatedAt": bson.M{"$gte": mark}}
	responses := []models.EventResponse{}
	cur, err := EventResponsesCollection.Find(ctx, filter)
	if err != nil {
		return nil, nil, err
	}
	if err := cur.All(ctx, &responses); err != nil {
		return nil, nil, err
	}
	attendees := []models.Attendee{}
	cur, err = AttendeesCollection.Find(ctx, filter)
	if err != nil {
		return nil, nil, err
	}
	if err := cur.All(ctx, &attendees); err != nil {
		return nil, nil, err
	}
	return responses, attendees, nil
}
