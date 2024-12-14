package sqlite

import (
	"database/sql"
	"log"

	_ "modernc.org/sqlite"
)

func InitDB(dbPath string) *sql.DB {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("Error opening SQLite database connection: %v\n", err)
		return db
	}

	err = db.Ping()
	if err != nil {
		log.Fatalf("Error pinging SQLite database: %v\n", err)
		return nil
	}

	_, err = db.Exec("PRAGMA foreign_keys = ON;")
	if err != nil {
		log.Fatalf("Error setting PRAGMA foreign_keys = ON: %v\n", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS video (
			videoID INTEGER PRIMARY KEY AUTOINCREMENT,
			path TEXT NOT NULL,
			fileName TEXT NOT NULL,
			createdAt DATETIME,
			modifiedAt DATETIME,
			frameRate REAL,
			videoCodec TEXT,
			audioCodec TEXT,
			width INTEGER,
			height INTEGER,
			duration INTEGER NOT NULL,
			size INTEGER NOT NULL,
			bitRate INTEGER
		);
	`)
	if err != nil {
		log.Fatalf("Error initializing the video table: %v\n", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS videohash (
			videohashID INTEGER PRIMARY KEY AUTOINCREMENT,
			videoID INTEGER NOT NULL,
			hashValue TEXT NOT NULL,
			hashType TEXT NOT NULL,
			duration INTEGER NOT NULL,
			neighbours TEXT,			
            bucket INTEGER,
			FOREIGN KEY (videoID) REFERENCES video (videoID) ON DELETE CASCADE
		);
	`)
	if err != nil {
		log.Fatalf("Error initializing the videohash table: %v\n", err)
	}

	log.Println("Successfully initialized the database!")
	return db
}
