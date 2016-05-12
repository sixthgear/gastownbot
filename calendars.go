package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
)

// Create a new Nullbot CalenderAPI Model Object
func NewService() (service *calendar.Service, err error) {

	var ctx context.Context
	var config *oauth2.Config
	var client *http.Client
	var buf []byte

	ctx = context.Background()

	// Read config file (required config/client_secret.json)
	buf, err = ioutil.ReadFile("./config/client_secret.json")
	if err != nil {
		return nil, fmt.Errorf("Unable to read client secret file: %v", err)
	}

	config, err = google.ConfigFromJSON(buf, calendar.CalendarScope)
	if err != nil {
		return nil, fmt.Errorf("Unable to parse client secret file to config: %v", err)
	}

	// Get client object
	client, err = getClient(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("Unable to create HTTP client: %v", err)
	}

	service, err = calendar.New(client)
	if err != nil {
		return nil, fmt.Errorf("Unable to retrieve calendar Client %v", err)
	}

	return service, nil
}

// getClient uses a Context and Config to retrieve a Token
// then generate a Client. It returns the generated Client.
func getClient(ctx context.Context, config *oauth2.Config) (*http.Client, error) {

	if cacheFile, err := tokenCacheFile(); err != nil {
		return nil, fmt.Errorf("Unable to get path to cached credential file. %v", err)
	} else if tok, err := tokenFromFile(cacheFile); err != nil {
		tok = getTokenFromWeb(config)
		saveToken(cacheFile, tok)
		return config.Client(ctx, tok), nil
	} else {
		return config.Client(ctx, tok), nil
	}

}

// getTokenFromWeb uses Config to request a Token.
// It returns the retrieved Token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {

	var code string

	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	if _, err := fmt.Scan(&code); err != nil {
		panic(fmt.Sprintf("Unable to read authorization code %v", err))
	}

	tok, err := config.Exchange(oauth2.NoContext, code)
	if err != nil {
		panic(fmt.Sprintf("Unable to retrieve token from web %v", err))
	}

	return tok
}

// tokenCacheFile generates credential file path/filename.
// It returns the generated credential path/filename.
func tokenCacheFile() (string, error) {

	// usr, err := user.Current()
	// if err != nil {
	// 	return "", err
	// }
	// tokenCacheDir := filepath.Join(usr.HomeDir, ".credentials")
	// os.MkdirAll(tokenCacheDir, 0700)
	return filepath.Join("./config", url.QueryEscape("calendar-gastownbot.json")), nil
}

// tokenFromFile retrieves a Token from a given file path.
// It returns the retrieved Token and any read error encountered.
func tokenFromFile(file string) (*oauth2.Token, error) {

	if f, err := os.Open(file); err != nil {
		return nil, err
	} else {
		t := &oauth2.Token{}
		err = json.NewDecoder(f).Decode(t)
		defer f.Close()
		return t, err
	}
}

// saveToken uses a file path to create a file and store the
// token in it.
func saveToken(file string, token *oauth2.Token) {

	fmt.Printf("Saving credential file to: %s\n", file)
	if f, err := os.Create(file); err != nil {
		panic(fmt.Sprintf("Unable to cache oauth token: %v", err))
	} else {
		defer f.Close()
		json.NewEncoder(f).Encode(token)
	}
}
