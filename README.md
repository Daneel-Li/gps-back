# GPS-BACK - Multi-Vendor GPS Device Management Platform

[![English](https://img.shields.io/badge/English-blue.svg)](README.md)
[![中文](https://img.shields.io/badge/中文-red.svg)](README_ZN.md)

[![Go Version](https://img.shields.io/badge/Go-1.24.0-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Build Status](https://img.shields.io/badge/Build-Passing-brightgreen.svg)]()

GPS-BACK is an enterprise-grade GPS device management and positioning service platform developed with Go, supporting multi-vendor device integration and providing complete functionality including device management, real-time positioning, track querying, and device control.

## ✨ Key Features

- 🔌 **Multi-Vendor Support**: Plugin architecture supporting BTT, V53 and other vendor devices
- 📍 **Multiple Positioning Methods**: GPS, WiFi, LBS multi-level positioning technology
- 🔄 **Real-time Communication**: WebSocket real-time data push
- 📱 **WeChat Integration**: WeChat Mini Program login, WeChat Pay (to be improved)
- 🔐 **Security Authentication**: JWT authentication + API Key dual protection
- 📊 **Data Analysis**: Device track analysis, step counting
- 🚨 **Alert System**: Device anomaly alerts, electronic fences

## 🏗️ System Architecture

```
┌─────────────────────────────────────────┐
│              HTTP/HTTPS API             │
│            (Gorilla Mux)                │
├─────────────────────────────────────────┤
│              Handlers Layer             │
│         (SimpleHandler + Middleware)    │
├─────────────────────────────────────────┤
│             Services Layer              │
│        (Business Logic + Service Container) │
├─────────────────────────────────────────┤
│               DAO Layer                 │
│           (Data Access Objects)         │
├─────────────────────────────────────────┤
│             Database Layer              │
│         (MySQL + TaosDB)                │
└─────────────────────────────────────────┘

           ┌─────────────────┐
           │  Vendor Drivers │
           │   (Vendor Drivers) │
           ├─────────────────┤
           │  BTT (MQTT)     │
           │  V53 (TCP)      │
           │  SG (HTTP)      │
           └─────────────────┘
```

## 🚀 Quick Start

### Requirements

- Go 1.24.0+
- MySQL 8.0+
- TaosDB 3.0+ (optional)
- Redis (recommended)

### Installation

1. **Clone the project**
```bash
git clone https://github.com/Daneel-Li/gps-back.git
cd gps-back
```

2. **Install dependencies**
```bash
go mod download
```

3. **Configure database**
```bash
# Create database
mysql -u root -p < scripts/mysql.sql

# If using TaosDB (optional)
taos -s "source scripts/tdengine.sql"
```

4. **Configuration file**
```bash
cp config.json.example config.json
# Edit config.json to configure database connection and other information
```

5. **Generate certificate files**
```bash
# Generate JWT private key
openssl genpkey -algorithm RSA -out jwt_private.key -pkcs8

# Generate HTTPS certificate (development environment)
openssl req -x509 -newkey rsa:4096 -keyout privkey.pem -out fullchain.pem -days 365 -nodes
```

6. **Start service**
```bash
go run cmd/server/main.go
```

Service will start at `https://localhost:8443`

## 📖 API Documentation

### Authentication APIs

#### WeChat Login
```http
POST /api/v1/login
Content-Type: application/json
X-API-Key: your-api-key

{
  "code": "wx_login_code"
}
```

### Device Management

#### Get Device List
```http
GET /api/v1/devices
Authorization: Bearer <jwt_token>
X-API-Key: your-api-key
```

#### Get Single Device
```http
GET /api/v1/devices/{device_id}
Authorization: Bearer <jwt_token>
X-API-Key: your-api-key
```

#### Bind Device
```http
POST /api/v1/assignments
Authorization: Bearer <jwt_token>
X-API-Key: your-api-key
Content-Type: application/json

{
  "origin_sn": "device_serial_number",
  "device_type": "btt",
  "label": "My Device"
}
```

#### Unbind Device
```http
DELETE /api/v1/assignments/{device_id}
Authorization: Bearer <jwt_token>
X-API-Key: your-api-key
```

### Track Query

#### Get Device Track
```http
GET /api/v1/devices/{device_id}/track?startTime=2024-01-01 00:00:00&endTime=2024-01-01 23:59:59
Authorization: Bearer <jwt_token>
X-API-Key: your-api-key
```

### Device Control

#### Send Device Command
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

Supported commands:
- `locate`: Immediate positioning
- `reboot`: Remote restart
- `find`: Find device
- `set_interval`: Set reporting interval

### WebSocket Connection

```javascript
const ws = new WebSocket('wss://your-domain:8443/ws');
ws.onmessage = function(event) {
  const data = JSON.parse(event.data);
  console.log('Real-time data:', data);
};
```

## 🔧 Configuration

### config.json Configuration File

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

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `CONFIG_PATH` | Configuration file path | `./config.json` |
| `LOG_LEVEL` | Log level | `INFO` |
| `SERVER_PORT` | Service port | `8443` |

## 🔌 Vendor Driver Development

### Implement VendorDriver Interface

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

### Register Driver

```go
// Register in initVendorDrivers function
yourDriver := yourvendor.NewHandler(config)
serviceContainer.RegisterDriver("your_vendor", yourDriver)
```

## 📊 Database Design

### Main Data Tables

- `users` - User information
- `devices` - Device main table
- `device_his_data_*` - Device historical data (table per device)
- `device_his_pos_*` - Position track data (table per device)
- `orders` - Order management

Detailed database structure please refer to `scripts/mysql.sql`

## 🧪 Testing

### Run Unit Tests
```bash
go test ./...
```

### Run Integration Tests
```bash
go test -tags=integration ./...
```

### Test Tools

The project provides multiple test tools:

```bash
# MQTT connection test
go run cmd/testMqtt/main.go

# JT808 protocol test
go run cmd/test_jt808/main.go

# TaosDB connection test
go run cmd/testTao/main.go
```

## 📈 Performance Optimization

### Database Optimization
- Use connection pool to manage database connections
- Store historical data by device in separate tables
- Index optimization for query performance

### Cache Strategy
- Local cache to reduce duplicate calculations
- WiFi positioning result cache
- Geocoding result cache

### Concurrent Processing
- Multiple goroutines to handle device messages
- Asynchronous processing of non-critical business logic

## 🚀 Deployment Guide

### Docker Deployment

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

### Using Docker Compose

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

### Production Deployment

1. **Use reverse proxy** (Nginx/Caddy)
2. **Configure HTTPS certificates** (Let's Encrypt)
3. **Set up monitoring and logging** (Prometheus + Grafana)
4. **Database backup strategy**
5. **Load balancing** (multi-instance deployment)

## 🤝 Contributing

We welcome all forms of contributions!

### Development Workflow

1. Fork the project
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Create a Pull Request

### Code Standards

- Follow Go official code standards
- Use `gofmt` to format code
- Add necessary comments and documentation
- Write unit tests

### Commit Message Format

```
type(scope): description

[optional body]

[optional footer]
```

Types:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation update
- `style`: Code formatting adjustment
- `refactor`: Code refactoring
- `test`: Test related
- `chore`: Build or auxiliary tool changes

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details

## 🙏 Acknowledgments

- [Gorilla Mux](https://github.com/gorilla/mux) - HTTP router
- [GORM](https://gorm.io/) - Go ORM library
- [Paho MQTT](https://github.com/eclipse/paho.mqtt.golang) - MQTT client
- [JWT-Go](https://github.com/dgrijalva/jwt-go) - JWT implementation

## 📞 Contact Us

- Project homepage: https://github.com/Daneel-Li/gps-back
- Issue feedback: https://github.com/Daneel-Li/gps-back/issues
- Email: shengda.lsd@gmail.com

## 🗺️ Roadmap

- [ ] Support more vendor device protocols
- [ ] Mobile SDK development
- [ ] Data visualization dashboard
- [ ] AI track analysis
- [ ] Microservice architecture refactoring
- [ ] Internationalization support

---

If this project helps you, please give us a ⭐️ Star!
