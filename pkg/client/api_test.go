package client

import (
	"strconv"
	"testing"

	"github.com/alibabacloud-go/cr-20181201/v2/client"
)

func TestSortTags(t *testing.T) {
	var (
		imageCreate *string
		tag         *string
	)
	// images := make([]*client.ListRepoTagResponseBodyImages, 3) 配合指定index赋值使用，append用空slice
	var images []*client.ListRepoTagResponseBodyImages
	for i := 0; i < 10; i++ {
		nowImageCreate := "166747750700" + strconv.Itoa(i)
		imageCreate = &nowImageCreate
		nowTag := "v0.0." + strconv.Itoa(i)
		tag = &nowTag
		images = append(images, &client.ListRepoTagResponseBodyImages{
			ImageCreate: imageCreate,
			ImageUpdate: imageCreate,
			Tag:         tag,
		})
	}

	sortList := ListRepoTagResponseBodyImagesSlice(images).SortTags()
	if *sortList[0].Tag != "v0.0.9" {
		t.Errorf("sort fail, now is -> %s", *sortList[0].Tag)
	}

}
