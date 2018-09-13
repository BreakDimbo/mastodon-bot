package main

import (
	"bot/bredis"
	"bot/config"
	"bot/const"
	"bot/log"
	"bot/theater/bot"
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis"
)

func sendLine(actors map[string]*bot.Actor) {
	filename := config.ScriptFilePath()
	f, err := os.Open(filename)
	if err != nil {
		panic(err)
	}

	defer func() {
		for _, actor := range actors {
			close(actor.LineCh)
		}
	}()

	defer wg.Done()

	var id int
	input := bufio.NewScanner(f)
	for input.Scan() {
		id++
		content := input.Text()
		ep, name, line, err := parseText(content)
		if err != nil {
			log.SLogger.Errorf("parse text:[%s] error: %v", content, err)
			continue
		}

		acted, err := checkActed(ep, strconv.Itoa(id))
		if acted || err != nil {
			continue
		}

		for checkNight() {
			time.Sleep(5 * time.Minute)
		}

		actor, ok := actors[name]
		if !ok {
			log.SLogger.Errorf("not find actor by name: %s on line id: %d", name, id)
			continue
		}

		select {
		case actor.LineCh <- line:
			log.SLogger.Infof("acts ep %s id %d", ep, id)
		default:
			log.SLogger.Errorf("actor %s LineCh blocked with line id: %d", actor.Name, id)
		}

		time.Sleep(20 * time.Minute)
	}
}

/*
line example:

ep/id/name/line
*/
func parseText(content string) (string, string, string, error) {
	s := strings.Split(content, "/")
	if len(s) < 3 {
		return "", "", "", fmt.Errorf("split content [%s] len less 3 error", content)
	}

	ep, name, line := s[0], s[1], s[2]
	return ep, name, line, nil
}

func checkActed(ep string, id string) (bool, error) {
	key := fmt.Sprintf("%s:%s", cons.Stein, ep)
	value, err := bredis.Client.Get(key).Result()
	if err == nil {
		valueInt, _ := strconv.Atoi(value)
		idInt, _ := strconv.Atoi(id)

		if idInt <= valueInt {
			return true, nil
		}

		err := bredis.Client.Set(key, id, 7*24*time.Hour).Err()
		if err != nil {
			log.SLogger.Errorf("set ep %s with id %s from redis error: %v", ep, id, err)
			return false, err
		}
		return false, nil

	} else if err == redis.Nil {
		err := bredis.Client.Set(key, id, 7*24*time.Hour).Err()
		if err != nil {
			log.SLogger.Errorf("set ep %s with id %s from redis error: %v", ep, id, err)
			return false, err
		}
		return false, nil
	}

	log.SLogger.Errorf("get ep %s with id %s from redis error: %v", ep, id, err)
	return false, err
}

func checkNight() bool {
	now := time.Now()
	start := 11
	end := 20
	if now.Hour() > start && now.Hour() < end {
		return true
	}
	return false
}
