package main

import (
	"log"
	"os"
)

var (
	InfoLogger  *log.Logger
	ErrorLogger *log.Logger
)

func init() {
	errFile, err := os.Create("error.log")

	if err != nil {
		log.Fatalf("failed to open error log file: %v", err)
	}

	ErrorLogger = log.New(errFile, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)

	infoFile, err := os.Create("info.log")

	if err != nil {
		log.Fatalf("failed to open error log file: %v", err)
	}
	InfoLogger = log.New(infoFile, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
}
