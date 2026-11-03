package db_test

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/shivasymbl/muster/server/db"
	"github.com/shivasymbl/muster/server/models"
	"github.com/shivasymbl/muster/server/services/mcp"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// isolatedMongo starts a throwaway mongod (MUSTER_TEST_MONGOD=/path/to/mongod) on a free
// port with a temp dbpath, or uses MUSTER_TEST_MONGO_URI for a server you
// own. Neither set: the test is skipped, and that is NOT a pass for the CAS gate.
func isolatedMongo(t *testing.T) *mongo.Database {
	t.Helper()
	uri := os.Getenv("MUSTER_TEST_MONGO_URI")
	if bin := os.Getenv("MUSTER_TEST_MONGOD"); uri == "" && bin != "" {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		port := l.Addr().(*net.TCPAddr).Port
		l.Close()
		cmd := exec.Command(bin, "--dbpath", t.TempDir(), "--port", fmt.Sprint(port), "--bind_ip", "127.0.0.1", "--nounixsocket")
		if err := cmd.Start(); err != nil {
			t.Fatalf("start mongod: %v", err)
		}
		t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
		uri = fmt.Sprintf("mongodb://127.0.0.1:%d", port)
	}
	if uri == "" {
		t.Skip("set MUSTER_TEST_MONGOD or MUSTER_TEST_MONGO_URI; the CAS gate is NOT proven without it")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatal(err)
	}
	for {
		if err := client.Ping(ctx, nil); err == nil {
			break
		} else if ctx.Err() != nil {
			t.Fatalf("mongo not reachable: %v", err)
		}
		time.Sleep(200 * time.Millisecond)
	}
	d := client.Database(fmt.Sprintf("muster_cas_%d", time.Now().UnixNano()))
	t.Cleanup(func() { d.Drop(context.Background()); client.Disconnect(context.Background()) })
	db.EventsCollection = d.Collection("events")
	db.EventResponsesCollection = d.Collection("eventResponses")
	db.AttendeesCollection = d.Collection("attendees")
	return d
}

func slots(hours ...int) []primitive.DateTime {
	out := []primitive.DateTime{}
	for _, h := range hours {
		out = append(out, primitive.NewDateTimeFromTime(time.Date(2018, 6, 18, h, 0, 0, 0, time.UTC)))
	}
	return out
}

func readAnswer(t *testing.T, eventID primitive.ObjectID, user *models.User) (mcp.AnswerSnapshot, models.EventResponse) {
	t.Helper()
	var row models.EventResponse
	err := db.EventResponsesCollection.FindOne(context.Background(), bson.M{"userId": user.Id.Hex(), "eventId": eventID}).Decode(&row)
	if err == mongo.ErrNoDocuments {
		return mcp.AnswerSnapshot{}, row
	}
	if err != nil {
		t.Fatal(err)
	}
	return mcp.AnswerSnapshot{Exists: true, UpdatedAt: row.UpdatedAt, Availability: row.Response.Availability, IfNeeded: row.Response.IfNeeded}, row
}

// TestMCPAvailabilityCAS proves the conditional update against a real, isolated mongod.
// It does not claim cross-document atomicity: numResponses and attendee flags are separate writes.
func TestMCPAvailabilityCAS(t *testing.T) {
	isolatedMongo(t)
	ctx := context.Background()

	t.Run("first_answer_then_stale_snapshot_loses", func(t *testing.T) {
		eventID, user := primitive.NewObjectID(), &models.User{Id: primitive.NewObjectID()}
		db.EventsCollection.InsertOne(ctx, bson.M{"_id": eventID, "numResponses": 0})
		ok, err := db.ReplaceAvailabilityIf(user, eventID, mcp.AnswerSnapshot{}, slots(9), slots())
		if err != nil || !ok {
			t.Fatalf("first insert ok=%v err=%v", ok, err)
		}
		stale, _ := readAnswer(t, eventID, user)
		if ok, _ := db.ReplaceAvailabilityIf(user, eventID, stale, slots(10), slots()); !ok {
			t.Fatal("fresh snapshot must win")
		}
		ok, err = db.ReplaceAvailabilityIf(user, eventID, stale, slots(11), slots())
		if err != nil || ok {
			t.Fatalf("stale snapshot ok=%v err=%v", ok, err)
		}
		got, _ := readAnswer(t, eventID, user)
		if len(got.Availability) != 1 || got.Availability[0] != slots(10)[0] {
			t.Fatalf("stale write landed: %v", got.Availability)
		}
		// A second "first answer" with no snapshot must lose to the existing row.
		if ok, _ := db.ReplaceAvailabilityIf(user, eventID, mcp.AnswerSnapshot{}, slots(12), slots()); ok {
			t.Fatal("create-over-existing succeeded")
		}
		var ev struct {
			NumResponses int `bson:"numResponses"`
		}
		db.EventsCollection.FindOne(ctx, bson.M{"_id": eventID}).Decode(&ev)
		if ev.NumResponses != 1 {
			t.Fatalf("numResponses %d", ev.NumResponses)
		}
	})

	t.Run("legacy_writer_in_between_is_detected", func(t *testing.T) {
		eventID, user := primitive.NewObjectID(), &models.User{Id: primitive.NewObjectID()}
		db.EventsCollection.InsertOne(ctx, bson.M{"_id": eventID})
		db.ReplaceAvailabilityIf(user, eventID, mcp.AnswerSnapshot{}, slots(9), slots())
		snap, _ := readAnswer(t, eventID, user)
		// Legacy unconditional replace, as the website or a legacy array call does.
		if err := db.ReplaceAvailability(user, eventID, slots(13), slots(14)); err != nil {
			t.Fatal(err)
		}
		if ok, _ := db.ReplaceAvailabilityIf(user, eventID, snap, slots(15), slots()); ok {
			t.Fatal("overwrote a legacy write")
		}
	})

	t.Run("preserves_other_response_fields", func(t *testing.T) {
		eventID, user := primitive.NewObjectID(), &models.User{Id: primitive.NewObjectID()}
		useCal := true
		db.EventResponsesCollection.InsertOne(ctx, models.EventResponse{
			UserId: user.Id.Hex(), EventId: eventID,
			Response: &models.Response{UserId: user.Id, Availability: slots(9), IfNeeded: slots(), UseCalendarAvailability: &useCal},
		})
		snap, _ := readAnswer(t, eventID, user)
		if ok, err := db.ReplaceAvailabilityIf(user, eventID, snap, slots(10), slots()); !ok || err != nil {
			t.Fatalf("ok=%v err=%v", ok, err)
		}
		_, row := readAnswer(t, eventID, user)
		if row.Response.UseCalendarAvailability == nil || !*row.Response.UseCalendarAvailability {
			t.Fatal("calendar flag dropped")
		}
	})

	t.Run("concurrent_writers_on_one_snapshot_exactly_one_wins", func(t *testing.T) {
		eventID, user := primitive.NewObjectID(), &models.User{Id: primitive.NewObjectID()}
		db.EventsCollection.InsertOne(ctx, bson.M{"_id": eventID})
		db.ReplaceAvailabilityIf(user, eventID, mcp.AnswerSnapshot{}, slots(9), slots())
		snap, _ := readAnswer(t, eventID, user)
		var wins int32
		var wg sync.WaitGroup
		start := make(chan struct{})
		for i := 0; i < 16; i++ {
			wg.Add(1)
			go func(h int) {
				defer wg.Done()
				<-start
				if ok, err := db.ReplaceAvailabilityIf(user, eventID, snap, slots(h), slots()); err == nil && ok {
					atomic.AddInt32(&wins, 1)
				}
			}(10 + i)
		}
		close(start)
		wg.Wait()
		if wins != 1 {
			t.Fatalf("%d writers won", wins)
		}
	})

	t.Run("concurrent_first_answers", func(t *testing.T) {
		// No unique index on (userId,eventId) exists, so plain upserts double-insert (seen: wins=2).
		// ReplaceAvailabilityIf compensates after insert. Safety only: at most one winner and
		// stored documents == winners. All racers may lose (each gets CONFLICT and can retry).
		// A true first-answer CAS still needs a unique index, which this task does not add.
		eventID, user := primitive.NewObjectID(), &models.User{Id: primitive.NewObjectID()}
		db.EventsCollection.InsertOne(ctx, bson.M{"_id": eventID})
		var wins int32
		var wg sync.WaitGroup
		start := make(chan struct{})
		for i := 0; i < 16; i++ {
			wg.Add(1)
			go func(h int) {
				defer wg.Done()
				<-start
				if ok, err := db.ReplaceAvailabilityIf(user, eventID, mcp.AnswerSnapshot{}, slots(h), slots()); err == nil && ok {
					atomic.AddInt32(&wins, 1)
				}
			}(10 + i)
		}
		close(start)
		wg.Wait()
		n, _ := db.EventResponsesCollection.CountDocuments(ctx, bson.M{"userId": user.Id.Hex(), "eventId": eventID})
		t.Logf("first-answer race: wins=%d documents=%d", wins, n)
		if wins > 1 || n != int64(wins) {
			t.Fatalf("first-answer race not safe: wins=%d documents=%d", wins, n)
		}
	})
}
