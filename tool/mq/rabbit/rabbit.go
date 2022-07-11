package rabbit

import (
    "errors"
    "github.com/streadway/amqp"
)

const (
    ReconnectDelay = 10 // 连接断开后多久重连
    ResendDelay    = 3  // 消息发送超时时间
)

var (
    ParamsError   = errors.New("地址、路由和队列不能为空")
    CallBackError = errors.New("消费回调函数不能为空")
)

func NewRabbitProduct(rbInfo RbInfo, callBack func(amqp.Delivery)) (*RbMqClient, error) {
    if len(rbInfo.Addr) == 0 || len(rbInfo.ExchangeName) == 0 || len(rbInfo.QueueName) == 0 {
        return nil, ParamsError
    }
    product := &RbMqClient{
        rbInfo:              rbInfo,
        done:                make(chan struct{}),
        coonNotifyConnected: make(chan struct{}),
    }
    if callBack != nil {
        product.callBack = callBack
    }
    return product, nil
}
