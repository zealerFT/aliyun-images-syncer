package tools

import (
	"reflect"
)

const (
	FailFermi = "fail-fermi"
)

// RepoTagsMapDiff 对比主从镜像最新tag是否一致
// master 有 salve 无，则需要加入sync map
// master 有 salve 但不等，则需要加入sync map
// master 有tag是失败的预定字段fail-fermi，则直接跳过，期待下一次循环可以正常 :)
func RepoTagsMapDiff(master, slave map[string]string) map[string]string {
	// 完全相同，表示相安无事，无需同步
	if reflect.DeepEqual(master, slave) {
		return nil
	}

	mapDiff := make(map[string]string)
	for index, row := range master {
		// 失败标志直接跳过
		if row == FailFermi {
			continue
		}
		// 存在，则要判断tag是否相等
		if _, ok := slave[index]; ok {
			// 相等或者是失败的tag标识，则跳到下一轮
			if row == slave[index] {
				continue
			} else {
				mapDiff[index] = row
			}
		} else {
			// 不存在，则直接加入待sync map
			mapDiff[index] = row
		}
	}

	return mapDiff
}
