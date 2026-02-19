package database

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var MongoClient *mongo.Client

func InitMongoDB(uri string) error {
	var err error

	MongoClient, err = mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		log.Printf("Failed to connect to MongoDB: %v", err)
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
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

// helper: determine host and dbname with env fallbacks
func getHostAndDB(host, dbname string) (string, string) {
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
	return host, dbname
}

// helper: pick credentials from args or env
func getCredentials(user, pass string) (string, string, bool) {
	if user != "" && pass != "" {
		return user, pass, true
	}
	envUser := os.Getenv("MONGO_USER")
	envPass := os.Getenv("MONGO_PASSWORD")
	if envUser != "" && envPass != "" {
		return envUser, envPass, true
	}
	return "", "", false
}

// helper: build base URI with escaped credentials
func buildBaseURI(user, pass, host, db string) string {
	u := &url.URL{
		Scheme: "mongodb",
		Host:   host,
		Path:   "/" + db,
	}
	if user != "" || pass != "" {
		u.User = url.UserPassword(user, pass)
	}
	return u.String()
}

// helper: build params string (authSource + extra)
func buildParams() string {
	authSource := os.Getenv("MONGODB_AUTH_SOURCE")
	extraParams := os.Getenv("MONGODB_PARAMS")
	if authSource == "" {
		authSource = "admin"
	}
	params := fmt.Sprintf("authSource=%s", url.QueryEscape(authSource))
	if extraParams != "" {
		params = params + "&" + extraParams
	}
	return params
}

// BuildMongoURI constructs a MongoDB URI from provided components. It will
// prefer an explicit MONGODB_URI environment variable when present (and not the default placeholder).
// Otherwise, if user and pass are provided (or via env), it will assemble mongodb://user:pass@host/db
// and append authSource/params when credentials are used. Final fallback is the explicit MONGODB_URI
// or default mongodb://localhost:27017/orbit.
func BuildMongoURI(user, pass, host, dbname string) string {
	defaultURI := "mongodb://localhost:27017/orbit"
	dockerDefaultURI := "mongodb://mongo:27017"
	uri := os.Getenv("MONGODB_URI")

	// Check if we have credentials available
	credUser, credPass, haveCreds := getCredentials(user, pass)

	// If MONGODB_URI is set, use it, UNLESS it's a default placeholder AND we have credentials to use instead.
	if uri != "" && uri != defaultURI {
		if uri == dockerDefaultURI && haveCreds {
			// Ignore the docker default URI if we have secrets/credentials to use
			log.Printf("Warning: MONGODB_URI is set to %q but credentials are available; ignoring this placeholder value and using a credential-based URI instead.", dockerDefaultURI)
		} else {
			return uri
		}
	}

	host, dbname = getHostAndDB(host, dbname)
	if haveCreds {
		base := buildBaseURI(credUser, credPass, host, dbname)
		params := buildParams()
		if params != "" {
			if strings.Contains(base, "?") {
				return base + "&" + params
			}
			return base + "?" + params
		}
		return base
	}

	if uri != "" {
		return uri
	}
	return defaultURI
}
