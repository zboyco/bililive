package main

import (
	"log"

	"github.com/zboyco/bililive"
)

func main() {
	liveRoom := &bililive.LiveRoom{
		RoomID: 650,
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
