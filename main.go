package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/schollz/progressbar/v3"
)

const (
	VOTES_PER_IP    = 25
	OVPN_DIRECTORY  = "/Users/parkerpulfer/Repos/openvpn"
	AUTH_FILE       = "/Users/parkerpulfer/Repos/openvpn/auth.txt"
	OPENVPN_TIMEOUT = 30 * time.Second
	POLL_ID         = 14689433
	OPTION_ID       = 65243716
	MAX_RETRIES     = 3
)

var OPENVPN_PATH string

func logInfo(format string, args ...interface{}) {
	fmt.Printf("[INFO] "+format+"\n", args...)
}

func logError(format string, args ...interface{}) {
	fmt.Printf("[ERROR] "+format+"\n", args...)
}

func EnsureOpenVPNPath() error {
	path, err := exec.LookPath("openvpn")
	if err != nil {
		return fmt.Errorf("OpenVPN binary not found. Please ensure it is installed")
	}
	OPENVPN_PATH = path
	logInfo("OpenVPN binary located at: %s", OPENVPN_PATH)
	return nil
}

func FlushDNS() {
	if runtime.GOOS == "darwin" {
		logInfo("Flushing DNS...")
		exec.Command("sudo", "dscacheutil", "-flushcache").Run()
		exec.Command("sudo", "killall", "-HUP", "mDNSResponder").Run()
		logInfo("DNS cache flushed.")
	}
}

func GetRandomOVPNFile(directory string) (string, error) {
	files, err := filepath.Glob(filepath.Join(directory, "*.ovpn"))
	if err != nil || len(files) == 0 {
		return "", fmt.Errorf("no .ovpn files found in directory: %s", directory)
	}
	randomFile := files[rand.Intn(len(files))]
	logInfo("Selected .ovpn file: %s", randomFile)
	return randomFile, nil
}

func forceKillVPN() {
	logInfo("Forcefully terminating VPN connections...")

	cmd := exec.Command("pkill", "openvpn")
	output, err := cmd.Output()
	if err != nil {
		logError("Failed to find OpenVPN process: %v", err)
		return
	}

	pids := strings.Fields(string(output))
	if len(pids) == 0 {
		logInfo("No OpenVPN process found to kill.")
		return
	}

	for _, pid := range pids {
		logInfo("Killing OpenVPN process with PID: %s", pid)
		killCmd := exec.Command("sudo", "kill", "-9", pid)
		if err := killCmd.Run(); err != nil {
			logError("Failed to kill OpenVPN process with PID %s: %v", pid, err)
		} else {
			logInfo("Successfully killed OpenVPN process with PID %s.", pid)
		}
	}

	logInfo("Flushing routing table...")
	if err := exec.Command("sudo", "route", "flush").Run(); err != nil {
		logError("Failed to flush routing table: %v", err)
	} else {
		logInfo("Routing table flushed successfully.")
	}

	logInfo("Flushing DNS cache...")
	FlushDNS()
	logInfo("VPN cleanup completed.")
}

func CheckVPNConnection() bool {
	resp, err := http.Get("https://api.ipify.org?format=json")
	if err != nil {
		logError("Failed to check public IP: %v", err)
		return false
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	logInfo("Current Public IP: %s", string(body))
	return true
}

func parseVoteData(body string) (string, string, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader([]byte(body)))
	if err != nil {
		return "", "", fmt.Errorf("failed to parse HTML document: %w", err)
	}

	voteButton := doc.Find(".vote-button")
	if voteButton.Length() == 0 {
		logError("Vote button not found in the HTML.")
		return "", "", fmt.Errorf("vote button not found")
	}

	dataVote, exists := voteButton.Attr("data-vote")
	if !exists {
		logError("Data-vote attribute not found in the button.")
		return "", "", fmt.Errorf("vote data not found in the button")
	}

	var voteData map[string]interface{}
	if err := json.Unmarshal([]byte(dataVote), &voteData); err != nil {
		logError("Failed to parse vote data JSON: %v", err)
		return "", "", fmt.Errorf("failed to parse vote data JSON: %w", err)
	}

	token, ok := voteData["n"].(string)
	if !ok {
		logError("Token not found in vote data JSON.")
		return "", "", fmt.Errorf("token not found in vote data")
	}

	pz := doc.Find("input[name='pz']").AttrOr("value", "")
	if pz == "" {
		logError("PZ value not found in the HTML.")
		return "", "", fmt.Errorf("pz value not found")
	}

	logInfo("Extracted token: %s and PZ: %s", token, pz)
	return token, pz, nil
}

func voteWithCurrentIP(client *http.Client, pollID int, optionID int) error {
	progressBar := progressbar.Default(int64(VOTES_PER_IP), "Voting")
	userAgents := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/113.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Linux; Android 9; SM-G960F) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/113.0.0.0 Mobile Safari/537.36",
	}

	for i := 0; i < VOTES_PER_IP; i++ {
		userAgent := userAgents[rand.Intn(len(userAgents))]

		req, err := http.NewRequest("GET", fmt.Sprintf("https://poll.fm/%d", pollID), nil)
		if err != nil {
			return fmt.Errorf("failed to create poll page request: %w", err)
		}

		req.Header.Set("User-Agent", userAgent)
		req.Header.Set("Accept", "*/*")
		req.Header.Set("Connection", "keep-alive")
		req.Header.Set("Cache-Control", "no-cache")
		req.Header.Set("Accept-Language", "en-US,en;q=0.5")
		req.Header.Set("Sec-Fetch-Dest", "empty")
		req.Header.Set("Sec-Fetch-Mode", "cors")
		req.Header.Set("Sec-Fetch-Site", "same-origin")

		resp, err := client.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			logError("Failed to fetch poll page: %v", err)
			return err
		}

		body, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			logError("Failed to read poll page response: %v", err)
			return err
		}

		token, pz, err := parseVoteData(string(body))
		if err != nil {
			logError("Failed to parse vote data: %v", err)
			return err
		}

		voteURL := fmt.Sprintf("https://poll.fm/vote?va=10&pt=0&r=1&p=%d&a=%d&o=&t=%d&token=%s&pz=%s", pollID, optionID, i+1, token, pz)
		logInfo("Generated vote URL: %s", voteURL)

		voteReq, err := http.NewRequest("GET", voteURL, nil)
		if err != nil {
			logError("Failed to create vote request: %v", err)
			return err
		}

		voteReq.Header.Set("User-Agent", userAgent)
		voteReq.Header.Set("Accept", "*/*")
		voteReq.Header.Set("Connection", "keep-alive")
		voteReq.Header.Set("Cache-Control", "no-cache")

		voteResp, err := client.Do(voteReq)
		if err != nil || voteResp.StatusCode != http.StatusOK {
			logError("Failed to submit vote: %v", err)
			continue
		}

		progressBar.Add(1)
		time.Sleep(2 * time.Second)
	}

	progressBar.Finish()
	return nil
}

func main() {
	rand.Seed(time.Now().UnixNano())

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-signalChan
		logInfo("Received termination signal: %v", sig)
		forceKillVPN()
		os.Exit(0)
	}()

	logInfo("Starting application...")

	if err := EnsureOpenVPNPath(); err != nil {
		logError("%v", err)
		return
	}

	client := &http.Client{}

	for {
		// Select a random .ovpn configuration file
		configPath, err := GetRandomOVPNFile(OVPN_DIRECTORY)
		if err != nil {
			logError("Failed to select .ovpn file: %v", err)
			continue
		}

		logInfo("Starting OpenVPN with config: %s", configPath)
		cmd := exec.Command(OPENVPN_PATH, "--config", configPath, "--auth-user-pass", AUTH_FILE, "--auth-nocache", "--daemon", "--verb", "4")
		if err := cmd.Start(); err != nil {
			logError("Failed to start OpenVPN: %v", err)
			continue
		}

		// Wait for the VPN connection to establish
		logInfo("Waiting for VPN to establish...")
		time.Sleep(OPENVPN_TIMEOUT)

		// Verify VPN connection
		if !CheckVPNConnection() {
			logError("VPN connection failed. Retrying...")
			forceKillVPN()
			continue
		}

		// Attempt to vote
		logInfo("Voting with current IP...")
		if err := voteWithCurrentIP(client, POLL_ID, OPTION_ID); err != nil {
			logError("Voting failed with current VPN: %v", err)
		} else {
			logInfo("Voting completed successfully.")
		}

		// Terminate the VPN connection
		logInfo("Terminating VPN connection...")
		if err := cmd.Process.Kill(); err != nil {
			logError("Failed to terminate OpenVPN process: %v", err)
		}

		forceKillVPN()
		logInfo("VPN connection terminated. Proceeding to the next cycle...")
	}
}
