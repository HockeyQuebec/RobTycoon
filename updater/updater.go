package updater

import (
	"fmt"
	"net/http"
	"os"
	"runtime"

	"github.com/inconshreveable/go-update"
)

const (
	repoURL        = "https://github.com/HockeyQuebec/RobTycoon/releases/latest/download"
	programName    = "RobTycoon"
	currentVersion = "1.0.0"
)

// CheckForUpdates checks and applies updates if available
func CheckForUpdates() {
	latestBinary := fmt.Sprintf("%s/%s-%s-%s", repoURL, programName, runtime.GOOS, runtime.GOARCH)

	fmt.Printf("Checking for updates from %s...\n", latestBinary)
	resp, err := http.Get(latestBinary)
	if err != nil || resp.StatusCode != http.StatusOK {
		fmt.Printf("No updates available or failed to fetch: %v\n", err)
		return
	}
	defer resp.Body.Close()

	fmt.Println("Update available. Downloading...")
	err = update.Apply(resp.Body, update.Options{})
	if err != nil {
		fmt.Printf("Update failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Update complete. Restart the program to use the latest version.")
}
