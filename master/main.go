package main

import (
	"file-proxy/master/config"
	"file-proxy/master/echo"
	"flag"
	"fmt"
	"github.com/abrander/ginproxy"
	"github.com/gansidui/gotcp"
	"github.com/gin-gonic/gin"
	jsoniter "github.com/json-iterator/go"
	cregex "github.com/mingrammer/commonregex"
	"go.uber.org/zap"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type worker struct {
	TcpAddress  string   `json:"tcpAddress"`
	Host        string   `json:"host"`
	HttpAddress string   `json:"httpAddress"`
	FilePaths   []string `json:"filePaths"`
}

var workers = struct {
	sync.RWMutex
	ws map[string]*worker
}{ws: make(map[string]*worker)}

type proxyMaster struct{}

func (pm *proxyMaster) OnConnect(c *gotcp.Conn) bool {
	zap.L().Debug("Worker 连接成功", zap.String("Worker Address", c.GetRawConn().RemoteAddr().String()))
	return true
}

func (pm *proxyMaster) OnMessage(c *gotcp.Conn, p gotcp.Packet) bool {
	echoProtocol := p.(*echo.EchoPacket)
	worker := &worker{}
	if err := jsoniter.Unmarshal(echoProtocol.GetBody(), worker); err != nil {
		c.AsyncWritePacket(
			echo.NewEchoPacket(
				[]byte(fmt.Sprintf(`{"code":"400","msg":"%s"}`, err.Error())),
				false),
			time.Second)
		zap.L().Error("接收 Worker 数据失败", zap.Any("Error", err.Error()))
		return true
	}
	worker.TcpAddress = c.GetRawConn().RemoteAddr().String()
	worker.HttpAddress = strings.ReplaceAll(worker.HttpAddress, cregex.IPs(worker.HttpAddress)[0], cregex.IPs(worker.TcpAddress)[0])
	workers.Lock()
	workers.ws[worker.Host] = worker
	workers.Unlock()
	c.AsyncWritePacket(echo.NewEchoPacket([]byte(`{"code":"200","msg":"OK"}`), false), time.Second)
	zap.L().Debug("接收 Worker 数据成功", zap.String("Worker Packet", string(echoProtocol.GetBody())))
	return true
}

func (pm *proxyMaster) OnClose(c *gotcp.Conn) {
	defer workers.RUnlock()
	workers.RLock()

	tcpAddress := c.GetRawConn().RemoteAddr().String()
	for _, w := range workers.ws {
		if tcpAddress == w.TcpAddress {
			delete(workers.ws, w.Host)
		}
	}
	zap.L().Debug("Worker 断开连接", zap.String("Worker Address", tcpAddress))
}

var fConfig = flag.String("config", "master.yml", "configuration file to load")

func main() {
	flag.Parse()

	conf, err := config.LoadFile(*fConfig)
	if err != nil {
		panic(err)
	}

	go func() {
		tcpAddr, err := net.ResolveTCPAddr("tcp4", conf.ServerConfig.TcpAddress)
		if err != nil {
			panic(err)
		}
		listener, err := net.ListenTCP("tcp", tcpAddr)
		if err != nil {
			panic(err)
		}

		config := &gotcp.Config{
			PacketSendChanLimit:    20,
			PacketReceiveChanLimit: 20,
		}
		srv := gotcp.NewServer(config, &proxyMaster{}, &echo.EchoProtocol{})

		srv.Start(listener, time.Second)
	}()

	router := gin.Default()

	router.GET("/file/list", func(c *gin.Context) {
		defer workers.RUnlock()
		workers.RLock()

		host := c.Query("host")
		data := []worker{}
		if host != "" {
			for _, w := range workers.ws {
				if host == w.Host {
					data = append(data, *w)
					break
				}
			}
		} else {
			for _, w := range workers.ws {
				data = append(data, *w)
			}
		}
		c.JSON(http.StatusOK, map[string]interface{}{
			"code": http.StatusOK,
			"msg":  http.StatusText(http.StatusOK),
			"data": data,
		})
	})

	router.GET("/file/download", func(c *gin.Context) {
		host := c.Query("host")
		w := workers.ws[host]
		if w == nil {
			c.Status(http.StatusNotFound)
			return
		}
		g, _ := ginproxy.NewGinProxy(w.HttpAddress)
		g.Handler(c)
	})

	if err := router.Run(conf.ServerConfig.HttpAddress); err != nil {
		zap.L().Error("服务启动失败", zap.Any("Error", err.Error()))
		panic(err)
	}
}
