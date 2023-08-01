## 功能
阿里云容器镜像服务ACR基础版不提供镜像同步功能，本服务通过调用接口的方式来实现镜像同步，并可根据需求调整自动同步的时间（建议每次同步不小于60s），高度集成了CI/CD的部署方式，服务内嵌了healcheck和metrics（基础版，可根据自己需求修改），满足部署到k8s上的HPA和prometheus的自定义指标监控。

## 描述
- 使用阿里云容器镜像服务api:https://next.api.aliyun.com/api/cr/2018-12-01/ListRepository?spm=5176.2020520104.0.0.5d88709awGz44N&sdkStyle=old&lang=GO&tab=DEMO
  来完成2个阿里云镜像服务镜像自动同步功能，主要思路是分别拉取主从镜像列表，对比tag，有更新则自动触发: 主（dev）pull -> tag -> 从（prod）push -> clean
- 使用github.com/AliyunContainerService/image-syncer服务来解决镜像同步的磁盘消耗问题，只在内存中操作。使用镜像的manifest完成同步，Image Index 就是manifest 的集合
manifest 记录了image config 和 layers
- 免费版本的阿里云镜像访问api有qps限制，每隔60s检查一次
- 支持devops，具体请查看gitlab-ci.yml

## 服务
- 轮询
  启动项目后，会有2个同步操作，一个是http的api触发，还有一个就是轮询，轮询的时间间隔可以通过polling来配置，默认5分钟
- http
  有的时候，有突发fix动作，可能不经过测试直接上线（其实不推荐啦），那么最长可能要等待5分钟才能让dev的镜像同步到prod，这个时候就可以通过api触发的方式来达到即时触发同步的操作，这个api可以由类似飞书或者企业微信这种对话方式触发，更加高效和方便！
- SIGTERM
  两个服务都会受到SIGTERM型号控制，可以优雅的中断，不用担心僵尸协程！

## 使用
- 查看flag，补充符合描述的参数
- 编译
  ```
  make build
  ```
- 直接执行 
  ```
  bin/fermi
  ```