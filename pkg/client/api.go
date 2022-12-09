package client

import (
	"context"
	"sort"
	"strconv"
	"sync"
	"time"

	cr20181201 "github.com/alibabacloud-go/cr-20181201/v2/client"
	util "github.com/alibabacloud-go/tea-utils/v2/service"
	"github.com/alibabacloud-go/tea/tea"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

type AlibabacloudApi struct {
	Master             *Alibabacloud
	Slave              *Alibabacloud
	RepoNamespaceName  *string
	RepoNamespaceNames []string
	Logger             *logrus.Logger
	PageSize           *int32
}

type Alibabacloud struct {
	Client          *cr20181201.Client
	Account         *string
	Password        *string
	InstanceId      *string // 实例id
	AccessKeyId     *string
	AccessKeySecret *string
	Endpoint        *string
	Network         *string
}

type ApiClientEnum int

const (
	Master    ApiClientEnum = 1
	Slave     ApiClientEnum = 2
	FailFermi               = "fail-fermi"
)

type ApiError struct {
	error string
}

func (e *ApiError) Error() string {
	return e.error
}

func NewAlibabacloudApi(clientMain *Alibabacloud, clientSlave *Alibabacloud, repoNamespaceName *string, repoNamespaceNames []string, logger *logrus.Logger) *AlibabacloudApi {
	// 获取镜像上限，默认是30，超过30的时候镜像无法同步到
	pageSize := int32(1000)
	return &AlibabacloudApi{
		Master:             clientMain,
		Slave:              clientSlave,
		RepoNamespaceName:  repoNamespaceName,
		RepoNamespaceNames: repoNamespaceNames,
		Logger:             logger,
		PageSize:           &pageSize,
	}
}

func (api *AlibabacloudApi) CurrentAlibabacloudApi(apiClientEnum ApiClientEnum) *Alibabacloud {
	if apiClientEnum == 1 {
		return api.Master
	}
	return api.Slave
}

// ListRepository https://www.alibabacloud.com/help/zh/container-registry/latest/api-doc-cr-2018-12-01-api-doc-listrepository
func (api *AlibabacloudApi) ListRepository(apiClientEnum ApiClientEnum, args ...[]*string) (*cr20181201.ListRepositoryResponse, error) {
	listRepositoryRequest := &cr20181201.ListRepositoryRequest{
		InstanceId:        api.CurrentAlibabacloudApi(apiClientEnum).InstanceId,
		RepoStatus:        tea.String("NORMAL"),
		RepoNamespaceName: api.RepoNamespaceName,
		PageSize:          api.PageSize,
	}

	runtime := &util.RuntimeOptions{}
	res, tryErr := func() (_result *cr20181201.ListRepositoryResponse, _e error) {
		defer func() {
			if r := tea.Recover(recover()); r != nil {
				_e = r
			}
		}()
		// 复制代码运行请自行打印 API 的返回值
		resp, _err := api.CurrentAlibabacloudApi(apiClientEnum).Client.ListRepositoryWithOptions(listRepositoryRequest, runtime)
		if _err != nil {
			return nil, _err
		}

		return resp, nil
	}()

	// 处理错误
	if tryErr != nil {
		var _error = &tea.SDKError{}
		if _t, ok := tryErr.(*tea.SDKError); ok {
			_error = _t
		} else {
			_error.Message = tea.String(tryErr.Error())
		}
		return nil, _error
	}

	// 检查返回值
	if *res.Body.IsSuccess != true || *res.StatusCode != 200 {
		resSring := util.ToJSONString(tea.ToMap(res))
		return nil, &ApiError{
			error: "ListRepository接口未报错，但数据列表异常" + *resSring,
		}
	}
	return res, nil
}

// ListRepoTagWithOptions https://www.alibabacloud.com/help/zh/container-registry/latest/api-doc-cr-2018-12-01-api-doc-listrepotag
func (api *AlibabacloudApi) ListRepoTagWithOptions(apiClientEnum ApiClientEnum, listRepoTagRequest *cr20181201.ListRepoTagRequest) (*cr20181201.ListRepoTagResponse, error) {
	// listRepoTagRequest := &cr20181201.ListRepoTagRequest{}
	runtime := &util.RuntimeOptions{}
	res, tryErr := func() (_result *cr20181201.ListRepoTagResponse, _e error) {
		defer func() {
			if r := tea.Recover(recover()); r != nil {
				_e = r
			}
		}()
		// 存在qps限制，免费版本 qps 20 ，每天50w次/接口
		resp, err := api.CurrentAlibabacloudApi(apiClientEnum).Client.ListRepoTagWithOptions(listRepoTagRequest, runtime)
		if err != nil {
			return nil, err
		}
		return resp, nil

	}()

	if tryErr != nil {
		var _error = &tea.SDKError{}
		if _t, ok := tryErr.(*tea.SDKError); ok {
			_error = _t
		} else {
			_error.Message = tea.String(tryErr.Error())
		}
		return nil, _error
	}

	// 检查返回值
	if *res.Body.IsSuccess != true || *res.StatusCode != 200 {
		resSring := util.ToJSONString(tea.ToMap(res))
		return nil, &ApiError{
			error: "ListRepoTagWithOptions接口未报错，但数据列表异常" + *resSring,
		}
	}

	return res, nil
}

// ListRepoTagWithOptionsByRoutine 使用协程方式跑数据，避免批量执行的效率问题
func (api *AlibabacloudApi) ListRepoTagWithOptionsByRoutine(apiClientEnum ApiClientEnum, listRepositoryResponseBodyRepositories []*cr20181201.ListRepositoryResponseBodyRepositories) (map[string]string, error) {
	// 准备数据
	repoRequestMaps := api.repoRequestMap(apiClientEnum, listRepositoryResponseBodyRepositories)

	requestsSlice := repoRequestMaps[api.CurrentAlibabacloudApi(apiClientEnum).InstanceId]
	// 阿里云镜像api免费版本有qps限制，需要保证少于 20/s ,假设我们的镜像仓库需要有200镜像，一组差不多5，协程20个，然后sleep再下一个
	repoRequestsSlice := RepoRequestsSlice(requestsSlice).RepoRequestsSliceSplit(3)

	// 保重10分钟中内要完成，避免协程泄漏
	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Second)
	defer cancel()
	wg, _ := errgroup.WithContext(ctx)

	// 线程安全 map
	m := sync.Map{}
	lastRepoTagMap := make(map[string]string)

	// 按length 开启多个goroutine
	for _, row := range repoRequestsSlice {
		repos := row
		wg.Go(func() error {
			for _, repo := range repos {
				// 单次错误，不做记录，避免某一次访问的失败导致所以失败，因为要不断的循环事务，下次同步可能就自动修复更新了
				pageSize := int32(1000) // 为了避免服务挂掉重启的时候，漏掉一些tag，因为一些老tag可能会用到，这里检查每个镜像仓库所有的tag
				res, err := api.ListRepoTagWithOptions(apiClientEnum, &cr20181201.ListRepoTagRequest{InstanceId: repo.InstanceId, RepoId: repo.RepoId, PageSize: &pageSize})
				if err != nil || len(res.Body.Images) <= 0 {
					api.Logger.Errorf("ListRepoTagWithOptions %s && %s get error: %v", *repo.InstanceId, *repo.RepoId, err)
					// 失败的请求先扔到map里，因为可能会遇到qps限制和阿里抽风的情况，存入约定好的标识，遇到直接跳过检查！
					m.Store(*repo.RepoName, FailFermi)
					continue
				}
				sortRes := ListRepoTagResponseBodyImagesSlice(res.Body.Images).SortTags()
				// 存储hash : "clickhouse-cluster" => "v0.0.1"  tag为最新push的images
				// 11-17 更新为检查最新的30个tag，因为存在断点重传的情况，而且这个服务5分钟执行一次，可能存在5分钟中同一个镜像增加多次更新
				for _, sortTag := range sortRes {
					m.Store(*repo.RepoName+":"+*sortTag.Tag, *sortTag.Tag)
				}
			}
			return nil
		})
		time.Sleep(500 * time.Millisecond)
	}

	if err := wg.Wait(); err != nil {
		return nil, err
	}

	m.Range(func(key, value interface{}) bool {
		k, ok1 := key.(string)
		v, ok2 := value.(string)
		if ok1 && ok2 {
			lastRepoTagMap[k] = v
		}
		return true
	})

	return lastRepoTagMap, nil
}

type ListRepoTagResponseBodyImagesSlice []*cr20181201.ListRepoTagResponseBodyImages

// SortTags 按照tag更新时间排序，存储更新同一个tag的情况
func (tagsSlice ListRepoTagResponseBodyImagesSlice) SortTags() []*cr20181201.ListRepoTagResponseBodyImages {
	sort.SliceStable(tagsSlice, func(i, j int) bool {
		iImageUpdate, _ := strconv.Atoi(*tagsSlice[i].ImageUpdate)
		jImageUpdate, _ := strconv.Atoi(*tagsSlice[j].ImageUpdate)
		return iImageUpdate > jImageUpdate
	})
	return tagsSlice
}

type RepoRequests struct {
	InstanceId *string
	RepoId     *string
	RepoName   *string
}

type RepoRequestsSlice []*RepoRequests

// RepoRequestsSliceSplit 把slice按照指定数量拆分成多个小slice，方便减少单次的查询量
func (slice RepoRequestsSlice) RepoRequestsSliceSplit(length int) []RepoRequestsSlice {
	base := RepoRequestsSlice{}
	var baseSplit []RepoRequestsSlice
	for _, pc := range slice {
		base = append(base, pc)
		if len(base) < length {
			continue
		}
		// 每当处理完指定数量的slice后，填充到split里，并且重置基础slice
		baseSplit = append(baseSplit, base)
		base = RepoRequestsSlice{}
	}
	if len(base) > 0 {
		baseSplit = append(baseSplit, base)
	}
	return baseSplit
}

// repoRequestMap 获取repo tags list，获取当前ns下所以repo的tag list
func (api *AlibabacloudApi) repoRequestMap(apiClientEnum ApiClientEnum, listRepositoryResponseBodyRepositories []*cr20181201.ListRepositoryResponseBodyRepositories) map[*string][]*RepoRequests {
	repoMap := make(map[*string][]*RepoRequests)
	instanceId := api.CurrentAlibabacloudApi(apiClientEnum).InstanceId
	var requests []*RepoRequests
	for _, row := range listRepositoryResponseBodyRepositories {
		requests = append(requests, &RepoRequests{
			InstanceId: row.InstanceId,
			RepoId:     row.RepoId,
			RepoName:   row.RepoName,
		})
	}
	repoMap[instanceId] = requests
	return repoMap
}
