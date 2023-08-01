package client

import (
	"container/list"
	"context"
	"fmt"
	"strings"
	sync2 "sync"
	"time"

	"aliyun-images-syncer/pkg/middleware"
	"aliyun-images-syncer/pkg/sync"
	"aliyun-images-syncer/pkg/tools"

	cr20181201 "github.com/alibabacloud-go/cr-20181201/v2/client"
	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	console "github.com/alibabacloud-go/tea-console/client"
	util "github.com/alibabacloud-go/tea-utils/v2/service"
	"github.com/rs/zerolog/log"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

type Client struct {
	// 主(dev)镜像仓库，用来拉镜像列表
	clientMaster *cr20181201.Client
	// 从(prod)镜像仓库，用来同步镜像
	clientSlave *cr20181201.Client
	// 日志对象
	Logger *logrus.Logger
	// 阿里镜像仓库api
	alibabacloudApi *AlibabacloudApi
	// 邮件服务
	MailClient *middleware.MailClient

	// a sync.Task list
	taskList *list.List

	// a URLPair list
	urlPairList *list.List

	// failed list
	failedTaskList         *list.List
	failedTaskGenerateList *list.List

	config *Config

	routineNum int
	retries    int

	// mutex
	taskListChan               chan int
	urlPairListChan            chan int
	failedTaskListChan         chan int
	failedTaskGenerateListChan chan int

	// dig
	Dep *Dependency
}

// URLPair is a pair of source and destination url
type URLPair struct {
	source      string
	destination string
}

// CreateClient 使用AK&SK初始化账号Client
func CreateClient(accessKeyIdMaster, accessKeySecretMaster, endpointMaster, accountMaster, passwordMaster *string,
	accessKeyIdSlave, accessKeySecretSlave, endpointSlave, accountSlave, passwordSlave *string,
	repoNamespaceName, instanceIdMaster, instanceIdSlave *string,
	publicNetworkMaster, publicNetworkSlave *string,
	mailHost, mailUserName, mailAuthCode, mailTo string,
	logFile string, repoNamespaceNames []string, dep *Dependency) (client *Client, err error) {

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var (
		clientMaster *cr20181201.Client
		clientSlave  *cr20181201.Client
	)

	wg, gCtx := errgroup.WithContext(ctx)
	// 初始化主镜像仓库的client
	wg.Go(func() error {
		clientMaster, err = CreateAliOpenapiClient(gCtx, accessKeyIdMaster, accessKeySecretMaster, endpointMaster)
		if err != nil {
			return err
		}
		return nil
	})

	// 初始化副镜像仓库的client
	wg.Go(func() error {
		clientSlave, err = CreateAliOpenapiClient(gCtx, accessKeyIdSlave, accessKeySecretSlave, endpointSlave)
		if err != nil {
			return err
		}
		return nil
	})

	if err := wg.Wait(); err != nil {
		return nil, err
	}

	logger := NewFileLogger(logFile)
	return &Client{
		clientMaster: clientMaster,
		clientSlave:  clientSlave,
		Logger:       logger,
		MailClient:   middleware.NewMailClient(mailHost, mailUserName, mailAuthCode, mailTo),
		// 封装一个api的包
		alibabacloudApi: NewAlibabacloudApi(
			&Alibabacloud{
				Client:          clientMaster,
				Account:         accountMaster,
				Password:        passwordMaster,
				InstanceId:      instanceIdMaster,
				AccessKeyId:     accessKeyIdMaster,
				AccessKeySecret: accessKeySecretMaster,
				Endpoint:        endpointMaster,
				Network:         publicNetworkMaster,
			},
			&Alibabacloud{
				Client:          clientSlave,
				Account:         accountSlave,
				Password:        passwordSlave,
				InstanceId:      instanceIdSlave,
				AccessKeyId:     accessKeyIdSlave,
				AccessKeySecret: accessKeySecretSlave,
				Endpoint:        endpointSlave,
				Network:         publicNetworkSlave,
			},
			repoNamespaceName,
			repoNamespaceNames,
			logger,
		),
		Dep: dep,
	}, err
}

func CreateAliOpenapiClient(ctx context.Context, accessKeyId, accessKeySecret, endpoint *string) (*cr20181201.Client, error) {
	config := &openapi.Config{
		// 您的 AccessKey ID
		AccessKeyId: accessKeyId,
		// 您的 AccessKey Secret
		AccessKeySecret: accessKeySecret,
		// 访问的域名 cr.cn-shanghai.aliyuncs.co
		Endpoint: endpoint,
	}
	client := &cr20181201.Client{}
	client, err := cr20181201.NewClient(config)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func (c *Client) Run() string {
	log.Log().Msg("Start scanning ...")

	if _, ok := c.Dep.Lru.Get("syncing"); ok {
		log.Log().Msg("Syncing, please wait ...")
		return "Some images is syncing, please wait ..."
	}
	c.Dep.Lru.Add("syncing", 1)

	// time.Sleep(5 * time.Second)
	for _, ns := range c.alibabacloudApi.RepoNamespaceNames {
		c.Sync(ns)
	}

	c.Dep.Lru.Remove("syncing")
	log.Log().Msg("End scanning ...")
	return "success"
}
func (c *Client) Sync(ns string) {
	fmt.Printf("Start scanning the difference between master and slave images ...,namespance is %s\n", ns)

	// prepare
	c.Prepare()
	c.alibabacloudApi.RepoNamespaceName = &ns

	// 1. get mster and slave tags
	var (
		respListMaster *cr20181201.ListRepositoryResponse
		respListSlave  *cr20181201.ListRepositoryResponse
		err            error
		tagMapsMaster  map[string]string
		tagMapsSlave   map[string]string
	)

	wg, _ := errgroup.WithContext(context.Background())
	wg.Go(func() error {
		// 获取镜像仓库列表
		respListMaster, err = c.alibabacloudApi.ListRepository(Master)
		if err != nil {
			c.Logger.Error("Master ListRepository err", err)
			return err
		}
		// 获取每一个镜像最新tag
		tagMapsMaster, err = c.alibabacloudApi.ListRepoTagWithOptionsByRoutine(Master, respListMaster.Body.Repositories)
		if err != nil {
			c.Logger.Error("Master ListRepoTagWithOptionsByRoutine err", err)
			return err
		}
		return nil
	})

	time.Sleep(300 * time.Millisecond)

	wg.Go(func() error {
		// 获取镜像仓库列表
		respListSlave, err = c.alibabacloudApi.ListRepository(Slave)
		if err != nil {
			c.Logger.Error("Slave err", err)
			return err
		}
		// 获取每一个镜像最新tag
		tagMapsSlave, err = c.alibabacloudApi.ListRepoTagWithOptionsByRoutine(Slave, respListSlave.Body.Repositories)
		if err != nil {
			c.Logger.Error("Slave ListRepoTagWithOptionsByRoutine err", err)
			return err
		}
		return nil
	})

	if err := wg.Wait(); err != nil {
		fmt.Println("get images list fail，Wait for the next inspection...", err)
		return
	}

	// tagMapsMaster = map[string]string{"dictionary:v1.3.2": "v1.3.2", "ds3_serving:v1.3.6": "v1.3.6", "hodorcms:v0.0.2-beta.5": "v0.0.2-beta.5", "images-sync:v0.0.5": "v0.0.5", "images-sync:v0.0.7": "v0.0.7", "images-sync:v0.0.8": "v0.0.8", "images-sync:v0.1.0": "v0.1.0", "images-sync:v0.1.1": "v0.1.1"}
	// tagMapsSlave = map[string]string{"dictionary:v1.3.2": "v1.3.2"}

	// 2. filter need sync data
	syncMap := tools.RepoTagsMapDiff(tagMapsMaster, tagMapsSlave)
	fmt.Println("Get the data that needs to be synchronized ...")
	console.Log(util.ToJSONString(syncMap))
	if len(syncMap) <= 0 {
		fmt.Println("No image update，Wait for the next inspection...")
		return
	}

	// 3. syncing
	fmt.Println("Start to generate sync tasks, please wait ...")

	configs, err := c.alibabacloudApi.NewSyncConfig(syncMap, []string{}, []string{})
	if err != nil {
		c.Logger.Error("NewSyncConfig err", err)
		return
	}
	c.config = configs

	// 下面是基于：github.com/AliyunContainerService/image-syncer manifest构建images，修改了config的配置，只使用内存不占用磁盘，经过测试这种方式最稳定！
	// open num of goroutines and wait c for close
	openRoutinesGenTaskAndWaitForFinish := func() {
		wg := sync2.WaitGroup{}
		for i := 0; i < c.routineNum; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					urlPair, empty := c.GetAURLPair()
					// no more task to generate
					if empty {
						break
					}
					moreURLPairs, err := c.GenerateSyncTask(urlPair.source, urlPair.destination)
					if err != nil {
						c.Logger.Errorf("Generate sync task %s to %s error: %v", urlPair.source, urlPair.destination, err)
						// put to failedTaskGenerateList
						c.PutAFailedURLPair(urlPair)
					}
					if moreURLPairs != nil {
						c.PutURLPairs(moreURLPairs)
					}
				}
			}()
		}
		wg.Wait()
	}

	openRoutinesHandleTaskAndWaitForFinish := func() {
		wg := sync2.WaitGroup{}
		for i := 0; i < c.routineNum; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					task, empty := c.GetATask()
					// no more tasks need to handle
					if empty {
						break
					}
					if err := task.Run(); err != nil {
						// put to failedTaskList
						c.PutAFailedTask(task)
					}
				}
			}()
		}

		wg.Wait()
	}

	for source, dest := range c.config.GetImageList() {
		c.urlPairList.PushBack(&URLPair{
			source:      source,
			destination: dest,
		})
	}

	// generate sync tasks
	openRoutinesGenTaskAndWaitForFinish()

	fmt.Println("Start to handle sync tasks, please wait ...")

	// generate goroutines to handle sync tasks
	openRoutinesHandleTaskAndWaitForFinish()

	for times := 0; times < c.retries; times++ {
		if c.failedTaskGenerateList.Len() != 0 {
			c.urlPairList.PushBackList(c.failedTaskGenerateList)
			c.failedTaskGenerateList.Init()
			// retry to generate task
			fmt.Println("Start to retry to generate sync tasks, please wait ...")
			openRoutinesGenTaskAndWaitForFinish()
		}

		if c.failedTaskList.Len() != 0 {
			c.taskList.PushBackList(c.failedTaskList)
			c.failedTaskList.Init()
		}

		if c.taskList.Len() != 0 {
			// retry to handle task
			fmt.Println("Start to retry sync tasks, please wait ...")
			openRoutinesHandleTaskAndWaitForFinish()
		}
	}

	fmt.Printf("Finished, %v sync tasks failed, %v tasks generate failed\n", c.failedTaskList.Len(), c.failedTaskGenerateList.Len())
	c.Logger.Infof("Finished, %v sync tasks failed, %v tasks generate failed", c.failedTaskList.Len(), c.failedTaskGenerateList.Len())

}

// Prepare 每轮sync 初始化一些configs
func (c *Client) Prepare() {
	c.taskList = list.New()
	c.urlPairList = list.New()
	c.failedTaskList = list.New()
	c.failedTaskGenerateList = list.New()
	c.taskListChan = make(chan int, 1)
	c.urlPairListChan = make(chan int, 1)
	c.failedTaskListChan = make(chan int, 1)
	c.failedTaskGenerateListChan = make(chan int, 1)
	c.routineNum = 5
	c.retries = 2
}

// GenerateSyncTask creates synchronization tasks from source and destination url, return URLPair array if there are more than one tags
func (c *Client) GenerateSyncTask(source string, destination string) ([]*URLPair, error) {
	if source == "" {
		return nil, fmt.Errorf("source url should not be empty")
	}

	sourceURL, err := tools.NewRepoURL(source)
	if err != nil {
		return nil, fmt.Errorf("url %s format error: %v", source, err)
	}

	// if dest is not specific, use default registry and namespace
	if destination == "" {
		if c.config.defaultDestRegistry != "" && c.config.defaultDestNamespace != "" {
			destination = c.config.defaultDestRegistry + "/" + c.config.defaultDestNamespace + "/" +
				sourceURL.GetRepoWithTag()
		} else {
			return nil, fmt.Errorf("the default registry and namespace should not be nil if you want to use them")
		}
	}

	destURL, err := tools.NewRepoURL(destination)
	if err != nil {
		return nil, fmt.Errorf("url %s format error: %v", destination, err)
	}

	tags := sourceURL.GetTag()

	// multi-tags config
	if moreTag := strings.Split(tags, ","); len(moreTag) > 1 {
		if destURL.GetTag() != "" && destURL.GetTag() != sourceURL.GetTag() {
			return nil, fmt.Errorf("multi-tags source should not correspond to a destination with tag: %s:%s",
				sourceURL.GetURL(), destURL.GetURL())
		}

		// contains more than one tag
		var urlPairs []*URLPair
		for _, t := range moreTag {
			urlPairs = append(urlPairs, &URLPair{
				source:      sourceURL.GetURLWithoutTag() + ":" + t,
				destination: destURL.GetURLWithoutTag() + ":" + t,
			})
		}

		return urlPairs, nil
	}

	var imageSource *sync.ImageSource
	var imageDestination *sync.ImageDestination

	if auth, exist := c.config.GetAuth(sourceURL.GetRegistry(), sourceURL.GetNamespace()); exist {
		c.Logger.Infof("Find auth information for %v, username: %v", sourceURL.GetURL(), auth.Username)
		imageSource, err = sync.NewImageSource(sourceURL.GetRegistry(), sourceURL.GetRepoWithNamespace(), sourceURL.GetTag(),
			auth.Username, auth.Password, auth.Insecure)
		if err != nil {
			return nil, fmt.Errorf("generate %s image source error: %v", sourceURL.GetURL(), err)
		}
	} else {
		c.Logger.Infof("Cannot find auth information for %v, pull actions will be anonymous", sourceURL.GetURL())
		imageSource, err = sync.NewImageSource(sourceURL.GetRegistry(), sourceURL.GetRepoWithNamespace(), sourceURL.GetTag(),
			"", "", false)
		if err != nil {
			return nil, fmt.Errorf("generate %s image source error: %v", sourceURL.GetURL(), err)
		}
	}

	// if tag is not specific, return tags
	if sourceURL.GetTag() == "" {
		if destURL.GetTag() != "" {
			return nil, fmt.Errorf("tag should be included both side of the config: %s:%s", sourceURL.GetURL(), destURL.GetURL())
		}

		// get all tags of this source repo
		tags, err := imageSource.GetSourceRepoTags()
		if err != nil {
			return nil, fmt.Errorf("get tags failed from %s error: %v", sourceURL.GetURL(), err)
		}
		c.Logger.Infof("Get tags of %s successfully: %v", sourceURL.GetURL(), tags)

		// generate url pairs for tags
		var urlPairs = []*URLPair{}
		for _, tag := range tags {
			urlPairs = append(urlPairs, &URLPair{
				source:      sourceURL.GetURL() + ":" + tag,
				destination: destURL.GetURL() + ":" + tag,
			})
		}
		return urlPairs, nil
	}

	// if source tag is set but without destination tag, use the same tag as source
	destTag := destURL.GetTag()
	if destTag == "" {
		destTag = sourceURL.GetTag()
	}

	if auth, exist := c.config.GetAuth(destURL.GetRegistry(), destURL.GetNamespace()); exist {
		c.Logger.Infof("Find auth information for %v, username: %v", destURL.GetURL(), auth.Username)
		imageDestination, err = sync.NewImageDestination(destURL.GetRegistry(), destURL.GetRepoWithNamespace(),
			destTag, auth.Username, auth.Password, auth.Insecure)
		if err != nil {
			return nil, fmt.Errorf("generate %s image destination error: %v", sourceURL.GetURL(), err)
		}
	} else {
		c.Logger.Infof("Cannot find auth information for %v, push actions will be anonymous", destURL.GetURL())
		imageDestination, err = sync.NewImageDestination(destURL.GetRegistry(), destURL.GetRepoWithNamespace(),
			destTag, "", "", false)
		if err != nil {
			return nil, fmt.Errorf("generate %s image destination error: %v", destURL.GetURL(), err)
		}
	}

	c.PutATask(sync.NewTask(imageSource, imageDestination, c.config.osFilterList, c.config.archFilterList, c.Logger))
	c.Logger.Infof("Generate a task for %s to %s", sourceURL.GetURL(), destURL.GetURL())
	return nil, nil
}

// GetATask return a sync.Task struct if the task list is not empty
func (c *Client) GetATask() (*sync.Task, bool) {
	c.taskListChan <- 1
	defer func() {
		<-c.taskListChan
	}()

	task := c.taskList.Front()
	if task == nil {
		return nil, true
	}
	c.taskList.Remove(task)

	return task.Value.(*sync.Task), false
}

// PutATask puts a sync.Task struct to task list
func (c *Client) PutATask(task *sync.Task) {
	c.taskListChan <- 1
	defer func() {
		<-c.taskListChan
	}()

	if c.taskList != nil {
		c.taskList.PushBack(task)
	}
}

// GetAURLPair gets a URLPair from urlPairList
func (c *Client) GetAURLPair() (*URLPair, bool) {
	c.urlPairListChan <- 1
	defer func() {
		<-c.urlPairListChan
	}()

	urlPair := c.urlPairList.Front()
	if urlPair == nil {
		return nil, true
	}
	c.urlPairList.Remove(urlPair)

	return urlPair.Value.(*URLPair), false
}

// PutURLPairs puts a URLPair array to urlPairList
func (c *Client) PutURLPairs(urlPairs []*URLPair) {
	c.urlPairListChan <- 1
	defer func() {
		<-c.urlPairListChan
	}()

	if c.urlPairList != nil {
		for _, urlPair := range urlPairs {
			c.urlPairList.PushBack(urlPair)
		}
	}
}

// GetAFailedTask gets a failed task from failedTaskList
func (c *Client) GetAFailedTask() (*sync.Task, bool) {
	c.failedTaskListChan <- 1
	defer func() {
		<-c.failedTaskListChan
	}()

	failedTask := c.failedTaskList.Front()
	if failedTask == nil {
		return nil, true
	}
	c.failedTaskList.Remove(failedTask)

	return failedTask.Value.(*sync.Task), false
}

// PutAFailedTask puts a failed task to failedTaskList
func (c *Client) PutAFailedTask(failedTask *sync.Task) {
	c.failedTaskListChan <- 1
	defer func() {
		<-c.failedTaskListChan
	}()

	if c.failedTaskList != nil {
		c.failedTaskList.PushBack(failedTask)
	}
}

// GetAFailedURLPair get a URLPair from failedTaskGenerateList
func (c *Client) GetAFailedURLPair() (*URLPair, bool) {
	c.failedTaskGenerateListChan <- 1
	defer func() {
		<-c.failedTaskGenerateListChan
	}()

	failedURLPair := c.failedTaskGenerateList.Front()
	if failedURLPair == nil {
		return nil, true
	}
	c.failedTaskGenerateList.Remove(failedURLPair)

	return failedURLPair.Value.(*URLPair), false
}

// PutAFailedURLPair puts a URLPair to failedTaskGenerateList
func (c *Client) PutAFailedURLPair(failedURLPair *URLPair) {
	c.failedTaskGenerateListChan <- 1
	defer func() {
		<-c.failedTaskGenerateListChan
	}()

	if c.failedTaskGenerateList != nil {
		c.failedTaskGenerateList.PushBack(failedURLPair)
	}
}
