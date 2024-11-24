package main

import (
	"RobTycoon/updater"
	"fmt"
)

func main() {
	fmt.Println("Starting application...")
	updater.CheckForUpdates()
	fmt.Println("Running the main application logic...")
	// Add the rest of your program logic here
}
