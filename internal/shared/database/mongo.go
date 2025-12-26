package database

import (
	"context"
	"log"
	"time"

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
		Keys: map[string]interface{}{
			"status":     1,
			"expires_at": 1,
		},
	})
	if err != nil {
		return err
	}
	
	// Create unique index on action_id
	_, err = pendingActionsCollection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: map[string]interface{}{
			"action_id": 1,
		},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return err
	}
	
	// Create index on conversation_id for correlation queries
	conversationsCollection := db.Collection("conversations")
	_, err = conversationsCollection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: map[string]interface{}{
			"correlation_id": 1,
		},
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
