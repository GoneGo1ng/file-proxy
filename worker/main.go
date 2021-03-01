package main

import (
	"github.com/GoneGo1ng/file-proxy/worker/config"
	"github.com/GoneGo1ng/file-proxy/worker/echo"
	"flag"
	"fmt"
	"github.com/gin-gonic/gin"
	jsoniter "github.com/json-iterator/go"
	"github.com/srfrog/slices"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type worker struct {
	Host        string   `json:"host"`
	HttpAddress string   `json:"httpAddress"`
	FilePaths   []string `json:"filePaths"`
}

var conn *net.TCPConn

var dc chan int

var connected bool

var fConfig = flag.String("config", "worker.yml", "configuration file to load")

func main() {
	dc = make(chan int, 1)

	conf, err := config.LoadFile(*fConfig)
	if err != nil {
		panic(err)
	}

	worker := &worker{
		Host:        conf.ServerConfig.Hostname,
		HttpAddress: "http://" + conf.ServerConfig.HttpAddress + "/",
		FilePaths:   conf.FilePaths,
	}

	go keepDial(conf.ServerConfig.MasterTcpAddress)

	go func() {
		for {
			select {
			case <-dc:
				keepDial(conf.ServerConfig.MasterTcpAddress)
			}
		}
	}()

	go func() {
		if conf.ReloadConfig.Enabled {
			inputConfig := &config.InputConfig{}
			echoMergedFilePaths(conf.ReloadConfig.Path, inputConfig, conf.FilePaths, worker)
			for range time.Tick(conf.ReloadConfig.Period) {
				echoMergedFilePaths(conf.ReloadConfig.Path, inputConfig, conf.FilePaths, worker)
			}
		} else {
			for range time.Tick(10 * time.Second) {
				echoFilePaths(worker)
			}
		}
	}()

	router := gin.Default()
	router.GET("/file/download", func(c *gin.Context) {
		filePath := c.Query("filePath")
		fileInfo, err := pathExists(filePath)
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		filename := fileInfo.Name()
		c.Writer.Header().Add("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
		c.Writer.Header().Add("Content-Type", "application/octet-stream")
		c.File(filePath)
	})

	if err := router.Run(conf.ServerConfig.HttpAddress); err != nil {
		zap.L().Error("服务启动失败", zap.Any("Error", err.Error()))
		panic(err)
	}
}

func pathExists(path string) (os.FileInfo, error) {
	f, err := os.Stat(path)
	if err == nil {
		return f, nil
	}
	if os.IsNotExist(err) {
		return f, nil
	}
	return f, err
}

func keepDial(address string) {
	if err := dial(address); err == nil {
		connected = true
		return
	}
	for range time.Tick(10 * time.Second) {
		if err := dial(address); err == nil {
			connected = true
			break
		}
	}
}

func dial(address string) error {
	tcpAddr, err := net.ResolveTCPAddr("tcp4", address)
	if err != nil {
		zap.L().Error("master tcp 连接失败", zap.Any("Error", err.Error()))
		return err
	}
	conn, err = net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		zap.L().Error("master tcp 连接失败", zap.Any("Error", err.Error()))
		return err
	}
	zap.L().Error("master tcp 连接成功", zap.String("RemoteAddr", conn.RemoteAddr().String()))
	return nil
}

func echoFilePaths(worker *worker) {
	if conn == nil || !connected {
		return
	}
	buff, _ := jsoniter.Marshal(worker)
	echoProtocol := &echo.EchoProtocol{}
	if _, err := conn.Write(echo.NewEchoPacket(buff, false).Serialize()); err != nil {
		connected = false
		dc <- 1
		zap.L().Error("发送 Worker 消息失败", zap.Any("Error", err.Error()))
		return
	}
	p, err := echoProtocol.ReadPacket(conn)
	if err != nil {
		connected = false
		dc <- 1
		zap.L().Error("接收 Master 消息失败", zap.Any("Error", err.Error()))
		return
	}
	ep := p.(*echo.EchoPacket)
	result := map[string]interface{}{}
	if err := jsoniter.Unmarshal(ep.GetBody(), &result); err != nil {
		zap.L().Error("解析 Master 消息失败", zap.Any("Error", err.Error()))
		return
	}
	if result["code"] == nil || result["code"].(string) != "200" {
		zap.L().Error("Master 返回错误消息", zap.Any("Error", result["msg"]))
		return
	}
	zap.L().Debug("发送 Worker 消息成功", zap.Any("Master Echo", result))
}

func echoMergedFilePaths(path string, inputConfig *config.InputConfig, filePaths []string, worker *worker) {
	buff, err := ioutil.ReadFile(path)
	if err != nil {
		zap.L().Error("获取配置文件失败", zap.String("Path", path), zap.Any("Error", err.Error()))
		return
	}
	err = yaml.UnmarshalStrict(buff, inputConfig)
	if err != nil {
		zap.L().Error("解析配置文件失败", zap.String("Path", path), zap.Any("Error", err.Error()))
		return
	}
	var files []string
	for _, pat := range inputConfig.FilePaths {
		fs, err := filepath.Glob(pat)
		if err != nil {
			zap.L().Error("获取文件路径失败", zap.String("Path", pat), zap.Any("Error", err.Error()))
			continue
		}
		files = append(files, fs...)
	}
	worker.FilePaths = slices.Unique(slices.Merge(files, filePaths))
	zap.L().Debug("获取文件路径成功", zap.String("FilePaths", strings.Join(worker.FilePaths, " | ")))
	echoFilePaths(worker)
}
