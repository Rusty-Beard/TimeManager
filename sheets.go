package main

import (
	"context"
	tg "github.com/go-telegram-bot-api/telegram-bot-api"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
	"io/ioutil"
	"sort"
	"strconv"
	"strings"
	"time"
)

func initSheets() (*sheets.Service, error) {
	secret, err := ioutil.ReadFile(credPath)
	if err != nil {
		return nil, &advError{Op: "initSheets", desc: "Не удалось прочитать secret file ", Err: err}
	}

	config, err := google.JWTConfigFromJSON(secret, "https://www.googleapis.com/auth/spreadsheets")
	if err != nil {
		return nil, &advError{Op: "initSheets", desc: "Неверный файл credentials.json", Err: err}
	}
	client := config.Client(context.Background())

	srv, err := sheets.NewService(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		return nil, &advError{Op: "initSheets", desc: "Не удалось инициализировать клиент gApi", Err: err}
	}
	return srv, nil
}

func createSheetTitle(title, id string) string {
	return title + " (" + id + ")"
}

func findSheetName(chatId string) (string, int64, error) {
	resp, err := srv.Spreadsheets.Get(cfg.sheetId).Do()
	if err != nil {
		return "", 0, err
	}
	for _, sheet := range resp.Sheets {
		if strings.Contains(sheet.Properties.Title, chatId) {
			return sheet.Properties.Title, sheet.Properties.SheetId, nil
		}
	}
	return "", 0, nil
}

func addSheet(rec Record) error {
	var vr sheets.ValueRange
	vr.Values = make([][]interface{}, 1)
	if rec.Date != "" {
		vr.Values[0] = make([]interface{}, 11)
		vr.Values[0][8] = rec.Date
	} else {
		var Values []interface{}
		Values = append(Values, time.Now().In(loc).Format("02.01.2006 15:04:05"), rec.Name, rec.FirstHash, rec.Description, rec.Link, nil, rec.EndTime, nil, nil, rec.SecondHash, rec.TaskNumber)
		vr.Values[0] = Values
	}

	req := sheets.Request{
		AddSheet: &sheets.AddSheetRequest{
			Properties: &sheets.SheetProperties{
				Title: createSheetTitle(rec.Chat, rec.ChatId),
			},
		},
	}

	rbb := &sheets.BatchUpdateSpreadsheetRequest{
		Requests: []*sheets.Request{&req},
	}

	_, err := srv.Spreadsheets.BatchUpdate(cfg.sheetId, rbb).Context(context.Background()).Do()
	if err != nil {
		return &advError{Op: "sendRecord", desc: "Не удалось создать таблицу", Err: err}
	}
	_, err = srv.Spreadsheets.Values.Append(cfg.sheetId, createSheetTitle(rec.Chat, rec.ChatId), &vr).ValueInputOption("USER_ENTERED").Do()
	if err != nil {
		return &advError{Op: "sendRecord", desc: "Не удалось добавить значения в новую таблицу", Err: err}
	}
	return nil
}

func sendRecord(rec Record) error {
	var vr sheets.ValueRange
	vr.Values = make([][]interface{}, 2)
	title, id, err := findSheetName(rec.ChatId)
	if err != nil {
		return &advError{Op: "sendRecord", desc: "Ошибка при получении страниц", Err: err}
	}
	if title == "" {
		return addSheet(rec)
	}

	vr1, err := srv.Spreadsheets.Values.Get(cfg.sheetId, title).Do()
	if err != nil {
		return &advError{Op: "sendRecord", desc: "Ошибка при получении данных из gSheets", Err: err}
	}

	if rec.Date != "" {
		vr.Values[0] = make([]interface{}, 11)
		vr.Values[1] = make([]interface{}, 11)
		vr.Values[1][8] = rec.Date
	} else {
		var Values []interface{}
		idx := strconv.Itoa(len(vr1.Values) + 1)
		Values = append(Values, time.Now().In(loc).Format("02.01.2006 15:04:05"), rec.Name, rec.FirstHash, rec.Description, rec.Link, nil, rec.EndTime, "=G"+idx+"-F"+idx+"+(F"+idx+">G"+idx+")", nil, rec.SecondHash, rec.TaskNumber)
		vr.Values[0] = Values

		if len(vr1.Values[len(vr1.Values)-1]) > 6 {
			vr.Values[0][5] = vr1.Values[len(vr1.Values)-1][6]
		} else {
			vr.Values[0][5] = ""
		}

		if len(vr1.Values[len(vr1.Values)-1]) < 9 {
			vr.Values[0][8] = ""
		} else {
			vr.Values[0][8] = vr1.Values[len(vr1.Values)-1][8].(string)
		}
	}

	_, err = srv.Spreadsheets.Values.Append(cfg.sheetId, title+"!A"+strconv.Itoa(len(vr1.Values)+1), &vr).ValueInputOption("USER_ENTERED").Do()
	if err != nil {
		return &advError{Op: "sendRecord", desc: "Не удалось добавить значения в таблицу", Err: err}
	}

	if title != createSheetTitle(rec.Chat, rec.ChatId) {
		renameRequest := sheets.Request{
			UpdateSheetProperties: &sheets.UpdateSheetPropertiesRequest{
				Properties: &sheets.SheetProperties{Title: createSheetTitle(rec.Chat, rec.ChatId), SheetId: id},
				Fields:     "title",
			},
		}

		rbb := &sheets.BatchUpdateSpreadsheetRequest{
			Requests: []*sheets.Request{&renameRequest},
		}

		_, err := srv.Spreadsheets.BatchUpdate(cfg.sheetId, rbb).Context(context.Background()).Do()
		if err != nil {
			return &advError{Op: "sendRecord", desc: "Не удалось обновить имя таблицы", Err: err}
		}
	}

	return nil
}

func summary(id int64, title string, date string) error {
	valueRange, err := srv.Spreadsheets.Values.Get(cfg.sheetId, createSheetTitle(title, strconv.FormatInt(id, 10)[4:])).Do()
	if err != nil {
		return err
	}

	filtered := filterValues(valueRange.Values, date)
	if len(filtered) < 2 {
		OutBox <- tg.NewMessage(id, "За эту дату не было записей")
		return nil
	}

	filtered = filtered[1:]
	tasks := make(map[string]time.Duration)
	first := make(map[string]time.Duration)
	for _, value := range filtered {
		if len(value) < 8 || value[7].(string) == "" {
			continue
		}
		duration, err := time.ParseDuration(strings.Replace(value[7].(string), ":", "h", 1) + "m")
		if err != nil {
			continue
		}
		if value[2].(string) != "" {
			first[strings.TrimSpace(strings.ToLower(value[2].(string)))] += duration
		}

		if len(value) < 10 || value[9].(string) == "" {
			continue
		}
		tasks[strings.TrimSpace(strings.ToLower(value[9].(string)))] += duration
	}

	newMsg := tg.NewMessage(id, formatTasks(tasks, first, date))
	newMsg.ParseMode = "Markdown"
	OutBox <- tg.NewMessage(id, formatTasks(tasks, first, date))
	return nil
}
func formatTasks(tasks, first map[string]time.Duration, date string) string {
	result := "#СВОДКА " + date
	result += "\n\nВремязатраты на задачи:"
	keys := make([]string, 0, len(tasks))
	for k := range tasks {
		keys = append(keys, k)
	}

	sort.Slice(keys, func(i, j int) bool { return tasks[keys[i]] > tasks[keys[j]] })
	for _, v := range keys {
		duration := tasks[v].String()
		duration = duration[:len(duration)-2]
		result += "\n#" + v + " " + strings.Replace(strings.Replace(duration, "h", "ч", 1), "m", "м", 1)
	}

	result += "\n\nИтого:"
	keys = make([]string, 0, len(first))
	for k := range first {
		keys = append(keys, k)
	}

	sort.Slice(keys, func(i, j int) bool { return first[keys[i]] > first[keys[j]] })
	for _, v := range keys {
		duration := first[v].String()
		duration = duration[:len(duration)-2]
		result += "\n#" + v + " " + strings.Replace(strings.Replace(duration, "h", "ч", 1), "m", "м", 1)
	}

	return result
}
func filterValues(values [][]interface{}, date string) [][]interface{} {
	var result [][]interface{}
	for _, value := range values {
		if len(value) > 8 && value[8].(string) == date {
			result = append(result, value)
		}
	}
	return result
}

func finance(operation, sum, name, description, link string) error {
	var vr sheets.ValueRange
	var Values []interface{}
	Values = append(Values, "#"+operation)

	if strings.ToLower(operation) == "доход" {
		Values = append(Values, sum, "")
	} else {
		Values = append(Values, "", sum)
	}

	if name == "" {
		name = ""
	} else {
		name = "#" + name
	}

	Values = append(Values, name, description, link)
	vr.Values = append(vr.Values, Values)
	_, err := srv.Spreadsheets.Values.Append(cfg.sheetId, "УЧЕТ ФИНАСОВ", &vr).ValueInputOption("USER_ENTERED").Do()
	return err
}

func important(first, description, second, link string) error {
	var vr sheets.ValueRange
	var Values []interface{}
	Values = append(Values, time.Now().In(loc).Format("02.01.2006 15:04:05"), first, description, second, link)
	vr.Values = append(vr.Values, Values)
	_, err := srv.Spreadsheets.Values.Append(cfg.sheetId, "СБОРА ВАЖНЫХ ВОПРОСОВ", &vr).ValueInputOption("USER_ENTERED").Do()
	return err
}

func mentions(chat, link, message string) error {
	var vr sheets.ValueRange
	var Values []interface{}
	Values = append(Values, time.Now().In(loc).Format("02.01.2006 15:04:05"), chat, link, message)
	vr.Values = append(vr.Values, Values)
	_, err := srv.Spreadsheets.Values.Append(cfg.sheetId, "УПОМИНАНИЯ", &vr).ValueInputOption("USER_ENTERED").Do()
	return err
}
