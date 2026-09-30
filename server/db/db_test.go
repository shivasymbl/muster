package db_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/shivasymbl/muster/server/db"
)

func TestGetDailyUserLogByDate(t *testing.T) {
	db.GetDailyUserLogByDate(time.Now(), 7)
}

func TestGenerateShortEventId(t *testing.T) {
	db.Init()

	id := db.GenerateShortEventId()
	fmt.Println(id)
}
