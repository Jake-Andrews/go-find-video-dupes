package sqlite

import (
	"database/sql"
	"log/slog"

	_ "modernc.org/sqlite"
)

func InitDB(dbPath string) *sql.DB {
	slog.Info("Initializing database connection", slog.String("Path", dbPath))
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		slog.Error("Error opening SQLite database connection", slog.String("Path", dbPath), slog.Any("error", err))
		return db
	}

	err = db.Ping()
	if err != nil {
		slog.Error("Error pinging SQLite database", slog.Any("error", err))
		return nil
	}

	_, err = db.Exec("PRAGMA foreign_keys = ON;")
	if err != nil {
		slog.Error("Error setting PRAGMA foreign_keys", slog.Any("error", err))
	}
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS video (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			xxhash TEXT NOT NULL,
			path TEXT NOT NULL,
			fileName TEXT NOT NULL,
			createdAt DATETIME,
			modifiedAt DATETIME,
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
			device INTEGER,
			sampleRateAvg INTEGER,
			avgFrameRate REAL,
			-- NEW column to reference the videohash
			FK_video_videohash INTEGER,
			FOREIGN KEY (FK_video_videohash) REFERENCES videohash (id) ON DELETE CASCADE
		);
	`)
	if err != nil {
		slog.Error("Error creating the video table", slog.Any("error", err))
	}
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS videohash (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			hashValue TEXT NOT NULL,
			hashType TEXT NOT NULL,
			duration INTEGER NOT NULL,
			neighbours TEXT,
			bucket INTEGER
			-- No foreign key to "video" here
		);
	`)
	if err != nil {
		slog.Error("Error creating the videohash table", slog.Any("error", err))
	}
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS screenshot (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			FK_screenshot_videohash INTEGER NOT NULL,
			screenshots TEXT NOT NULL,
			FOREIGN KEY (FK_screenshot_videohash) REFERENCES videohash (id) ON DELETE CASCADE
		);
	`)
	if err != nil {
		slog.Error("Error creating the screenshot table", slog.Any("error", err))
	}

	slog.Info("Database initialized successfully")
	return db
}
