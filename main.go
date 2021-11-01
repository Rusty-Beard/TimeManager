package main

import (
	"flag"
	tg "github.com/go-telegram-bot-api/telegram-bot-api"
	"google.golang.org/api/sheets/v4"
	"log"
	"os"
	"runtime/debug"
	"time"
)

var (
	cfg        Config
	bot        *tg.BotAPI
	srv        *sheets.Service
	logger     *log.Logger
	logPath    string
	configPath string
	credPath   string
	loc        *time.Location
)

func init() {
	flag.StringVar(&configPath, "config", "./config.ini", "path to config file")
	flag.StringVar(&logPath, "log", "./bot.log", "path to log file")
	flag.StringVar(&credPath, "creds", "./credentials.json", "path to client_secret file")
	flag.Parse()

	logfile, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		logger = log.New(os.Stdout, "", log.LstdFlags)
	} else {
		logger = log.New(logfile, "", log.LstdFlags)
	}

	err = cfg.loadConfig()
	if err != nil {
		logger.Fatal(err)
	}

	srv, err = initSheets()
	if err != nil {
		logger.Fatal(err)
	}

	bot, err = tg.NewBotAPI(cfg.botHash)
	if err != nil {
		logger.Fatal("Не удалось запустить бота" + err.Error())
	}

	loc, err = time.LoadLocation("Asia/Bangkok")
	if err != nil {
		logger.Fatal("Не удалось инициализировать время " + err.Error())
	}
}

func main() {
	logger.Println("Начинаю работу")
	defer Quit()
	listen()
}
func Quit() {
	err := recover()
	logger.Println("Завершение main:", err, "stack: ", string(debug.Stack()))
	if err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}
