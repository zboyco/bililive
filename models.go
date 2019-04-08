package bililive

import "net"

// LiveRoom 直播间
type LiveRoom struct {
	RoomID              int              // 房间ID（兼容短ID）
	ReceiveMsg          func(*MsgModel)  // 接收消息方法
	ReceiveGift         func(*GiftModel) // 接收礼物方法
	ReceivePopularValue func(uint32)     // 接收人气值方法
	conn                *net.TCPConn
}

// 进入房间信息
type enterInfo struct {
	RoomID int    `json:"roomid"`
	UserID uint64 `json:"uid"`
}

// 房间信息
type roomInfoResult struct {
	Code int           `json:"code"`
	Data *roomInfoData `json:"data"`
}

// 房间数据
type roomInfoData struct {
	RoomID int `json:"room_id"`
}

// 角色信息
type characterInfoReuslt struct {
	DMServer string `xml:"dm_server"`
	DMPort   int    `xml:"dm_port"`
}

// 命令模型
type cmdModel struct {
	CMD  string        `json:"cmd"`
	Info []interface{} `json:"info"`
	Data *GiftModel    `json:"data"`
}

// GiftModel 礼物模型
type GiftModel struct {
	GiftName string `json:"giftName"`  // 礼物名称
	Num      int    `json:"num"`       // 数量
	UserName string `json:"uname"`     // 用户名称
	GiftID   int    `json:"giftId"`    // 礼物ID
	Price    int    `json:"price"`     // 价格
	CoinType string `json:"coin_type"` // 硬币类型
	FaceURL  string `json:"face"`      // 头像url
}

// MsgModel 消息
type MsgModel struct {
	UserName string // 用户昵称
	Content  string // 内容
}
