package reTGanslatorBot

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	urlpath "path"

	"github.com/DzyubSpirit/reTGanslatorBot/bot"
	"github.com/DzyubSpirit/reTGanslatorBot/config"
	_ "github.com/GoogleCloudPlatform/functions-framework-go/funcframework"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

func init() {
	v, ok := os.LookupEnv("RUN_LOCAL")
	srv := NewServer(nil, "")
	if ok && v != "true" && v != "false" {
		log.Fatalln("wrong value for RUN_LOCAL env, expected 'true' of 'false'")
	}
	if ok && v == "false" {
		srv = NewServerFromEnv()
	}
	functions.HTTP("WebhookHandler", srv.ServeHTTP)
}

func NewServerFromEnv() *Server {
	botWebhookToken := os.Getenv("WEBHOOK_TOKEN")

	botToken := os.Getenv("BOT_TOKEN")
	if botToken == "" {
		log.Fatalf("BOT_TOKEN has to be specified")
	}

	path := "serverless_function_source_code/config.json"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Fall back to the current working directory if that file doesn't exist.
		path = "config.json"
	}

	configFile, err := os.Open(path)
	if err != nil {
		log.Fatalf("Failed to open config.json: %v", err)
	}

	configBytes, err := ioutil.ReadAll(configFile)
	if err != nil {
		log.Fatalf("Failed to read config.json: %v", err)
	}

	var config config.Config
	if err = json.Unmarshal(configBytes, &config); err != nil {
		log.Fatalf("Failed to parse config.json: %v", err)
	}

	tgBot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatalf("Bot API failed to initialize: %v", err)
	}

	u := bot.NewHandler(config, tgBot)

	tgBot.Debug = true

	log.Printf("Authorized on account %s", tgBot.Self.UserName)

	return NewServer(u, botWebhookToken)
}

type updater interface {
	HandleUpdate(tgbotapi.Update) error
}

type Server struct {
	token   string
	updater updater
	handler http.Handler
}

func NewServer(updater updater, token string) *Server {
	s := Server{
		updater: updater,
		token:   token,
	}
	s.handler = s.buildHandler()
	return &s
}

func (s Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r)
}

func (s Server) buildHandler() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/webhook/"+s.token, s.updateHandler)

	return mux
}

func (s Server) updateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		log.Printf("Unknown HTTP method: %s", r.Method)
		httpErr(w, http.StatusMethodNotAllowed)
		return
	}

	if token := urlpath.Base(r.URL.Path); token != s.token {
		log.Printf("Wrong token %s", token)
		httpErr(w, http.StatusNotFound)
		return
	}

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Printf("Failed to read the incoming payload: %v", err)
		httpErr(w, http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	var update tgbotapi.Update
	if err := json.Unmarshal(data, &update); err != nil {
		log.Printf("Failed to unmarshal incoming update: %v", err)
		httpErr(w, http.StatusBadRequest)
		return
	}

	if err := s.updater.HandleUpdate(update); err != nil {
		log.Printf("Handle incoming update: %v", err)
		httpErr(w, http.StatusInternalServerError)
	}
}

func httpErr(w http.ResponseWriter, code int) {
	http.Error(w, http.StatusText(code), code)
}
