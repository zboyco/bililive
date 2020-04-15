package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"net/http"
	_ "net/http/pprof"

	"github.com/zboyco/bililive"
)

func main() {
	roomID := flag.Int("id", 0, "id")
	flag.Parse()
	if *roomID <= 0 {
		log.Fatalln("房间号错误!")
		return
	}
	//远程获取pprof数据
	go func() {
		log.Println(http.ListenAndServe("localhost:8080", nil))
	}()
	liveRoom := &bililive.LiveRoom{
		Debug:              false, // 不输出日志
		AnalysisRoutineNum: 1,     // 消息分析协程数量，默认为1，为1可以保证通知顺序与接收到消息顺序相同
		RoomID:             *roomID,
		RoomInfo: func(m *bililive.RoomDetailModel) {
			isLive := "直播中"
			if m.RoomInfo.LiveStatus != 1 {
				isLive = "未开播"
			}
			liveStartTime := time.Unix(m.RoomInfo.LiveStartTime, 0).Format("2006-01-02 15:04:05")
			alreadyLiveMinutes := time.Now().Sub(time.Unix(m.RoomInfo.LiveStartTime, 0)).Minutes()
			log.Printf("【房间信息】%s ，标题:【%s】，分区:【%s-%s】，开播时间:【%s】，已播时间:【%f分钟】", isLive, m.RoomInfo.Title, m.RoomInfo.ParentAreaName, m.RoomInfo.AreaName, liveStartTime, alreadyLiveMinutes)
		},
		ReceivePopularValue: func(v uint32) {
			log.Printf("【直播人气】%v", v)
		},
		RoomChange: func(m *bililive.RoomChangeModel) {
			log.Printf("【信息变更】标题:【%s】，分区:【%s-%s】", m.Title, m.ParentAreaName, m.AreaName)
		},
		RoomRank: func(m *bililive.RankModel) {
			rankTime := time.Unix(m.Timestamp, 0).Format("2006-01-02 15:04:05")
			log.Printf("【小时排名】%s %s", rankTime, m.RankDesc)
		},
		UserEnter: func(m *bililive.UserEnterModel) {
			log.Printf("【用户进入】欢迎 %v(%v) 进入直播间", m.UserName, m.UserID)
		},
		GuardEnter: func(m *bililive.GuardEnterModel) {
			log.Printf("【舰长进入】欢迎 舰长 - %v(%v) 进入直播间", m.UserName, m.UserID)
		},
		ReceiveMsg: func(msg *bililive.MsgModel) {
			log.Printf("【弹幕消息】%v(%v):  %v", msg.UserName, time.Unix(msg.Timestamp, 0), msg.Content)
		},
		ReceiveGift: func(gift *bililive.GiftModel) {
			coin := "银瓜子"
			if gift.CoinType == "gold" {
				coin = "金瓜子"
				log.Printf("【礼物通知】%s(%v):  %s(%d) * %d [价值 %d个%s]", gift.UserName, time.Unix(gift.Timestamp, 0), gift.GiftName, gift.GiftID, gift.Num, gift.Price*gift.Num, coin)
			}
			//log.Printf("【礼物通知】%s:  %s(%d) * %d [价值 %d个%s]", gift.UserName, gift.GiftName, gift.GiftID, gift.Num, gift.Price*gift.Num, coin)
		},
		GiftComboSend: func(m *bililive.ComboSendModel) {
			log.Printf("【礼物连击】%v 赠送 %v 连击 %v 次", m.UserName, m.GiftName, m.ComboNum)
		},
		GiftComboEnd: func(m *bililive.ComboEndModel) {
			log.Printf("【连击结束】%v 赠送 %v(价值%v) 总共连击 %v 次", m.UserName, m.GiftName, m.Price, m.ComboNum)
		},
		GuardBuy: func(m *bililive.GuardBuyModel) {
			log.Printf("【用户上船】欢迎 %v - %v(%v) 上船", m.GiftName, m.UserName, m.UserID)
		},
		FansUpdate: func(m *bililive.FansUpdateModel) {
			log.Printf("【粉丝更新】当前粉丝数 %d", m.Fans)
		},
		SpecialGift: func(m *bililive.SpecialGiftModel) {
			if m.Storm.Action == "start" {
				log.Println(fmt.Sprintf("【节奏风暴】开始，id：%s，数量：%d，内容：%s", m.Storm.ID, m.Storm.Num, m.Storm.Content))
			}
			if m.Storm.Action == "end" {
				log.Println(fmt.Sprintf("【节奏风暴】结束，id：%s", m.Storm.ID))
			}
		},
		SuperChatMessage: func(m *bililive.SuperChatMessageModel) {
			log.Printf("【醒目留言】%s ：%s | 价值 %d 元", m.UserInfo.UserName, m.Message, m.Price)
		},
	}
	liveRoom.Start()
}
