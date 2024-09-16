package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/hugolgst/rich-go/client"
	"github.com/martinlindhe/notify"
)

var (
	inactivityTimer *time.Timer
	inactivityTime  int
	timerMutex      sync.Mutex
	activityStart   time.Time
	isLoggedIn      bool

	prevTitle  string
	prevNumber string
	prevImage  string

	logger *log.Logger

	notificationEnabled bool

	iconAbsolutePath string
)

func init() {
	var notificationFlag int
	var inactivityTimeFlag int
	flag.IntVar(&notificationFlag, "notification", 1, "Enable notifications (1) or disable notifications (0)")
	flag.IntVar(&inactivityTimeFlag, "inactivity", 64, "Specify inactiveTime wait")
	flag.Parse()

	notificationEnabled = notificationFlag == 1
	inactivityTime = inactivityTimeFlag

	file, err := os.OpenFile("rumi-rich-presence.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
	if err != nil {
		fmt.Println("Error opening log file:", err)
		os.Exit(1)
	}
	logger = log.New(file, "", log.Ldate|log.Ltime|log.Lshortfile)

	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}
	iconAbsolutePath = filepath.Dir(ex) + "\\rumi-icon.jpg"
}

func rumiTellsYou(msg string) {
	if notificationEnabled {
		notify.Notify("Rumi", "Rumi Tells You", msg, iconAbsolutePath)
	}
}

func resetInactivityTimer() {
	timerMutex.Lock()
	defer timerMutex.Unlock()

	if inactivityTimer != nil {
		inactivityTimer.Stop()
	}

	inactivityTimer = time.AfterFunc(time.Duration(inactivityTime)*time.Second, func() {
		prevTitle = ""
		prevNumber = ""
		prevImage = ""
		logger.Printf("No activity for %s seconds. Logging out.", strconv.Itoa(inactivityTime))
		client.Logout()
		isLoggedIn = false
		activityStart = time.Time{}
		rumiTellsYou("No activity in " + strconv.Itoa(inactivityTime) + " second. Rich presence turned off.")
	})
}

func enableCORS(res http.ResponseWriter) {
	res.Header().Set("Access-Control-Allow-Origin", "*")
	res.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	res.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

func response(message []byte, httpCode int, res http.ResponseWriter) {
	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(httpCode)
	res.Write(message)
}

func startWatching(res http.ResponseWriter, req *http.Request) {
	enableCORS(res)

	title := req.URL.Query().Get("title")
	number := req.URL.Query().Get("number")
	image := req.URL.Query().Get("image")

	if title == "" || number == "" || image == "" {
		response([]byte(`{"message": "Where are the parameters?", "statusCode": 400}`), http.StatusBadRequest, res)
		return
	}

	if title == prevTitle && number == prevNumber && image == prevImage {
		logger.Println("No changes in activity, skipping update.")
		resetInactivityTimer()
		response([]byte(`{"message": "No changes, skipping update", "statusCode": 200}`), http.StatusOK, res)
		return
	}

	logger.Printf("SetActivity with value=%s, %s, %s\n", title, number, image)

	rumiTellsYou("You are now watching " + title)

	if activityStart.IsZero() {
		activityStart = time.Now()
	}

	if !isLoggedIn {
		err := client.Login("DISCORD_APP_ID")
		if err != nil {
			logger.Println("Error logging in:", err)
			return
		}
		isLoggedIn = true
	}

	err := client.SetActivity(client.Activity{
		State:      "Episode " + number,
		Details:    "Watching " + title,
		LargeImage: image,
		LargeText:  title,
		Timestamps: &client.Timestamps{
			Start: &activityStart,
		},
	})
	if err != nil {
		logger.Println("Error setting activity:", err)
	}

	prevTitle = title
	prevNumber = number
	prevImage = image

	resetInactivityTimer()

	response([]byte(`{"message": "Activity set", "statusCode": 200}`), http.StatusOK, res)
}

func stopWatching(res http.ResponseWriter, req *http.Request) {
	enableCORS(res)

	rumiTellsYou("You are now stop watching")

	client.Logout()
	response([]byte(`{"message": "Success", "statusCode": 200}`), http.StatusOK, res)
}

func main() {
	http.HandleFunc("/", func(res http.ResponseWriter, req *http.Request) {
		enableCORS(res)
		message := []byte(`{"message": "Rumi Rich Presence up and running", "statusCode": 200}`)
		response(message, http.StatusOK, res)
	})

	http.HandleFunc("/watch/start", startWatching)
	http.HandleFunc("/watch/stop", stopWatching)

	logger.Println("rumi-rich-presence running")
	err := http.ListenAndServe(":6969", nil)
	if err != nil {
		logger.Println("Error starting server:", err)
		os.Exit(1)
	}
}
