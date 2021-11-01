package main

import (
	tg "github.com/go-telegram-bot-api/telegram-bot-api"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
)

var OutBox = make(chan tg.Chattable, 1000)
var summaryExpr = regexp.MustCompile(`(?i)^# ?сводка(?: (\d\d[.,]\d\d[.,])(?:(\d\d)|\d\d(\d\d)))?$`)
var recordExpr = regexp.MustCompile(`(?si-m)^(?:# ?([а-яa-z0-9ё_\-]+) +(\d{1,2}[:.,]\d{2})[ \n]+(?:# ?([а-яa-z0-9ё_\-]+)[ \n]?(?:(\d+(?:\.\d+)*)[ \n]?)?)?(.*?)$|# ?дата ((?:0?[1-9]|1\d|2\d|3[0,1])[.,](?:0?[1-9]|1[012])[.,])(?:(\d\d)$|\d\d(\d\d)$))`)
var financeExpr = regexp.MustCompile(`(?si)^# ?(доход|расход) (\d+) (?:# ?([^ \n]+))?(?: (.*))?$`)
var importantExpr = regexp.MustCompile(`(?si)^(# ?важный ?вопрос) ([^#]*)(?: (#?.*) (https?://.*))?$`)
const success = "Запись создана."
const fail = "Что-то пошло не так."

type Record struct {
	// 1 - первый хэштэг, 2 - время, 3 - второе хэштэг, 4 - номер задачи 5 - сообщение, 6 - дата
	FullTimeStamp string
	Name          string
	Chat          string
	ChatId        string
	FirstHash     string
	SecondHash    string
	TaskNumber    string
	Description   string
	Link          string
	EndTime       string
	Date          string
}

func listen() {
	go sender()
	u := tg.NewUpdate(0)
	u.Timeout = 60
	updates, _ := bot.GetUpdatesChan(u)
	for update := range updates {
		if msg := update.Message; msg != nil {
			go Messages(msg)
		}
	}
}
func actionsOnSuccess(chatId int64) {
	//OutBox <- tg.NewMessage(chatId, success)
}

func actionsOnFail(chatId int64, msg interface{}, err error) {
	OutBox <- tg.NewMessage(chatId, fail)
	logger.Println(msg, err)
}

func commandWrapper(chatId int64, msg interface{}, err error, needSuccess bool) {
	if err == nil {
		if needSuccess {
			actionsOnSuccess(chatId)
		}
	} else {
		actionsOnFail(chatId, msg, err)
	}
}

func Messages(msg *tg.Message) {
	defer func() {
		err := recover()
		if err != nil {
			logger.Println("В messages возникла ошибка: ", err, "stack: ", string(debug.Stack()))
		}
	}()

	if msg.ForwardFromMessageID != 0 {
		return
	}
	if strings.Contains(msg.Text, "@") {
		commandWrapper(msg.Chat.ID, "mentions error ,", mentions(msg.Chat.Title, makeLink(msg), msg.Text), true)
	}
	if int64(msg.From.ID) == cfg.admin {
		results := financeExpr.FindStringSubmatch(msg.Text)
		if results != nil {
			commandWrapper(msg.Chat.ID, "finance error ,", finance(results[1], results[2], results[3], results[4], makeLink(msg)), true)
			return
		}
	}

	results := summaryExpr.FindStringSubmatch(msg.Text)
	if results != nil {
		date := strings.Replace(results[1]+results[2]+results[3], ",", ".", -1)
		if date == "" {
			date = time.Now().In(loc).Format("02.01.06")
		}
		commandWrapper(msg.Chat.ID, "summary error ,", summary(msg.Chat.ID, msg.Chat.Title, date), false)
		return
	}
	results = importantExpr.FindStringSubmatch(msg.Text)
	if results != nil {
		if results[3] == "" {
			results[4] = makeLink(msg)
		}
		commandWrapper(msg.Chat.ID, "important error ,", important(results[1], results[2], results[3], results[4]), false)
		return
	}

	results = recordExpr.FindStringSubmatch(msg.Text)
	if results == nil {
		return
	}

	var newRecord Record
	// 1 - первый хэштэг, 2 - время, 3 - второе хэштэг, 4 - номер задачи 5 - сообщение, 6 - дата
	newRecord.FirstHash, newRecord.EndTime, newRecord.SecondHash, newRecord.TaskNumber, newRecord.Description, newRecord.Date = results[1], strings.Replace(strings.Replace(results[2], ".", ":", 1), ",", ":", 1), results[3], results[4], results[5], strings.Replace(results[6]+results[7]+results[8], ",", ".", -1)
	if newRecord.SecondHash != "" {
		descr := newRecord.Description
		newRecord.Description = "#" + newRecord.SecondHash + " "
		if newRecord.TaskNumber != "" {
			newRecord.Description += newRecord.TaskNumber + " "
		}
		newRecord.Description += descr
	}
	chat := strconv.FormatInt(msg.Chat.ID, 10)
	if chat[0:1] == "-" {
		if len(chat) > 4 && chat[:4] == "-100" {
			chat = chat[4:]
		} else {
			chat = chat[1:]
		}
	}
	newRecord.Link = makeLink(msg)
	newRecord.Name = msg.From.LastName + " " + msg.From.FirstName
	newRecord.Chat = msg.Chat.Title
	newRecord.ChatId = chat
	commandWrapper(msg.Chat.ID, newRecord, sendRecord(newRecord), true)
}

func sender() {
	defer func() {
		err := recover()
		if err != nil {
			logger.Println("В sender возникла ошибка: ", err, "stack: ", string(debug.Stack()))
		}
		go sender()
	}()
	for msg := range OutBox {
		_, err := bot.Send(msg)
		if err != nil {
			logger.Println("Бот не смог отправить сообщение: ", msg, " err: ", err)
		}
	}
}

func makeLink(msg *tg.Message) string {
	chat := strconv.FormatInt(msg.Chat.ID, 10)
	if chat[0:1] == "-" {
		if len(chat) > 4 && chat[:4] == "-100" {
			chat = chat[4:]
		} else {
			chat = chat[1:]
		}
	}
	msgId := strconv.Itoa(msg.MessageID)
	return "https://t.me/c/" + chat + "/" + msgId
}
