package main

import (
	"bytes"
	"context"
	"fmt"
	"github.com/bradleyfalzon/ghinstallation"
	"github.com/google/go-github/v53/github"
	"github.com/joho/godotenv"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
)

func main() {
	conf, err := getConfigFromEnv()
	if err != nil {
		log.Fatalf("Error loading config from environment: %v", err)
	}

	logger := log.Default()
	logger.SetFlags(log.Ltime | log.Ldate | log.LUTC)

	appTransport, err := ghinstallation.NewKeyFromFile(http.DefaultTransport, conf.GHAppID, conf.GHAppInstallationID, conf.GHAppKeyPath)
	appClient := github.NewClient(&http.Client{Transport: appTransport})

	http.HandleFunc("/link", handleLink(conf.GHOauthID))
	http.HandleFunc("/authorize", handleAuthorize(conf.GHOauthID, conf.GHOAuthSecret, appClient))
	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal(err)
	}
}

func getConfigFromEnv() (Config, error) {
	_ = godotenv.Load()

	ghOAuthID, ok := os.LookupEnv("GITHUB_OAUTH_ID")
	if !ok {
		return Config{}, errors.New("GITHUB_OAUTH_ID is required")
	}
	ghOAuthSecret, ok := os.LookupEnv("GITHUB_OAUTH_SECRET")
	if !ok {
		return Config{}, errors.New("GITHUB_OAUTH_SECRET is required")
	}
	ghAppIDStr, ok := os.LookupEnv("GITHUB_APP_ID")
	if !ok {
		return Config{}, errors.New("GITHUB_APP_ID is required")
	}
	ghAppID, err := strconv.ParseInt(ghAppIDStr, 0, 64)
	if err != nil {
		return Config{}, errors.New("GITHUB_APP_ID is not a valid int64")
	}
	ghInstallationIDStr, ok := os.LookupEnv("GITHUB_INSTALLATION_ID")
	if !ok {
		return Config{}, errors.New("GITHUB_INSTALLATION_ID is required")
	}
	ghAppInstallationID, err := strconv.ParseInt(ghInstallationIDStr, 0, 64)
	if err != nil {
		return Config{}, errors.New("GITHUB_INSTALLATION_ID is not a valid int64")
	}

	ghAppKeyPath, ok := os.LookupEnv("GITHUB_APP_KEY_PATH")
	if !ok {
		return Config{}, errors.New("GITHUB_APP_KEY_PATH is required")
	}

	return Config{
		GHOauthID:           ghOAuthID,
		GHOAuthSecret:       ghOAuthSecret,
		GHAppID:             ghAppID,
		GHAppInstallationID: ghAppInstallationID,
		GHAppKeyPath:        ghAppKeyPath,
	}, nil
}

type Config struct {
	GHOauthID     string
	GHOAuthSecret string

	GHAppID             int64
	GHAppInstallationID int64
	GHAppKeyPath        string
}

func handleLink(ghClientID string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if strings.Contains(r.UserAgent(), "bot") {
			return
		}

		log.Println("Redirecting a new request for linking")

		http.Redirect(w, r, fmt.Sprintf("https://github.com/login/oauth/authorize?%s", url.Values{
			"client_id": []string{ghClientID},
			"scope":     []string{string(github.ScopeRepo)},
		}.Encode()), http.StatusSeeOther)
	}
}

func handleAuthorize(ghClientID, ghClientSecret string, appClient *github.Client) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		err := r.ParseForm()
		if err != nil {
			http.Error(w, "could not parse form", http.StatusInternalServerError)
			return
		}

		code := r.Form.Get("code")
		accessToken, err := getAccessToken(code, ghClientID, ghClientSecret)
		if err != nil {
			http.Error(w, fmt.Sprintf("error exchanging code for access token: %v", err), 500)
			return
		}
		log.Println("Successfully received an access token")

		ctx := context.Background()
		tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: accessToken})
		client := github.NewClient(oauth2.NewClient(ctx, tokenSource))

		isInOrg, err := isUserInEpicOrg(client)
		if err != nil {
			http.Error(w, fmt.Sprintf("error getting org status: %v", err), 500)
			return
		}

		if !isInOrg {
			log.Println("User was not in the EpicGames organisation")
			http.Error(w, fmt.Sprintf("You are not in the EpicGames organisation. Please follow these directions and try again: https://www.unrealengine.com/en-US/ue-on-github"), 403)
			return
		}

		user, _, err := client.Users.Get(ctx, "")
		if err != nil {
			http.Error(w, fmt.Sprintf("error getting authenticated user: %v", err), 500)
			return
		}

		log.Printf("User %s was in the EpicGames organisation\n", *user.Name)

		err = addUserAsExternalCollaborator(appClient, *user.Login)
		if err != nil {
			http.Error(w, fmt.Sprintf("Could not add you as an external collaborator: %v", err), 500)
			return
		}

		http.Redirect(w, r, "https://github.com/SatisfactoryModding/UnrealEngine", http.StatusSeeOther)
	}
}

func getAccessToken(code, ghClientID, ghClientSecret string) (string, error) {
	var b bytes.Buffer
	resp, err := http.Post(fmt.Sprintf("https://github.com/login/oauth/access_token?client_id=%s&client_secret=%s&code=%s",
		ghClientID, ghClientSecret, code), "application/json", &b)
	if err != nil {
		return "", errors.Wrap(err, "error posting to GitHub")
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "error reading GitHub's response")
	}
	query, err := url.ParseQuery(string(data))
	if err != nil {
		return "", errors.Wrap(err, "error parsing returned query")
	}
	return query.Get("access_token"), nil
}

func isUserInEpicOrg(client *github.Client) (bool, error) {
	ctx := context.Background()

	_, _, err := client.Repositories.Get(ctx, "SatisfactoryModdingUE", "UnrealEngine")
	if err, ok := err.(*github.ErrorResponse); ok { // We rely on an implementation bug to check if the user can access a repo
		if err.Response.StatusCode == 404 {
			return false, nil
		}
		if err.Response.StatusCode == 403 {
			return true, nil
		}
	}
	if err != nil {
		return false, errors.Wrap(err, "error getting org memberships")
	}

	return true, nil
}

func addUserAsExternalCollaborator(authenticatedClient *github.Client, user string) error {
	_, _, err := authenticatedClient.Repositories.AddCollaborator(context.Background(), "SatisfactoryModding", "UnrealEngine", user, &github.RepositoryAddCollaboratorOptions{
		Permission: "pull",
	})
	return err
}
