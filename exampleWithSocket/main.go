package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"

	"github.com/zboyco/bililive"
)

type msg struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Count int    `json:"count"`
	User  string `json:"username"`
	Price int    `json:"price"`
}

func main() {
	ip := flag.String("ip", "", "ip")
	port := flag.Int("port", 0, "port")
	flag.Parse()

	socket := newSocket(*ip, *port)

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
	liveRoom := &bililive.LiveRoom{
		RoomID: roomID,
		ReceiveGift: func(gift *bililive.GiftModel) {
			log.Printf("【礼物】%v:  %v(%v) * %v  价格 %v  连击 %v", gift.UserName, gift.GiftName, gift.GiftID, gift.Num, gift.Price, gift.Combo)
			m := msg{
				ID:    gift.GiftID,
				Name:  gift.GiftName,
				Count: gift.Num,
				User:  gift.UserName,
				Price: gift.Price,
			}
			body, _ := json.Marshal(&m)
			buff := bytes.NewBuffer([]byte{})
			binary.Write(buff, binary.BigEndian, int16(len(body)))
			binary.Write(buff, binary.LittleEndian, body)
			socket.sendTCP(buff.Bytes())
		},
		ReceivePopularValue: func(value uint32) {
			log.Printf("【人气】:  %v", value)
		},
	}
	go liveRoom.Start(context.TODO())
	scanner(socket)
}

func scanner(s *socket) {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		name := ""
		count := 1
		switch scanner.Text() {
		case "A":
			name = "辣条"
			count = 1 + rand.Intn(521)

		case "B":
			name = "凉了"
			count = 1 + rand.Intn(260)

		case "C":
			name = "吃瓜"
			count = 1 + rand.Intn(100)

		case "D":
			name = "flag"
			count = 1 + rand.Intn(100)

		case "E":
			name = "爆米花"
			count = 1 + rand.Intn(50)

		case "F":
			name = "233"
			count = 1 + rand.Intn(30)

		case "G":
			name = "比心"
			count = 1 + rand.Intn(20)

		case "H":
			name = "干杯"
			count = 1 + rand.Intn(10)

		case "I":
			name = "666"

		case "J":
			name = "咸鱼"

		case "K":
			name = "冰阔落"

		case "L":
			name = "炮车"

		case "M":
			name = "情书"

		case "N":
			name = "真香"

		case "O":
			name = "给大佬递茶"

		case "P":
			name = "盛典门票"

		case "Q":
			name = "喵娘"

		case "R":
			name = "B坷垃"

		case "S":
			name = "礼花"

		case "T":
			name = "氪金键盘"

		case "U":
			name = "疯狂打call"

		case "V":
			name = "节奏风暴"

		case "W":
			name = "摩天大楼"

		case "X":
			name = "嗨翻全城"

		case "Y":
			name = "小电视飞船"
		case "exit":
			return
		default:
			break
		}
		if name != "" {
			m := msg{
				Name:  name,
				Count: count,
				User:  "测试",
			}
			body, _ := json.Marshal(&m)
			buff := bytes.NewBuffer([]byte{})
			binary.Write(buff, binary.BigEndian, int16(len(body)))
			binary.Write(buff, binary.LittleEndian, body)
			s.sendTCP(buff.Bytes())
		}
	}
}
