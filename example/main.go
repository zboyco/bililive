package main

import (
	"flag"
	"log"

	"github.com/zboyco/bililive"
)

func main() {
	roomID := flag.Int("id", 0, "id")
	flag.Parse()
	if *roomID <= 0 {
		log.Fatalln("房间号错误!")
		return
	}
	liveRoom := &bililive.LiveRoom{
		RoomID: *roomID,
		ReceivePopularValue: func(v uint32) {
			log.Printf("人气值:%v", v)
		},
		ReceiveMsg: func(msg *bililive.MsgModel) {
			log.Printf("%v:%v", msg.UserName, msg.Content)
		},
		ReceiveGift: func(gift *bililive.GiftModel) {
			log.Printf("%v:%v(%v) * %v", gift.UserName, gift.GiftName, gift.GiftID, gift.Num)
		},
	}
	liveRoom.Start()
}
