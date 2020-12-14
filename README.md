# File Proxy

File Proxy 是一个简单的文件代理服务。其作用在于解决存在网络隔离的服务器之间的文件传输问题。

File Proxy 分为 Master 和 Worker 两个部分。

- Master: 部署在跳板机上，与 Worker 建立 TCP 连接，接收 Worker 发送可提供下载的文件路径。并提供查询下载文件列表的接口，以及对应的文件下载接口。
- Worker: 部署在需要下载文件的服务器上，与 Master 建立 TCP 连接，向 Master 发送可提供下载的文件路径。并提供对应的文件下载接口。

# Architecture overview

```
┌───────────┐       ┌───────────┐
│           │       │           │
│   Files   │──────>│  Worker1  │───┐
│           │       │           │   │
└───────────┘       └───────────┘   │
                                    │
┌───────────┐       ┌───────────┐   │    ┌───────────┐       ┌────────────────┐
│           │       │           │   │    │           │       │                │
│   Files   │──────>│  Worker2  │───┼───>│   Master  │──────>│ Download Files │
│           │       │           │   │    │           │       │                │
└───────────┘       └───────────┘   │    └───────────┘       └────────────────┘
                                    │
┌───────────┐       ┌───────────┐   │
│           │       │           │   │
│   Files   │──────>│  Worker3  │───┘
│           │       │           │
└───────────┘       └───────────┘
```

# Usage

## Master

master --config master.yml

## Worker

worker --config worker.yml

# Master API

## 查询下载文件列表

### 简要描述

- 查询下载文件列表

### 请求URL

- ` http://x.x.x.x:9631/file/list?host=shenqi-PC `

### 请求方式

- GET

### 请求参数说明

|参数名 |必选 |类型 |说明 |
|:--- |:--- |:--- |--- |
|host |否 |string |Worker 主机名称 |

### 返回示例

```json
{
 	"code": 200,
 	"data": [{
 		"tcpAddress": "127.0.0.1:58363",
 		"host": "shenqi-PC",
 		"httpAddress": "http://127.0.0.1:9641/",
 		"filePaths": ["E:\\test\\foo.txt"]
 	}],
 	"msg": "OK"
 }
```

### 返回参数说明

|参数名 |类型 |说明 |
|:--- |:--- |--- |
|tcpAddress |string |Worker TCP 地址 |
|host |string |Worker 主机名称 |
|httpAddress |string |Worker HTTP 地址 |
|filePaths |string |Worker 可提供下载的文件路径 |

## 下载文件

### 简要描述

- 下载文件

### 请求URL

- ` http://x.x.x.x:9631/file/download?host=shenqi-PC&filePath=E:\\test\\foo.txt `

### 请求方式

- GET

### 请求参数说明

|参数名 |必选 |类型 |说明 |
|:--- |:--- |:--- |--- |
|host |是 |string |Worker 主机名称 |
|filePath |是 |string |Worker 需要下载的文件路径 |

### 返回示例

```
Content-Disposition: attachment; filename=filename
Content-Type: application/octet-stream
```
