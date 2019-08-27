# rabbitmqPool
- rabbitmq的客户端连接池
- (使用rabbitmq文档提供的使用实例时，发现connection和channel没有实现复用，效率很低，由于一直没有找到实现connection和channel复用的相关代码库，故自己实现一个)
- 在一台4核8G机子上ab压测http 10000次请求，1000并发时，qps为2000+。代码见https://github.com/Braveheart7854/asyncMessageSystem
- 适当增大ConnectionNum和ChannelNum，可以提高性能

# 实现的功能
- 可以设置client与rabbitmq服务端的connection池
- 每个connection上建立的channel池
- 每个connection与channel都有断线重连机制
- 每个消息都有失败重试机制
- 每个消息都有confirm机制，避免生产者消息丢失
- 目前提供了生产者，而消费者后续会补上
- 目前exchange的type类型默认是topic，没有支持其他三种类型，如果需要，请留言我给加上

# 依赖包
github.com/streadway/amqp

# 开始使用
- go get github.com/Braveheart7854/rabbitmqPool

- 代码如下
-     rabbitmqPool.AmqpServer = rabbitmqPool.Service{
- 		AmqpUrl:"amqp://guest:guest@localhost:5672/",
- 		ConnectionNum:10,
- 		ChannelNum:100,
- 	  }
-     rabbitmqPool.InitAmqp()	
- 	  message,err := rabbitmqPool.AmqpServer.PutIntoQueue(ExchangeName,RouteKey,data)
- 	  if err != nil{
- 	  //若有错误，则表示消息发送失败，做好失败消息处理
- 	  rabbitmqPool.Logger.Notice(message)
-  	  }
