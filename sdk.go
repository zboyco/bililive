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
	"net"
	"runtime"
	"strconv"
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
func (room *LiveRoom) Start() {
	err := room.findServer()
	if err != nil {
		log.Panic(err)
	}

	chConn := room.createConnect()

	room.chSocketMessage = make(chan []byte, 10)
	room.chRoomDetail = make(chan *RoomDetailModel, 1)
	room.chOperation = make(chan *operateInfo, 1000)
	room.chSysMessage = make(chan *SysMsgModel, 3)
	room.chMsg = make(chan *MsgModel, 300)
	room.chGift = make(chan *GiftModel, 100)
	room.chPopularValue = make(chan uint32, 1)
	room.chUserEnter = make(chan *UserEnterModel, 10)
	room.chGuardEnter = make(chan *GuardEnterModel, 3)
	room.chGiftComboSend = make(chan *ComboSendModel, 10)
	room.chGiftComboEnd = make(chan *ComboEndModel, 5)
	room.chGuardBuy = make(chan *GuardBuyModel, 3)
	room.chFansUpdate = make(chan *FansUpdateModel, 1)
	room.chRank = make(chan *RankModel, 5)
	room.chRoomChange = make(chan *RoomChangeModel, 1)
	room.chSuperChatMessage = make(chan *SuperChatMessageModel, 2)

	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	for i, max := 0, runtime.NumCPU(); i < max; i++ {
		go room.analysis(ctx)
		go room.notice(ctx)
	}

	go room.roomDetail(ctx)

	room.conn = <-chConn
	room.enterRoom()
	go room.heartBeat(ctx)
	go room.split(ctx)
	room.receive()
}

func (room *LiveRoom) roomDetail(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			resRoomDetail, err := httpSend(fmt.Sprintf(roomDetailURL, room.RoomID))
			if err != nil {
				log.Println(err)
			}
			roomInfo := roomDetailResult{}
			json.Unmarshal(resRoomDetail, &roomInfo)
			room.chRoomDetail <- roomInfo.Data.RoomInfo
		}
		time.Sleep(5 * 60 * time.Second)
	}

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
	room.server = danmuConfig.Data.HostServerList[0].Host
	room.port = danmuConfig.Data.HostServerList[0].Port
	room.token = danmuConfig.Data.Token
	return nil
}

func (room *LiveRoom) createConnect() <-chan *net.TCPConn {
	result := make(chan *net.TCPConn)
	go func() {
		defer close(result)
		conn, err := connect(room.server, room.port)
		if err != nil {
			log.Panic(err)
		}
		result <- conn
	}()
	return result
}

func (room *LiveRoom) enterRoom() {
	enterInfo := &enterInfo{
		RoomID:    room.RoomID,
		UserID:    0,
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

func (room *LiveRoom) notice(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case m := <-room.chSysMessage:
			go room.SysMessage(m)
		case m := <-room.chPopularValue:
			go room.ReceivePopularValue(m)
		case m := <-room.chUserEnter:
			go room.UserEnter(m)
		case m := <-room.chGuardEnter:
			go room.GuardEnter(m)
		case m := <-room.chMsg:
			go room.ReceiveMsg(m)
		case m := <-room.chGift:
			go room.ReceiveGift(m)
		case m := <-room.chGiftComboSend:
			go room.GiftComboSend(m)
		case m := <-room.chGiftComboEnd:
			go room.GiftComboEnd(m)
		case m := <-room.chGuardBuy:
			go room.GuardBuy(m)
		case m := <-room.chFansUpdate:
			go room.FansUpdate(m)
		case m := <-room.chRank:
			go room.RoomRank(m)
		case m := <-room.chRoomDetail:
			go room.RoomInfo(m)
		case m := <-room.chRoomChange:
			go room.RoomChange(m)
		case m := <-room.chSpecialGift:
			go room.SpecialGift(m)
		case m := <-room.chSuperChatMessage:
			go room.SuperChatMessage(m)
		}
	}
}

// 接收消息
func (room *LiveRoom) receive() {
	// 包头总长16个字节
	headerBuffer := make([]byte, WS_PACKAGE_HEADER_TOTAL_LENGTH)
	// headerBufferReader
	var headerBufferReader *bytes.Reader
	// 包体
	messageBody := make([]byte, 0)
	for {
		_, err := io.ReadFull(room.conn, headerBuffer)
		if err != nil {
			log.Panicln(err)
		}

		var head messageHeader
		headerBufferReader = bytes.NewReader(headerBuffer)
		binary.Read(headerBufferReader, binary.BigEndian, &head)

		if head.Length < WS_PACKAGE_HEADER_TOTAL_LENGTH {
			if room.Debug {
				log.Println("***************协议失败***************")
				log.Println("数据包长度:", head.Length)
				log.Panic("数据包长度不正确")
			}
			room.conn = <-room.createConnect()
			room.enterRoom()
			continue
		}

		payloadBuffer := make([]byte, head.Length-WS_PACKAGE_HEADER_TOTAL_LENGTH)
		_, err = io.ReadFull(room.conn, payloadBuffer)
		if err != nil {
			log.Panicln(err)
		}

		messageBody = append(headerBuffer, payloadBuffer...)

		room.chSocketMessage <- messageBody
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
				viewer := binary.BigEndian.Uint32(buffer.Buffer)
				room.ReceivePopularValue(viewer)
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
			case "SYS_MSG": // 系统消息
				if room.SysMessage != nil {
					m := &SysMsgModel{}
					json.Unmarshal(buffer.Buffer, m)
					room.chSysMessage <- m
				}
			case "ROOM_CHANGE": // 房间信息变更
				if room.RoomChange != nil {
					m := &RoomChangeModel{}
					json.Unmarshal(temp, m)
					room.chRoomChange <- m
				}
			case "WELCOME": // 用户进入
				if room.UserEnter != nil {
					m := &UserEnterModel{}
					json.Unmarshal(temp, m)
					room.chUserEnter <- m
				}
			case "WELCOME_GUARD": // 舰长进入
				if room.GuardEnter != nil {
					m := &GuardEnterModel{}
					json.Unmarshal(temp, m)
					room.chGuardEnter <- m
				}
			case "DANMU_MSG": // 弹幕
				if room.ReceiveMsg != nil {
					userInfo := result.Info[2].([]interface{})
					m := &MsgModel{
						Content:  result.Info[1].(string),
						UserName: userInfo[1].(string),
						UserID:   strconv.FormatFloat(userInfo[0].(float64), 'f', -1, 64),
					}
					room.chMsg <- m
				}
			case "SEND_GIFT": // 礼物通知
				if room.ReceiveGift != nil {
					m := &GiftModel{}
					json.Unmarshal(temp, m)
					room.chGift <- m
				}
			case "COMBO_SEND": // 连击
				if room.GiftComboSend != nil {
					m := &ComboSendModel{}
					json.Unmarshal(temp, m)
					room.chGiftComboSend <- m
				}
			case "COMBO_END": // 连击结束
				if room.GiftComboEnd != nil {
					m := &ComboEndModel{}
					json.Unmarshal(temp, m)
					room.chGiftComboEnd <- m
				}
			case "GUARD_BUY": // 上船
				if room.GuardBuy != nil {
					m := &GuardBuyModel{}
					json.Unmarshal(temp, m)
					room.chGuardBuy <- m
				}
			case "ROOM_REAL_TIME_MESSAGE_UPDATE": // 粉丝数更新
				if room.FansUpdate != nil {
					m := &FansUpdateModel{}
					json.Unmarshal(temp, m)
					room.chFansUpdate <- m
				}
			case "ROOM_RANK": // 小时榜
				if room.RoomRank != nil {
					m := &RankModel{}
					json.Unmarshal(temp, m)
					room.chRank <- m
				}
			case "SPECIAL_GIFT": // 特殊礼物
				if room.SpecialGift != nil {
					m := &SpecialGiftModel{}
					json.Unmarshal(temp, m)
					room.chSpecialGift <- m
				}
			case "SUPER_CHAT_MESSAGE": // 超级留言
				if room.SpecialGift != nil {
					m := &SpecialGiftModel{}
					json.Unmarshal(temp, m)
					room.chSpecialGift <- m
				}
			case "SUPER_CHAT_MESSAGE_JPN":
				if room.Debug {
					log.Println(string(buffer.Buffer))
				}
			case "ACTIVITY_BANNER_UPDATE_V2":
				if room.Debug {
					log.Println(string(buffer.Buffer))
				}
			case "ANCHOR_LOT_CHECKSTATUS":
				if room.Debug {
					log.Println(string(buffer.Buffer))
				}
			case "GUARD_MSG": // 舰长信息
				if room.Debug {
					log.Println(string(buffer.Buffer))
				}
			case "NOTICE_MSG": // 通知信息
				if room.Debug {
					log.Println(string(buffer.Buffer))
				}
			case "GUARD_LOTTERY_START": // 舰长抽奖开始
				if room.Debug {
					log.Println(string(buffer.Buffer))
				}
			case "USER_TOAST_MSG": // 用户通知消息
				if room.Debug {
					log.Println(string(buffer.Buffer))
				}
			case "ENTRY_EFFECT": // 进入效果
				if room.Debug {
					log.Println(string(buffer.Buffer))
				}
			case "WISH_BOTTLE": // 许愿瓶
				if room.Debug {
					log.Println(string(buffer.Buffer))
				}
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
