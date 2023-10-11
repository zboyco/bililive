package main

import (
	"context"
	"fmt"
	"github.com/zboyco/bililive"
)

func main() {
	live := &bililive.Live{
		//Debug:       true,
		StormFilter: true, // 过滤节奏风暴弹幕
		Raw: func(roomID int, msg []byte) {
			fmt.Println(roomID, string(msg))
		},
		//ReceivePopularValue: func(roomID int, value uint32) {
		//	log.Printf("【人气】:  %v", value)
		//},
	}

	live.Start(context.Background())
	_ = live.Join(27732004)
	live.Wait()
}
