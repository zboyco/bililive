package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
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
	m := &memberModel{}
	m.Reset()
	var point string
	var run bool
	go startWeb(m, &point, &run)
	live := &bililive.Live{
		Debug:       false,
		StormFilter: true, // 过滤节奏风暴弹幕
		ReceiveMsg: func(roomID int, msg *bililive.MsgModel) {
			// log.Printf("【弹幕】%v:  %v", msg.UserName, msg.Content)
			if run && point != "" && msg.Content == point {
				m.Add(msg.UserID, msg.UserName)
			}
		},
		ReceivePopularValue: func(roomID int, value uint32) {
			log.Printf("【人气】:  %v", value)
		},
	}
	fmt.Println()
	_ = open("http://127.0.0.1:8080/html")
	fmt.Println("浏览器输入 http://127.0.0.1:8080/html 访问...")
	fmt.Println()
	live.Start(context.TODO())
	_ = live.Join(roomID)
	live.Wait()
}

func open(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start"}
	case "darwin":
		cmd = "open"
	default: // "linux", "freebsd", "openbsd", "netbsd"
		cmd = "xdg-open"
	}
	args = append(args, url)
	return exec.Command(cmd, args...).Start()
}
