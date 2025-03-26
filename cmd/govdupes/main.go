package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"govdupes/internal/application"
	"govdupes/internal/config"
	"govdupes/internal/db/dbstore"
	"govdupes/internal/db/sqlite"
	"govdupes/internal/videoprocessor"
	"govdupes/internal/vm/viewmodel"
	"govdupes/ui"

	_ "modernc.org/sqlite"
)

func main() {
	slog.Info("Starting...")

	var cfg config.Config
	cfg.SetDefaults()
	logger := config.SetupLogger(cfg.LogFilePath)
	slog.SetDefault(logger)

	db := sqlite.InitDB(cfg.DatabasePath)
	vp := videoprocessor.NewFFmpegInstance(&cfg)
	vs := dbstore.NewVideoStore(db)

	a := application.NewApplication(&cfg, vs, vp)
	vm := viewmodel.NewViewModel(a)

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-signalChan
		slog.Info("Shutting down...")
		if err := db.Close(); err != nil {
			slog.Error("Error closing the database", slog.Any("error", err))
		} else {
			slog.Info("Database connection closed.")
		}
		os.Exit(0)
	}()

	ui.CreateUI(a, vm)
}
