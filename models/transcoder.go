package models

import (
	"context"
	"fmt"
	"github.com/angelorc/go-uploader/db"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"time"
)

const Collection = "transcoder"

type Transcoder struct {
	ID         primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	Percentage int                `json:"percentage" bson:"percentage"`
}

func NewTranscoder() *Transcoder {
	return &Transcoder{
		ID:         primitive.NewObjectID(),
		Percentage: 0,
	}
}

func (t *Transcoder) GetCollection() *mongo.Collection {
	db, _ := db.Connect()

	return db.Collection(Collection)
}

func (t *Transcoder) Create() error {
	collection := t.GetCollection()

	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	_, err := collection.InsertOne(ctx, t)
	if err != nil {
		return fmt.Errorf("cannot create mongo/transcoder")
	}

	return nil
}

func (t *Transcoder) Get() (*Transcoder, error) {
	collection := t.GetCollection()
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)

	filter := bson.D{
		{"_id", t.ID},
	}

	var transcoder Transcoder
	err := collection.FindOne(ctx, filter).Decode(&transcoder)
	if err != nil {
		return nil, err
	}

	return &transcoder, nil
}

func (t *Transcoder) UpdatePercentage(percentage int) error {
	collection := t.GetCollection()
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)

	filter := bson.D{
		{"_id", t.ID},
	}

	update := bson.D{
		{"$set", bson.D{
			{"percentage", percentage},
		}},
	}

	_, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}

	return nil
}

func (t *Transcoder) Delete() error {
	collection := t.GetCollection()
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)

	filter := bson.D{
		{"_id", t.ID},
	}

	_, err := collection.DeleteOne(ctx, filter)
	if err != nil {
		return err
	}

	return nil
}