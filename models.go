package bililive

import "net"

// LiveRoom 直播间
type LiveRoom struct {
	RoomID              int                    // 房间ID（兼容短ID）
	ReceiveMsg          func(*MsgModel)        // 接收消息方法
	ReceiveGift         func(*GiftModel)       // 接收礼物方法
	ReceivePopularValue func(uint32)           // 接收人气值方法
	UserEnter           func(*UserEnterModel)  // 用户进入方法
	GuardEnter          func(*GuardEnterModel) // 舰长进入方法
	GiftComboEnd        func(*ComboEndModel)   // 礼物连击结束方法
	GuardBuy            func(*GuardBuyModel)   // 上传模型

	chBuffer       chan *bufferInfo
	chMsg          chan *MsgModel
	chGift         chan *GiftModel
	chPopularValue chan uint32
	chUserEnter    chan *UserEnterModel
	chGuardEnter   chan *GuardEnterModel
	chGiftComboEnd chan *ComboEndModel
	chGuardBuy     chan *GuardBuyModel

	server string // 地址
	port   int    // 端口
	conn   *net.TCPConn
}

type bufferInfo struct {
	TypeID uint32
	Buffer []byte
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
	CMD  string                 `json:"cmd"`
	Info []interface{}          `json:"info"`
	Data map[string]interface{} `json:"data"`
}

// UserEnterModel 用户进入模型
type UserEnterModel struct {
	UserID   int    `json:"uid"`
	UserName string `json:"uname"`
	IsAdmin  bool   `json:"is_admin"`
	VIP      int    `json:"vip"`
	SVIP     int    `json:"svip"`
}

// GuardEnterModel 舰长进入模型
type GuardEnterModel struct {
	UserID     int    `json:"uid"`
	UserName   string `json:"username"`
	GuardLevel int    `json:"guard_level"`
}

// GiftModel 礼物模型
type GiftModel struct {
	GiftName string `json:"giftName"`       // 礼物名称
	Num      int    `json:"num"`            // 数量
	UserName string `json:"uname"`          // 用户名称
	GiftID   int    `json:"giftId"`         // 礼物ID
	Price    int    `json:"price"`          // 价格
	CoinType string `json:"coin_type"`      // 硬币类型
	FaceURL  string `json:"face"`           // 头像url
	Combo    int    `json:"super_gift_num"` // 连击
}

// MsgModel 消息
type MsgModel struct {
	UserName string // 用户昵称
	Content  string // 内容
}

// ComboEndModel 连击结束模型
type ComboEndModel struct {
	GiftName   string `json:"gift_name"`   // 礼物名称
	ComboNum   int    `json:"combo_num"`   // 连击数量
	UserName   string `json:"uname"`       // 用户名称
	GiftID     int    `json:"gift_id"`     // 礼物ID
	Price      int    `json:"price"`       // 价格
	GuardLevel int    `json:"guard_level"` // 舰长等级
}

// GuardBuyModel 上船模型
type GuardBuyModel struct {
	GiftName   string `json:"gift_name"`   // 礼物名称
	Num        int    `json:"num"`         // 数量
	UserID     int    `json:"uid"`         // 用户ID
	UserName   string `json:"username"`    // 用户名称
	GiftID     int    `json:"gift_id"`     // 礼物ID
	Price      int    `json:"price"`       // 价格
	GuardLevel int    `json:"guard_level"` // 舰长等级
}
