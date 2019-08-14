# rabbitmqPool
rabbitmq的客户端连接池

# 实现的功能
- 可以设置client与rabbitmq服务端的connection池
- 每个connection上建立的channel池
- 每个connection与channel都有断线重连机制
- 每个消息都有失败重试机制
- 每个消息都有confirm机制，避免生产者消息丢失
- 目前提供了生产者，而消费者后续会补上

# 依赖包
github.com/streadway/amqp

# 开始使用
- go get github.com/Braveheart7854/rabbitmqPool

- 代码如下
-     rabbitmqPool.AmqpServer = rabbitmqPool.Service{
- 		AmqpUrl:config.AmqpUrl,
- 		ConnectionNum:10,
- 		ChannelNum:100,
- 	  }
-     rabbitmqPool.InitAmqp()	
- 	  message,err := rabbitmqPool.AmqpServer.PutIntoQueue(ExchangeName,RouteKey,data)
- 	  if err != nil{
- 	  //若有错误，则表示消息发送失败，做好失败消息处理
- 	  Logger.Notice(message)
-  	  }
