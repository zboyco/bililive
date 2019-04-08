package bililive

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"encoding/xml"
	"errors"
	"log"
	"math/rand"
	"net"
	"strconv"
	"time"
)

const roomInfoURL string = "https://api.live.bilibili.com/room/v1/Room/room_init?id="
const cidInfoURL string = "http://live.bilibili.com/api/player?id=cid:"

// Start 开始接收
func (room *LiveRoom) Start() {
	err := room.createConnect()
	if err != nil {
		log.Panic(err)
	}
	room.enterRoom()
	go room.heartBeat()
	room.receive()
}

func (room *LiveRoom) createConnect() error {
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
	resCharacter, err := httpSend(cidInfoURL + strconv.Itoa(room.RoomID))
	if err != nil {
		return err
	}
	resStr := "<root>" + string(resCharacter) + "</root>"
	characterInfo := characterInfoReuslt{}
	xml.Unmarshal([]byte(resStr), &characterInfo)
	conn, err := connect(characterInfo.DMServer, characterInfo.DMPort)
	if err != nil {
		return err
	}
	room.conn = conn
	return nil
}

func (room *LiveRoom) enterRoom() {
	enterInfo := &enterInfo{
		RoomID: room.RoomID,
		UserID: rand.Uint64(),
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
func (room *LiveRoom) heartBeat() {
	for {
		room.sendData(2, []byte{})
		time.Sleep(30 * time.Second)
	}
}

// 接收消息
func (room *LiveRoom) receive() {
	buffer := make([]byte, 4)
	for {
		room.conn.Read(buffer)
		packetlength := binary.BigEndian.Uint32(buffer)

		if packetlength < 16 {
			log.Panic("协议失败")
		}

		room.conn.Read(buffer) // 过滤 magic,protocol_version

		room.conn.Read(buffer)
		typeID := binary.BigEndian.Uint32(buffer)

		room.conn.Read(buffer) // 过滤 params

		playloadlength := packetlength - 16

		if playloadlength == 0 {
			continue // 没有内容了
		}

		playloadBuffer := make([]byte, playloadlength)

		room.conn.Read(playloadBuffer)

		switch typeID {
		case 3:
			if room.ReceivePopularValue != nil {
				viewer := binary.BigEndian.Uint32(playloadBuffer)
				room.ReceivePopularValue(viewer)
			}
		case 5:
			//log.Println(string(playloadBuffer))
			if room.ReceiveMsg != nil {
				result := cmdModel{}
				json.Unmarshal(playloadBuffer, &result)
				switch result.CMD {
				case "DANMU_MSG":
					if room.ReceiveMsg != nil {
						userInfo := result.Info[2].([]interface{})
						msg := &MsgModel{
							Content:  result.Info[1].(string),
							UserName: userInfo[1].(string),
						}
						room.ReceiveMsg(msg)
					}
				case "SEND_GIFT":
					if room.ReceiveGift != nil {
						room.ReceiveGift(result.Data)
					}
				default:
					break
				}
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
