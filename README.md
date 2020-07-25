# GoMicro RocketMQ Broker Plugin

### Run test
for mac
```bash
sudo vim /etc/hosts
# input text bellow and save (!wq)
# 127.0.0.1       docker.for.mac.host.internal

docker-compose up -d

go test -v rocketmq_test.go
```