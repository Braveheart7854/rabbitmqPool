package rabbitmqPool

import (
	"encoding/json"
	"errors"
	"github.com/streadway/amqp"
	"log"
	"math/rand"
	"strings"
	"sync"
	"time"
)

type Service struct {
	AmqpUrl       string  //amqp地址
	ConnectionNum int     //连接数
	ChannelNum    int     //每个连接的channel数量

	connections   map[int]*amqp.Connection
	channels      map[int]channel
	idelChannels  []int
	busyChannels  map[int]int
	m             *sync.Mutex
}

type channel struct {
	ch *amqp.Channel
	notifyClose chan *amqp.Error
	notifyConfirm chan amqp.Confirmation
}

const (
	retryCount = 5
	waitConfirmTime = 3*time.Second
)

var AmqpServer Service

func InitAmqp(){
	if AmqpServer.AmqpUrl == "" {
		log.Fatal("rabbitmq's address can not be empty!")
	}
	if AmqpServer.ConnectionNum == 0 {
		AmqpServer.ConnectionNum = 10
	}
	if AmqpServer.ChannelNum == 0 {
		AmqpServer.ChannelNum = 10
	}
	AmqpServer.busyChannels = make(map[int]int)
	AmqpServer.m = new(sync.Mutex)
	AmqpServer.connectPool()
	AmqpServer.channelPool()
}

func failOnError(err error, msg string) {
	if err != nil {
		log.Panicf("%s: %s", msg, err)
	}
}

func (S *Service) connectPool(){
	S.connections = make(map[int]*amqp.Connection)
	for i:=0;i<S.ConnectionNum ;i++ {
		connection := S.connect()
		S.connections[i] = connection
	}
}

func (S *Service) channelPool(){
	S.channels = make(map[int]channel)
	//S.idelChannels = make(map[int]int)
	for index,connection := range S.connections{
		for j:=0;j<S.ChannelNum ;j++ {
			key := index*S.ChannelNum+j
			S.channels[key] = S.createChannel(index,connection)
			//S.idelChannels[key] = key
			S.idelChannels = append(S.idelChannels,key)
		}
	}
}

func (S *Service) connect()*amqp.Connection{
	conn, err := amqp.Dial(S.AmqpUrl)
	failOnError(err, "Failed to connect to RabbitMQ")
	//defer conn.Close()
	return conn
}

func (S *Service) recreateChannel(connectId int, err error)(ch *amqp.Channel){
	if strings.Index(err.Error(),"channel/connection is not open") >= 0 || strings.Index(err.Error(),"CHANNEL_ERROR - expected 'channel.open'") >=0{
		//S.connections[connectId].Close()
		var newConn *amqp.Connection
		if S.connections[connectId].IsClosed() {
			newConn = S.connect()
		}else {
			newConn = S.connections[connectId]
		}
		S.lockWriteConnect(connectId,newConn)
		//S.connections[connectId] = newConn
		ch, err = newConn.Channel()
		failOnError(err, "Failed to open a channel")
	}else{
		failOnError(err, "Failed to open a channel")
	}
	return
}

func (S *Service) lockWriteConnect(connectId int,newConn *amqp.Connection){
	S.m.Lock()
	defer S.m.Unlock()
	S.connections[connectId] = newConn
}

func (S *Service) createChannel(connectId int,conn *amqp.Connection)channel{
	var notifyClose = make(chan *amqp.Error)
	var notifyConfirm = make(chan amqp.Confirmation)

	cha := channel{
		notifyClose:notifyClose,
		notifyConfirm:notifyConfirm,
	}
	if conn.IsClosed() {
		conn = S.connect()
	}
	ch, err := conn.Channel()
	if err != nil {
		ch = S.recreateChannel(connectId, err)
	}

	ch.Confirm(false)
	ch.NotifyClose(cha.notifyClose)
	ch.NotifyPublish(cha.notifyConfirm)

	cha.ch = ch
	//go func() {
	//	select {
	//	case <-cha.notifyClose:
	//		fmt.Println("close channel")
	//	}
	//}()
	return cha
}

func (S *Service) getChannel()(*amqp.Channel,int){
	S.m.Lock()
	defer S.m.Unlock()
	idelLength := len(S.idelChannels)
	if idelLength > 0 {

		rand.Seed(time.Now().Unix())
		index := rand.Intn(idelLength)
		channelId := S.idelChannels[index]
		S.idelChannels = append(S.idelChannels[:index], S.idelChannels[index+1:]...)
		//S.busyChannels = make(map[int]int)
		S.busyChannels[channelId] = channelId

		ch := S.channels[channelId].ch
		//fmt.Println("channels count: ",len(S.channels))
		//fmt.Println("idel channels count: ",len(S.idelChannels))
		//fmt.Println("busy channels count: ",len(S.busyChannels))
		//fmt.Println("channel id: ",channelId)
		return ch,channelId
	}else{
		//return S.createChannel(0,S.connections[0]),-1
		return nil,-1
	}
}

func (S *Service) declareExchange(ch *amqp.Channel,exchangeName string,channelId int) *amqp.Channel{
	err := ch.ExchangeDeclare(
		exchangeName, // name
		"topic",      // type
		true,         // durable
		false,        // auto-deleted
		false,        // internal
		false,        // no-wait
		nil,          // arguments
	)
	if err != nil {
		ch = S.reDeclareExchange(channelId,exchangeName,err)
	}
	return ch
}

func (S *Service) reDeclareExchange(channelId int,exchangeName string, err error)(ch *amqp.Channel) {
	//fmt.Println("reDeclareExchange")

	var connectionId int
	if strings.Index(err.Error(), "channel/connection is not open") >= 0 {

		//S.channels[channelId].Close()
		if channelId == -1{
			connectionId = 0
		}else{
			connectionId = int(channelId/S.ChannelNum)
		}
		cha := S.createChannel(connectionId,S.connections[connectionId])

		S.lockWriteChannel(channelId,cha)
		//S.channels[channelId] = cha
		err := cha.ch.ExchangeDeclare(
			exchangeName, // name
			"topic",      // type
			true,         // durable
			false,        // auto-deleted
			false,        // internal
			false,        // no-wait
			nil,          // arguments
		)
		if err != nil {
			failOnError(err, "Failed to declare an exchange")
		}
		return cha.ch
	}else{

		failOnError(err, "Failed to declare an exchange")
		return nil
	}
}

func (S *Service) lockWriteChannel(channelId int,cha channel){
	S.m.Lock()
	defer S.m.Unlock()
	S.channels[channelId] = cha
}

func (S *Service) dataForm(notice interface{})string{
	body, err := json.Marshal(notice)
	if err != nil {
		log.Panic(err)
	}
	return string(body)
}

func (S *Service) publish(channelId int,ch *amqp.Channel,exchangeName string, routeKey string,data string)(err error){

	err = ch.Publish(
		exchangeName, // exchange
		routeKey,     //severityFrom(os.Args), // routing key
		false,        // mandatory
		false,        // immediate
		amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType: "application/json",
			Body:        []byte(data),
		})

	if err != nil {
		if strings.Index(err.Error(), "channel/connection is not open") >= 0 {
			err = S.rePublish(channelId, exchangeName, err, routeKey, data)
		}
	}
	return
}

func (S *Service) rePublish(channelId int,exchangeName string, errmsg error,routeKey string,data string)(err error){
	//fmt.Println("rePublish")

	ch := S.reDeclareExchange(channelId, exchangeName, errmsg )
	err = ch.Publish(
		exchangeName, // exchange
		routeKey,     //severityFrom(os.Args), // routing key
		false,        // mandatory
		false,        // immediate
		amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType: "application/json",
			Body:        []byte(data),
		})
	return
}

func (S *Service) backChannelId(channelId int,ch *amqp.Channel){
	S.m.Lock()
	defer S.m.Unlock()
	S.idelChannels = append(S.idelChannels,channelId)
	delete(S.busyChannels,channelId)
	return
}

func (S *Service) PutIntoQueue(exchangeName string, routeKey string, notice interface{}) (message interface{}, puberr error ){
	defer func() {
		msg := recover()
		if msg != nil {
			//fmt.Println("msg: ",msg)
			puberrMsg,_ := msg.(string)
			//fmt.Println("ok: ",ok)
			//fmt.Println("puberrMsg : ",puberrMsg)
			puberr = errors.New(puberrMsg)
			return
		}
	}()

	ch,channelId := S.getChannel()
	cha := channel{}
	if ch == nil {
		cha = S.createChannel(0,S.connections[0])
		defer cha.ch.Close()
		ch = cha.ch
		//fmt.Println("ch: ",ch)
	}
	ch = S.declareExchange(ch,exchangeName,channelId)

	data := S.dataForm(notice)
	var tryTime = 1

	for  {
		puberr = S.publish(channelId, ch, exchangeName, routeKey, data)
		if puberr !=nil {
			if tryTime <= retryCount {
				//log.Printf("%s: %s", "Failed to publish a message, try again.", puberr)
				tryTime ++
				continue
			}else{
				//log.Printf("%s: %s data: %s", "Failed to publish a message", puberr,data)
				S.backChannelId(channelId,ch)
				return notice,puberr
			}
		}

		select {
		case confirm := <-S.channels[channelId].notifyConfirm:
			//log.Printf(" [%s] Sent %d message %s", routeKey, confirm.DeliveryTag, data)
			if confirm.Ack {
				S.backChannelId(channelId, ch)

				return notice, nil
			}
			return notice,errors.New("ack failed")
		case chaConfirm := <-cha.notifyConfirm:
			//log.Println("加班车",data)
			if chaConfirm.Ack {
				return notice, nil
			}
			return notice,errors.New("ack failed")
		case <-time.After(waitConfirmTime):
			//	log.Printf("message: %s data: %s", "Can not receive the confirm.", data)
			S.backChannelId(channelId, ch)
			confirmErr := errors.New("Can not receive the confirm . ")
			return notice, confirmErr
		}
	}
	return
}