package mns

import (
	"github.com/aliyun/aliyun-mns-go-sdk"
	"github.com/ebar-go/ego/library"
	"github.com/ebar-go/ego/log"
	"encoding/json"
	"encoding/base64"
	"os"
	"github.com/ebar-go/ego/http/constant"
)

// Conf 阿里云MNS 配置项
type Conf struct {
	Url             string `json:"url"`
	AccessKeyId     string `json:"access_key_id"`
	AccessKeySecret string `json:"access_key_secret"`
}

var client *Client

// Client MNS客户端
type Client struct {
	conf Conf
	instance ali_mns.MNSClient
	queueItems map[string]*Queue
	topicItems map[string]*Topic
}

type Topic struct {
	Name string
	instance ali_mns.AliMNSTopic
}

type Params struct {
	Content interface{} `json:"content"`
	Tag string `json:"tag"`
	TraceId string `json:"trace_id"`
	ReferServiceName string `json:"refer_service_name"`
	Sign string `json:"sign"`
}

func (params Params) GenerateSign(key string) string {
	return  library.GetMd5String(params.TraceId + key)
}

// GetTopic 获取主体
func (client *Client) GetTopic(name string) *Topic {
	if client.topicItems == nil {
		client.topicItems = make(map[string]*Topic)
	}

	if _ , ok := client.topicItems[name]; !ok {
		client.topicItems[name] = &Topic{
			Name: name,
			instance: ali_mns.NewMNSTopic(name, client.instance),
		}
	}

	return client.topicItems[name]
}

func (params Params) generateSign(secretKey string)  {
	if params.Sign == "" {
		params.Sign = library.GetMd5String(params.TraceId + secretKey)
	}
}

// PublishMessage 发布消息
func (topic *Topic) PublishMessage(params Params, filterTag string) (*ali_mns.MessageSendResponse, error) {
	bytes , err := json.Marshal(params)
	if err != nil {
		return nil, err
	}

	if params.ReferServiceName == "" {
		params.ReferServiceName = os.Getenv(constant.SystemNameKey)
	}

	if params.TraceId == "" {
		params.TraceId = library.UniqueId()
	}

	if params.Sign == "" {
		params.Sign = params.GenerateSign(client.conf.AccessKeySecret)
	}

	request := ali_mns.MessagePublishRequest{
		MessageBody: base64.StdEncoding.EncodeToString(bytes),
		MessageTag: filterTag,
	}
	resp, err := topic.instance.PublishMessage(request)
	if err != nil {
		return nil, err
	}

	logContext := log.Context{
		"action" : "publishMessage",
		"publish_time" : library.GetTimeStr(),
		"msectime" : library.GetTimeStampFloatStr(),
		"message_id" : resp.MessageId,
		"status_code" : resp.Code,
		"topic_name" : topic.Name,
		"message_tag" : params.Tag,
		"global_trace_id" : library.GetTraceId(),
		"trace_id": params.TraceId,
		"filter_tag" : filterTag,
		"sign" : params.Sign,
	}
	log.Mq().Info("publishMessage", logContext)

	return &resp, nil
}

// InitClient 初始化客户端
func InitClient(conf Conf) *Client {
	if client == nil {
		client = &Client{
			conf:conf,
		}
		client.queueItems = make(map[string]*Queue)
		client.topicItems = make(map[string]*Topic)
	}

	client.instance = ali_mns.NewAliMNSClient(conf.Url,
		conf.AccessKeyId,
		conf.AccessKeySecret)

	return client
}

// GetClient 获取客户端
func GetClient() *Client {
	return client
}

// ListenQueues 监听队列
func (client *Client) ListenQueues() {
	if len(client.queueItems) == 0 {
		return
	}

	for _, item := range client.queueItems {
		if item.Handler == nil {
			continue
		}

		go item.ReceiveMessage(int64(item.WaitSecond))
	}
}

// AddQueue 添加队列
func (client *Client) AddQueue(queue *Queue) {
	queue.instance = ali_mns.NewMNSQueue(queue.Name, client.instance)
	client.queueItems[queue.Name] = queue
}

// GetQueue 获取队列
func (client *Client) GetQueue(name string) *Queue{
	if client.queueItems == nil {
		client.queueItems = make(map[string]*Queue)
	}

	if _ , ok := client.queueItems[name]; !ok {
		client.queueItems[name] = &Queue{
			Name: name,
			instance: ali_mns.NewMNSQueue(name, client.instance),
			Handler: nil,
		}
	}

	return client.queueItems[name]
}

// Queue 队列结构体
type Queue struct {
	Name string // 队列名称
	instance ali_mns.AliMNSQueue // 队列实例
	Handler QueueHandler // 处理方式
	WaitSecond int
}

// QueueHandler 队列消息的处理器
type QueueHandler func(params Params) error

// SetHandler 设置队列消息处理器
func (queue *Queue) SetHandler(handler QueueHandler)  {
	queue.Handler = handler
}

// SendMessage 发送消息
func (queue *Queue) SendMessage(message string) (ali_mns.MessageSendResponse, error) {
	msg := ali_mns.MessageSendRequest{
		MessageBody:  message,
		DelaySeconds: 0,
		Priority:     8}

	resp, err := queue.instance.SendMessage(msg)
	return resp, err
}

func (queue *Queue) GetLogContext(actionName string) log.Context {
	context := log.Context{}
	context["queue_name"] = queue.Name
	context["receiveTime"] = library.GetTimeStr()
	return context
}

// ReceiveMessage 接收消息并处理
func (queue *Queue) ReceiveMessage(waitSeconds int64) {

	if waitSeconds == 0 {
		waitSeconds = 30
	}
	endChan := make(chan int)
	respChan := make(chan ali_mns.MessageReceiveResponse)
	errChan := make(chan error)
	go func() {
		select {
		case resp := <-respChan:
			{
				context := queue.GetLogContext("receiveMessage")

				var params Params

				// 解析消息
				if err := json.Unmarshal([]byte(library.DecodeBase64Str(resp.MessageBody)), &params); err != nil {
					library.Debug("消息结构异常:",queue.Name, err.Error(), resp.MessageBody )
				}else {
					context["messageBody"] = params.Content
					context["tag"] = params.Tag
					context["trace_id"] = params.TraceId
					log.Mq().Info("mns_receive", context)

					if err := queue.Handler(params); err != nil {
						library.Debug("处理消息失败:",queue.Name, err.Error() )

						// TODO ChangeMessageVisibility
					}else {
						// 处理成功，删除消息
						if err := queue.DeleteMessage(resp.ReceiptHandle); err != nil {
							library.Debug("删除消息失败:",queue.Name, err.Error() )
						}else {
							library.Debug("删除消息成功:", queue.Name)
						}

						endChan <- 1
					}
				}

			}
		case err := <-errChan:
			{
				library.Debug(err)
				endChan <- 1
			}
		}
	}()

	// 通过chan去接收数据
	queue.instance.ReceiveMessage(respChan, errChan, waitSeconds)
	<-endChan
}

// DeleteMessage 删除消息
func (queue *Queue) DeleteMessage(receiptHandler string ) error{
	return queue.instance.DeleteMessage(receiptHandler)
}
