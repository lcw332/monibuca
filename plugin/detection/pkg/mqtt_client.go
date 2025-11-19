package detection

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// MQTTConfig MQTT配置
type MQTTConfig struct {
	Enable    bool     `json:"enable" default:"false" desc:"是否启用MQTT"`
	ClientID  string   `json:"clientId" desc:"客户端ID"`
	Pub       []string `json:"pub" default:"[]" desc:"发布主题列表"`
	Qos       []int    `json:"qos" default:"[]" desc:"QoS等级列表"`
	Endpoint  string   `json:"endpoint" desc:"MQTT服务器地址"`
	Format    string   `json:"format" default:"json" desc:"消息格式(json/pb)"`
	Username  string   `json:"username" desc:"用户名"`
	Password  string   `json:"password" desc:"密码"`
	KeepAlive int      `json:"keepAlive" default:"30" desc:"心跳间隔(秒)"`
}

// MQTTClient MQTT客户端封装
type MQTTClient struct {
	config    *MQTTConfig
	client    mqtt.Client
	logger    *slog.Logger
	mutex     sync.RWMutex
	connected bool
}

// NewMQTTClient 创建MQTT客户端
func NewMQTTClient(config *MQTTConfig, logger *slog.Logger) *MQTTClient {
	if config == nil || !config.Enable {
		return nil
	}

	client := &MQTTClient{
		config: config,
		logger: logger,
	}

	client.connect()
	return client
}

// connect 建立MQTT连接
func (m *MQTTClient) connect() {
	if m.config.Endpoint == "" {
		m.logger.Error("MQTT endpoint is empty")
		return
	}

	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s", m.config.Endpoint))

	// 设置客户端ID，如果为空则生成一个
	clientID := m.config.ClientID
	if clientID == "" {
		clientID = fmt.Sprintf("detection_client_%d", time.Now().Unix())
	}
	opts.SetClientID(clientID)

	if m.config.Username != "" {
		opts.SetUsername(m.config.Username)
	}
	if m.config.Password != "" {
		opts.SetPassword(m.config.Password)
	}

	keepAlive := 30
	if m.config.KeepAlive > 0 {
		keepAlive = m.config.KeepAlive
	}
	opts.SetCleanSession(true)
	opts.SetKeepAlive(time.Duration(keepAlive) * time.Second)
	opts.SetDefaultPublishHandler(m.onMessage)
	// retry
	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(5 * time.Second)
	opts.SetWriteTimeout(1 * time.Second)
	opts.SetPingTimeout(10 * time.Second)

	opts.OnConnect = m.onConnect
	opts.OnConnectionLost = m.onConnectionLost

	m.client = mqtt.NewClient(opts)

	if token := m.client.Connect(); token.Wait() && token.Error() != nil {
		m.logger.Error("MQTT connect failed", "error", token.Error())
		return
	}

	m.logger.Info("MQTT client connected", "endpoint", m.config.Endpoint, "clientID", clientID)
}

// onConnect 连接回调
func (m *MQTTClient) onConnect(client mqtt.Client) {
	m.mutex.Lock()
	m.connected = true
	m.mutex.Unlock()
	m.logger.Info("MQTT connected")
}

// onConnectionLost 连接丢失回调
func (m *MQTTClient) onConnectionLost(client mqtt.Client, err error) {
	m.mutex.Lock()
	m.connected = false
	m.mutex.Unlock()
	m.logger.Error("MQTT connection lost", "error", err)
}

// onMessage 消息接收回调
func (m *MQTTClient) onMessage(client mqtt.Client, msg mqtt.Message) {
	m.logger.Debug("MQTT message received", "topic", msg.Topic(), "payload", string(msg.Payload()))
}

// IsConnected 检查是否连接
func (m *MQTTClient) IsConnected() bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.connected && m.client != nil && m.client.IsConnected()
}

// Publish 发布消息，支持指定QoS等级
func (m *MQTTClient) Publish(topic string, qos int, payload interface{}) error {
	if !m.IsConnected() {
		return fmt.Errorf("MQTT not connected")
	}

	// 根据配置格式化消息
	var data []byte
	var err error

	switch m.config.Format {
	case "json":
		data, err = json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("json marshal error: %w", err)
		}
	case "pb":
		// 如果需要支持protobuf，可以在这里添加
		data, err = json.Marshal(payload) // 暂时还是使用JSON
		if err != nil {
			return fmt.Errorf("marshal error: %w", err)
		}
	default:
		// 默认使用JSON格式
		// 确保 payload 是一个可以被序列化的类型
		switch p := payload.(type) {
		case string:
			// 如果 payload 是字符串，直接使用
			data = []byte(p)
		case []byte:
			// 如果 payload 是字节切片，直接使用
			data = p
		case nil:
			// 处理空载荷的情况
			data = []byte{}
		default:
			// 其他类型尝试 JSON 序列化
			data, err = json.Marshal(payload)
			if err != nil {
				m.logger.Warn("Failed to marshal payload, converting to string",
					"type", fmt.Sprintf("%T", payload))
				// 尝试转换为字符串表示
				data = []byte(fmt.Sprintf("%+v", payload))
			}
		}
	}

	// 确保QoS值在有效范围内(0-2)
	if qos < 0 || qos > 2 {
		qos = 0 // 默认使用QoS 0
	}

	m.logger.Debug("Publishing MQTT message", "topic", topic, "qos", qos)

	token := m.client.Publish(topic, byte(qos), false, data)
	token.Wait()

	if token.Error() != nil {
		m.logger.Error("MQTT publish failed", "error", token.Error())
		return token.Error()
	}

	return nil
}

// PublishWithIndex 发布消息，支持索引替换
func (m *MQTTClient) PublishWithIndex(topicTemplate string, index int, algId uint8, streamPath string, payload interface{}) error {
	if !m.IsConnected() {
		return fmt.Errorf("MQTT not connected")
	}

	// 替换主题模板中的变量
	topic := topicTemplate
	topic = strings.ReplaceAll(topic, "${streamPath}", strings.ReplaceAll(streamPath, "/", "_"))
	topic = strings.ReplaceAll(topic, "+", fmt.Sprintf("%d", algId))

	return m.Publish(topic, m.config.Qos[index], payload)
}

// GetClient 获取原生MQTT客户端
func (m *MQTTClient) GetClient() mqtt.Client {
	return m.client
}

// Close 关闭MQTT连接
func (m *MQTTClient) Close() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.client != nil && m.client.IsConnected() {
		m.client.Disconnect(250)
		m.connected = false
		m.logger.Info("MQTT client disconnected")
	}
}
