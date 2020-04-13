package bililive

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"runtime"
	"strconv"
	"time"
)

const (
	roomInfoURL                      string = "https://api.live.bilibili.com/room/v1/Room/room_init?id="
	cidInfoURL                       string = "https://api.live.bilibili.com/room/v1/Danmu/getConf?room_id="
	WS_OP_HEARTBEAT                  int32  = 2
	WS_OP_HEARTBEAT_REPLY            int32  = 3
	WS_OP_MESSAGE                    int32  = 5
	WS_OP_USER_AUTHENTICATION        int32  = 7
	WS_OP_CONNECT_SUCCESS            int32  = 8
	WS_PACKAGE_HEADER_TOTAL_LENGTH   int32  = 16
	WS_PACKAGE_OFFSET                int32  = 0
	WS_HEADER_OFFSET                 int32  = 4
	WS_VERSION_OFFSET                int32  = 6
	WS_OPERATION_OFFSET              int32  = 8
	WS_SEQUENCE_OFFSET               int32  = 12
	WS_BODY_PROTOCOL_VERSION_NORMAL  int32  = 0
	WS_BODY_PROTOCOL_VERSION_DEFLATE int32  = 2
	WS_HEADER_DEFAULT_VERSION        int32  = 1
	WS_HEADER_DEFAULT_OPERATION      int32  = 1
	WS_HEADER_DEFAULT_SEQUENCE       int32  = 1
	WS_AUTH_OK                       int32  = 0
	WS_AUTH_TOKEN_ERROR              int32  = -101
)

// Start 开始接收
func (room *LiveRoom) Start() {
	err := room.findServer()
	if err != nil {
		log.Panic(err)
	}

	room.conn = <-room.createConnect()

	room.chBuffer = make(chan *bufferInfo, 1000)
	room.chMsg = make(chan *MsgModel, 300)
	room.chGift = make(chan *GiftModel, 100)
	room.chPopularValue = make(chan uint32, 1)
	room.chUserEnter = make(chan *UserEnterModel, 10)
	room.chGuardEnter = make(chan *GuardEnterModel, 3)
	room.chGiftComboEnd = make(chan *ComboEndModel, 10)
	room.chGuardBuy = make(chan *GuardBuyModel, 3)

	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	for i, max := 0, runtime.NumCPU(); i < max; i++ {
		go room.analysis(ctx)
		go room.notice(ctx)
	}
	room.enterRoom()
	go room.heartBeat(ctx)
	room.receive()
}

func (room *LiveRoom) findServer() error {
	resRoom, err := httpSend(roomInfoURL + strconv.Itoa(room.RoomID))
	if err != nil {
		return err
	}
	roomInfo := roomInfoResult{}
	json.Unmarshal(resRoom, &roomInfo)
	if roomInfo.Code != 0 {
		return errors.New("房间不正确")
	}
	room.RoomID = roomInfo.Data.RoomID
	resDanmuConfig, err := httpSend(cidInfoURL + strconv.Itoa(room.RoomID))
	if err != nil {
		return err
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
		ProtoVer:  1,
		Platform:  "web",
		ClientVer: "1.10.6",
		Type:      2,
		Key:       room.token,
	}

	playload, err := json.Marshal(enterInfo)
	if err != nil {
		log.Panic(err)
	}
	room.sendData(WS_OP_USER_AUTHENTICATION, playload)
}

func connect(host string, port int) (*net.TCPConn, error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp4", host+":"+strconv.Itoa(port))
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

		room.sendData(2, []byte{})
		time.Sleep(30 * time.Second)
	}
}

func (room *LiveRoom) notice(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
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
		case m := <-room.chGiftComboEnd:
			go room.GiftComboEnd(m)
		case m := <-room.chGuardBuy:
			go room.GuardBuy(m)
		}
	}
}

// 接收消息
func (room *LiveRoom) receive() {
	for {
		// 包头总长16个字节
		headBuffer := make([]byte, 16)
		_, err := io.ReadFull(room.conn, headBuffer)
		if err != nil {
			log.Panicln(err)
		}

		var head messageHeader
		buf := bytes.NewReader(headBuffer)
		binary.Read(buf, binary.BigEndian, &head)

		if head.Length < 16 {
			log.Println("***************协议失败***************")
			log.Println("数据包长度:", head.Length)
			err := room.createConnect()
			if err != nil {
				log.Panic(err)
			}
			room.enterRoom()
			continue
		}

		if head.Length == 16 {
			continue
		}

		payloadBuffer := make([]byte, head.Length-16)
		_, err = io.ReadFull(room.conn, payloadBuffer)
		if err != nil {
			log.Panicln(err)
		}

		room.chBuffer <- &bufferInfo{Operation: head.Operation, Buffer: payloadBuffer}
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

		buffer := <-room.chBuffer
		switch buffer.Operation {
		case WS_OP_HEARTBEAT_REPLY:
			if room.ReceivePopularValue != nil {
				viewer := binary.BigEndian.Uint32(buffer.Buffer)
				room.ReceivePopularValue(viewer)
			}
		case WS_OP_USER_AUTHENTICATION:
			log.Println(string(buffer.Buffer))
		case WS_OP_CONNECT_SUCCESS:
			log.Println(string(buffer.Buffer))
		case WS_OP_MESSAGE:
			result := cmdModel{}
			err := json.Unmarshal(buffer.Buffer, &result)
			if err != nil {
				log.Println(err)
				log.Println(string(buffer.Buffer))
				continue
			}
			temp, err := json.Marshal(result.Data)
			if err != nil {
				log.Println(err)
				continue
			}
			switch result.CMD {
			case "WELCOME":
				if room.UserEnter != nil {
					m := &UserEnterModel{}
					json.Unmarshal(temp, m)
					room.chUserEnter <- m
				}
			case "WELCOME_GUARD":
				if room.GuardEnter != nil {
					m := &GuardEnterModel{}
					json.Unmarshal(temp, m)
					room.chGuardEnter <- m
				}
			case "DANMU_MSG":
				if room.ReceiveMsg != nil {
					userInfo := result.Info[2].([]interface{})
					m := &MsgModel{
						Content:  result.Info[1].(string),
						UserName: userInfo[1].(string),
						UserID:   strconv.FormatFloat(userInfo[0].(float64), 'f', -1, 64),
					}
					room.chMsg <- m
				}
			case "SEND_GIFT":
				if room.ReceiveGift != nil {
					m := &GiftModel{}
					json.Unmarshal(temp, m)
					room.chGift <- m
				}
			case "COMBO_END":
				if room.GiftComboEnd != nil {
					m := &ComboEndModel{}
					json.Unmarshal(temp, m)
					room.chGiftComboEnd <- m
				}
			case "GUARD_BUY":
				if room.GuardBuy != nil {
					m := &GuardBuyModel{}
					json.Unmarshal(temp, m)
					room.chGuardBuy <- m
				}
			default:
				// log.Println(result.Data)
				log.Println(string(buffer.Buffer))
				break
			}
		default:
			break
		}
	}
}

// 发送数据
func (room *LiveRoom) sendData(operation int32, playload []byte) {

	b := bytes.NewBuffer([]byte{})
	head := messageHeader{
		Length:          int32(len(playload) + 16),
		HeaderLength:    16,
		ProtocolVersion: 1,
		Operation:       operation,
		SequenceID:      1,
	}
	binary.Write(b, binary.BigEndian, head)

	binary.Write(b, binary.LittleEndian, playload)

	room.conn.Write(b.Bytes())
}
