package internal

import (
	"io"
	"log"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// Backup the original log output
	originalLogOutput := log.Writer()

	// Redirect log output to discard during tests
	log.SetOutput(io.Discard)

	// Run the tests
	exitCode := m.Run()

	// Restore the original log output after tests complete
	log.SetOutput(originalLogOutput)

	// Exit with the test result code
	os.Exit(exitCode)
}
