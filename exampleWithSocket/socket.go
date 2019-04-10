package main

import (
	"errors"
	"fmt"
	"log"
	"net"
)

type socket struct {
	tcpAddr *net.TCPAddr
	conn    *net.TCPConn
}

func newSocket(ip string, port int) *socket {
	tcpAddr, err := net.ResolveTCPAddr("tcp4", fmt.Sprintf("%v:%v", ip, port))
	if err != nil {
		panic(err)
	}
	return &socket{
		tcpAddr: tcpAddr,
	}
}

func (s *socket) connect() error {
	conn, err := net.DialTCP("tcp", nil, s.tcpAddr)
	if err != nil {
		return errors.New("###通讯报告###：创建服务器连接失败")
	}
	s.conn = conn
	return nil
}

func (s *socket) sendTCP(buff []byte) {
	if s.conn == nil {
		err := s.connect()
		if err != nil {
			log.Println(err)
			return
		}
	}
	_, err := s.conn.Write(buff)
	if err != nil {
		s.conn.Close()
		s.conn = nil
		s.sendTCP(buff)
	}
}
