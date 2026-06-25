package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"net/http"
	"net/url"
	"runtime"
	"github.com/go-fluxer/gofluxer"
)

type Config struct {
	Token        string `json:"FLUXERBOTTOKEN"`
	Prefix       string `json:"FLUXERBOTPREFIX"`
	Webport      string `json:"WEBPORT"`
	ClientID     string `json:"OAUTH_CLIENTID"`
	ClientSecret string `json:"OAUTH_CLIENTSECRET"`
	RedirectURI  string `json:"OAUTH_REDIRECTURI"`
}
func LoadConfig(filename string) (Config, error) {
	var config Config
	configFile, err := os.Open(filename)
	if err != nil {
		return config, err
	}
	defer configFile.Close()
	jsonParser := json.NewDecoder(configFile)
	err = jsonParser.Decode(&config)
	return config, err
}

type GuildSettings struct {
	LogChannelID string `json:"log_channel_id"`
}
type Database struct {
	mu       sync.RWMutex
	Filename string
	Guilds   map[string]GuildSettings
}
var db *Database
func InitDB(filename string) *Database {
	d := &Database{
		Filename: filename,
		Guilds:   make(map[string]GuildSettings),
	}
	file, err := os.ReadFile(filename)
	if err == nil {
		json.Unmarshal(file, &d.Guilds)
	}
	return d
}
func (d *Database) Save() error {
	data, err := json.MarshalIndent(d.Guilds, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(d.Filename, data, 0644)
}

func main() {
	cfg, err := LoadConfig("config.json")
	if err != nil {
		fmt.Printf("Something went wrong while loading config.json file: %v\n", err)
		return
	}
	db = InitDB("guilds.json")

	bot := gofluxer.NewBot(cfg.Token, cfg.Prefix)
	// bot.NewBotInstance("https://example.com/api/v1", "wss://example.com/gateway/?v=1")
	// The bot.NewBotInstance() function allows you to use a 3rd party Fluxer instance instead of using the main one.
	bot.NewBotConfig(true, 500)



	bot.OnReady(func() {
		fmt.Println("Demo [Fluxer.app] Dashboard Bot is Ready")
	})

	bot.OnUserJoin(func(u *gofluxer.UserJoinPayload) {
		db.mu.RLock()
		settings, exists := db.Guilds[u.GuildID]
		db.mu.RUnlock()
		if exists && settings.LogChannelID != "" {
			welcomeMsg := fmt.Sprintf("**MEMBER JOINED:** Welcome %s to %s!", u.User.Username, u.GuildID)
			bot.SendMessage(settings.LogChannelID, welcomeMsg)
		}
	})

	bot.OnUserLeave(func(l *gofluxer.UserLeavePayload) {
		db.mu.RLock()
		settings, exists := db.Guilds[l.GuildID]
		db.mu.RUnlock()
		if exists && settings.LogChannelID != "" {
			welcomeMsg := fmt.Sprintf("**MEMBER LEAVE:** %s (ID: %s) has left %s.\n", l.User.Username, l.UserID, l.GuildName)
			bot.SendMessage(settings.LogChannelID, welcomeMsg)
		}
	})

	bot.OnMessageDelete(func(d *gofluxer.MessageDeletePayload) {
		db.mu.RLock()
		settings, exists := db.Guilds[d.GuildID]
		db.mu.RUnlock()
		if exists && settings.LogChannelID != "" {
			deleteMsg := fmt.Sprintf("**MESSAGE DELETED:** The following message ID %s was deleted from <#%s>.", d.MessageID, d.ChannelID)
			bot.SendMessage(settings.LogChannelID, deleteMsg)
		}
	})

	// MAIN COMMANDS

	bot.AddCommand("help", func(m *gofluxer.Message, args []string) {
		bot.SendEmbed(m.ChannelID, map[string]interface{}{
			"title":       "Bot",
			"description": "help\nping\nsay\nesay\nsetlogchannel\ndisablelogchannel",
			"color":       0x00FFFF,
		})
	})

	bot.AddCommand("ping", func(m *gofluxer.Message, args []string) {
		bot.SendMessage(m.ChannelID, "Pong! 🏓")
	})

	bot.AddCommand("say", func(m *gofluxer.Message, args []string) {
		if len(args) == 0 {
			bot.SendMessage(m.ChannelID, "What do you want me to say?")
			return
		}
		bot.SendMessage(m.ChannelID, strings.Join(args, " "))
	})

	bot.AddCommand("esay", func(m *gofluxer.Message, args []string) {
		fullInput := strings.Join(args, " ")
		parts := strings.Split(fullInput, "|")
		if len(parts) < 2 {
			bot.SendMessage(m.ChannelID, "Usage: essay Title | Description")
			return
		}

		bot.SendEmbed(m.ChannelID, map[string]interface{}{
			"title":       strings.TrimSpace(parts[0]),
			"description": strings.TrimSpace(parts[1]),
			"color":       0x00FFFF,
		})
	})

	// CONFIG COMMANDS

	bot.AddCommand("setlogchannel", func(m *gofluxer.Message, args []string) {
		if !bot.IsOwner(m) {
			bot.SendMessage(m.ChannelID, "You need to have the **SERVER OWNER** permission to use this command.")
			return
		}
		if len(args) == 0 {
			bot.SendMessage(m.ChannelID, "Arguments required. Please provide a valid target Channel ID.")
			return
		}

		targetChannelID := args[0]
		db.mu.Lock()
		db.Guilds[m.GuildID] = GuildSettings{LogChannelID: targetChannelID}
		err := db.Save()
		db.mu.Unlock()
		if err != nil {
			bot.SendMessage(m.ChannelID, "An error occured while saving the configuration.")
			return
		}
		bot.SendMessage(m.ChannelID, fmt.Sprintf("Logging channel successfully set to <#%s>!", targetChannelID))
	})

	bot.AddCommand("disablelogchannel", func(m *gofluxer.Message, args []string) {
		if !bot.IsOwner(m) {
			bot.SendMessage(m.ChannelID, "You need to have the **SERVER OWNER** permission to use this command.")
			return
		}

		db.mu.Lock()
		_, exists := db.Guilds[m.GuildID]
		if exists {
			delete(db.Guilds, m.GuildID)
			err = db.Save()
		}
		db.mu.Unlock()
		if !exists {
			bot.SendMessage(m.ChannelID, "Logging is already disabled or wasn't set up for this server.")
			return
		}
		if err != nil {
			bot.SendMessage(m.ChannelID, "An error occured while saving the configuration.")
			return
		}
		bot.SendMessage(m.ChannelID, "Logging has been disabled.")
	})



	// WEB DASHBOARD

	oauth := gofluxer.NewOAuth(cfg.ClientID, cfg.ClientSecret, cfg.RedirectURI)
	var activeToken string

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if activeToken == "" {
			loginURL := fmt.Sprintf("https://web.fluxer.app/oauth2/authorize?client_id=%s&redirect_uri=%s&response_type=code&scope=identify+guilds", cfg.ClientID, url.QueryEscape(cfg.RedirectURI))
			fmt.Fprintf(w, "<h1>Fluxer Dashboard Demo Project</h1><a href='%s'>Login with Fluxer</a>", loginURL)
			return
		}

		user, _ := oauth.GetUser(activeToken)
		guilds, _ := oauth.GetGuilds(activeToken)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		fmt.Fprintf(w, "<h2>Welcome, %s (ID: %s)</h2>", user.Username, user.ID)
		fmt.Fprint(w, "<h3>Your Administered Servers:</h3><ul>")
		for _, g := range guilds {
			isTrueOwner := (user.ID != "" && g.OwnerID != "" && user.ID == g.OwnerID)
			var hasAdmin bool
			var permValue uint64
			_, err := fmt.Sscanf(g.Permissions, "%d", &permValue)
			if err == nil {
				hasAdmin = (permValue & 8) == 8 || permValue == 18446744073709551615
			}

			if isTrueOwner || hasAdmin {
				if !bot.IsBotInGuild(g.ID) {
					fmt.Fprintf(w, `<li>
						<strong>%s</strong> (ID: %s) <br>
						<span style="color:orange;">⚠️ Bot not in server.</span> <br>
						<a href="https://fluxer.app/oauth2/authorize?client_id=%s&scope=bot" target="_blank">Invite Bot</a>
						<hr>
					</li>`, g.Name, g.ID, cfg.ClientID)
					continue
				}

				channels, err := bot.GetGuildChannels(g.ID)
				var dropdownOptions strings.Builder

				if err != nil {
					dropdownOptions.WriteString("<option value=''>Error loading channels</option>")
				} else {
					dropdownOptions.WriteString("<option value=''>-- Select a Log Channel --</option>")
					
					db.mu.RLock()
					currentSettings := db.Guilds[g.ID]
					db.mu.RUnlock()

					for _, ch := range channels {
						if ch.Type == 0 { 
							selected := ""
							if ch.ID == currentSettings.LogChannelID {
								selected = "selected"
							}
							fmt.Fprintf(&dropdownOptions, `<option value="%s" %s>#%s</option>`, ch.ID, selected, ch.Name)
						}
					}
				}

				db.mu.RLock()
				currentSettings := db.Guilds[g.ID]
				db.mu.RUnlock()

				fmt.Fprintf(w, `<li>
					<strong>%s</strong> (ID: %s) <br>
					<span style="color:green;">✅ Bot Active</span> <br>
					Current Log Channel ID: <code>%s</code>
					<form action="/update-guild" method="POST" style="margin-top:5px;">
						<input type="hidden" name="guild_id" value="%s">
						<select name="log_channel" style="padding: 4px; margin-right: 5px;">%s</select>
						<button type="submit">Save Settings</button>
						<button type="submit" name="action" value="disable" style="color:red; margin-left: 5px;">Disable Logging</button>
					</form>
					<form action="/send-message" method="POST" style="margin-top:5px;">
						<input type="hidden" name="guild_id" value="%s">
						<input type="text" name="msg" placeholder="Send broadcast message...">
						<button type="submit">Broadcast</button>
					</form>
					<hr>
				</li>`, g.Name, g.ID, currentSettings.LogChannelID, g.ID, dropdownOptions.String(), g.ID)
			}
		}
		fmt.Fprint(w, "</ul><a href='/logout'>Logout</a>")
	})

	http.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var mem runtime.MemStats
		runtime.ReadMemStats(&mem)

		statusPayload := map[string]interface{}{
			"fluxerbot_prefix":            cfg.Prefix,
			"fluxerbot_id":                cfg.ClientID,
			"fluxerbot_guilds":            len(db.Guilds),
			"target_framework":            runtime.Version(),
			"osplatform":                  runtime.GOOS,
			"target_framework_goroutines": runtime.NumGoroutine(),
			"memory":                      float64(mem.Alloc) / 1024 / 1024,
		}

		json.NewEncoder(w).Encode(statusPayload)
	})

	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "Missing authorization token code", http.StatusBadRequest)
			return
		}
		tokenResp, err := oauth.AuthCode(code)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		activeToken = tokenResp.AccessToken
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	http.HandleFunc("/update-guild", func(w http.ResponseWriter, r *http.Request) {
		if activeToken == "" || r.Method != http.MethodPost {
			http.Error(w, "Unauthorized action block", http.StatusUnauthorized)
			return
		}

		guildID := r.FormValue("guild_id")
		logChannel := r.FormValue("log_channel")
		action := r.FormValue("action")
		user, _ := oauth.GetUser(activeToken)
		guilds, _ := oauth.GetGuilds(activeToken)

		isAuthorized := false
		for _, g := range guilds {
			if g.ID == guildID {
				isTrueOwner := (user.ID != "" && g.OwnerID != "" && user.ID == g.OwnerID)
				var hasAdmin bool
				var permValue uint64
				_, err := fmt.Sscanf(g.Permissions, "%d", &permValue)
				if err == nil {
					hasAdmin = (permValue & 8) == 8 || permValue == 18446744073709551615
				}
				if isTrueOwner || hasAdmin {
					isAuthorized = true
				}
				break
			}
		}
		if !isAuthorized {
			http.Error(w, "You need to have the **SERVER OWNER** permission to manage this server.", http.StatusForbidden)
			return
		}

		db.mu.Lock()
		if action == "disable" {
			delete(db.Guilds, guildID)
		} else {
			db.Guilds[guildID] = GuildSettings{LogChannelID: logChannel}
		}
		db.Save()
		db.mu.Unlock()
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	http.HandleFunc("/send-message", func(w http.ResponseWriter, r *http.Request) {
		if activeToken == "" || r.Method != http.MethodPost {
			http.Error(w, "Unauthorized action", http.StatusUnauthorized)
			return
		}

		guildID := r.FormValue("guild_id")
		msg := r.FormValue("msg")
		user, _ := oauth.GetUser(activeToken)
		guilds, _ := oauth.GetGuilds(activeToken)
		isAuthorized := false
		for _, g := range guilds {
			if g.ID == guildID {
				isTrueOwner := (user.ID != "" && g.OwnerID != "" && user.ID == g.OwnerID)
				var hasAdmin bool
				var permValue uint64
				_, err := fmt.Sscanf(g.Permissions, "%d", &permValue)
				if err == nil {
					hasAdmin = (permValue & 8) == 8 || permValue == 18446744073709551615
				}
				if isTrueOwner || hasAdmin {
					isAuthorized = true
				}
				break
			}
		}
		if !isAuthorized || msg == "" {
			http.Error(w, "Action forbidden or input empty", http.StatusForbidden)
			return
		}

		db.mu.RLock()
		settings, exists := db.Guilds[guildID]
		db.mu.RUnlock()
		if exists && settings.LogChannelID != "" {
			bot.SendMessage(settings.LogChannelID, fmt.Sprintf("**DASHBOARD BROADCAST:** %s", msg))
		}

		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	http.HandleFunc("/logout", func(w http.ResponseWriter, r *http.Request) {
		activeToken = ""
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	go func() {
		address := fmt.Sprintf(":%s", cfg.Webport)
		fmt.Printf("[Webserver] Webserver is starting. Running at port: %s...\n", cfg.Webport)
		if err := http.ListenAndServe(address, nil); err != nil {
			fmt.Printf("[Webserver] Webserver error: %v\n", err)
		}
	}()
	bot.Run()
}