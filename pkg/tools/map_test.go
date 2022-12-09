package tools

import (
	"reflect"
	"strings"
	"testing"
)

type RepoTagsMapDiffCase struct {
	Cases []*RepoTagsMap
}

type RepoTagsMap struct {
	Master map[string]string
	Slave  map[string]string
	Target map[string]string
}

func TestRepoTagsMapDiff(t *testing.T) {
	cases := &RepoTagsMapDiffCase{
		Cases: []*RepoTagsMap{
			{ // master 和 salve 完全相同
				Master: map[string]string{
					"alix:v0.0.1":                 "v0.0.1",
					"aliyun-images-syncer:v0.0.8": "v0.0.8",
					"pivot:v3.0.5-alpha.2":        "v3.0.5-alpha.2",
				},
				Slave: map[string]string{
					"alix:v0.0.1":                 "v0.0.1",
					"aliyun-images-syncer:v0.0.8": "v0.0.8",
					"pivot:v3.0.5-alpha.2":        "v3.0.5-alpha.2",
				},
				Target: (map[string]string{}),
			},
			{ // master 和 salve 对应上
				Master: map[string]string{
					"alix:v0.0.1":                 "v0.0.1",
					"aliyun-images-syncer:v0.0.8": "v0.0.8",
					"pivot:v3.0.5-alpha.2":        "v3.0.5-alpha.2",
				},
				Slave: map[string]string{
					"alix:v0.0.1":                 "v0.0.1",
					"aliyun-images-syncer:v0.0.7": "v0.0.7",
					"pivot:v3.0.5-alpha.1":        "v3.0.5-alpha.1",
				},
				Target: map[string]string{
					"aliyun-images-syncer:v0.0.8": "v0.0.8",
					"pivot:v3.0.5-alpha.2":        "v3.0.5-alpha.2",
				},
			}, { // master 比 slave 多的情况
				Master: map[string]string{
					"alix:v0.0.1":                 "v0.0.1",
					"aliyun-images-syncer:v0.0.8": "v0.0.8",
					"pivot:v3.0.5-alpha.2":        "v3.0.5-alpha.2",
				},
				Slave: map[string]string{
					"alix:v0.0.1": "v0.0.1",
				},
				Target: map[string]string{
					"aliyun-images-syncer:v0.0.8": "v0.0.8",
					"pivot:v3.0.5-alpha.2":        "v3.0.5-alpha.2",
				},
			}, { // salve 没有数据的情况
				Master: map[string]string{
					"alix:v0.0.1":                 "v0.0.1",
					"aliyun-images-syncer:v0.0.8": "v0.0.8",
					"pivot:v3.0.5-alpha.2":        "v3.0.5-alpha.2",
				},
				Slave: map[string]string{},
				Target: map[string]string{
					"alix:v0.0.1":                 "v0.0.1",
					"aliyun-images-syncer:v0.0.8": "v0.0.8",
					"pivot:v3.0.5-alpha.2":        "v3.0.5-alpha.2",
				},
			}, { // case master 比 salve 镜像少的情况
				Master: map[string]string{
					"alix:v0.0.1": "v0.0.1",
				},
				Slave: map[string]string{
					"alix:v0.0.1":                 "v0.0.1",
					"aliyun-images-syncer:v0.0.8": "v0.0.8",
					"pivot:v3.0.5-alpha.2":        "v3.0.5-alpha.2",
				},
				Target: map[string]string{},
			}, { // case tag 完全不同类型的情况
				Master: map[string]string{
					"alix:v0.0.1":          "v0.0.1",
					"agent:vagt3.6.0-rc.4": "vagt3.6.0-rc.4",
				},
				Slave: map[string]string{
					"job:v0.0.1":                  "v0.0.1",
					"aliyun-images-syncer:v0.0.8": "v0.0.8",
					"pivot:v3.0.5-alpha.2":        "v3.0.5-alpha.2",
					"agent:alpha3.6.3":            "alpha3.6.3",
				},
				Target: map[string]string{
					"alix:v0.0.1":          "v0.0.1",
					"agent:vagt3.6.0-rc.4": "vagt3.6.0-rc.4",
				},
			}, { // case 假设master请求tag list 出错的情况
				Master: map[string]string{
					"alix:v0.0.1":      "v0.0.1",
					"agent:fail-fermi": "fail-fermi",
				},
				Slave: map[string]string{
					"job:v0.0.1":                  "v0.0.1",
					"aliyun-images-syncer:v0.0.8": "v0.0.8",
					"pivot:v3.0.5-alpha.2":        "v3.0.5-alpha.2",
					"agent:alpha3.6.3":            "alpha3.6.3",
				},
				Target: map[string]string{
					"alix:v0.0.1": "v0.0.1",
				},
			},
		},
	}

	for _, row := range cases.Cases {
		res := RepoTagsMapDiff(row.Master, row.Slave)
		if len(row.Target) == 0 && len(res) == 0 {
			// reflect.DeepEqual 需要不能含有无法比较的成员。
			continue
		}
		if !reflect.DeepEqual(res, row.Target) {
			t.Errorf("Unexpected results %v => %v", res, row.Target)
		}
	}

}

func pointMap_(maps map[string]string) map[*string]*string {
	newMap := make(map[*string]*string)
	for k, v := range maps {
		newMap[&k] = &v
	}
	return newMap
}

type CaseStrings struct {
	target     string
	caseString string
}

func TestString(t *testing.T) {
	stringss := []*CaseStrings{
		{
			target:     "images-sync",
			caseString: "images-sync:v0.0.5",
		},
		{
			target:     "images-sync",
			caseString: "images-sync:v0.1.1",
		},
		{
			target:     "bazel-py-nlp",
			caseString: "bazel-py-nlp:2020-09-08",
		},
		{
			target:     "bzproxy",
			caseString: "bzproxy:v0.0.1-alpha.3-dev",
		},
		{
			target:     "svlab-response_evaluation_service",
			caseString: "svlab-response_evaluation_service:v0.2.2d",
		},
		{
			target:     "svlab",
			caseString: "svlab:v0.2.:2d",
		},
	}

	for _, row := range stringss {
		rows := strings.Split(row.caseString, ":")
		if rows[0] != row.target {
			t.Errorf("Unexpected results %v => %v", rows[0], row.target)
		}
	}
}
