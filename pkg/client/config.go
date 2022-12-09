package client

import (
	"strings"
)

// Config information of sync client
type Config struct {
	// the authentication information of each registry
	AuthList map[string]Auth `json:"auth" yaml:"auth"`

	// a <source_repo>:<dest_repo> map
	ImageList map[string]string `json:"images" yaml:"images"`

	// only images with selected os can be sync
	osFilterList []string
	// only images with selected architecture can be sync
	archFilterList []string

	// If the destination registry and namespace is not provided,
	// the source image will be synchronized to defaultDestRegistry
	// and defaultDestNamespace with origin repo name and tag.
	defaultDestRegistry  string
	defaultDestNamespace string
}

// Auth describes the authentication information of a registry
type Auth struct {
	Username string `json:"username" yaml:"username"`
	Password string `json:"password" yaml:"password"`
	Insecure bool   `json:"insecure" yaml:"insecure"`
}

// NewSyncConfig creates a Config struct
// 需要开启：阿里云镜像仓库需要配置"仓库管理" => "访问控制" => "公网" => "访问入口 -> 开启" => "删除所有白名单后，公网下机器均可通过凭证访问企业版实例"
func (api *AlibabacloudApi) NewSyncConfig(imageListMap map[string]string, osFilterList, archFilterList []string) (*Config, error) {
	var config Config

	// auth
	// 注意这里使用的Network公网地址
	authList := make(map[string]Auth)
	authList[*api.Master.Network] = Auth{
		Username: *api.Master.Account,
		Password: *api.Master.Password,
		Insecure: false,
	}
	authList[*api.Slave.Network] = Auth{
		Username: *api.Slave.Account,
		Password: *api.Slave.Password,
		Insecure: false,
	}

	// images
	// 这里的images是以{镜像:tag}的形式来保存的，比如{"alpine:v0.0.1":"v0.0.1"}
	imageList := make(map[string]string)
	for image, _ := range imageListMap {
		realImage := strings.Split(image, ":")
		imageList[*api.Master.Network+"/"+*api.RepoNamespaceName+"/"+image] = *api.Slave.Network + "/" + *api.RepoNamespaceName + "/" + realImage[0]
	}

	config.defaultDestNamespace = "aliyun"
	config.defaultDestRegistry = "test-registry.cn-shanghai.cr.aliyuncs.com"
	config.osFilterList = osFilterList
	config.archFilterList = archFilterList
	config.AuthList = authList
	config.ImageList = imageList

	// fmt.Println("config.ImageList:", config.ImageList)
	// fmt.Println("config.AuthList:", config.AuthList)
	return &config, nil
}

// GetAuth gets the authentication information in Config
func (c *Config) GetAuth(registry string, namespace string) (Auth, bool) {
	// key of each AuthList item can be "registry/namespace" or "registry" only
	registryAndNamespace := registry + "/" + namespace

	if moreSpecificAuth, exist := c.AuthList[registryAndNamespace]; exist {
		return moreSpecificAuth, exist
	}

	auth, exist := c.AuthList[registry]
	return auth, exist
}

// GetImageList gets the ImageList map in Config
func (c *Config) GetImageList() map[string]string {
	return c.ImageList
}
