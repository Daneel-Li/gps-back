# GPS-BACK - 多厂商GPS设备管理平台

[![Go Version](https://img.shields.io/badge/Go-1.24.0-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Build Status](https://img.shields.io/badge/Build-Passing-brightgreen.svg)]()

GPS-BACK 是一个基于 Go 语言开发的企业级 GPS 设备管理和定位服务平台，支持多厂商设备接入，提供设备管理、实时定位、轨迹查询、设备控制等完整功能。

## ✨ 主要特性

- 🔌 **多厂商支持**: 插件化架构，支持 BTT、V53 等多种厂商设备
- 📍 **多种定位方式**: GPS、WiFi、LBS 多重定位技术
- 🔄 **实时通信**: WebSocket 实时数据推送
- 📱 **微信集成**: 微信小程序登录、微信支付（待完善）
- 🔐 **安全认证**: JWT 认证 + API Key 双重保护
- 📊 **数据分析**: 设备轨迹分析、步数统计
- 🚨 **告警系统**: 设备异常告警、电子围栏

## 🏗️ 系统架构

```
┌─────────────────────────────────────────┐
│              HTTP/HTTPS API             │
│            (Gorilla Mux)                │
├─────────────────────────────────────────┤
│              Handlers Layer             │
│         (SimpleHandler + 中间件)         │
├─────────────────────────────────────────┤
│             Services Layer              │
│        (业务逻辑 + 服务容器)              │
├─────────────────────────────────────────┤
│               DAO Layer                 │
│           (数据访问对象)                  │
├─────────────────────────────────────────┤
│             Database Layer              │
│         (MySQL + TaosDB)                │
└─────────────────────────────────────────┘

           ┌─────────────────┐
           │  Vendor Drivers │
           │   (厂商驱动)     │
           ├─────────────────┤
           │  BTT (MQTT)     │
           │  V53 (TCP)      │
           │  SG (HTTP)      │
           └─────────────────┘
```

## 🚀 快速开始

### 环境要求

- Go 1.24.0+
- MySQL 8.0+
- TaosDB 3.0+ (可选)
- Redis (推荐)

### 安装部署

1. **克隆项目**
```bash
git clone https://github.com/Daneel-Li/gps-back.git
cd gps-back
```

2. **安装依赖**
```bash
go mod download
```

3. **配置数据库**
```bash
# 创建数据库
mysql -u root -p < scripts/mysql.sql

# 如果使用 TaosDB (可选)
taos -s "source scripts/tdengine.sql"
```

4. **配置文件**
```bash
cp config.json.example config.json
# 编辑 config.json 配置数据库连接等信息
```

5. **生成证书文件**
```bash
# 生成 JWT 私钥
openssl genpkey -algorithm RSA -out jwt_private.key -pkcs8

# 生成 HTTPS 证书 (开发环境)
openssl req -x509 -newkey rsa:4096 -keyout privkey.pem -out fullchain.pem -days 365 -nodes
```

6. **启动服务**
```bash
go run cmd/server/main.go
```

服务将在 `https://localhost:8443` 启动

## 📖 API 文档

### 认证接口

#### 微信登录
```http
POST /api/v1/login
Content-Type: application/json
X-API-Key: your-api-key

{
  "code": "wx_login_code"
}
```

### 设备管理

#### 获取设备列表
```http
GET /api/v1/devices
Authorization: Bearer <jwt_token>
X-API-Key: your-api-key
```

#### 获取单个设备
```http
GET /api/v1/devices/{device_id}
Authorization: Bearer <jwt_token>
X-API-Key: your-api-key
```

#### 绑定设备
```http
POST /api/v1/assignments
Authorization: Bearer <jwt_token>
X-API-Key: your-api-key
Content-Type: application/json

{
  "origin_sn": "device_serial_number",
  "device_type": "btt",
  "label": "我的设备"
}
```

#### 解绑设备
```http
DELETE /api/v1/assignments/{device_id}
Authorization: Bearer <jwt_token>
X-API-Key: your-api-key
```

### 轨迹查询

#### 获取设备轨迹
```http
GET /api/v1/devices/{device_id}/track?startTime=2024-01-01 00:00:00&endTime=2024-01-01 23:59:59
Authorization: Bearer <jwt_token>
X-API-Key: your-api-key
```

### 设备控制

#### 发送设备指令
```http
PUT /api/v1/devices/{device_id}/command
Authorization: Bearer <jwt_token>
X-API-Key: your-api-key
Content-Type: application/json

{
  "action": "locate",
  "args": {}
}
```

支持的指令：
- `locate`: 立即定位
- `reboot`: 远程重启
- `find`: 寻找设备
- `set_interval`: 设置上报间隔

### WebSocket 连接

```javascript
const ws = new WebSocket('wss://your-domain:8443/ws');
ws.onmessage = function(event) {
  const data = JSON.parse(event.data);
  console.log('实时数据:', data);
};
```

## 🔧 配置说明

### config.json 配置文件

```json
{
  "mysql": {
    "username": "root",
    "password": "your_password",
    "host": "localhost",
    "port": "3306",
    "dbname": "my_db"
  },
  "mqtt": {
    "broker": "tcp://mqtt.example.com:1883",
    "clientid": "my_client",
    "username": "mqtt_user",
    "password": "mqtt_pass"
  },
  "tls": {
    "cert_path": "./yourcert.pem",
    "key_path": "./yourkey.pem"
  },
  "jwt_issuer": "your-domain.com",
  "api_key": "your-api-key",
  "server_port": 8443,
  "log_level": "INFO"
}
```

### 环境变量

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `CONFIG_PATH` | 配置文件路径 | `./config.json` |
| `LOG_LEVEL` | 日志级别 | `INFO` |
| `SERVER_PORT` | 服务端口 | `8443` |

## 🔌 厂商驱动开发

### 实现 VendorDriver 接口

```go
type VendorDriver interface {
    SetMessageHandler(handler MessageHandler)
    Start() error
    Activate(originSN string) error
    Deactivate(originSN string) error
    SetReportInterval(CommandID int64, originSN string, interval int) error
    Locate(CommandID int64, originSN string) error
    Reboot(CommandID int64, originSN string) error
    PowerOff(CommandID int64, originSN string) error
    Find(CommandID int64, originSN string) error
}
```

### 注册驱动

```go
// 在 initVendorDrivers 函数中注册
yourDriver := yourvendor.NewHandler(config)
serviceContainer.RegisterDriver("your_vendor", yourDriver)
```

## 📊 数据库设计

### 主要数据表

- `users` - 用户信息
- `devices` - 设备主表
- `device_his_data_*` - 设备历史数据 (按设备分表)
- `device_his_pos_*` - 位置轨迹数据 (按设备分表)
- `orders` - 订单管理

详细的数据库结构请参考 `scripts/mysql.sql`

## 🧪 测试

### 运行单元测试
```bash
go test ./...
```

### 运行集成测试
```bash
go test -tags=integration ./...
```

### 测试工具

项目提供了多个测试工具：

```bash
# MQTT 连接测试
go run cmd/testMqtt/main.go

# JT808 协议测试
go run cmd/test_jt808/main.go

# TaosDB 连接测试
go run cmd/testTao/main.go
```

## 📈 性能优化

### 数据库优化
- 使用连接池管理数据库连接
- 按设备分表存储历史数据
- 索引优化查询性能

### 缓存策略
- 本地缓存减少重复计算
- WiFi 定位结果缓存
- 地理编码结果缓存

### 并发处理
- 多 goroutine 处理设备消息
- 异步处理非关键业务逻辑

## 🚀 部署指南

### Docker 部署

```dockerfile
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod download
RUN go build -o gps-back cmd/server/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/gps-back .
COPY --from=builder /app/config.json .
CMD ["./gps-back"]
```

### 使用 Docker Compose

```yaml
version: '3.8'
services:
  gps-back:
    build: .
    ports:
      - "8443:8443"
    environment:
      - CONFIG_PATH=/app/config.json
    volumes:
      - ./config.json:/app/config.json
      - ./certs:/app/certs
    depends_on:
      - mysql
      - redis

  mysql:
    image: mysql:8.0
    environment:
      MYSQL_ROOT_PASSWORD: your_password
      MYSQL_DATABASE: mxm
    volumes:
      - mysql_data:/var/lib/mysql
      - ./scripts/mysql.sql:/docker-entrypoint-initdb.d/init.sql

volumes:
  mysql_data:
```

### 生产环境部署

1. **使用反向代理** (Nginx/Caddy)
2. **配置 HTTPS 证书** (Let's Encrypt)
3. **设置监控和日志** (Prometheus + Grafana)
4. **数据库备份策略**
5. **负载均衡** (多实例部署)

## 🤝 贡献指南

我们欢迎所有形式的贡献！

### 开发流程

1. Fork 项目
2. 创建特性分支 (`git checkout -b feature/amazing-feature`)
3. 提交更改 (`git commit -m 'Add amazing feature'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 创建 Pull Request

### 代码规范

- 遵循 Go 官方代码规范
- 使用 `gofmt` 格式化代码
- 添加必要的注释和文档
- 编写单元测试

### 提交信息格式

```
type(scope): description

[optional body]

[optional footer]
```

类型：
- `feat`: 新功能
- `fix`: 修复 bug
- `docs`: 文档更新
- `style`: 代码格式调整
- `refactor`: 代码重构
- `test`: 测试相关
- `chore`: 构建或辅助工具的变动

## 📄 许可证

本项目采用 MIT 许可证 - 详情请参阅 [LICENSE](LICENSE) 文件

## 🙏 致谢

- [Gorilla Mux](https://github.com/gorilla/mux) - HTTP 路由
- [GORM](https://gorm.io/) - Go ORM 库
- [Paho MQTT](https://github.com/eclipse/paho.mqtt.golang) - MQTT 客户端
- [JWT-Go](https://github.com/dgrijalva/jwt-go) - JWT 实现

## 📞 联系我们

- 项目主页: https://github.com/Daneel-Li/gps-back
- 问题反馈: https://github.com/Daneel-Li/gps-back/issues
- 邮箱: shengda.lsd@gmail.com

## 🗺️ 路线图

- [ ] 支持更多厂商设备协议
- [ ] 移动端 SDK 开发
- [ ] 数据可视化大屏
- [ ] AI 轨迹分析
- [ ] 微服务架构重构
- [ ] 国际化支持

---

如果这个项目对你有帮助，请给我们一个 ⭐️ Star！
