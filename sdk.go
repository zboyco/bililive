package bililive

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"log"
	"math/rand"
	"net"
	"runtime"
	"strconv"
	"time"
)

const roomInfoURL string = "https://api.live.bilibili.com/room/v1/Room/room_init?id="
const cidInfoURL string = "https://api.live.bilibili.com/room/v1/Danmu/getConf?room_id="

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
	room.server = danmuConfig.Data.Host
	room.port = danmuConfig.Data.Port
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
		RoomID: room.RoomID,
		UserID: 9999999999 + rand.Uint64(),
	}

	playload, err := json.Marshal(enterInfo)
	if err != nil {
		log.Panic(err)
	}
	room.sendData(7, playload)
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
		// 包头总长16个字节,包括 数据包长(4),magic(2),protocol_version(2),typeid(4),params(4)
		headBuffer := make([]byte, 16)
		_, err := io.ReadFull(room.conn, headBuffer)
		if err != nil {
			log.Panicln(err)
		}

		packetLength := binary.BigEndian.Uint32(headBuffer[:4])

		if packetLength < 16 || packetLength > 3072 {
			log.Println("***************协议失败***************")
			log.Println("数据包长度:", packetLength)
			err := room.createConnect()
			if err != nil {
				log.Panic(err)
			}
			room.enterRoom()
			continue
		}

		typeID := binary.BigEndian.Uint32(headBuffer[8:12]) // 读取typeid

		playloadlength := packetLength - 16

		if playloadlength == 0 {
			continue
		}

		playloadBuffer := make([]byte, playloadlength)
		_, err = io.ReadFull(room.conn, playloadBuffer)
		if err != nil {
			log.Panicln(err)
		}

		room.chBuffer <- &bufferInfo{TypeID: typeID, Buffer: playloadBuffer}
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
		switch buffer.TypeID {
		case 3:
			if room.ReceivePopularValue != nil {
				viewer := binary.BigEndian.Uint32(buffer.Buffer)
				room.ReceivePopularValue(viewer)
			}
		case 5:
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
				// log.Println(string(buffer.Buffer))
				break
			}
		default:
			break
		}
	}
}

// 发送数据
func (room *LiveRoom) sendData(action int, playload []byte) {
	packetlength := len(playload) + 16

	b := bytes.NewBuffer([]byte{})
	binary.Write(b, binary.BigEndian, int32(packetlength))

	binary.Write(b, binary.BigEndian, int16(16))

	binary.Write(b, binary.BigEndian, int16(1))

	binary.Write(b, binary.BigEndian, int32(action))

	binary.Write(b, binary.BigEndian, int32(1))

	binary.Write(b, binary.LittleEndian, playload)

	room.conn.Write(b.Bytes())
}
