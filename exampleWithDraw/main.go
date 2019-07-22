package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/zboyco/bililive"
)

func main() {
	fmt.Print("请输入BiliBili直播房间号: ")
	reader := bufio.NewScanner(os.Stdin)
	roomIDStr := ""
	if reader.Scan() {
		roomIDStr = reader.Text()
	}
	roomID, err := strconv.Atoi(roomIDStr)
	if err != nil {
		log.Fatalln("请输入正确的房间号")
	}
	if roomID <= 0 {
		log.Fatalln("房间号错误!")
	}
	m := &memberModel{
		body: make(map[string]bool),
		arr:  make([]string, 0),
	}
	var point string
	var run bool
	go startWeb(m, &point, &run)
	liveRoom := &bililive.LiveRoom{
		RoomID: roomID,
		ReceiveMsg: func(msg *bililive.MsgModel) {
			// log.Printf("【弹幕】%v:  %v", msg.UserName, msg.Content)
			if run && point != "" && msg.Content == point {
				// if run {
				m.Lock()
				if _, ok := m.body[msg.UserName]; !ok {
					m.body[msg.UserName] = true
					m.arr = append(m.arr, msg.UserName)
				}
				m.Unlock()
			}
		},
		ReceivePopularValue: func(value uint32) {
			log.Printf("【人气】:  %v", value)
		},
	}
	liveRoom.Start()
}
