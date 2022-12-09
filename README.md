## 描述
- 使用阿里云容器镜像服务api:https://next.api.aliyun.com/api/cr/2018-12-01/ListRepository?spm=5176.2020520104.0.0.5d88709awGz44N&sdkStyle=old&lang=GO&tab=DEMO
  来完成2个阿里云镜像服务镜像自动同步功能，主要思路是分别拉取主从镜像列表，对比tag，有更新则自动触发: 主（dev）pull -> tag -> 从（prod）push -> clean
- 使用开源的github.com/AliyunContainerService/image-syncer服务来解决镜像同步的磁盘消耗问题，只在内存中操作。使用镜像的manifest完成同步，Image Index 就是manifest 的集合
manifest 记录了image config 和 layers
- 免费版本的阿里云镜像访问api有qps限制，每隔60s检查一次
- 支持devops，具体请查看gitlab-ci.yml
## 使用
- 查看flag，补充符合描述的参数
- make build