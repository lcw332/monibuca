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

// MQTTClient Interface
type MQTTClient interface {
	IsConnected() bool
	Publish(topic string, qos int, payload interface{}) error
	PublishWithIndex(topicTemplate string, index int, algId uint8, streamPath string, payload interface{}) error
	Close()
	GetClient() mqtt.Client
}

// mqttClientImpl MQTT客户端封装
type mqttClientImpl struct {
	config    *MQTTConfig
	client    mqtt.Client
	logger    *slog.Logger
	mutex     sync.RWMutex
	connected bool
}

var (
	// 存储每个配置对应的MQTT客户端实例
	clientInstances = sync.Map{}
)

// NewMQTTClient 创建MQTT客户端，一个配置仅允许存在一个 client
func NewMQTTClient(config *MQTTConfig, logger *slog.Logger) (MQTTClient, error) {
	if config == nil || !config.Enable {
		return nil, fmt.Errorf("MQTT not enabled")
	}

	// 使用配置的Endpoint作为唯一标识符来确保一个配置只有一个客户端实例
	configKey := fmt.Sprintf("%s-%s", config.Broker, config.ClientID)

	// 尝试从缓存中获取现有客户端
	if instance, ok := clientInstances.Load(configKey); ok {
		return instance.(MQTTClient), nil
	}

	client := &mqttClientImpl{
		config: config,
		logger: logger,
	}

	client.connect()

	// 将新创建的客户端存储到缓存中
	clientInstances.Store(configKey, client)

	return client, nil
}

// connect 建立MQTT连接
func (m *mqttClientImpl) connect() {
	if m.config.Broker == "" {
		m.logger.Error("MQTT Broker is empty")
		return
	}

	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s", m.config.Broker))

	// 设置客户端ID，如果为空则生成一个
	clientID := m.config.ClientID
	if clientID == "" {
		clientID = fmt.Sprintf("monibuca_detection_client_%d", time.Now().Unix())
	}
	opts.SetClientID(clientID)

	if m.config.Username != "" {
		opts.SetUsername(m.config.Username)
	}
	if m.config.Password != "" {
		opts.SetPassword(m.config.Password)
	}

	keepAlive := 60
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
	opts.SetWriteTimeout(5 * time.Second)
	opts.SetPingTimeout(10 * time.Second)

	opts.OnConnect = m.onConnect
	opts.OnConnectionLost = m.onConnectionLost

	m.client = mqtt.NewClient(opts)

	if token := m.client.Connect(); token.WaitTimeout(30*time.Second) && token.Error() != nil {
		m.logger.Error("MQTT connect failed", "error", token.Error())
		return
	}

	m.logger.Info("MQTT client connected", "endpoint", m.config.Broker, "clientID", clientID)
}

// onConnect 连接回调
func (m *mqttClientImpl) onConnect(client mqtt.Client) {
	m.mutex.Lock()
	m.connected = true
	m.mutex.Unlock()
	m.logger.Info("MQTT connected")
}

// onConnectionLost 连接丢失回调
func (m *mqttClientImpl) onConnectionLost(client mqtt.Client, err error) {
	m.mutex.Lock()
	m.connected = false
	m.mutex.Unlock()
	m.logger.Error("MQTT connection lost", "error", err)
}

// onMessage 消息接收回调
func (m *mqttClientImpl) onMessage(client mqtt.Client, msg mqtt.Message) {
	m.logger.Debug("MQTT message received", "topic", msg.Topic(), "payload", string(msg.Payload()))
}

// IsConnected 检查是否连接
func (m *mqttClientImpl) IsConnected() bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.connected && m.client != nil && m.client.IsConnected()
}

// Publish 发布消息，支持指定QoS等级
func (m *mqttClientImpl) Publish(topic string, qos int, payload interface{}) error {
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
func (m *mqttClientImpl) PublishWithIndex(topicTemplate string, index int, algId uint8, streamPath string, payload interface{}) error {
	if !m.IsConnected() {
		return fmt.Errorf("MQTT not connected")
	}

	// 替换主题模板中的变量
	topic := topicTemplate
	topic = strings.ReplaceAll(topic, "${streamPath}", strings.ReplaceAll(streamPath, "/", "_"))

	// 只有当主题中包含 "+" 占位符时才替换算法ID
	if strings.Contains(topic, "+") {
		topic = strings.ReplaceAll(topic, "+", fmt.Sprintf("%d", algId))
	}
	//else {
	//	// 如果没有占位符，为了区分不同算法的结果，可以在主题后添加算法ID
	//	topic = fmt.Sprintf("%s/%d", topic, algId)
	//}

	return m.Publish(topic, m.config.Qos[index], payload)
}

// GetClient 获取原生MQTT客户端
func (m *mqttClientImpl) GetClient() mqtt.Client {
	return m.client
}

// Close 关闭MQTT连接
func (m *mqttClientImpl) Close() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.client != nil && m.client.IsConnected() {
		m.client.Disconnect(250)
		m.connected = false
		m.logger.Info("MQTT client disconnected")
	}
}
