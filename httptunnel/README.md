# httptunnel

## 说明
隧道同一个端口同时支持http和https. support http and https in same port。  
原理是通过判断通信的前四个字节讲请求转发到不同的地址。  
例如后端有有两个分别在8080端口提供http服务和在8043端口提供https的服务，那么我们配置监听5656端口，客户端通过访问5656端口即可自动根据请求的类型（http https）分发到对应的端口。  

## 运行

```
./httptunnel httptunnel.conf
```


httptunnel.conf配置样例：
```
listen=0.0.0.0:5656
http=127.0.0.1:8080  # nginx listen http port
https=127.0.0.1:8043 # nginx listen https port
buff=1000


idle_timeout=80 # default 80 second
dial_timeout=3 # default 3 second
keep_alive=280 # default 280 second

log_path=/tmp
log_name=httptunnel.log
log_level=1
log_console=false
```
