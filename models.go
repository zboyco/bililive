package bililive

import (
	"net"
)

// LiveRoom 直播间
type LiveRoom struct {
	Debug               bool                         // 是否显示日志
	AnalysisRoutineNum  int                          // 消息分析协程数量，默认为1，为1可以保证通知顺序与接收到消息顺序相同
	RoomID              int                          // 房间ID（兼容短ID）
	Live                func(*RoomDetailModel)       // 直播开始通知
	End                 func(*RoomDetailModel)       // 直播结束通知
	ReceiveMsg          func(*MsgModel)              // 接收消息方法
	ReceiveGift         func(*GiftModel)             // 接收礼物方法
	ReceivePopularValue func(uint32)                 // 接收人气值方法
	UserEnter           func(*UserEnterModel)        // 用户进入方法
	GuardEnter          func(*GuardEnterModel)       // 舰长进入方法
	GiftComboSend       func(*ComboSendModel)        // 礼物连击方法
	GiftComboEnd        func(*ComboEndModel)         // 礼物连击结束方法
	GuardBuy            func(*GuardBuyModel)         // 上船
	FansUpdate          func(*FansUpdateModel)       // 粉丝数更新
	RoomRank            func(*RankModel)             // 小时榜
	RoomChange          func(*RoomChangeModel)       // 房间信息变更
	SpecialGift         func(*SpecialGiftModel)      // 特殊礼物
	SuperChatMessage    func(*SuperChatMessageModel) // 超级留言
	SysMessage          func(*SysMsgModel)           // 系统信息

	chSocketMessage chan []byte
	chOperation     chan *operateInfo

	server string // 地址
	port   int    // 端口
	token  string // key
	conn   *net.TCPConn
}

type messageHeader struct {
	Length          int32
	HeaderLength    int16
	ProtocolVersion int16
	Operation       int32
	SequenceID      int32
}

type operateInfo struct {
	Operation int32
	Buffer    []byte
}

// 进入房间信息
type enterInfo struct {
	RoomID    int    `json:"roomid"`
	UserID    int64  `json:"uid"`
	ProtoVer  int    `json:"protover"`
	Platform  string `json:"platform"`
	ClientVer string `json:"clientver"`
	Type      int    `json:"type"`
	Key       string `json:"key"`
}

// 房间信息
type roomInfoResult struct {
	Code int           `json:"code"`
	Data *roomInfoData `json:"data"`
}

type roomDetailResult struct {
	Code    int              `json:"code"`
	Message string           `json:"message"`
	Data    *RoomDetailModel `json:"data"`
}

// RoomDetailModel 房间信息详情
type RoomDetailModel struct {
	RoomInfo struct {
		RoomID         int    `json:"room_id"`
		ShortID        int    `json:"short_id"`
		LiveStatus     int    `json:"live_status"`
		LiveStartTime  int64  `json:"live_start_time"`
		Title          string `json:"title"`
		AreaID         int    `json:"area_id"`
		ParentAreaID   int    `json:"parent_area_id"`
		AreaName       string `json:"area_name"`
		ParentAreaName string `json:"parent_area_name"`
	} `json:"room_info"`
	AnchorInfo struct {
		RelationInfo struct {
			Attention int `json:"attention"`
		} `json:"relation_info"`
	} `json:"anchor_info"`
}

// 房间数据
type roomInfoData struct {
	RoomID int `json:"room_id"`
}

// 弹幕信息
type danmuConfigResult struct {
	Data *danmuData `json:"data"`
}

type danmuData struct {
	Host           string `json:"host"`
	Port           int    `json:"port"`
	HostServerList []struct {
		Host    string
		Port    int `json:"port"`
		WssPort int `json:"wss_port"`
		WsPort  int `json:"ws_port"`
	} `json:"host_server_list"`
	Token string `json:"token"`
}

// 命令模型
type cmdModel struct {
	CMD  string                 `json:"cmd"`
	Info []interface{}          `json:"info"`
	Data map[string]interface{} `json:"data"`
}

// SysMsgModel 系统信息
type SysMsgModel struct {
	Cmd     string `json:"cmd"`
	Msg     string `json:"msg"`
	MsgText string `json:"msg_text"`
}

// UserEnterModel 用户进入模型
type UserEnterModel struct {
	UserID   int64  `json:"uid"`
	UserName string `json:"uname"`
	IsAdmin  bool   `json:"is_admin"`
	VIP      int    `json:"vip"`
	SVIP     int    `json:"svip"`
}

// GuardEnterModel 舰长进入模型
type GuardEnterModel struct {
	UserID     int64  `json:"uid"`
	UserName   string `json:"username"`
	GuardLevel int    `json:"guard_level"`
}

// GiftModel 礼物模型
type GiftModel struct {
	GiftName  string `json:"giftName"`       // 礼物名称
	Num       int    `json:"num"`            // 数量
	UserName  string `json:"uname"`          // 用户名称
	UserID    int64  `json:"uid"`            // 用户ID
	GiftID    int    `json:"giftId"`         // 礼物ID
	Price     int    `json:"price"`          // 价格
	CoinType  string `json:"coin_type"`      // 硬币类型
	FaceURL   string `json:"face"`           // 头像url
	Combo     int    `json:"super_gift_num"` // 连击
	Timestamp int64  `json:"timestamp"`      // 时间
}

// MsgModel 消息
type MsgModel struct {
	UserID    int64  // 用户ID
	UserName  string // 用户昵称
	Content   string // 内容
	Timestamp int64  // 时间
}

// ComboSendModel 连击模型
type ComboSendModel struct {
	UserName string `json:"uname"`     // 用户名称
	GiftName string `json:"gift_name"` // 礼物名称
	GiftID   int    `json:"gift_id"`   // 礼物ID
	ComboNum int    `json:"combo_num"` // 连击数量
}

// ComboEndModel 连击结束模型
type ComboEndModel struct {
	GiftName   string `json:"gift_name"`   // 礼物名称
	ComboNum   int    `json:"combo_num"`   // 连击数量
	UserName   string `json:"uname"`       // 用户名称
	GiftID     int    `json:"gift_id"`     // 礼物ID
	Price      int    `json:"price"`       // 价格
	GuardLevel int    `json:"guard_level"` // 舰长等级
	StartTime  int64  `json:"start_time"`  // 开始时间
	EndTime    int64  `json:"end_time"`    // 结束时间
}

// GuardBuyModel 上船模型
type GuardBuyModel struct {
	GiftName   string `json:"gift_name"`   // 礼物名称
	Num        int    `json:"num"`         // 数量
	UserID     int64  `json:"uid"`         // 用户ID
	UserName   string `json:"username"`    // 用户名称
	GiftID     int    `json:"gift_id"`     // 礼物ID
	Price      int    `json:"price"`       // 价格
	GuardLevel int    `json:"guard_level"` // 舰长等级
}

// FansUpdateModel 粉丝更新模型
type FansUpdateModel struct {
	RoomID    int `json:"roomid"`
	Fans      int `json:"fans"`
	RedNotice int `json:"red_notice"`
}

// RankModel 小时榜模型
type RankModel struct {
	RoomID    int    `json:"roomid"`
	RankDesc  string `json:"rank_desc"`
	Timestamp int64  `json:"timestamp"`
}

// RoomChangeModel 房间基础信息变更
type RoomChangeModel struct {
	Title          string `json:"title"`
	AreaID         int    `json:"area_id"`
	ParentAreaID   int    `json:"parent_area_id"`
	AreaName       string `json:"area_name"`
	ParentAreaName string `json:"parent_area_name"`
}

// SpecialGiftModel 特殊礼物模型
type SpecialGiftModel struct {
	Storm struct {
		ID      string `json:"id"`
		Action  string `json:"action"`
		Content string `json:"content"`
		Num     int    `json:"num"`
		Time    int    `json:"time"`
	} `json:"39"`
}

// SuperChatMessageModel 超级留言模型
type SuperChatMessageModel struct {
	Price    int    `json:"price"`
	Message  string `json:"message"`
	UserInfo struct {
		UserName string `json:"uname"`
	} `json:"user_info"`
}
