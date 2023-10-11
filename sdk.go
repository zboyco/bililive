package bililive

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/andybalholm/brotli"
	jsoniter "github.com/json-iterator/go"
	uuid "github.com/satori/go.uuid"
	"github.com/spf13/cast"
	"io"
	"log"
	"math/rand"
	"net"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

const (
	roomInitURL                    string = "https://api.live.bilibili.com/room/v1/Room/room_init?id=%d"
	roomConfigURL                  string = "https://api.live.bilibili.com/room/v1/Danmu/getConf?room_id=%d"
	danmuInfoURL                   string = "https://api.live.bilibili.com/xlive/web-room/v1/index/getDanmuInfo?id=%d&type=0"
	WS_OP_HEARTBEAT                int32  = 2
	WS_OP_HEARTBEAT_REPLY          int32  = 3
	WS_OP_MESSAGE                  int32  = 5
	WS_OP_USER_AUTHENTICATION      int32  = 7
	WS_OP_CONNECT_SUCCESS          int32  = 8
	WS_PACKAGE_HEADER_TOTAL_LENGTH int32  = 16
	//WS_PACKAGE_OFFSET                int32 = 0
	//WS_HEADER_OFFSET                 int32 = 4
	//WS_VERSION_OFFSET                int32 = 6
	//WS_OPERATION_OFFSET              int32 = 8
	//WS_SEQUENCE_OFFSET               int32 = 12
	//WS_BODY_PROTOCOL_VERSION_NORMAL  int32 = 0
	WS_BODY_PROTOCOL_VERSION_DEFLATE int16 = 2
	WS_BODY_PROTOCOL_VERSION_BROTLI  int16 = 3
	WS_HEADER_DEFAULT_VERSION        int16 = 1
	//WS_HEADER_DEFAULT_OPERATION      int32 = 1
	WS_HEADER_DEFAULT_SEQUENCE int32 = 1
	WS_AUTH_OK                 int32 = 0
	WS_AUTH_TOKEN_ERROR        int32 = -101
)

var ret = regexp.MustCompile("<%(.*?)%>")

// Start 开始接收
func (live *Live) Start(ctx context.Context) {
	live.ctx = ctx

	rand.Seed(time.Now().Unix())
	if live.AnalysisRoutineNum <= 0 {
		live.AnalysisRoutineNum = 1
	}

	live.room = make(map[int]*liveRoom)
	live.chSocketMessage = make(chan *socketMessage, 30)
	live.chOperation = make(chan *operateInfo, 300)
	live.storming = make(map[int]bool)
	live.stormContent = make(map[int]map[int64]string)
	live.wg = sync.WaitGroup{}
	live.lock = sync.Mutex{}

	for i := 0; i < live.AnalysisRoutineNum; i++ {
		live.wg.Add(1)
		go func() {
			defer live.wg.Done()
			live.analysis(ctx)
		}()
	}

	live.wg.Add(1)
	go func() {
		defer live.wg.Done()
		live.split(ctx)
	}()
}

func (live *Live) Wait() {
	live.wg.Wait()
}

// JoinWithLogin 带登录态登录
func (live *Live) JoinWithLogin(viewerUID int, viewerCookie string, roomIDs ...int) error {
	if len(roomIDs) == 0 {
		return errors.New("没有要添加的房间")
	}
	live.lock.Lock()
	defer live.lock.Unlock()
	for _, roomID := range roomIDs {
		if _, exist := live.room[roomID]; exist {
			return fmt.Errorf("房间 %d 已存在", roomID)
		}
	}

	for _, roomID := range roomIDs {
		nextCtx, cancel := context.WithCancel(live.ctx)

		room := &liveRoom{
			roomID:       roomID,
			cancel:       cancel,
			viewerUID:    viewerUID,
			viewerCookie: viewerCookie,
		}
		live.room[roomID] = room
		room.enter()
		go room.heartBeat(nextCtx)
		live.stormContent[roomID] = make(map[int64]string)
		go room.receive(nextCtx, live.chSocketMessage)
	}
	return nil
}

// Join 添加房间
func (live *Live) Join(roomIDs ...int) error {
	return live.JoinWithLogin(0, "", roomIDs...)
}

// Remove 移出房间
func (live *Live) Remove(roomIDs ...int) error {
	if len(roomIDs) == 0 {
		return errors.New("没有要移出的房间")
	}
	live.lock.Lock()
	defer live.lock.Unlock()
	for _, roomID := range roomIDs {
		if room, exist := live.room[roomID]; exist {
			room.cancel()
			delete(live.room, roomID)
		}
	}
	return nil
}

func (live *Live) Rooms() []int {
	live.lock.Lock()
	defer live.lock.Unlock()
	var roomIDs []int
	for roomID := range live.room {
		roomIDs = append(roomIDs, roomID)
	}
	return roomIDs
}

// 拆分数据
func (live *Live) split(ctx context.Context) {
	var (
		message            *socketMessage
		head               messageHeader
		headerBufferReader *bytes.Reader
		payloadBuffer      []byte
	)
	for {
		select {
		case <-ctx.Done():
			return
		case message = <-live.chSocketMessage:
			for len(message.body) > 0 {
				headerBufferReader = bytes.NewReader(message.body[:WS_PACKAGE_HEADER_TOTAL_LENGTH])
				_ = binary.Read(headerBufferReader, binary.BigEndian, &head)
				payloadBuffer = message.body[WS_PACKAGE_HEADER_TOTAL_LENGTH:head.Length]
				message.body = message.body[head.Length:]
				if head.Length == WS_PACKAGE_HEADER_TOTAL_LENGTH {
					continue
				}
				if head.ProtocolVersion == WS_BODY_PROTOCOL_VERSION_DEFLATE {
					message.body = doZlibUnCompress(payloadBuffer)
					continue
				}
				if head.ProtocolVersion == WS_BODY_PROTOCOL_VERSION_BROTLI {
					message.body = doBrotliUnCompress(payloadBuffer)
					continue
				}
				if live.Debug {
					log.Println("debug", head.Operation, head.ProtocolVersion, len(payloadBuffer), len(message.body), string(payloadBuffer))
				}
				live.chOperation <- &operateInfo{RoomID: message.roomID, Operation: head.Operation, Buffer: payloadBuffer}
			}
		}

	}
}

// 分析接收到的数据
func (live *Live) analysis(ctx context.Context) {
analysis:
	for {
		select {
		case <-ctx.Done():
			return
		case buffer := <-live.chOperation:
			switch buffer.Operation {
			case WS_OP_HEARTBEAT_REPLY:
				if live.ReceivePopularValue != nil {
					m := binary.BigEndian.Uint32(buffer.Buffer)
					live.ReceivePopularValue(buffer.RoomID, m)
				}
			case WS_OP_CONNECT_SUCCESS:
				if live.Debug {
					log.Println("CONNECT_SUCCESS", string(buffer.Buffer))
				}
			case WS_OP_MESSAGE:
				if live.Raw != nil {
					live.Raw(buffer.RoomID, buffer.Buffer)
				}
				result := cmdModel{}
				err := json.Unmarshal(buffer.Buffer, &result)
				if err != nil {
					if live.Debug {
						log.Println(err)
						log.Println(string(buffer.Buffer))
					}
					continue
				}
				temp, err := json.Marshal(result.Data)
				if err != nil {
					if live.Debug {
						log.Println(err)
					}
					continue
				}
				switch result.CMD {
				case "LIVE": // 直播开始
					log.Println(string(buffer.Buffer))
					if live.Live != nil {
						live.Live(buffer.RoomID)
					}
				case "CLOSE": // 关闭
					fallthrough
				case "PREPARING": // 准备
					fallthrough
				case "END": // 结束
					log.Println(string(buffer.Buffer))
					if live.End != nil {
						live.End(buffer.RoomID)
					}
				case "SYS_MSG": // 系统消息
					if live.SysMessage != nil {
						m := &SysMsgModel{}
						_ = json.Unmarshal(buffer.Buffer, m)
						live.SysMessage(buffer.RoomID, m)
					}
				case "ROOM_CHANGE": // 房间信息变更
					if live.RoomChange != nil {
						m := &RoomChangeModel{}
						_ = json.Unmarshal(temp, m)
						live.RoomChange(buffer.RoomID, m)
					}
				case "DANMU_MSG": // 弹幕
					if live.ReceiveMsg != nil {
						msgContent := result.Info[1].(string)
						if live.StormFilter && live.storming[buffer.RoomID] {
							for _, value := range live.stormContent[buffer.RoomID] {
								if msgContent == value {
									continue analysis
								}
							}
						}
						userInfo := result.Info[2].([]interface{})
						medalInfo := result.Info[3].([]interface{})
						m := &MsgModel{
							UserID:    int64(userInfo[0].(float64)),
							UserName:  userInfo[1].(string),
							UserLevel: int(result.Info[4].([]interface{})[0].(float64)),
							Content:   msgContent,
							Timestamp: int64(result.Info[9].(map[string]interface{})["ts"].(float64)),
						}
						if len(medalInfo) >= 4 {
							m.MedalLevel = int(medalInfo[0].(float64))
							m.MedalName = medalInfo[1].(string)
							m.MedalUpName = medalInfo[2].(string)
							m.MedalRoomID = int64(medalInfo[3].(float64))
						}
						if len(result.Info) > 16 {
							if levelInfo, ok := result.Info[16].([]interface{}); ok && len(levelInfo) > 0 {
								m.WealthyLevel = cast.ToInt(levelInfo[0])
							}
						}
						m.GuardLevel = int(result.Info[7].(float64))
						live.ReceiveMsg(buffer.RoomID, m)
					}
				case "SEND_GIFT": // 礼物通知
					if live.ReceiveGift != nil {
						m := &GiftModel{}
						_ = json.Unmarshal(temp, m)
						m.RawMsg = buffer.Buffer
						live.ReceiveGift(buffer.RoomID, m)
					}
				case "COMBO_SEND": // 连击
					if live.GiftComboSend != nil {
						m := &ComboSendModel{}
						_ = json.Unmarshal(temp, m)
						live.GiftComboSend(buffer.RoomID, m)
					}
				case "COMBO_END": // 连击结束
					if live.GiftComboEnd != nil {
						m := &ComboEndModel{}
						_ = json.Unmarshal(temp, m)
						live.GiftComboEnd(buffer.RoomID, m)
					}
				case "GUARD_BUY": // 上船
					if live.GuardBuy != nil {
						m := &GuardBuyModel{}
						_ = json.Unmarshal(temp, m)
						live.GuardBuy(buffer.RoomID, m)
					}
				case "ROOM_REAL_TIME_MESSAGE_UPDATE": // 粉丝数更新
					if live.FansUpdate != nil {
						m := &FansUpdateModel{}
						_ = json.Unmarshal(temp, m)
						live.FansUpdate(buffer.RoomID, m)
					}
				case "ROOM_RANK": // 小时榜
					if live.RoomRank != nil {
						m := &RankModel{}
						_ = json.Unmarshal(temp, m)
						live.RoomRank(buffer.RoomID, m)
					}
				case "SPECIAL_GIFT": // 特殊礼物
					m := &SpecialGiftModel{}
					_ = json.Unmarshal(temp, m)
					if m.Storm.Action == "start" {
						m.Storm.ID, _ = strconv.ParseInt(m.Storm.TempID.(string), 10, 64)
					}
					if m.Storm.Action == "end" {
						m.Storm.ID = int64(m.Storm.TempID.(float64))
					}
					if live.StormFilter && live.ReceiveMsg != nil {
						if m.Storm.Action == "start" {
							live.storming[buffer.RoomID] = true
							live.stormContent[buffer.RoomID][m.Storm.ID] = m.Storm.Content
							//log.Println("添加过滤弹幕：", m.Storm.ID, m.Storm.Content)
						}
						if m.Storm.Action == "end" {
							delete(live.stormContent[buffer.RoomID], m.Storm.ID)
							live.storming[buffer.RoomID] = len(live.stormContent) > 0
							//log.Println("移除过滤弹幕：", m.Storm.ID, live.storming)
						}
					}
					if live.SpecialGift != nil {
						live.SpecialGift(buffer.RoomID, m)
					}
				case "SUPER_CHAT_MESSAGE": // 醒目留言
					if live.SuperChatMessage != nil {
						m := &SuperChatMessageModel{}
						_ = json.Unmarshal(temp, m)
						live.SuperChatMessage(buffer.RoomID, m)
					}
				case "SUPER_CHAT_MESSAGE_JPN":
					if live.Debug {
						log.Println(string(buffer.Buffer))
					}
				case "ENTRY_EFFECT": // 进入效果
					if live.EntryEffect != nil {
						m := &EntryEffectModel{}
						_ = json.Unmarshal(temp, m)
						tokens := ret.FindStringSubmatch(m.CopyWriting)
						if len(tokens) > 1 {
							m.UName = strings.TrimSpace(tokens[1])
						}
						live.EntryEffect(buffer.RoomID, m)
					}
				case "INTERACT_WORD": // 互动消息
					if live.InteractWord != nil {
						m := &InteractWordModel{}
						_ = json.Unmarshal(temp, m)
						live.InteractWord(buffer.RoomID, m)
					}
				case "PK_BATTLE_START_NEW": //
					if live.PkBattleStartNew != nil {
						m := &PkBattleStartNewModel{}
						_ = json.Unmarshal(buffer.Buffer, &m)
						live.PkBattleStartNew(buffer.RoomID, m)
					}
				case "PK_BATTLE_PROCESS_NEW": //
					if live.PkBattleProcessNew != nil {
						m := &PkBattleProcessNewModel{}
						_ = json.Unmarshal(buffer.Buffer, &m)
						live.PkBattleProcessNew(buffer.RoomID, m)
					}
				case "PK_BATTLE_END": //
					if live.PkBattleEnd != nil {
						m := &PkBattleEndModel{}
						_ = json.Unmarshal(buffer.Buffer, &m)
						live.PkBattleEnd(buffer.RoomID, m)
					}
				case "SYS_GIFT": // 系统礼物
					fallthrough
				case "BLOCK": // 未知
					fallthrough
				case "ROUND": // 未知
					fallthrough
				case "REFRESH": // 刷新
					fallthrough
				case "ACTIVITY_BANNER_UPDATE_V2": //
					fallthrough
				case "ANCHOR_LOT_CHECKSTATUS": //
					fallthrough
				case "GUARD_MSG": // 舰长信息
					fallthrough
				case "NOTICE_MSG": // 通知信息
					fallthrough
				case "GUARD_LOTTERY_START": // 舰长抽奖开始
					fallthrough
				case "USER_TOAST_MSG": // 用户通知消息
					fallthrough
				case "WISH_BOTTLE": // 许愿瓶
					fallthrough
				case "ROOM_BLOCK_MSG":
					fallthrough
				case "WEEK_STAR_CLOCK":
					fallthrough
				default:
					if live.Debug {
						log.Println(string(buffer.Buffer))
					}
				}
			}
		}
	}
}

func (room *liveRoom) findServer() error {
	resRoom, err := httpSend(fmt.Sprintf(roomInitURL, room.roomID), room.viewerCookie)
	if err != nil {
		return err
	}
	roomInfo := roomInfoResult{}
	_ = json.Unmarshal(resRoom, &roomInfo)
	if roomInfo.Code != 0 || roomInfo.Data == nil {
		return errors.New("房间不正确")
	}
	room.realRoomID = roomInfo.Data.RoomID
	room.uid = roomInfo.Data.UID
	rspDanmuInfo, err := httpSend(fmt.Sprintf(danmuInfoURL, room.realRoomID), room.viewerCookie)
	if err != nil {
		return err
	}

	danmuInfo := danmuInfoResp{}
	_ = json.Unmarshal(rspDanmuInfo, &danmuInfo)
	if danmuInfo.Code != 0 {
		return errors.New("获取弹幕服务器失败")
	}
	//room.server = danmuConfig.Data.Host
	//room.port = danmuConfig.Data.Port
	serverList := []*hostServerList{}
	for _, server := range danmuInfo.Data.HostList {
		serverList = append(serverList, &hostServerList{
			Host:    server.Host,
			Port:    server.Port,
			WssPort: server.WssPort,
			WsPort:  server.WsPort,
		})
	}
	room.hostServerList = serverList
	room.token = danmuInfo.Data.Token
	room.currentServerIndex = 0
	return nil
}

func (room *liveRoom) createConnect() {
	for {
		if room.hostServerList == nil || len(room.hostServerList) == room.currentServerIndex {
			for {
				err := room.findServer()
				if err != nil {
					log.Println("find server err:", err)
					time.Sleep(500 * time.Millisecond)
					continue
				}
				break
			}
		}
		counter := 0
		for {
			log.Println("尝试创建连接：", room.hostServerList[room.currentServerIndex].Host, room.hostServerList[room.currentServerIndex].Port)
			conn, err := connect(room.hostServerList[room.currentServerIndex].Host, room.hostServerList[room.currentServerIndex].Port)
			if err != nil {
				log.Println("connect err:", err)
				if counter == 3 {
					room.currentServerIndex++
					break
				}
				time.Sleep(1 * time.Second)
				counter++
				continue
			}
			room.conn = conn
			log.Println("连接创建成功：", room.hostServerList[room.currentServerIndex].Host, room.hostServerList[room.currentServerIndex].Port)
			room.currentServerIndex++
			return
		}
	}
}

func (room *liveRoom) enter() {
	room.createConnect()
	enterInfo := &enterInfo{
		RoomID:   room.realRoomID,
		BuVID:    uuid.NewV4().String(),
		UserID:   int64(room.uid),
		ProtoVer: 3,
		Platform: "web",
		Type:     2,
		Key:      room.token,
	}
	if room.viewerUID != 0 {
		enterInfo.UserID = int64(room.viewerUID)
	}
	payload, err := json.Marshal(enterInfo)
	if err != nil {
		log.Println(err)
		return
	}
	room.sendData(WS_OP_USER_AUTHENTICATION, payload)
}

// 心跳
func (room *liveRoom) heartBeat(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		room.sendData(WS_OP_HEARTBEAT, []byte{})
		time.Sleep(30 * time.Second)
	}
}

// 接收消息
func (room *liveRoom) receive(ctx context.Context, chSocketMessage chan<- *socketMessage) {
	// 包头总长16个字节
	headerBuffer := make([]byte, WS_PACKAGE_HEADER_TOTAL_LENGTH)
	// headerBufferReader
	var headerBufferReader *bytes.Reader
	// 包体
	var messageBody []byte
	counter := 0
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		_, err := io.ReadFull(room.conn, headerBuffer)
		if err != nil {
			log.Println("ReadFull: error", err)
			room.enter()
			counter++
			continue
		}
		var head messageHeader
		headerBufferReader = bytes.NewReader(headerBuffer)
		_ = binary.Read(headerBufferReader, binary.BigEndian, &head)
		if head.Length < WS_PACKAGE_HEADER_TOTAL_LENGTH {
			if counter >= 10 {
				log.Println("***************协议失败***************")
				log.Println("数据包长度:", head.Length)
				log.Println("数据包长度不正确")
			}
			room.enter()
			counter++
			continue
		}
		payloadBuffer := make([]byte, head.Length-WS_PACKAGE_HEADER_TOTAL_LENGTH)
		_, err = io.ReadFull(room.conn, payloadBuffer)
		if err != nil {
			log.Println("ReadFull err:", err)
			room.enter()
			counter++
			continue
		}
		messageBody = append(headerBuffer, payloadBuffer...)
		chSocketMessage <- &socketMessage{
			roomID: room.roomID,
			body:   messageBody,
		}
		counter = 0
	}
}

// 发送数据
func (room *liveRoom) sendData(operation int32, payload []byte) {

	b := bytes.NewBuffer([]byte{})
	head := messageHeader{
		Length:          int32(len(payload)) + WS_PACKAGE_HEADER_TOTAL_LENGTH,
		HeaderLength:    int16(WS_PACKAGE_HEADER_TOTAL_LENGTH),
		ProtocolVersion: WS_HEADER_DEFAULT_VERSION,
		Operation:       operation,
		SequenceID:      WS_HEADER_DEFAULT_SEQUENCE,
	}
	err := binary.Write(b, binary.BigEndian, head)
	if err != nil {
		log.Println(err)
	}

	err = binary.Write(b, binary.LittleEndian, payload)
	if err != nil {
		log.Println(err)
	}

	_, err = room.conn.Write(b.Bytes())
	if err != nil {
		log.Println(err)
	}
}

func connect(host string, port int) (*net.TCPConn, error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp4", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return nil, err
	}
	return net.DialTCP("tcp", nil, tcpAddr)
}

// 进行zlib解压缩
func doZlibUnCompress(compressSrc []byte) []byte {
	b := bytes.NewReader(compressSrc)
	var out bytes.Buffer
	r, err := zlib.NewReader(b)
	if err != nil {
		log.Println("zlib", err)
	}
	_, err = io.Copy(&out, r)
	if err != nil {
		log.Println("zlib copy", err)
	}
	return out.Bytes()
}

// 进行brotli解压缩
func doBrotliUnCompress(compressSrc []byte) []byte {
	br := brotli.NewReader(nil)
	br.Reset(bytes.NewReader(compressSrc))
	bs, err := io.ReadAll(br)
	if err != nil {
		log.Println("doBrotliUnCompress ReadAll", err)
		return []byte{}
	}
	return bs
}
