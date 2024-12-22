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

	_, err = db.Exec(
		`CREATE TABLE IF NOT EXISTS video (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
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
			bitRate INTEGER,
			numHardLinks INTEGER,
			symbolicLink TEXT,
			isSymbolicLink INTEGER,
			isHardLink INTEGER,
			inode INTEGER,
			device INTEGER
		);`,
	)
	if err != nil {
		log.Fatalf("Error initializing the video table: %v\n", err)
	}

	_, err = db.Exec(
		`CREATE TABLE IF NOT EXISTS videohash (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			FK_videohash_video INTEGER NOT NULL,
			hashValue TEXT NOT NULL,
			hashType TEXT NOT NULL,
			duration INTEGER NOT NULL,
			neighbours TEXT,
			bucket INTEGER,
			FOREIGN KEY (FK_videohash_video) REFERENCES video (id) ON DELETE CASCADE
		);`,
	)
	if err != nil {
		log.Fatalf("Error initializing the videohash table: %v\n", err)
	}

	_, err = db.Exec(
		`CREATE TABLE IF NOT EXISTS screenshot (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			FK_screenshot_videohash INTEGER NOT NULL,
			screenshots TEXT NOT NULL,
			FOREIGN KEY (FK_screenshot_videohash) REFERENCES videohash (id) ON DELETE CASCADE
		);`,
	)
	if err != nil {
		log.Fatalf("Error initializing the screenshot table: %v\n", err)
	}

	log.Println("Successfully initialized the database!")
	return db
}
