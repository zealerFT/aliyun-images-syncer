package main

import (
	"fmt"
	"time"

	"aliyun-images-syncer/util/svcutil"
)

func main() {
	// 自己控制是否结束轮询
	svcutil.WaitFor(":8001", func(stop <-chan struct{}) error {
		for {
			select {
			case <-stop:
				return nil
			default:
				fmt.Println(time.Now())
			}
		}
	})
}

// 任务结束或收到结束Signal，服务结束
func main1() {
	svcutil.StandBy(":8002", func() {
		fmt.Println("doing ...")
		time.Sleep(3 * time.Second)
		fmt.Println("done ...")
	})
}

// 优雅轮询，传入ticker，持续执行时候不会因为比ticker时候短而提前结束，而是等待主程序执行完后在定时，并且在遇到结束信号时终结
func main2() {
	ticker := time.NewTicker(3 * time.Second)
	svcutil.NeverStopByTicker(":8003", ticker, func() {
		fmt.Println("doing ...")
	})
}
