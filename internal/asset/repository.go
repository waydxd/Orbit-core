package asset

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/waydxd/Orbit-core/internal/shared/models"
)

const (
	collectionEventImages = "event_images"
	collectionUserAvatars = "user_avatars"
)

// ErrAssetNotFound is returned when the requested asset does not exist.
var ErrAssetNotFound = errors.New("asset not found")

// Repository defines the storage operations for binary asset data.
type Repository interface {
	SaveEventImage(ctx context.Context, eventID string, data []byte, contentType string) (string, error)
	GetEventImage(ctx context.Context, imageID string) (*models.EventImage, error)
	DeleteEventImage(ctx context.Context, imageID string) error
	SaveUserAvatar(ctx context.Context, userID string, data []byte, contentType string) (string, error)
	GetUserAvatar(ctx context.Context, imageID string) (*models.UserAvatar, error)
}

// MongoRepository implements Repository using MongoDB.
type MongoRepository struct {
	client *mongo.Client
	dbName string
}

// NewMongoRepository creates a new MongoRepository and ensures required indexes exist.
func NewMongoRepository(ctx context.Context, client *mongo.Client, dbName string) (*MongoRepository, error) {
	r := &MongoRepository{client: client, dbName: dbName}
	if err := r.createIndexes(ctx); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *MongoRepository) createIndexes(ctx context.Context) error {
	db := r.client.Database(r.dbName)

	_, err := db.Collection(collectionEventImages).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "metadata.event_id", Value: 1}},
	})
	if err != nil {
		return err
	}

	_, err = db.Collection(collectionUserAvatars).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "metadata.user_id", Value: 1}},
	})
	return err
}

// SaveEventImage stores a binary event image in MongoDB and returns the new image ID.
func (r *MongoRepository) SaveEventImage(ctx context.Context, eventID string, data []byte, contentType string) (string, error) {
	id := uuid.New().String()
	doc := models.EventImage{
		ID:      id,
		BinData: data,
		Metadata: models.EventImageMetadata{
			EventID:     eventID,
			ContentType: contentType,
			Size:        int64(len(data)),
			CreatedAt:   time.Now().UTC(),
		},
	}
	_, err := r.client.Database(r.dbName).Collection(collectionEventImages).InsertOne(ctx, doc)
	if err != nil {
		return "", err
	}
	return id, nil
}

// GetEventImage retrieves an event image by its ID.
func (r *MongoRepository) GetEventImage(ctx context.Context, imageID string) (*models.EventImage, error) {
	var img models.EventImage
	err := r.client.Database(r.dbName).Collection(collectionEventImages).
		FindOne(ctx, bson.M{"_id": imageID}).Decode(&img)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrAssetNotFound
	}
	if err != nil {
		return nil, err
	}
	return &img, nil
}

// DeleteEventImage removes an event image document by its ID.
func (r *MongoRepository) DeleteEventImage(ctx context.Context, imageID string) error {
	res, err := r.client.Database(r.dbName).Collection(collectionEventImages).
		DeleteOne(ctx, bson.M{"_id": imageID})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return ErrAssetNotFound
	}
	return nil
}

// SaveUserAvatar upserts the user's avatar in MongoDB (overwrites any existing document for that user).
func (r *MongoRepository) SaveUserAvatar(ctx context.Context, userID string, data []byte, contentType string) (string, error) {
	id := uuid.New().String()
	now := time.Now().UTC()

	// Delete any existing avatar for this user before inserting the new one.
	_, _ = r.client.Database(r.dbName).Collection(collectionUserAvatars).
		DeleteMany(ctx, bson.M{"metadata.user_id": userID})

	doc := models.UserAvatar{
		ID:      id,
		BinData: data,
		Metadata: models.UserAvatarMetadata{
			UserID:      userID,
			ContentType: contentType,
			Size:        int64(len(data)),
			UpdatedAt:   now,
		},
	}
	_, err := r.client.Database(r.dbName).Collection(collectionUserAvatars).InsertOne(ctx, doc)
	if err != nil {
		return "", err
	}
	return id, nil
}

// GetUserAvatar retrieves a user avatar by its image ID.
func (r *MongoRepository) GetUserAvatar(ctx context.Context, imageID string) (*models.UserAvatar, error) {
	var av models.UserAvatar
	err := r.client.Database(r.dbName).Collection(collectionUserAvatars).
		FindOne(ctx, bson.M{"_id": imageID}).Decode(&av)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrAssetNotFound
	}
	if err != nil {
		return nil, err
	}
	return &av, nil
}
