package cmd

import (
	"fmt"
	"os"
	"time"

	"aliyun-images-syncer/pkg/client"
	"aliyun-images-syncer/util/svcutil"

	"github.com/spf13/cobra"
)

var (
	logPath, repoNamespaceName, instanceIdMaster, instanceIdSlave, accountMaster, passwordMaster, accountSlave, passwordSlave, accessKeyIdMaster, accessKeySecretMaster, endpointMaster, accessKeyIdSlave, accessKeySecretSlave, endpointSlave string
	publicNetworkMaster, publicNetworkSlave                                                                                                                                                                                                    string
	procNum, retries, polling                                                                                                                                                                                                                  int
	mailHost, mailUserName, mailAuthCode, mailTo                                                                                                                                                                                               string
	repoNamespaceNames                                                                                                                                                                                                                         []string
)

// RootCmd describes "image-syncer" command
var RootCmd = &cobra.Command{
	Use:     "images-sync",
	Aliases: []string{"images-sync"},
	Short:   "A docker registry image real time synchronization tool！by fermi",
	Long:    `A Fast and Flexible docker registry image real time synchronization tool implement by Go.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// work starts here
		_client, err := client.CreateClient(
			&accessKeyIdMaster, &accessKeySecretMaster, &endpointMaster, &accountMaster, &passwordMaster,
			&accessKeyIdSlave, &accessKeySecretSlave, &endpointSlave, &accountSlave, &passwordSlave,
			&repoNamespaceName, &instanceIdMaster, &instanceIdSlave,
			&publicNetworkMaster, &publicNetworkSlave,
			mailHost, mailUserName, mailAuthCode, mailTo,
			logPath, repoNamespaceNames,
		)
		if err != nil {
			return fmt.Errorf("init sync client error: %v", err)
		}

		pollingTime := time.Duration(polling) * time.Second
		_client.MailClient.SendMail("<h1>主从镜像同步服务images-sync 启动！<h1>")
		fmt.Println("轮询间隔pollingTime:", pollingTime)

		//  优雅轮询并且启动健康检查，并且在接收到失败信号好，结束程序
		ticker := time.NewTicker(pollingTime)
		svcutil.NeverStopByTicker(":8000", ticker, func() {
			_client.Logger.Info("正常运行啊，兄弟～")
			_client.Run()
		})

		// mail 通知
		// _client.MailClient.SendMail("<h1>images-sync exit 退出，如非人为退出，请在pod未重新启动后检查具体情况！<h1>")

		return nil

	},
}

func init() {
	RootCmd.PersistentFlags().StringVar(&repoNamespaceName, "repoNamespaceName", "", "镜像仓库namespace, 默认主从是一样ns")

	RootCmd.PersistentFlags().StringArrayVarP(&repoNamespaceNames, "repoNamespaceNames", "n", []string{"one", "two"}, "镜像仓库namespace, 默认主从是一样ns")

	RootCmd.PersistentFlags().StringVar(&accessKeyIdMaster, "accessKeyIdMaster", "", "主阿里云镜像仓库-key")
	RootCmd.PersistentFlags().StringVar(&accessKeySecretMaster, "accessKeySecretMaster", "", "主阿里云镜像仓库-secret")
	RootCmd.PersistentFlags().StringVar(&endpointMaster, "endpointMaster", "cr.cn-shanghai.aliyuncs.com", "主阿里云镜像仓库-host")
	RootCmd.PersistentFlags().StringVar(&publicNetworkMaster, "publicNetworkMaster", "", "主阿里云镜像仓库-公网host")
	RootCmd.PersistentFlags().StringVar(&accountMaster, "accountMaster", "", "主阿里云镜像仓库-账号")
	RootCmd.PersistentFlags().StringVar(&passwordMaster, "passwordMaster", "", "主阿里云镜像仓库-密码")
	RootCmd.PersistentFlags().StringVar(&instanceIdMaster, "instanceIdMaster", "", "主阿里云镜像仓库-实例id")

	RootCmd.PersistentFlags().StringVar(&accessKeyIdSlave, "accessKeyIdSlave", "", "从阿里云镜像仓库-key")
	RootCmd.PersistentFlags().StringVar(&accessKeySecretSlave, "accessKeySecretSlave", "", "从阿里云镜像仓库-secret")
	RootCmd.PersistentFlags().StringVar(&endpointSlave, "endpointSlave", "cr.cn-shanghai.aliyuncs.com", "从阿里云镜像仓库-host")
	RootCmd.PersistentFlags().StringVar(&publicNetworkSlave, "publicNetworkSlave", "", "从阿里云镜像仓库-专用网络")
	RootCmd.PersistentFlags().StringVar(&accountSlave, "accountSlave", "", "从阿里云镜像仓库-账号")
	RootCmd.PersistentFlags().StringVar(&passwordSlave, "passwordSlave", "", "从阿里云镜像仓库-密码")
	RootCmd.PersistentFlags().StringVar(&instanceIdSlave, "instanceIdSlave", "", "从阿里云镜像仓库-实例id")

	RootCmd.PersistentFlags().IntVarP(&procNum, "proc", "p", 5, "检查频率，默认10s检查一次是否有更新")
	RootCmd.PersistentFlags().IntVarP(&retries, "retries", "r", 2, "重试次数times to retry failed task")
	RootCmd.PersistentFlags().StringVar(&logPath, "log", "", "日志log file path (default in os.Stderr)")

	RootCmd.PersistentFlags().IntVarP(&polling, "polling", "o", 300, "轮询检查的时间间隔，默认300s执行一次")

	// mail 相关 UserName, MailTo, SendName
	RootCmd.PersistentFlags().StringVar(&mailHost, "mailHost", "", "邮箱域名")
	RootCmd.PersistentFlags().StringVar(&mailUserName, "mailUserName", "", "邮箱账号")
	RootCmd.PersistentFlags().StringVar(&mailAuthCode, "mailAuthCode", "", "邮箱密钥")
	RootCmd.PersistentFlags().StringVar(&mailTo, "mailTo", "", "邮件内容接收方")
}

// Execute executes the RootCmd
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}
