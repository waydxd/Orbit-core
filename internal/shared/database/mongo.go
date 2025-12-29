package database

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var MongoClient *mongo.Client

func InitMongoDB(uri string) error {
	var err error
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	MongoClient, err = mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Printf("Failed to connect to MongoDB: %v", err)
		return err
	}

	// Check the connection
	err = MongoClient.Ping(ctx, nil)
	if err != nil {
		log.Printf("Failed to ping MongoDB: %v", err)
		return err
	}

	log.Println("Connected to MongoDB!")

	// Create indexes
	if err := createIndexes(MongoClient); err != nil {
		log.Printf("Warning: Failed to create MongoDB indexes: %v", err)
		// Don't return error - indexes are optional for basic functionality
	}

	return nil
}

func createIndexes(client *mongo.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db := client.Database("orbit") // Default database name

	// Create index on pending_actions for efficient GetExpiredActions query
	pendingActionsCollection := db.Collection("pending_actions")
	_, err := pendingActionsCollection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "status", Value: 1},
			{Key: "expires_at", Value: 1},
		},
	})
	if err != nil {
		return err
	}

	// Create unique index on action_id
	_, err = pendingActionsCollection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "action_id", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return err
	}

	// Create index on conversation_id for correlation queries
	conversationsCollection := db.Collection("conversations")
	_, err = conversationsCollection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "correlation_id", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return err
	}

	log.Println("MongoDB indexes created successfully")
	return nil
}

func DisconnectMongoDB() {
	if MongoClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := MongoClient.Disconnect(ctx); err != nil {
			cancel()
			log.Printf("Failed to disconnect from MongoDB: %v", err)
		}
	}
}

// BuildMongoURI constructs a MongoDB URI from provided components. It will
// prefer an explicit MONGODB_URI environment variable when present (and not the default placeholder).
// Otherwise, if user and pass are provided, it will assemble mongodb://user:pass@host/db.
// If user/pass are empty it will attempt to use MONGO_USER/MONGO_PASSWORD env vars.
// Final fallback is the explicit MONGODB_URI or default mongodb://localhost:27017/orbit.
func BuildMongoURI(user, pass, host, dbname string) string {
	defaultURI := "mongodb://localhost:27017/orbit"
	uri := os.Getenv("MONGODB_URI")
	if uri != "" && uri != defaultURI {
		return uri
	}

	// If caller provided host/dbname empty, allow env fallbacks
	if host == "" {
		host = os.Getenv("MONGODB_HOST")
		if host == "" {
			host = "mongo:27017"
		}
	}
	if dbname == "" {
		dbname = os.Getenv("MONGODB_DB")
		if dbname == "" {
			dbname = "orbit"
		}
	}

	// If we have explicit user/pass passed in use them
	if user != "" && pass != "" {
		return fmt.Sprintf("mongodb://%s:%s@%s/%s", user, pass, host, dbname)
	}

	// Fallback to environment variables for user/pass
	envUser := os.Getenv("MONGO_USER")
	envPass := os.Getenv("MONGO_PASSWORD")
	if envUser != "" && envPass != "" {
		return fmt.Sprintf("mongodb://%s:%s@%s/%s", envUser, envPass, host, dbname)
	}

	// If we had an explicit URI (even the default), return it as last resort
	if uri != "" {
		return uri
	}

	return defaultURI
}
