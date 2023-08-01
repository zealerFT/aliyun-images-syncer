package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"aliyun-images-syncer/pkg/client"
	"aliyun-images-syncer/util/svcutil"

	client2 "aliyun-images-syncer/pkg/client"

	"github.com/gin-gonic/gin"
	"github.com/golang/glog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

const KeyDep = "key:dep"

var (
	token, logPath, repoNamespaceName, instanceIdMaster, instanceIdSlave, accountMaster, passwordMaster, accountSlave, passwordSlave, accessKeyIdMaster, accessKeySecretMaster, endpointMaster, accessKeyIdSlave, accessKeySecretSlave, endpointSlave string
	publicNetworkMaster, publicNetworkSlave                                                                                                                                                                                                           string
	procNum, retries, polling                                                                                                                                                                                                                         int
	mailHost, mailUserName, mailAuthCode, mailTo                                                                                                                                                                                                      string
	repoNamespaceNames                                                                                                                                                                                                                                []string
)

// RootCmd describes "image-syncer" command
var RootCmd = &cobra.Command{
	Use:     "images-sync",
	Aliases: []string{"images-sync"},
	Short:   "A docker registry image real time synchronization tool！by fermi",
	Long:    `A Fast and Flexible docker registry image real time synchronization tool implement by Go.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// dig
		dep := client2.DIDependency()
		// work starts here
		_client, err := client.CreateClient(
			&accessKeyIdMaster, &accessKeySecretMaster, &endpointMaster, &accountMaster, &passwordMaster,
			&accessKeyIdSlave, &accessKeySecretSlave, &endpointSlave, &accountSlave, &passwordSlave,
			&repoNamespaceName, &instanceIdMaster, &instanceIdSlave,
			&publicNetworkMaster, &publicNetworkSlave,
			mailHost, mailUserName, mailAuthCode, mailTo,
			logPath, repoNamespaceNames, dep,
		)
		if err != nil {
			return fmt.Errorf("init sync client error: %v", err)
		}

		pollingTime := time.Duration(polling) * time.Second
		log.Debug().Msgf("轮询间隔pollingTime: %v", pollingTime)

		go Http(_client, token)

		//  优雅轮询并且启动健康检查，并且在接收到失败信号好，结束程序
		svcutil.NeverStopByTicker(":8000", time.NewTicker(pollingTime), func() {
			log.Info().Msg("Normal operation, bro～")
			_client.Run()
		})

		log.Log().Msg("images-sync NeverStopByTicker shutting down :)")

		return nil

	},
}

func Http(client *client2.Client, token string) {
	r := gin.Default()
	r.Use(
		func(c *gin.Context) {
			c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
			c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")
			if c.Request.Method == "OPTIONS" {
				c.AbortWithStatus(204)
			}
		},
		func(c *gin.Context) {
			c.Set(KeyDep, client)
			c.Next()
		},
	)

	group := r.Group("/api")
	route := group.Use(Auth(token))
	route.GET("/sync", Sync)
	server := &http.Server{Addr: ":8001", Handler: r}
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			glog.Fatalf("server failure: %v", err)
		}
	}()

	log.Log().Msg("images-sync http is begining :)")

	termination := make(chan os.Signal)
	signal.Notify(termination, syscall.SIGINT, syscall.SIGTERM)
	<-termination

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		glog.Fatalf("Failed to shut down: %v", err)
	}

	log.Log().Msg("images-sync http shutting down :)")
}

func Sync(c *gin.Context) {
	client_ := c.MustGet(KeyDep).(*client2.Client)
	res := client_.Run()
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": res})
}

func Auth(token string) gin.HandlerFunc {
	return func(c *gin.Context) {
		t := Token(c)
		if t != token {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "Unauthorized"})
			return
		}
		c.Next()
	}
}

func Token(c *gin.Context) string {
	value := c.GetHeader("Authorization")
	if value != "" && strings.HasPrefix(value, "Bearer ") {
		return strings.TrimPrefix(value, "Bearer ")
	}
	return ""
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

	// http auth token
	RootCmd.PersistentFlags().StringVar(&token, "token", "feiteng", "http接口鉴权token")
}

// Execute executes the RootCmd
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}
