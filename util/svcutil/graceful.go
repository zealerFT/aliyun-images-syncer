package svcutil

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path"
	"runtime"
	"sync"
	"syscall"
	"time"

	"aliyun-images-syncer/util/healthcheck"
	"aliyun-images-syncer/util/metrics"
	"aliyun-images-syncer/util/waitutil"

	"github.com/sirupsen/logrus"
)

func init() {
	// Log as JSON instead of the default ASCII formatter.
	logrus.SetFormatter(&logrus.JSONFormatter{
		CallerPrettyfier: func(f *runtime.Frame) (string, string) {
			filename := path.Base(f.File)
			return fmt.Sprintf("%s()", f.Function), fmt.Sprintf("%s:%d", filename, f.Line)
		},
	})

	// Output to stdout instead of the default stderr
	// Can be any io.Writer, see below for File example
	logrus.SetOutput(os.Stdout)

	// Only log the warning severity or above.
	logrus.SetLevel(logrus.DebugLevel)
}

// StandBy graceful doUtilStop func and with HTTP health check at addr
// it will block and stop when function is finished
func StandBy(addr string, f func()) {
	stop := WaitSignals()

	grace := &GracefulDo{}
	done := grace.Do(addr, f)

	for {
		select {
		// 函数正常执行结束后，chan将返回空，这里是正常结束
		case <-done:
			return
		// 当遇到指定single信号，直接结束
		case <-stop:
			return
		}
	}
}

// NeverStop graceful doUtilStop func and with HTTP health check at addr
// it will block and never stop
func NeverStop(addr string, f func()) {
	// wait for signal
	stop := WaitSignals()
	grace := &GracefulDo{}
	grace.DoUtilStop(addr, stop, f)
}

// NeverStopByTicker 根据定时器入参，轮询主函数，在遇到错误信号的时候，提前结束服务
func NeverStopByTicker(addr string, ticker *time.Ticker, f func()) {
	// wait for signal
	stop := WaitSignals()
	grace := &GracefulDo{}
	grace.DoPolling(addr, stop, ticker, f)
}

type GracefulDo struct {
	once        sync.Once
	onceMetrics sync.Once
}

func (g *GracefulDo) Do(addr string, f func()) <-chan struct{} {
	stop := make(chan struct{})

	// start health at once，为pod提供的监控检查livenessProbe and readinessProbe
	go g.withHealthCheck(addr, stop)

	go func() {
		// 这里的defer是为了结束stop chan，这样外层的select将捕获到f()执行结束
		defer close(stop)

		func() {
			defer waitutil.HandleCrash()
			f()

		}()
	}()

	return stop
}

func (g *GracefulDo) DoUtilStop(addr string, stop <-chan struct{}, f func()) {
	// start health at once
	go g.withHealthCheck(addr, stop)

	// 只在指定single信号接收时，终结程序
	select {
	case <-stop:
		return
	default:
	}

	func() {
		// f()执行方法抛错时，记录错误
		defer waitutil.HandleCrash()
		f()
	}()

	// NOTE: b/c there is no priority selection in golang
	// it is possible for this to race, meaning we could
	// trigger t.C and stopCh, and t.C select falls through.
	// In order to mitigate we re-check stopCh at the beginning
	// of every loop to prevent extra executions of f().
	// 一直阻塞方法，只有在接收信号时结束
	<-stop
}

// DoPolling 轮询
func (g *GracefulDo) DoPolling(addr string, stop <-chan struct{}, ticker *time.Ticker, f func()) {
	defer waitutil.HandleCrash()

	// start health at once
	go g.withHealthCheck(addr, stop)

	// start prometheus metris
	go g.withPrometheusMetrics(":23333", stop)

	for {
		select {
		case <-stop:
			return
		// ticker.C:
		// 该 Ticker 包含一个通道字段，并会每隔时间段 d 就向该通道发送当时的时间。向其自身的 C 字段发送当时的时间。
		// 它会调整时间间隔或者丢弃 tick 信息以适应反应慢的接收者。所以不用担心f()执行时间过长会开始下一次轮询的情况
		case t := <-ticker.C:
			fmt.Println("本次轮询时间为:", t)
			f()
		}
	}
}

func (g *GracefulDo) withHealthCheck(addr string, stop <-chan struct{}) {
	g.once.Do(func() {
		HTTPHealthCheck(addr, stop)
	})
}

func (g *GracefulDo) withPrometheusMetrics(addr string, stop <-chan struct{}) {
	g.onceMetrics.Do(func() {
		metrics.HttpMetricsCheck(addr, stop)
	})
}

// WaitFor 简单版本(并不优雅) 处理退出, 需要相关处理函数 f 能够阻塞执行
func WaitFor(addr string, f func(stop <-chan struct{}) error) {
	stop := WaitSignals()
	quit := make(chan struct{})
	go func() {
		HTTPHealthCheck(addr, quit)
	}()

	if err := f(stop); err != nil {
		logrus.Errorf("%+v", err)
	}
	quit <- struct{}{}
}

// HTTPHealthCheck HTTP 模式健康检查，会阻塞执行
func HTTPHealthCheck(addr string, stop <-chan struct{}) {
	server := &http.Server{Addr: addr, Handler: healthcheck.NewHandler()}
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logrus.Errorf("[BgHealthCheck] health server close with err: %+v", err)
		}
	}()
	<-stop
	server.SetKeepAlivesEnabled(false)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logrus.Errorf("[BgHealthCheck] stop server graceful stop with err: %+v", err)
	}
}

// WaitSignals 监听退出信号
func WaitSignals() chan struct{} {
	stop := make(chan struct{})

	quit := make(chan os.Signal)
	signal.Notify(quit, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGUSR1, syscall.SIGUSR2)
	signal.Notify(quit, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGUSR1, syscall.SIGUSR2)

	go func() {
		<-quit
		close(stop)
	}()

	return stop
}
