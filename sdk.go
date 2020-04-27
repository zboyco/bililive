package bililive

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"time"
)

const (
	roomInitURL                    string = "https://api.live.bilibili.com/room/v1/Room/room_init?id=%d"
	roomConfigURL                  string = "https://api.live.bilibili.com/room/v1/Danmu/getConf?room_id=%d"
	roomDetailURL                  string = "https://api.live.bilibili.com/xlive/web-room/v1/index/getInfoByRoom?room_id=%d"
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
	WS_HEADER_DEFAULT_VERSION        int16 = 1
	//WS_HEADER_DEFAULT_OPERATION      int32 = 1
	WS_HEADER_DEFAULT_SEQUENCE int32 = 1
	WS_AUTH_OK                 int32 = 0
	WS_AUTH_TOKEN_ERROR        int32 = -101
)

// Start 开始接收
func (room *LiveRoom) Start(ctx context.Context) {
	rand.Seed(time.Now().Unix())
	if room.AnalysisRoutineNum == 0 {
		room.AnalysisRoutineNum = 1
	}

	chConn := room.createConnect()

	room.chSocketMessage = make(chan []byte, 10)
	room.chOperation = make(chan *operateInfo, 100)
	if room.StormFilter && room.ReceiveMsg != nil {
		room.stormContent = make(map[int64]string)
	}

	nextCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	for i := 0; i < room.AnalysisRoutineNum; i++ {
		go room.analysis(nextCtx)
	}

	room.conn = <-chConn
	room.enterRoom()
	go room.heartBeat(nextCtx)
	go room.split(nextCtx)
	room.receive(ctx)
}

func (room *LiveRoom) noticeRoomDetail() *RoomDetailModel {
	resRoomDetail, err := httpSend(fmt.Sprintf(roomDetailURL, room.RoomID))
	if err != nil {
		if room.Debug {
			log.Println(err)
		}
		return nil
	}
	roomInfo := roomDetailResult{}
	err = json.Unmarshal(resRoomDetail, &roomInfo)
	if err != nil {
		if room.Debug {
			log.Println(err)
		}
		return nil
	}
	return roomInfo.Data
}

func (room *LiveRoom) findServer() error {
	resRoom, err := httpSend(fmt.Sprintf(roomInitURL, room.RoomID))
	if err != nil {
		return err
	}
	if room.Debug {
		log.Println(string(resRoom))
	}
	roomInfo := roomInfoResult{}
	json.Unmarshal(resRoom, &roomInfo)
	if roomInfo.Code != 0 {
		return errors.New("房间不正确")
	}
	room.RoomID = roomInfo.Data.RoomID
	resDanmuConfig, err := httpSend(fmt.Sprintf(roomConfigURL, room.RoomID))
	if err != nil {
		return err
	}
	if room.Debug {
		log.Println(string(resDanmuConfig))
	}
	danmuConfig := danmuConfigResult{}
	json.Unmarshal(resDanmuConfig, &danmuConfig)
	room.server = danmuConfig.Data.Host
	room.port = danmuConfig.Data.Port
	room.hostServerList = danmuConfig.Data.HostServerList
	room.token = danmuConfig.Data.Token
	room.currentServerIndex = 0
	return nil
}

func (room *LiveRoom) createConnect() <-chan *net.TCPConn {
	result := make(chan *net.TCPConn)
	go func() {
		defer close(result)

		for {
			if room.hostServerList == nil || len(room.hostServerList) == room.currentServerIndex {
				err := room.findServer()
				if err != nil {
					log.Panic(err)
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
					time.Sleep(3 * time.Second)
					counter++
					continue
				}
				result <- conn
				log.Println("连接创建成功：", room.hostServerList[room.currentServerIndex].Host, room.hostServerList[room.currentServerIndex].Port)
				room.currentServerIndex++
				return
			}
		}
	}()
	return result
}

func (room *LiveRoom) enterRoom() {
	enterInfo := &enterInfo{
		RoomID:    room.RoomID,
		UserID:    9999999999 + rand.Int63(),
		ProtoVer:  2,
		Platform:  "web",
		ClientVer: "1.10.6",
		Type:      2,
		Key:       room.token,
	}

	payload, err := json.Marshal(enterInfo)
	if err != nil {
		log.Panic(err)
	}
	room.sendData(WS_OP_USER_AUTHENTICATION, payload)
}

func connect(host string, port int) (*net.TCPConn, error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp4", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return nil, err
	}
	return net.DialTCP("tcp", nil, tcpAddr)
}

// 心跳
func (room *LiveRoom) heartBeat(ctx context.Context) {
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
func (room *LiveRoom) receive(ctx context.Context) {
	// 包头总长16个字节
	headerBuffer := make([]byte, WS_PACKAGE_HEADER_TOTAL_LENGTH)
	// headerBufferReader
	var headerBufferReader *bytes.Reader
	// 包体
	messageBody := make([]byte, 0)
	counter := 0
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		_, err := io.ReadFull(room.conn, headerBuffer)
		if err != nil {
			if room.Debug || counter >= 10 {
				log.Panic(err)
			}
			log.Println("read err:", err)
			room.conn = <-room.createConnect()
			room.enterRoom()
			counter++
			continue
		}

		var head messageHeader
		headerBufferReader = bytes.NewReader(headerBuffer)
		binary.Read(headerBufferReader, binary.BigEndian, &head)

		if head.Length < WS_PACKAGE_HEADER_TOTAL_LENGTH {
			if room.Debug || counter >= 10 {
				log.Println("***************协议失败***************")
				log.Println("数据包长度:", head.Length)
				log.Panic("数据包长度不正确")
			}
			room.conn = <-room.createConnect()
			room.enterRoom()
			counter++
			continue
		}

		payloadBuffer := make([]byte, head.Length-WS_PACKAGE_HEADER_TOTAL_LENGTH)
		_, err = io.ReadFull(room.conn, payloadBuffer)
		if err != nil {
			if room.Debug || counter >= 10 {
				log.Panic(err)
			}
			log.Println("read err:", err)
			room.conn = <-room.createConnect()
			room.enterRoom()
			counter++
			continue
		}

		messageBody = append(headerBuffer, payloadBuffer...)

		room.chSocketMessage <- messageBody
		counter = 0
	}
}

// 拆分数据
func (room *LiveRoom) split(ctx context.Context) {
	var (
		messageBody        []byte
		head               messageHeader
		headerBufferReader *bytes.Reader
		payloadBuffer      []byte
	)
	for {
		messageBody = <-room.chSocketMessage
		for len(messageBody) > 0 {
			select {
			case <-ctx.Done():
				return
			default:
			}

			headerBufferReader = bytes.NewReader(messageBody[:WS_PACKAGE_HEADER_TOTAL_LENGTH])
			binary.Read(headerBufferReader, binary.BigEndian, &head)
			payloadBuffer = messageBody[WS_PACKAGE_HEADER_TOTAL_LENGTH:head.Length]
			messageBody = messageBody[head.Length:]

			if head.Length == WS_PACKAGE_HEADER_TOTAL_LENGTH {
				continue
			}

			if head.ProtocolVersion == WS_BODY_PROTOCOL_VERSION_DEFLATE {
				messageBody = doZlibUnCompress(payloadBuffer)
				continue
			}
			if room.Debug {
				log.Println(string(payloadBuffer))
			}
			room.chOperation <- &operateInfo{Operation: head.Operation, Buffer: payloadBuffer}
		}
	}
}

// 分析接收到的数据
func (room *LiveRoom) analysis(ctx context.Context) {
analysis:
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		buffer := <-room.chOperation
		switch buffer.Operation {
		case WS_OP_HEARTBEAT_REPLY:
			if room.ReceivePopularValue != nil {
				m := binary.BigEndian.Uint32(buffer.Buffer)
				room.ReceivePopularValue(m)
			}
		case WS_OP_CONNECT_SUCCESS:
			if room.Debug {
				log.Println("CONNECT_SUCCESS", string(buffer.Buffer))
			}
		case WS_OP_MESSAGE:
			result := cmdModel{}
			err := json.Unmarshal(buffer.Buffer, &result)
			if err != nil {
				if room.Debug {
					log.Println(err)
					log.Println(string(buffer.Buffer))
				}
				continue
			}
			temp, err := json.Marshal(result.Data)
			if err != nil {
				if room.Debug {
					log.Println(err)
				}
				continue
			}
			switch result.CMD {
			case "LIVE": // 直播开始
				if room.Live != nil {
					room.Live(room.noticeRoomDetail())
				}
			case "CLOSE": // 关闭
				fallthrough
			case "PREPARING": // 准备
				fallthrough
			case "END": // 结束
				log.Println(string(buffer.Buffer))
				if room.End != nil {
					room.End(room.noticeRoomDetail())
				}
			case "SYS_MSG": // 系统消息
				if room.SysMessage != nil {
					m := &SysMsgModel{}
					json.Unmarshal(buffer.Buffer, m)
					room.SysMessage(m)
				}
			case "ROOM_CHANGE": // 房间信息变更
				if room.RoomChange != nil {
					m := &RoomChangeModel{}
					json.Unmarshal(temp, m)
					room.RoomChange(m)
				}
			case "WELCOME": // 用户进入
				if room.UserEnter != nil {
					m := &UserEnterModel{}
					json.Unmarshal(temp, m)
					room.UserEnter(m)
				}
			case "WELCOME_GUARD": // 舰长进入
				if room.GuardEnter != nil {
					m := &GuardEnterModel{}
					json.Unmarshal(temp, m)
					room.GuardEnter(m)
				}
			case "DANMU_MSG": // 弹幕
				if room.ReceiveMsg != nil {
					msgContent := result.Info[1].(string)

					if room.StormFilter && room.storming {
						for _, value := range room.stormContent {
							if msgContent == value {
								log.Println("过滤弹幕：", value)
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
					if medalInfo != nil && len(medalInfo) >= 4 {
						m.MedalLevel = int(medalInfo[0].(float64))
						m.MedalName = medalInfo[1].(string)
						m.MedalUpName = medalInfo[2].(string)
						m.MedalRoomID = int64(medalInfo[3].(float64))
					}
					room.ReceiveMsg(m)
				}
			case "SEND_GIFT": // 礼物通知
				if room.ReceiveGift != nil {
					m := &GiftModel{}
					json.Unmarshal(temp, m)
					room.ReceiveGift(m)
				}
			case "COMBO_SEND": // 连击
				if room.GiftComboSend != nil {
					m := &ComboSendModel{}
					json.Unmarshal(temp, m)
					room.GiftComboSend(m)
				}
			case "COMBO_END": // 连击结束
				if room.GiftComboEnd != nil {
					m := &ComboEndModel{}
					json.Unmarshal(temp, m)
					room.GiftComboEnd(m)
				}
			case "GUARD_BUY": // 上船
				if room.GuardBuy != nil {
					m := &GuardBuyModel{}
					json.Unmarshal(temp, m)
					room.GuardBuy(m)
				}
			case "ROOM_REAL_TIME_MESSAGE_UPDATE": // 粉丝数更新
				if room.FansUpdate != nil {
					m := &FansUpdateModel{}
					json.Unmarshal(temp, m)
					room.FansUpdate(m)
				}
			case "ROOM_RANK": // 小时榜
				if room.RoomRank != nil {
					m := &RankModel{}
					json.Unmarshal(temp, m)
					room.RoomRank(m)
				}
			case "SPECIAL_GIFT": // 特殊礼物
				log.Println(string(buffer.Buffer))
				m := &SpecialGiftModel{}
				json.Unmarshal(temp, m)
				if room.StormFilter && room.ReceiveMsg != nil {
					if m.Storm.Action == "start" {
						room.storming = true
						room.stormContent[m.Storm.ID] = m.Storm.Content
						log.Println("添加过滤弹幕：", m.Storm.ID, m.Storm.Content)
					}
					if m.Storm.Action == "end" {
						delete(room.stormContent, m.Storm.ID)
						room.storming = len(room.stormContent) > 0
						log.Println("移除过滤弹幕：", m.Storm.ID)
					}
				}
				if room.SpecialGift != nil {
					room.SpecialGift(m)
				}
			case "SUPER_CHAT_MESSAGE": // 醒目留言
				if room.SuperChatMessage != nil {
					m := &SuperChatMessageModel{}
					json.Unmarshal(temp, m)
					room.SuperChatMessage(m)
				}
			case "SUPER_CHAT_MESSAGE_JPN":
				if room.Debug {
					log.Println(string(buffer.Buffer))
				}
			case "SYS_GIFT": // 系统礼物
				fallthrough
			case "BLOCK": // 未知
				fallthrough
			case "ROUND": // 未知
				fallthrough
			case "REFRESH": // 刷新
				log.Println(string(buffer.Buffer))
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
			case "ENTRY_EFFECT": // 进入效果
				fallthrough
			case "WISH_BOTTLE": // 许愿瓶
				fallthrough
			case "ROOM_BLOCK_MSG":
				fallthrough
			case "WEEK_STAR_CLOCK":
				fallthrough
			default:
				if room.Debug {
					log.Println(string(buffer.Buffer))
				}
			}
		default:
			break
		}
	}
}

// 发送数据
func (room *LiveRoom) sendData(operation int32, payload []byte) {

	b := bytes.NewBuffer([]byte{})
	head := messageHeader{
		Length:          int32(len(payload)) + WS_PACKAGE_HEADER_TOTAL_LENGTH,
		HeaderLength:    int16(WS_PACKAGE_HEADER_TOTAL_LENGTH),
		ProtocolVersion: WS_HEADER_DEFAULT_VERSION,
		Operation:       operation,
		SequenceID:      WS_HEADER_DEFAULT_SEQUENCE,
	}
	err := binary.Write(b, binary.BigEndian, head)
	if err != nil && room.Debug {
		log.Println(err)
	}

	err = binary.Write(b, binary.LittleEndian, payload)
	if err != nil && room.Debug {
		log.Println(err)
	}

	_, err = room.conn.Write(b.Bytes())
	if err != nil && room.Debug {
		log.Println(err)
	}
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
