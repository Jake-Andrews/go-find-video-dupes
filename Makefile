BINARY_NAME=govdupes

MAIN_GO_FILE=cmd/govdupes/main.go

# Database file to delete after running the program
DB_FILE=videos.db

all: run clean_db

build:
	@echo "Building the Go program..."
	@go build -o $(BINARY_NAME) $(MAIN_GO_FILE)

run: build
	@echo "Running the Go program..."
	@./$(BINARY_NAME)

clean_db:
	@echo "Deleting the database file $(DB_FILE)..."
	@rm -f $(DB_FILE)

clean:
	@echo "Cleaning up..."
	@rm -f $(BINARY_NAME)

.PHONY: all build run clean_db clean

