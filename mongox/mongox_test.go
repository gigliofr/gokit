package mongox

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestConnectEmptyURI(t *testing.T) {
	if _, err := Connect(context.Background(), "  "); err == nil {
		t.Fatal("atteso errore con URI vuoto")
	}
}

func TestConnectUnreachable(t *testing.T) {
	start := time.Now()
	_, err := Connect(context.Background(), "mongodb://127.0.0.1:1/?serverSelectionTimeoutMS=200", WithTimeout(time.Second))
	if err == nil || !strings.Contains(err.Error(), "mongox") {
		t.Fatalf("atteso errore mongox, ottenuto %v", err)
	}
	if time.Since(start) > 3*time.Second {
		t.Fatal("il timeout non è stato rispettato")
	}
}

func TestPingNil(t *testing.T) {
	if err := Ping(context.Background(), nil); err == nil {
		t.Fatal("atteso errore con client nil")
	}
	Disconnect(nil, time.Second) // non deve andare in panic
}

// Test di integrazione: eseguito solo con MONGOX_TEST_URI impostato.
func TestIntegration(t *testing.T) {
	uri := os.Getenv("MONGOX_TEST_URI")
	if uri == "" {
		t.Skip("MONGOX_TEST_URI non impostato")
	}
	ctx := context.Background()
	c, err := Connect(ctx, uri, WithAppName("gokit-test"))
	if err != nil {
		t.Fatal(err)
	}
	defer Disconnect(c, 5*time.Second)
	db := c.Database("gokit_test")
	err = EnsureIndexes(ctx, db, Indexes{
		"users": {{Keys: bson.D{{Key: "email", Value: 1}}, Options: options.Index().SetUnique(true)}},
		"empty": []mongo.IndexModel{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := Ping(ctx, c); err != nil {
		t.Fatal(err)
	}
	_ = db.Drop(ctx)
}
