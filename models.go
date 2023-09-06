package bililive

import (
	"context"
	"net"
	"sync"
)

const (
	InteractWordMsgTypeEnter         = 1 // 入场
	InteractWordMsgTypeFollow        = 2 // 关注
	InteractWordMsgTypeShare         = 3 // 分享
	InteractWordMsgTypeSpecialFollow = 4 // 特别关注
	InteractWordMsgTypeDualFollow    = 5 // 互粉
	BattleTypePk                     = 1 // 大乱斗
	BattleTypeVideoPk                = 2 // 视频大乱斗
	BattleTypeFriendPk               = 6 // 好友PK
)

// Live 直播间
type Live struct {
	Debug               bool                                // 是否显示日志
	AnalysisRoutineNum  int                                 // 消息分析协程数量，默认为1，为1可以保证通知顺序与接收到消息顺序相同
	StormFilter         bool                                // 过滤节奏风暴弹幕，默认false不过滤
	Raw                 func(int, []byte)                   // 原始数据
	Live                func(int)                           // 直播开始通知
	End                 func(int)                           // 直播结束通知
	ReceiveMsg          func(int, *MsgModel)                // 接收消息方法
	ReceiveGift         func(int, *GiftModel)               // 接收礼物方法
	ReceivePopularValue func(int, uint32)                   // 接收人气值方法
	InteractWord        func(int, *InteractWordModel)       // 互动消息
	EntryEffect         func(int, *EntryEffectModel)        // 特效入场
	GiftComboSend       func(int, *ComboSendModel)          // 礼物连击方法
	GiftComboEnd        func(int, *ComboEndModel)           // 礼物连击结束方法
	GuardBuy            func(int, *GuardBuyModel)           // 上船
	FansUpdate          func(int, *FansUpdateModel)         // 粉丝数更新
	RoomRank            func(int, *RankModel)               // 小时榜
	RoomChange          func(int, *RoomChangeModel)         // 房间信息变更
	SpecialGift         func(int, *SpecialGiftModel)        // 特殊礼物
	SuperChatMessage    func(int, *SuperChatMessageModel)   // 超级留言
	SysMessage          func(int, *SysMsgModel)             // 系统信息
	PkBattleStartNew    func(int, *PkBattleStartNewModel)   // 大乱斗开始
	PkBattleProcessNew  func(int, *PkBattleProcessNewModel) // 大乱斗状态更新（绝杀）
	PkBattleEnd         func(int, *PkBattleEndModel)        // 大乱斗结束

	wg  sync.WaitGroup
	ctx context.Context

	chSocketMessage chan *socketMessage
	chOperation     chan *operateInfo

	storming     map[int]bool             // 是否节奏风暴
	stormContent map[int]map[int64]string // 节奏风暴内容

	room map[int]*liveRoom // 直播间
	lock sync.Mutex
}

type socketMessage struct {
	roomID int // 房间ID（兼容短ID）
	body   []byte
}

type liveRoom struct {
	roomID             int // 房间ID（兼容短ID）
	realRoomID         int
	uid                int
	cancel             context.CancelFunc
	server             string // 地址
	port               int    // 端口
	hostServerList     []*hostServerList
	currentServerIndex int
	token              string // key
	conn               *net.TCPConn
	viewerUID          int
	viewerCookie       string
}

type messageHeader struct {
	Length          int32
	HeaderLength    int16
	ProtocolVersion int16
	Operation       int32
	SequenceID      int32
}

type operateInfo struct {
	RoomID    int
	Operation int32
	Buffer    []byte
}

// 进入房间信息
type enterInfo struct {
	RoomID    int    `json:"roomid"`
	BuVID     string `json:"buvid"`
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

// 房间数据
type roomInfoData struct {
	RoomID int `json:"room_id"`
	UID    int `json:"uid"`
}

// 弹幕信息
type danmuConfigResult struct {
	Data *danmuData `json:"data"`
}

type danmuData struct {
	Host           string            `json:"host"`
	Port           int               `json:"port"`
	HostServerList []*hostServerList `json:"host_server_list"`
	Token          string            `json:"token"`
}

type hostServerList struct {
	Host    string `json:"host"`
	Port    int    `json:"port"`
	WssPort int    `json:"wss_port"`
	WsPort  int    `json:"ws_port"`
}

type danmuInfoResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Ttl     int    `json:"ttl"`
	Data    struct {
		Group            string  `json:"group"`
		BusinessId       int     `json:"business_id"`
		RefreshRowFactor float64 `json:"refresh_row_factor"`
		RefreshRate      int     `json:"refresh_rate"`
		MaxDelay         int     `json:"max_delay"`
		Token            string  `json:"token"`
		HostList         []struct {
			Host    string `json:"host"`
			Port    int    `json:"port"`
			WssPort int    `json:"wss_port"`
			WsPort  int    `json:"ws_port"`
		} `json:"host_list"`
	} `json:"data"`
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

type EntryEffectModel struct {
	ID                   int           `json:"id"`
	UID                  int           `json:"uid"`
	TargetID             int           `json:"target_id"`
	MockEffect           int           `json:"mock_effect"`
	Face                 string        `json:"face"`
	UName                string        `json:"uname"`
	PrivilegeType        int           `json:"privilege_type"`
	CopyWriting          string        `json:"copy_writing"`
	CopyColor            string        `json:"copy_color"`
	HighlightColor       string        `json:"highlight_color"`
	Priority             int           `json:"priority"`
	BasemapUrl           string        `json:"basemap_url"`
	ShowAvatar           int           `json:"show_avatar"`
	EffectiveTime        int           `json:"effective_time"`
	WebBasemapUrl        string        `json:"web_basemap_url"`
	WebEffectiveTime     int           `json:"web_effective_time"`
	WebEffectClose       int           `json:"web_effect_close"`
	WebCloseTime         int           `json:"web_close_time"`
	Business             int           `json:"business"`
	CopyWritingV2        string        `json:"copy_writing_v2"`
	IconList             []interface{} `json:"icon_list"`
	MaxDelayTime         int           `json:"max_delay_time"`
	TriggerTime          int64         `json:"trigger_time"`
	Identities           int           `json:"identities"`
	EffectSilentTime     int           `json:"effect_silent_time"`
	EffectiveTimeNew     int           `json:"effective_time_new"`
	WebDynamicUrlWebp    string        `json:"web_dynamic_url_webp"`
	WebDynamicUrlApng    string        `json:"web_dynamic_url_apng"`
	MobileDynamicUrlWebp string        `json:"mobile_dynamic_url_webp"`
	WealthyInfo          interface{}   `json:"wealthy_info"`
	NewStyle             int           `json:"new_style"`
}

type InteractWordModel struct {
	Uid        int    `json:"uid"`
	Uname      string `json:"uname"`
	UnameColor string `json:"uname_color"`
	Identities []int  `json:"identities"`
	MsgType    int    `json:"msg_type"`
	RoomID     int    `json:"roomid"`
	Timestamp  int    `json:"timestamp"`
	Score      int64  `json:"score"`
	FansMedal  struct {
		TargetID         int    `json:"target_id"`
		MedalLevel       int    `json:"medal_level"`
		MedalName        string `json:"medal_name"`
		MedalColor       int    `json:"medal_color"`
		MedalColorStart  int    `json:"medal_color_start"`
		MedalColorEnd    int    `json:"medal_color_end"`
		MedalColorBorder int    `json:"medal_color_border"`
		IsLighted        int    `json:"is_lighted"`
		GuardLev         int    `json:"guard_lev"`
		Special          string `json:"special"`
		IconId           int    `json:"icon_id"`
		AnchorRoomid     int    `json:"anchor_roomid"`
		Score            int    `json:"score"`
	} `json:"fans_medal"`
	IsSpread     int    `json:"is_spread"`
	SpreadInfo   string `json:"spread_info"`
	Contribution struct {
		Grade int `json:"grade"`
	} `json:"contribution"`
	SpreadDesc     string `json:"spread_desc"`
	TailIcon       int    `json:"tail_icon"`
	TriggerTime    int64  `json:"trigger_time"`
	PrivilegeType  int    `json:"privilege_type"`
	CoreUserType   int    `json:"core_user_type"`
	TailText       string `json:"tail_text"`
	ContributionV2 struct {
		Grade    int    `json:"grade"`
		RankType string `json:"rank_type"`
		Text     string `json:"text"`
	} `json:"contribution_v2"`
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
	UserID       int64  // 用户ID
	UserName     string // 用户昵称
	UserLevel    int    // 用户等级
	MedalName    string // 勋章名
	MedalUpName  string // 勋章主播名称
	MedalRoomID  int64  // 勋章直播间ID
	MedalLevel   int    // 勋章等级
	Content      string // 内容
	Timestamp    int64  // 时间
	WealthyLevel int    // 财富等级
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
		ID      int64       `json:"-"`
		TempID  interface{} `json:"id"` // 因为b站通知节奏风暴开始和结束id类型不同，用这个变量作为中转
		Action  string      `json:"action"`
		Content string      `json:"content"`
		Num     int         `json:"num"`
		Time    int         `json:"time"`
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

type PkBattleStartNewModel struct {
	Cmd       string `json:"cmd"`
	PkId      int    `json:"pk_id"`
	PkStatus  int    `json:"pk_status"`
	Timestamp int    `json:"timestamp"`
	Data      struct {
		BattleType    int    `json:"battle_type"`
		FinalHitVotes int    `json:"final_hit_votes"`
		PkStartTime   int    `json:"pk_start_time"`
		PkFrozenTime  int    `json:"pk_frozen_time"`
		PkEndTime     int    `json:"pk_end_time"`
		PkVotesType   int    `json:"pk_votes_type"`
		PkVotesAdd    int    `json:"pk_votes_add"`
		PkVotesName   string `json:"pk_votes_name"`
		StarLightMsg  string `json:"star_light_msg"`
		PkCountdown   int    `json:"pk_countdown"`
		FinalConf     struct {
			Switch    int `json:"switch"`
			StartTime int `json:"start_time"`
			EndTime   int `json:"end_time"`
		} `json:"final_conf"`
		InitInfo struct {
			RoomId     int `json:"room_id"`
			DateStreak int `json:"date_streak"`
		} `json:"init_info"`
		MatchInfo struct {
			RoomId     int `json:"room_id"`
			DateStreak int `json:"date_streak"`
		} `json:"match_info"`
	} `json:"data"`
	Roomid string `json:"roomid"`
}

type PkBattleProcessNewModel struct {
	Cmd  string `json:"cmd"`
	Data struct {
		BattleType int `json:"battle_type"`
		InitInfo   struct {
			RoomId     int         `json:"room_id"`
			Votes      int         `json:"votes"`
			BestUname  string      `json:"best_uname"`
			VisionDesc int         `json:"vision_desc"`
			AssistInfo interface{} `json:"assist_info"`
		} `json:"init_info"`
		MatchInfo struct {
			RoomId     int    `json:"room_id"`
			Votes      int    `json:"votes"`
			BestUname  string `json:"best_uname"`
			VisionDesc int    `json:"vision_desc"`
			AssistInfo []struct {
				Rank  int    `json:"rank"`
				Uid   int    `json:"uid"`
				Face  string `json:"face"`
				Uname string `json:"uname"`
			} `json:"assist_info"`
		} `json:"match_info"`
		TraceId string `json:"trace_id"`
	} `json:"data"`
	PkId      int `json:"pk_id"`
	PkStatus  int `json:"pk_status"`
	Timestamp int `json:"timestamp"`
}

type PkBattleEndModel struct {
	Cmd       string `json:"cmd"`
	PkId      string `json:"pk_id"`
	PkStatus  int    `json:"pk_status"`
	Timestamp int    `json:"timestamp"`
	Data      struct {
		ShowStreak bool `json:"show_streak"`
		BattleType int  `json:"battle_type"`
		Timer      int  `json:"timer"`
		InitInfo   struct {
			RoomId     int    `json:"room_id"`
			Votes      int    `json:"votes"`
			WinnerType int    `json:"winner_type"`
			BestUname  string `json:"best_uname"`
			AssistInfo []struct {
				Rank  int    `json:"rank"`
				Uid   int    `json:"uid"`
				Score int    `json:"score"`
				Face  string `json:"face"`
				Uname string `json:"uname"`
			} `json:"assist_info"`
		} `json:"init_info"`
		MatchInfo struct {
			RoomId     int    `json:"room_id"`
			Votes      int    `json:"votes"`
			WinnerType int    `json:"winner_type"`
			BestUname  string `json:"best_uname"`
			AssistInfo []struct {
				Rank  int    `json:"rank"`
				Uid   int    `json:"uid"`
				Score int    `json:"score"`
				Face  string `json:"face"`
				Uname string `json:"uname"`
			} `json:"assist_info"`
		} `json:"match_info"`
		DmConf struct {
			FontColor string `json:"font_color"`
			BgColor   string `json:"bg_color"`
		} `json:"dm_conf"`
	} `json:"data"`
}
