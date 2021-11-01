package main

import (
	"fmt"
	"gopkg.in/ini.v1"
)

type Config struct {
	botHash string
	sheetId string
	admin   int64
	file    *ini.File
}

type advError struct {
	Op, desc string
	Err      error
	details  map[interface{}]interface{}
}

func (e *advError) Error() string {
	var details string
	var errMsg string
	if e.Err != nil {
		errMsg = e.Err.Error()
	}
	if len(e.details) == 0 {
		return e.Op + ":" + e.desc + ", err:" + errMsg
	}
	for k, v := range e.details {
		details += fmt.Sprintf("%s: %s;", k, v)
	}
	return fmt.Sprintf("%s: %s; err: %s; details: %s", e.Op, e.desc, errMsg, details)
}
func (e *advError) Unwrap() error { return e.Err }

func (cfg *Config) loadConfig() error {
	var err error
	cfg.file, err = ini.Load(configPath)
	if err != nil {
		return &advError{Op: "loadConfig", desc: "Не удалось загрузить ini файл", Err: err}
	}
	cfg.botHash = cfg.file.Section("main").Key("bot_hash").Value()
	cfg.sheetId = cfg.file.Section("main").Key("spread_sheet").Value()
	cfg.admin, err = cfg.file.Section("main").Key("admin_id").Int64()
	if err != nil {
		return &advError{Op: "loadConfig", desc: "Не удалось загрузить админа", Err: err}
	}

	return nil
}
